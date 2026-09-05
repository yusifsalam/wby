// Package grib is a thin HTTP client for the standalone gribsvc service
// (FastAPI + pygrib), which parses GRIB2 files the Go server downloads into a
// shared directory. The server uses it to read a gridded field (temperature, or
// precipitation rate) for the map overlay, replacing the sparse station
// interpolation.
package grib

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
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

// Per-call deadlines: a series call decodes every requested frame before
// answering, so it gets a longer budget than a single-hour extract.
const (
	extractTimeout = 15 * time.Second
	seriesTimeout  = 120 * time.Second
	nowcastTimeout = 60 * time.Second
)

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
		baseURL:    baseURL,
		file:       file,
		field:      f,
		step:       step,
		httpClient: &http.Client{},
	}
}

// RunNowcast asks gribsvc to (re)compute the radar extrapolation nowcast
// frames from the observed frames on disk. Blocking but cheap (~seconds);
// callers warm grids after it returns.
func (c *Client) RunNowcast(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, nowcastTimeout)
	defer cancel()
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
	var out bboxResponse
	ok, err := c.postJSON(ctx, "/grib/extract", extractTimeout, c.bboxRequest(c.file, minLon, minLat, maxLon, maxLat, at), &out)
	return out, ok, err
}

func (c *Client) bboxRequest(file string, minLon, minLat, maxLon, maxLat float64, at time.Time) bboxRequest {
	req := bboxRequest{
		File:  file,
		Param: c.field.param,
		BBox:  bbox{MinLon: minLon, MinLat: minLat, MaxLon: maxLon, MaxLat: maxLat},
		Step:  c.step,
	}
	if !at.IsZero() {
		req.Time = at.UTC().Format(time.RFC3339)
	}
	return req
}

// Response headers of gribsvc's /grib/extract_raster (see gribsvc/app/raster.py).
const (
	rasterHeaderRows      = "X-Grid-Rows"
	rasterHeaderCols      = "X-Grid-Cols"
	rasterHeaderMinLat    = "X-Grid-Min-Lat"
	rasterHeaderMaxLat    = "X-Grid-Max-Lat"
	rasterHeaderMinLon    = "X-Grid-Min-Lon"
	rasterHeaderMaxLon    = "X-Grid-Max-Lon"
	rasterHeaderValidTime = "X-Valid-Time"
	// Series responses: frame count and comma-separated RFC3339 valid times,
	// with the frames concatenated in that order in the body.
	rasterHeaderFrames     = "X-Grid-Frames"
	rasterHeaderValidTimes = "X-Valid-Times"
)

// maxRasterCells bounds the body we accept from gribsvc (~64 MB of float32).
const maxRasterCells = 16 << 20

// rasterGeometry is the grid shape and extent a raster response carries in
// its headers.
type rasterGeometry struct {
	rows, cols                     int
	minLat, maxLat, minLon, maxLon float64
}

func parseRasterGeometry(h http.Header) (rasterGeometry, error) {
	var g rasterGeometry
	var err error
	if g.rows, err = strconv.Atoi(h.Get(rasterHeaderRows)); err != nil {
		return g, fmt.Errorf("gribsvc raster: bad %s: %w", rasterHeaderRows, err)
	}
	if g.cols, err = strconv.Atoi(h.Get(rasterHeaderCols)); err != nil {
		return g, fmt.Errorf("gribsvc raster: bad %s: %w", rasterHeaderCols, err)
	}
	if g.rows <= 0 || g.cols <= 0 || g.rows*g.cols > maxRasterCells {
		return g, fmt.Errorf("gribsvc raster: unsupported dims %dx%d", g.rows, g.cols)
	}
	for name, dst := range map[string]*float64{
		rasterHeaderMinLat: &g.minLat, rasterHeaderMaxLat: &g.maxLat,
		rasterHeaderMinLon: &g.minLon, rasterHeaderMaxLon: &g.maxLon,
	} {
		if *dst, err = strconv.ParseFloat(h.Get(name), 64); err != nil {
			return g, fmt.Errorf("gribsvc raster: bad %s: %w", name, err)
		}
	}
	return g, nil
}

// postRaster posts a JSON request to a raster endpoint and returns the
// response for the caller to decode. ok is false with a nil error on a soft
// miss (404/422: file or field not available yet).
func (c *Client) postRaster(ctx context.Context, path string, timeout time.Duration, in any) (*http.Response, bool, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return nil, false, fmt.Errorf("marshal raster request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		cancel()
		return nil, false, fmt.Errorf("build raster request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		cancel()
		return nil, false, fmt.Errorf("call gribsvc raster: %w", err)
	}
	// The body outlives this call; tie the timeout to it.
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		resp.Body.Close()
		return nil, false, nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, false, fmt.Errorf("gribsvc raster returned %d: %s", resp.StatusCode, string(body))
	}
	return resp, true, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}

