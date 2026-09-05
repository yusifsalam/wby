package grib

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestTemperatureSamples(t *testing.T) {
	// 2x2 grid, one masked (null) point. Values are Kelvin.
	const body = `{
		"param":"2t","units":"K","valid_time":"2026-06-24T16:00:00Z",
		"rows":2,"cols":2,
		"lats":[[60.0,60.0],[60.1,60.1]],
		"lons":[[24.0,24.1],[24.0,24.1]],
		"values":[[293.15,null],[283.15,300.65]]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/grib/extract" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	samples, validTime, err := New(srv.URL, "harmonie_surface.grib2", "2t", 1).
		TemperatureSamples(context.Background(), 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validTime.IsZero() {
		t.Fatal("expected non-zero valid time")
	}
	// 4 gridpoints minus the 1 masked = 3 samples.
	if len(samples) != 3 {
		t.Fatalf("expected 3 samples, got %d", len(samples))
	}
	// 293.15 K -> 20.0 C (Kelvin->Celsius conversion).
	if got := samples[0].Temperature; math.Abs(got-20.0) > 1e-6 {
		t.Fatalf("expected 20.0 C, got %v", got)
	}
}

func TestTemperatureSamplesDownsampledSpansExtent(t *testing.T) {
	// A grid larger than maxRenderSamples must be strided down but still cover
	// the full latitude range — the bug was the renderer keeping only the
	// southern prefix.
	const n = 80
	lats := make([][]float64, n)
	lons := make([][]float64, n)
	values := make([][]*float64, n)
	for i := 0; i < n; i++ {
		lats[i] = make([]float64, n)
		lons[i] = make([]float64, n)
		values[i] = make([]*float64, n)
		for j := 0; j < n; j++ {
			lats[i][j] = 59.0 + 12.0*float64(i)/float64(n-1) // 59 -> 71
			lons[i][j] = 19.0 + 13.0*float64(j)/float64(n-1) // 19 -> 32
			v := 290.0
			values[i][j] = &v
		}
	}
	body, _ := json.Marshal(bboxResponse{ValidTime: "2026-06-24T16:00:00Z", Lats: lats, Lons: lons, Values: values})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	samples, _, err := New(srv.URL, "f.grib2", "2t", 1).
		TemperatureSamples(context.Background(), 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(samples) == 0 || len(samples) > maxRenderSamples {
		t.Fatalf("expected 1..%d samples, got %d", maxRenderSamples, len(samples))
	}
	var latMin, latMax = 90.0, -90.0
	for _, s := range samples {
		latMin = math.Min(latMin, s.Lat)
		latMax = math.Max(latMax, s.Lat)
	}
	if latMin > 60.0 {
		t.Fatalf("south not represented: latMin=%v", latMin)
	}
	if latMax < 70.0 {
		t.Fatalf("north not represented (the band bug): latMax=%v", latMax)
	}
}

func TestTemperatureSamplesSoftMiss(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnprocessableEntity} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"not available"}`))
		}))

		samples, _, err := New(srv.URL, "f.grib2", "2t", 1).
			TemperatureSamples(context.Background(), 19, 59, 32, 71, time.Time{})
		srv.Close()
		if err != nil {
			t.Fatalf("status %d: expected soft miss (nil error), got %v", status, err)
		}
		if len(samples) != 0 {
			t.Fatalf("status %d: expected no samples, got %d", status, len(samples))
		}
	}
}

