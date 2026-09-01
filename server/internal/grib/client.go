// Package grib is a thin HTTP client for the standalone gribsvc service
// (FastAPI + pygrib), which parses GRIB2 files the Go server downloads into a
// shared directory. The server uses it to read a gridded field (temperature, or
// precipitation rate) for the map overlay, replacing the sparse station
// interpolation.
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

// secondsPerHour converts the FMI precipitation-rate field (prate, kg m^-2 s^-1,
// instantaneous — confirmed not accumulated, so no de-accumulation is needed)
// to mm/h: 1 kg m^-2 s^-1 = 1 mm/s = 3600 mm/h.
const secondsPerHour = 3600

// fmiMissingValue is the sentinel FMI uses for absent gridpoints (e.g. the
// analysis-step precipitation rate). FMI encodes it as a large number rather
// than a GRIB bitmap, so gribsvc/pygrib hands it back as a real value instead
// of null. Raw values at or above this are treated as missing and dropped.
const fmiMissingValue = 9000.0

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
	field      field
	step       int
	httpClient *http.Client
}

// field describes how to select a GRIB message and convert its raw values into
// consumer-facing units. The conversion is affine (raw*scale + offset), and raw
// values at or above missingAbove are dropped as FMI fill (0 disables the gate).
type field struct {
	param        string
	scale        float64
	offset       float64
	missingAbove float64
}

// New returns a Client for the GRIB temperature field (2t, Kelvin), converting
// to Celsius. baseURL is the gribsvc base URL (e.g. "http://gribsvc:9090");
// file/param select the GRIB message; step subsamples the grid (every Nth point
// in each axis) to keep the sample count bounded.
func New(baseURL, file, param string, step int) *Client {
	return newClient(baseURL, file, step, field{
		param:        param,
		scale:        1,
		offset:       -kelvinToCelsius,
		missingAbove: fmiMissingValue,
	})
}

// NewPrecipitation returns a Client for the GRIB precipitation-rate field
// (prate, kg m^-2 s^-1), converting to mm/h. Unlike PrecipitationAmount, this
// field is instantaneous rather than accumulated, so the per-hour samples need
// no differencing.
func NewPrecipitation(baseURL, file, param string, step int) *Client {
	return newClient(baseURL, file, step, field{
		param:        param,
		scale:        secondsPerHour,
		offset:       0,
		missingAbove: fmiMissingValue,
	})
}

// NewRadar returns a Client for radar rain-rate GeoTIFF frames. gribsvc scales
// those to mm/h and nulls nodata cells itself (per the frame sidecar), so no
// conversion applies here. Frames are per-timestamp files, so callers use
// GridForFile rather than the fixed-file Grid.
func NewRadar(baseURL string, step int) *Client {
	return newClient(baseURL, "", step, field{
		param: "rr",
		scale: 1,
	})
}