// readRasterFrames reads n consecutive little-endian float32 frames of the
// given geometry from body, converting each into a FieldGrid in consumer units
// (NaN and FMI fill cells stay NaN). validTimes[i] stamps frame i.
func (c *Client) readRasterFrames(body io.Reader, g rasterGeometry, validTimes []time.Time) ([]*weather.FieldGrid, error) {
	cells := g.rows * g.cols
	want := cells * 4 * len(validTimes)
	raw, err := io.ReadAll(io.LimitReader(body, int64(want)+1))
	if err != nil {
		return nil, fmt.Errorf("read raster body: %w", err)
	}
	if len(raw) != want {
		return nil, fmt.Errorf("gribsvc raster: body %d bytes, want %d for %d frame(s) of %dx%d", len(raw), want, len(validTimes), g.rows, g.cols)
	}

	grids := make([]*weather.FieldGrid, 0, len(validTimes))
	for f, validTime := range validTimes {
		frame := raw[f*cells*4 : (f+1)*cells*4]
		values := make([]float32, cells)
		for i := range values {
			v := math.Float32frombits(binary.LittleEndian.Uint32(frame[4*i:]))
			if v != v || (c.field.missingAbove > 0 && float64(v) >= c.field.missingAbove) {
				values[i] = float32(math.NaN())
				continue
			}
			values[i] = float32(float64(v)*c.field.scale + c.field.offset)
		}
		grids = append(grids, &weather.FieldGrid{
			Rows:       g.rows,
			Cols:       g.cols,
			MinLat:     g.minLat,
			MaxLat:     g.maxLat,
			MinLon:     g.minLon,
			MaxLon:     g.maxLon,
			Values:     values,
			ObservedAt: validTime,
		})
	}
	return grids, nil
}

// fetchRaster posts the extract request to the binary raster endpoint and
// decodes the little-endian float32 body (row-major, north-to-south, NaN =
// masked) into a FieldGrid in consumer units. A nil grid with a nil error is a
// soft miss — the file or field isn't available yet.
func (c *Client) fetchRaster(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (*weather.FieldGrid, time.Time, error) {
	resp, ok, err := c.postRaster(ctx, "/grib/extract_raster", extractTimeout, c.bboxRequest(file, minLon, minLat, maxLon, maxLat, at))
	if err != nil || !ok {
		return nil, time.Time{}, err
	}
	defer resp.Body.Close()

	g, err := parseRasterGeometry(resp.Header)
	if err != nil {
		return nil, time.Time{}, err
	}
	validTime, _ := time.Parse(time.RFC3339, resp.Header.Get(rasterHeaderValidTime))
	grids, err := c.readRasterFrames(resp.Body, g, []time.Time{validTime})
	if err != nil {
		return nil, time.Time{}, err
	}
	return grids[0], validTime, nil
}

// postJSON posts a JSON body to a gribsvc path and decodes the response into
// out, within timeout. ok is false with a nil error on a soft miss — gribsvc
// answered 404/422 because the file or field isn't available yet.
func (c *Client) postJSON(ctx context.Context, path string, timeout time.Duration, in, out any) (bool, error) {
	payload, err := json.Marshal(in)
	if err != nil {
		return false, fmt.Errorf("marshal extract request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
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
// north-to-south (row 0 = MaxLat). Soft misses (file/field not ready) return a
// nil grid and nil error.
func (c *Client) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*weather.FieldGrid, time.Time, error) {
	return c.GridForFile(ctx, c.file, minLon, minLat, maxLon, maxLat, at)
}

// GridForFile is Grid against an explicit gribsvc file name — used for
// per-timestamp radar frames, where each instant is its own file and the
// requested time is implied by the name (at stays zero). It reads the binary
// raster endpoint: a full radar frame is ~2 MB this way against ~19 MB of JSON,
// and decoding it is a flat copy rather than a million small allocations, which
// keeps the 5-minute nowcast warm from starving request handling.
func (c *Client) GridForFile(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (*weather.FieldGrid, time.Time, error) {
	return c.fetchRaster(ctx, file, minLon, minLat, maxLon, maxLat, at)
}

type seriesRequest struct {
	File  string   `json:"file"`
	Param string   `json:"param"`
	BBox  bbox     `json:"bbox"`
	Step  int      `json:"step"`
	Times []string `json:"times,omitempty"`
}

// GridSeries extracts the configured field over the bbox for every requested
// hour in a single gribsvc pass over the file, returning one FieldGrid per hour
// present (ObservedAt carries the frame's valid time). Hours the file lacks are
// simply absent from the result. Soft misses (file not downloaded yet, or no
// matching fields at all) return an empty slice and nil error. Frames travel as
// one concatenated float32 raster rather than JSON: the hourly warm reads ~28
// of them, which as text cost about a second of gribsvc CPU each.
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

	resp, ok, err := c.postRaster(ctx, "/grib/extract_raster_series", seriesTimeout, reqBody)
	if err != nil || !ok {
		return nil, err
	}
	defer resp.Body.Close()

	g, err := parseRasterGeometry(resp.Header)
	if err != nil {
		return nil, err
	}
	frames, err := strconv.Atoi(resp.Header.Get(rasterHeaderFrames))
	if err != nil || frames < 0 {
		return nil, fmt.Errorf("gribsvc raster: bad %s: %v", rasterHeaderFrames, err)
	}
	var validTimes []time.Time
	if raw := resp.Header.Get(rasterHeaderValidTimes); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(part))
			if err != nil {
				return nil, fmt.Errorf("gribsvc raster: bad %s entry %q: %w", rasterHeaderValidTimes, part, err)
			}
			validTimes = append(validTimes, t)
		}
	}
	if len(validTimes) != frames {
		return nil, fmt.Errorf("gribsvc raster: %d frames but %d valid times", frames, len(validTimes))
	}
	return c.readRasterFrames(resp.Body, g, validTimes)
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