// rasterServer serves one binary raster (little-endian float32, north-to-south)
// the way gribsvc's /grib/extract_raster does.
func rasterServer(t *testing.T, rows, cols int, values []float32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/grib/extract_raster" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var req struct {
			File  string `json:"file"`
			Param string `json:"param"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Param == "" {
			t.Errorf("bad request body: %v (%+v)", err, req)
		}
		h := w.Header()
		h.Set("Content-Type", "application/octet-stream")
		h.Set("X-Grid-Rows", strconv.Itoa(rows))
		h.Set("X-Grid-Cols", strconv.Itoa(cols))
		h.Set("X-Grid-Min-Lat", "60.0")
		h.Set("X-Grid-Max-Lat", "60.1")
		h.Set("X-Grid-Min-Lon", "24.0")
		h.Set("X-Grid-Max-Lon", "24.1")
		h.Set("X-Valid-Time", "2026-06-24T16:00:00Z")
		body := make([]byte, 4*len(values))
		for i, v := range values {
			binary.LittleEndian.PutUint32(body[4*i:], math.Float32bits(v))
		}
		_, _ = w.Write(body)
	}))
}

func TestGridConvertsAndMasksRaster(t *testing.T) {
	// gribsvc already emits north-to-south rows. Grid must convert
	// Kelvin->Celsius, null the FMI fill sentinel and keep NaN cells masked.
	srv := rasterServer(t, 2, 2, []float32{283.15, 300.65, 293.15, 9999.0})
	defer srv.Close()

	grid, validTime, err := New(srv.URL, "f.grib2", "2t", 1).
		Grid(context.Background(), 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if grid == nil {
		t.Fatal("expected a grid")
	}
	if grid.Rows != 2 || grid.Cols != 2 {
		t.Fatalf("unexpected dims: %dx%d", grid.Rows, grid.Cols)
	}
	if grid.MinLat != 60.0 || grid.MaxLat != 60.1 || grid.MinLon != 24.0 || grid.MaxLon != 24.1 {
		t.Fatalf("unexpected bounds: %v..%v / %v..%v", grid.MinLat, grid.MaxLat, grid.MinLon, grid.MaxLon)
	}
	if validTime.IsZero() || !grid.ObservedAt.Equal(validTime) {
		t.Fatalf("expected valid time from header, got %v / %v", validTime, grid.ObservedAt)
	}
	want := []float64{10.0, 27.5, 20.0, math.NaN()}
	for i, w := range want {
		got := float64(grid.Values[i])
		if math.IsNaN(w) {
			if !math.IsNaN(got) {
				t.Fatalf("cell %d: expected fill sentinel to be NaN, got %v", i, got)
			}
			continue
		}
		if math.Abs(got-w) > 1e-4 {
			t.Fatalf("cell %d: expected %v, got %v", i, w, got)
		}
	}
}

func TestGridForFileRadarKeepsUnitsAndNaN(t *testing.T) {
	// Radar frames arrive in mm/h with nodata already NaN; no conversion applies.
	srv := rasterServer(t, 1, 3, []float32{0, 1.5, float32(math.NaN())})
	defer srv.Close()

	grid, _, err := NewRadar(srv.URL, 1).
		GridForFile(context.Background(), "radar_rr_20260624T1600Z.tif", 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if grid == nil || len(grid.Values) != 3 {
		t.Fatalf("expected 3 cells, got %+v", grid)
	}
	if grid.Values[0] != 0 || grid.Values[1] != 1.5 || !math.IsNaN(float64(grid.Values[2])) {
		t.Fatalf("unexpected values: %v", grid.Values)
	}
}

func TestGridForFileSoftMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"no such file"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	grid, _, err := NewRadar(srv.URL, 1).
		GridForFile(context.Background(), "radar_rr_19700101T0000Z.tif", 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("expected soft miss, got error: %v", err)
	}
	if grid != nil {
		t.Fatalf("expected nil grid on soft miss, got %+v", grid)
	}
}

func TestGridRejectsTruncatedRaster(t *testing.T) {
	// Header claims 2x2 but the body carries 3 cells.
	srv := rasterServer(t, 2, 2, []float32{1, 2, 3})
	defer srv.Close()

	if _, _, err := NewRadar(srv.URL, 1).
		GridForFile(context.Background(), "radar_rr_20260624T1600Z.tif", 19, 59, 32, 71, time.Time{}); err == nil {
		t.Fatal("expected an error for a truncated body")
	}
}

func TestGridSeriesReturnsFrameGrids(t *testing.T) {
	// Two hourly frames concatenated in one float32 body, north-to-south rows.
	// Each must become its own Kelvin->Celsius FieldGrid stamped with its time.
	frames := []float32{
		283.15, 300.65, 293.15, 9999.0, // 16:00
		284.15, 301.65, 294.15, float32(math.NaN()), // 17:00
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/grib/extract_raster_series" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var req struct {
			Times []string `json:"times"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Times) != 2 {
			t.Errorf("expected two requested times, got %v (%v)", req.Times, err)
		}
		h := w.Header()
		h.Set("X-Grid-Rows", "2")
		h.Set("X-Grid-Cols", "2")
		h.Set("X-Grid-Min-Lat", "60.0")
		h.Set("X-Grid-Max-Lat", "60.1")
		h.Set("X-Grid-Min-Lon", "24.0")
		h.Set("X-Grid-Max-Lon", "24.1")
		h.Set("X-Grid-Frames", "2")
		h.Set("X-Valid-Times", "2026-06-24T16:00:00Z,2026-06-24T17:00:00Z")
		body := make([]byte, 4*len(frames))
		for i, v := range frames {
			binary.LittleEndian.PutUint32(body[4*i:], math.Float32bits(v))
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	t16 := time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC)
	grids, err := New(srv.URL, "f.grib2", "2t", 1).
		GridSeries(context.Background(), 19, 59, 32, 71, []time.Time{t16, t16.Add(time.Hour)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(grids) != 2 {
		t.Fatalf("expected 2 grids, got %d", len(grids))
	}
	if !grids[0].ObservedAt.Equal(t16) || !grids[1].ObservedAt.Equal(t16.Add(time.Hour)) {
		t.Fatalf("unexpected frame times: %v, %v", grids[0].ObservedAt, grids[1].ObservedAt)
	}
	if grids[0].MinLat != 60.0 || grids[0].MaxLat != 60.1 || grids[1].MinLon != 24.0 || grids[1].MaxLon != 24.1 {
		t.Fatalf("unexpected bounds: %+v", grids[0])
	}
	if math.Abs(float64(grids[0].Values[0])-10.0) > 1e-4 || !math.IsNaN(float64(grids[0].Values[3])) {
		t.Fatalf("frame 0: expected 10.0C and fill->NaN, got %v", grids[0].Values)
	}
	if math.Abs(float64(grids[1].Values[0])-11.0) > 1e-4 || !math.IsNaN(float64(grids[1].Values[3])) {
		t.Fatalf("frame 1: expected 11.0C and NaN, got %v", grids[1].Values)
	}
}

func TestGridSeriesRejectsFrameCountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Grid-Rows", "1")
		h.Set("X-Grid-Cols", "1")
		h.Set("X-Grid-Min-Lat", "60.0")
		h.Set("X-Grid-Max-Lat", "60.0")
		h.Set("X-Grid-Min-Lon", "24.0")
		h.Set("X-Grid-Max-Lon", "24.0")
		h.Set("X-Grid-Frames", "2")
		h.Set("X-Valid-Times", "2026-06-24T16:00:00Z")
		_, _ = w.Write(make([]byte, 8))
	}))
	defer srv.Close()

	if _, err := New(srv.URL, "f.grib2", "2t", 1).
		GridSeries(context.Background(), 19, 59, 32, 71, nil); err == nil {
		t.Fatal("expected an error when frame count and valid times disagree")
	}
}

