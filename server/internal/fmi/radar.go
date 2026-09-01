package fmi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrRadarFrameUnavailable marks a frame the open WMS has not published (yet or
// anymore): GeoServer answers a ServiceException instead of a raster. Callers
// treat it as a soft miss rather than a fetch failure.
var ErrRadarFrameUnavailable = errors.New("radar frame unavailable")

// RadarClient downloads FMI open-data radar composites as EPSG:4326 GeoTIFF
// data grids (raw uint16 values, not styled tiles) from the keyless GeoServer
// WMS at openwms.fmi.fi.
type RadarClient struct {
	baseURL    string
	layer      string
	httpClient *http.Client
}

func NewRadarClient(baseURL, layer string) *RadarClient {
	return &RadarClient{
		baseURL: baseURL,
		layer:   layer,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// FetchFrame streams the composite valid at the given instant into w. bbox is
// "minLon,minLat,maxLon,maxLat" in EPSG:4326 degrees; the response raster is
// width x height cells over exactly that extent (GeoServer reprojects and
// resamples from the native radar grid). at must land on the composite's 5-min
// publish cadence or the WMS rejects the time dimension.
func (c *RadarClient) FetchFrame(ctx context.Context, at time.Time, bbox string, width, height int, w io.Writer) error {
	parts := strings.Split(bbox, ",")
	if len(parts) != 4 {
		return fmt.Errorf("radar bbox %q: want minLon,minLat,maxLon,maxLat", bbox)
	}
	// WMS 1.3.0 with EPSG:4326 uses lat,lon axis order.
	bbox13 := fmt.Sprintf("%s,%s,%s,%s",
		strings.TrimSpace(parts[1]), strings.TrimSpace(parts[0]),
		strings.TrimSpace(parts[3]), strings.TrimSpace(parts[2]))

	q := url.Values{
		"service": {"WMS"},
		"version": {"1.3.0"},
		"request": {"GetMap"},
		"layers":  {c.layer},
		"styles":  {"raster"},
		"crs":     {"EPSG:4326"},
		"bbox":    {bbox13},
		"width":   {fmt.Sprintf("%d", width)},
		"height":  {fmt.Sprintf("%d", height)},
		"format":  {"image/geotiff"},
		"time":    {at.UTC().Format(time.RFC3339)},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("build radar request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch radar frame: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if bytes.Contains(body, []byte("InvalidDimensionValue")) {
			return fmt.Errorf("%w: %s", ErrRadarFrameUnavailable, at.UTC().Format(time.RFC3339))
		}
		return fmt.Errorf("radar WMS returned %d: %s", resp.StatusCode, string(body))
	}

	// GeoServer reports errors as 200 + XML ServiceException (e.g. a time
	// outside the layer's published dimension). Sniff before streaming.
	head := make([]byte, 512)
	n, _ := io.ReadFull(resp.Body, head)
	head = head[:n]
	if isWMSException(resp.Header.Get("Content-Type"), head) {
		if bytes.Contains(head, []byte("InvalidDimensionValue")) {
			return fmt.Errorf("%w: %s", ErrRadarFrameUnavailable, at.UTC().Format(time.RFC3339))
		}
		rest, _ := io.ReadAll(io.LimitReader(resp.Body, 1536))
		return fmt.Errorf("radar WMS exception: %s", string(append(head, rest...)))
	}

	if _, err := w.Write(head); err != nil {
		return fmt.Errorf("write radar frame: %w", err)
	}
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("stream radar frame: %w", err)
	}
	return nil
}

func isWMSException(contentType string, head []byte) bool {
	if strings.Contains(contentType, "xml") {
		return true
	}
	return bytes.Contains(head, []byte("ServiceException"))
}
