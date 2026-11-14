package fmi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"strconv"
	"time"

	"wby/internal/weather"
)

var ErrWMSNotConfigured = errors.New("FMI WMS not configured")

// FetchWMSTile issues a WMS GetMap call against FMI and returns the PNG bytes.
// Returns ErrWMSNotConfigured when API key or WMS base URL are missing.
//
// The request uses EPSG:3857 (Web Mercator). The lat/lon bbox in
// WMSTileRequest is converted to Mercator meters here so the returned tile
// aligns visually with MapKit's Mercator basemap.
func (c *Client) FetchWMSTile(ctx context.Context, req weather.WMSTileRequest) ([]byte, error) {
	if c.wmsURL == "" || c.apiKey == "" {
		return nil, ErrWMSNotConfigured
	}
	if req.Layer == "" {
		return nil, fmt.Errorf("wms tile: empty layer")
	}

	minX, minY := lonLatToMercator(req.MinLon, req.MinLat)
	maxX, maxY := lonLatToMercator(req.MaxLon, req.MaxLat)

	params := url.Values{
		"service":     {"WMS"},
		"version":     {"1.3.0"},
		"request":     {"GetMap"},
		"layers":      {req.Layer},
		"styles":      {req.Style},
		"crs":         {"EPSG:3857"},
		"format":      {"image/png"},
		"transparent": {"true"},
		"bbox": {fmt.Sprintf("%s,%s,%s,%s",
			formatMeters(minX), formatMeters(minY),
			formatMeters(maxX), formatMeters(maxY),
		)},
		"width":  {strconv.Itoa(req.Width)},
		"height": {strconv.Itoa(req.Height)},
		"time":   {req.Time.UTC().Format(time.RFC3339)},
	}

	reqURL := fmt.Sprintf("%s/fmi-apikey/%s/wms?%s", c.wmsURL, c.apiKey, params.Encode())

	target := req.Time.UTC().Format(time.RFC3339)
	started := time.Now()
	body, err := c.getBody(ctx, "WMS", reqURL, maxTileBody)
	if err != nil {
		slog.Warn("FMI WMS request failed",
			"err", err,
			"duration_ms", time.Since(started).Milliseconds(),
			"layer", req.Layer,
			"target", target,
			"width", req.Width,
			"height", req.Height,
		)
		return nil, fmt.Errorf("fetch WMS tile for layer %s @ %s: %w", req.Layer, target, err)
	}
	slog.Info("FMI WMS request completed",
		"duration_ms", time.Since(started).Milliseconds(),
		"layer", req.Layer,
		"target", target,
		"width", req.Width,
		"height", req.Height,
		"bytes", len(body),
	)
	if !bytes.HasPrefix(body, pngSignature) {
		return nil, fmt.Errorf("WMS answered layer %s @ %s with something other than a PNG: %s",
			req.Layer, target, c.redact(truncate(string(body), 256)))
	}
	return body, nil
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

func lonLatToMercator(lon, lat float64) (x, y float64) {
	const earthRadius = 6378137.0
	x = lon * math.Pi / 180.0 * earthRadius
	y = math.Log(math.Tan(math.Pi/4+lat*math.Pi/360)) * earthRadius
	return x, y
}

func formatMeters(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