func TestGridSeriesSoftMiss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"detail":"no fields found"}`, http.StatusUnprocessableEntity)
	}))
	defer srv.Close()

	grids, err := New(srv.URL, "f.grib2", "2t", 1).
		GridSeries(context.Background(), 19, 59, 32, 71, []time.Time{time.Now()})
	if err != nil {
		t.Fatalf("expected soft miss, got error: %v", err)
	}
	if len(grids) != 0 {
		t.Fatalf("expected no grids on soft miss, got %d", len(grids))
	}
}

func TestPrecipitationSamplesConvertsRateAndDropsFill(t *testing.T) {
	// prate is kg m^-2 s^-1; one cell is the FMI fill sentinel (the analysis
	// step), one is masked. Expect mm/h conversion (x3600) and both gaps gone.
	const body = `{
		"param":"prate","units":"kg m**-2 s**-1","valid_time":"2026-06-27T04:00:00Z",
		"rows":2,"cols":2,
		"lats":[[60.0,60.0],[60.1,60.1]],
		"lons":[[24.0,24.1],[24.0,24.1]],
		"values":[[0.001,9999.0],[null,0.0]]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	samples, validTime, err := NewPrecipitation(srv.URL, "harmonie_surface.grib2", "prate", 1).
		Samples(context.Background(), 19, 59, 32, 71, time.Time{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validTime.IsZero() {
		t.Fatal("expected non-zero valid time")
	}
	// 4 cells minus the fill (9999) and the masked (null) = 2 samples.
	if len(samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(samples))
	}
	// 0.001 kg m^-2 s^-1 -> 3.6 mm/h.
	if got := samples[0].Value; math.Abs(got-3.6) > 1e-9 {
		t.Fatalf("expected 3.6 mm/h, got %v", got)
	}
	for _, s := range samples {
		if s.Value >= fmiMissingValue {
			t.Fatalf("fill sentinel leaked into samples: %v", s.Value)
		}
	}
}