func newClient(baseURL, file string, step int, f field) *Client {
	if step < 1 {
		step = 1
	}
	return &Client{
		baseURL: baseURL,
		file:    file,
		field:   f,
		step:    step,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// RunNowcast asks gribsvc to (re)compute the radar extrapolation nowcast
// frames from the observed frames on disk. Blocking but cheap (~seconds);
// callers warm grids after it returns.
func (c *Client) RunNowcast(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/nowcast/run", nil)
	if err != nil {
		return fmt.Errorf("build nowcast request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call gribsvc nowcast: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("gribsvc nowcast returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
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

// Samples extracts the configured field over the bbox and returns it as
// converted samples (in the field's consumer units), one per gridpoint that is
// neither masked nor FMI fill, plus the field's valid time.
//
// A non-nil at requests that exact hour from the file (RFC3339); a zero at lets
// gribsvc pick the first matching message. When the file/field isn't available
// yet (gribsvc 404/422) it returns an empty slice and no error, so the caller
// can fall back to its station-based source rather than failing the request.
func (c *Client) Samples(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) ([]weather.FieldSample, time.Time, error) {
	out, ok, err := c.fetchBBox(ctx, minLon, minLat, maxLon, maxLat, at)
	if err != nil || !ok {
		return nil, time.Time{}, err
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

	samples := make([]weather.FieldSample, 0)
	for i := 0; i < rows; i += stride {
		for j := 0; j < cols && j < len(out.Values[i]); j += stride {
			v := out.Values[i][j]
			if v == nil { // masked/NaN gridpoint
				continue
			}
			if c.field.missingAbove > 0 && *v >= c.field.missingAbove { // FMI fill sentinel
				continue
			}
			if i >= len(out.Lats) || j >= len(out.Lats[i]) || i >= len(out.Lons) || j >= len(out.Lons[i]) {
				continue
			}
			samples = append(samples, weather.FieldSample{
				Lat:        out.Lats[i][j],
				Lon:        out.Lons[i][j],
				Value:      *v*c.field.scale + c.field.offset,
				ObservedAt: validTime,
			})
		}
	}
	return samples, validTime, nil
}

// fetchBBox posts the extract request and decodes the grid. ok is false (with a
// nil error) on a soft miss — the file/field isn't available yet.
func (c *Client) fetchBBox(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (bboxResponse, bool, error) {
	return c.fetchBBoxFile(ctx, c.file, minLon, minLat, maxLon, maxLat, at)
}

func (c *Client) fetchBBoxFile(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (bboxResponse, bool, error) {
	reqBody := bboxRequest{
		File:  file,
		Param: c.field.param,
		BBox:  bbox{MinLon: minLon, MinLat: minLat, MaxLon: maxLon, MaxLat: maxLat},
		Step:  c.step,
	}
	if !at.IsZero() {
		reqBody.Time = at.UTC().Format(time.RFC3339)
	}

	var out bboxResponse
	ok, err := c.postJSON(ctx, "/grib/extract", reqBody, &out)
	return out, ok, err
}

// postJSON posts a JSON body to a gribsvc path and decodes the response into
// out. ok is false with a nil error on a soft miss — gribsvc answered 404/422
// because the file or field isn't available yet.
func (c *Client) postJSON(ctx context.Context, path string, in, out any) (bool, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return false, fmt.Errorf("marshal extract request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("build extract request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("call gribsvc extract: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return false, fmt.Errorf("gribsvc extract returned %d: %s", resp.StatusCode, string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return false, fmt.Errorf("decode extract response: %w", err)
	}
	return true, nil
}

// Grid extracts the configured field over the bbox as a regular lat/lon raster
// in consumer units, preserving the grid topology for texture upload. Rows are
// emitted north-to-south (row 0 = MaxLat) regardless of the gribsvc order. Soft
// misses (file/field not ready) return a nil grid and nil error.
func (c *Client) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*weather.FieldGrid, time.Time, error) {
	return c.GridForFile(ctx, c.file, minLon, minLat, maxLon, maxLat, at)
}

// GridForFile is Grid against an explicit gribsvc file name — used for
// per-timestamp radar frames, where each instant is its own file and the
// requested time is implied by the name (at stays zero).
func (c *Client) GridForFile(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (*weather.FieldGrid, time.Time, error) {
	out, ok, err := c.fetchBBoxFile(ctx, file, minLon, minLat, maxLon, maxLat, at)
	if err != nil || !ok {
		return nil, time.Time{}, err
	}

	validTime, _ := time.Parse(time.RFC3339, out.ValidTime)
	return c.buildGrid(out.Values, out.Lats, out.Lons, validTime), validTime, nil
}

type seriesRequest struct {
	File  string   `json:"file"`
	Param string   `json:"param"`
	BBox  bbox     `json:"bbox"`
	Step  int      `json:"step"`
	Times []string `json:"times,omitempty"`
}

type seriesFrame struct {
	ValidTime string       `json:"valid_time"`
	Values    [][]*float64 `json:"values"`
}

type seriesResponse struct {
	Lats   [][]float64   `json:"lats"`
	Lons   [][]float64   `json:"lons"`
	Frames []seriesFrame `json:"frames"`
}

// GridSeries extracts the configured field over the bbox for every requested
// hour in a single gribsvc pass over the file, returning one FieldGrid per hour
// present (ObservedAt carries the frame's valid time). Hours the file lacks are
// simply absent from the result. Soft misses (file not downloaded yet, or no
// matching fields at all) return an empty slice and nil error.
func (c *Client) GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*weather.FieldGrid, error) {
	reqBody := seriesRequest{
		File:  c.file,
		Param: c.field.param,
		BBox:  bbox{MinLon: minLon, MinLat: minLat, MaxLon: maxLon, MaxLat: maxLat},
		Step:  c.step,
	}
	for _, t := range times {
		reqBody.Times = append(reqBody.Times, t.UTC().Format(time.RFC3339))
	}

	var out seriesResponse
	ok, err := c.postJSON(ctx, "/grib/extract_series", reqBody, &out)
	if err != nil || !ok {
		return nil, err
	}

	grids := make([]*weather.FieldGrid, 0, len(out.Frames))
	for _, frame := range out.Frames {
		validTime, _ := time.Parse(time.RFC3339, frame.ValidTime)
		if grid := c.buildGrid(frame.Values, out.Lats, out.Lons, validTime); grid != nil {
			grids = append(grids, grid)
		}
	}
	return grids, nil
}

// buildGrid converts one gribsvc values raster plus its lat/lon lattice into a
// FieldGrid in consumer units. gribsvc returns rows south-to-north; the order
// is detected from the corner latitudes and flipped if needed so row 0 is the
// northernmost. Returns nil when the shapes are inconsistent.
func (c *Client) buildGrid(gridValues [][]*float64, lats, lons [][]float64, validTime time.Time) *weather.FieldGrid {
	rows := len(gridValues)
	if rows == 0 {
		return nil
	}
	cols := len(gridValues[0])
	if cols == 0 || len(lats) != rows || len(lons) != rows {
		return nil
	}

	northToSouth := lats[0][0] >= lats[rows-1][0]

	values := make([]*float64, 0, rows*cols)
	for r := 0; r < rows; r++ {
		src := r
		if !northToSouth {
			src = rows - 1 - r
		}
		row := gridValues[src]
		for j := 0; j < cols; j++ {
			var cell *float64
			if j < len(row) {
				if v := row[j]; v != nil && !(c.field.missingAbove > 0 && *v >= c.field.missingAbove) {
					converted := *v*c.field.scale + c.field.offset
					cell = &converted
				}
			}
			values = append(values, cell)
		}
	}

	minLatV, maxLatV := lats[0][0], lats[0][0]
	minLonV, maxLonV := lons[0][0], lons[0][0]
	for r := 0; r < rows; r++ {
		for j := 0; j < cols && j < len(lats[r]) && j < len(lons[r]); j++ {
			lat, lon := lats[r][j], lons[r][j]
			minLatV, maxLatV = math.Min(minLatV, lat), math.Max(maxLatV, lat)
			minLonV, maxLonV = math.Min(minLonV, lon), math.Max(maxLonV, lon)
		}
	}

	return &weather.FieldGrid{
		Rows:       rows,
		Cols:       cols,
		MinLat:     minLatV,
		MaxLat:     maxLatV,
		MinLon:     minLonV,
		MaxLon:     maxLonV,
		Values:     values,
		ObservedAt: validTime,
	}
}

// TemperatureSamples is a typed view of Samples for the temperature overlay,
// mapping each FieldSample's Celsius value into a TemperatureSample.
func (c *Client) TemperatureSamples(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) ([]weather.TemperatureSample, time.Time, error) {
	fields, validTime, err := c.Samples(ctx, minLon, minLat, maxLon, maxLat, at)
	if err != nil {
		return nil, validTime, err
	}
	samples := make([]weather.TemperatureSample, len(fields))
	for i, f := range fields {
		samples[i] = weather.TemperatureSample{
			Lat:         f.Lat,
			Lon:         f.Lon,
			Temperature: f.Value,
			ObservedAt:  f.ObservedAt,
		}
	}
	return samples, validTime, nil
}
