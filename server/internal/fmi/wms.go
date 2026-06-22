package fmi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build WMS request: %w", err)
	}

	started := time.Now()
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		slog.Warn("FMI WMS request failed",
			"err", err,
			"duration_ms", time.Since(started).Milliseconds(),
			"layer", req.Layer,
			"target", req.Time.UTC().Format(time.RFC3339),
			"width", req.Width,
			"height", req.Height,
		)
		return nil, fmt.Errorf("fetch WMS tile: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Warn("FMI WMS response read failed",
			"err", err,
			"status", resp.StatusCode,
			"duration_ms", time.Since(started).Milliseconds(),
			"layer", req.Layer,
			"target", req.Time.UTC().Format(time.RFC3339),
			"width", req.Width,
			"height", req.Height,
		)
		return nil, fmt.Errorf("read WMS response: %w", err)
	}
	slog.Info("FMI WMS request completed",
		"status", resp.StatusCode,
		"duration_ms", time.Since(started).Milliseconds(),
		"layer", req.Layer,
		"target", req.Time.UTC().Format(time.RFC3339),
		"width", req.Width,
		"height", req.Height,
		"bytes", len(body),
	)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WMS returned %d for layer %s @ %s: %s",
			resp.StatusCode, req.Layer, req.Time.UTC().Format(time.RFC3339), truncate(string(body), 256))
	}
	return body, nil
}

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
