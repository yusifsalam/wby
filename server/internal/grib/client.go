// Package grib is a thin HTTP client for the standalone gribsvc service
// (FastAPI + pygrib), which parses GRIB2 files the Go server downloads into a
// shared directory. The server uses it to read a gridded temperature field for
// the map overlay, replacing the sparse station interpolation.
package grib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"wby/internal/weather"
)

// kelvinToCelsius is the offset between the GRIB 2t field (Kelvin) and the
// Celsius the overlay/samples consumers expect.
const kelvinToCelsius = 273.15

// maxRenderSamples bounds how many gridpoints we return. The iOS metal overlay
// renderer consumes only the first kMaxSamples (2048) samples it's given (and
// gribsvc returns them south-to-north), so an unbounded grid would render as a
// southern band. We stride the grid down to this cap while keeping samples
// spread across the whole extent. Keep in sync with the iOS maxSampleCount /
// kMaxSamples constants.
const maxRenderSamples = 2048

// Client talks to a gribsvc instance over HTTP for one configured field.
type Client struct {
	baseURL    string
	file       string
	param      string
	step       int
	httpClient *http.Client
}

// New returns a Client pointed at the given gribsvc base URL (e.g.
// "http://gribsvc:9090"). file/param select the GRIB message; step subsamples
// the grid (every Nth point in each axis) to keep the sample count bounded.
func New(baseURL, file, param string, step int) *Client {
	if step < 1 {
		step = 1
	}
	return &Client{
		baseURL: baseURL,
		file:    file,
		param:   param,
		step:    step,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type bboxRequest struct {
	File  string `json:"file"`
	Param string `json:"param"`
	Time  string `json:"time,omitempty"`
	BBox  bbox   `json:"bbox"`
	Step  int    `json:"step"`
}

type bbox struct {
	MinLon float64 `json:"min_lon"`
	MinLat float64 `json:"min_lat"`
	MaxLon float64 `json:"max_lon"`
	MaxLat float64 `json:"max_lat"`
}

type bboxResponse struct {
	ValidTime string       `json:"valid_time"`
	Lats      [][]float64  `json:"lats"`
	Lons      [][]float64  `json:"lons"`
	Values    [][]*float64 `json:"values"`
}

// TemperatureSamples extracts the configured field over the bbox and returns it
// as Celsius temperature samples, one per (unmasked) gridpoint, plus the field's
// valid time.
//
// A non-nil at requests that exact hour from the file (RFC3339); a zero at lets
// gribsvc pick the first matching message. When the file/field isn't available
// yet (gribsvc 404/422) it returns an empty slice and no error, so the caller
// can fall back to its station-based source rather than failing the request.
func (c *Client) TemperatureSamples(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) ([]weather.TemperatureSample, time.Time, error) {
	reqBody := bboxRequest{
		File:  c.file,
		Param: c.param,
		BBox:  bbox{MinLon: minLon, MinLat: minLat, MaxLon: maxLon, MaxLat: maxLat},
		Step:  c.step,
	}
	if !at.IsZero() {
		reqBody.Time = at.UTC().Format(time.RFC3339)
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("marshal extract request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/grib/extract", bytes.NewReader(payload))
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("build extract request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("call gribsvc extract: %w", err)
	}
	defer resp.Body.Close()

	// File or field not available yet — a soft miss, let the caller fall back.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, time.Time{}, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, time.Time{}, fmt.Errorf("gribsvc extract returned %d: %s", resp.StatusCode, string(body))
	}

	var out bboxResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, time.Time{}, fmt.Errorf("decode extract response: %w", err)
	}

	validTime, _ := time.Parse(time.RFC3339, out.ValidTime)

	rows := len(out.Values)
	cols := 0
	if rows > 0 {
		cols = len(out.Values[0])
	}

	// Stride the regular grid down to <= maxRenderSamples, keeping a sub-lattice
	// that still spans the full extent (rather than the south-only prefix the
	// renderer would otherwise keep).
	stride := 1
	if total := rows * cols; total > maxRenderSamples {
		stride = int(math.Ceil(math.Sqrt(float64(total) / float64(maxRenderSamples))))
	}

	samples := make([]weather.TemperatureSample, 0)
	for i := 0; i < rows; i += stride {
		for j := 0; j < cols && j < len(out.Values[i]); j += stride {
			v := out.Values[i][j]
			if v == nil { // masked/NaN gridpoint
				continue
			}
			if i >= len(out.Lats) || j >= len(out.Lats[i]) || i >= len(out.Lons) || j >= len(out.Lons[i]) {
				continue
			}
			samples = append(samples, weather.TemperatureSample{
				Lat:         out.Lats[i][j],
				Lon:         out.Lons[i][j],
				Temperature: *v - kelvinToCelsius,
				ObservedAt:  validTime,
			})
		}
	}
	return samples, validTime, nil
}
