package grib

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
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

func TestGridFlipsConvertsAndMasks(t *testing.T) {
	// gribsvc rows are south-to-north (row 0 = lat 60.0). Grid must flip to
	// north-to-south (row 0 = max lat), convert Kelvin->Celsius, and null the
	// FMI fill sentinel.
	const body = `{
		"param":"2t","units":"K","valid_time":"2026-06-24T16:00:00Z",
		"rows":2,"cols":2,
		"lats":[[60.0,60.0],[60.1,60.1]],
		"lons":[[24.0,24.1],[24.0,24.1]],
		"values":[[293.15,9999.0],[283.15,300.65]]
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	grid, _, err := New(srv.URL, "f.grib2", "2t", 1).
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
	if grid.MinLat != 60.0 || grid.MaxLat != 60.1 {
		t.Fatalf("unexpected lat bounds: %v..%v", grid.MinLat, grid.MaxLat)
	}
	// Row 0 must now be the northern row (lat 60.1): 283.15K->10C, 300.65K->27.5C.
	if math.Abs(float64(grid.Values[0])-10.0) > 1e-4 {
		t.Fatalf("expected north-west cell 10.0C, got %v", grid.Values[0])
	}
	if math.Abs(float64(grid.Values[1])-27.5) > 1e-4 {
		t.Fatalf("expected north-east cell 27.5C, got %v", grid.Values[1])
	}
	// Southern row (originally row 0): 293.15K->20C, then the fill sentinel -> NaN.
	if math.Abs(float64(grid.Values[2])-20.0) > 1e-4 {
		t.Fatalf("expected south-west cell 20.0C, got %v", grid.Values[2])
	}
	if !math.IsNaN(float64(grid.Values[3])) {
		t.Fatalf("expected south-east fill cell to be NaN, got %v", grid.Values[3])
	}
}

func TestGridSeriesReturnsFrameGrids(t *testing.T) {
	// Two hourly frames sharing one lat/lon lattice (south-to-north rows). Each
	// frame must become its own flipped, Kelvin->Celsius FieldGrid.
	const body = `{
		"param":"2t","units":"K","rows":2,"cols":2,
		"lats":[[60.0,60.0],[60.1,60.1]],
		"lons":[[24.0,24.1],[24.0,24.1]],
		"frames":[
			{"valid_time":"2026-06-24T16:00:00Z","values":[[293.15,283.15],[283.15,300.65]]},
			{"valid_time":"2026-06-24T17:00:00Z","values":[[294.15,9999.0],[284.15,301.65]]}
		]
	}`

	var gotPath string
	var gotBody seriesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	hours := []time.Time{
		time.Date(2026, 6, 24, 16, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 24, 17, 0, 0, 0, time.UTC),
	}
	grids, err := New(srv.URL, "f.grib2", "2t", 1).
		GridSeries(context.Background(), 19, 59, 32, 71, hours)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/grib/extract_series" {
		t.Fatalf("unexpected path %q", gotPath)
	}
	if len(gotBody.Times) != 2 || gotBody.Times[0] != "2026-06-24T16:00:00Z" {
		t.Fatalf("unexpected times in request: %v", gotBody.Times)
	}
	if len(grids) != 2 {
		t.Fatalf("expected 2 grids, got %d", len(grids))
	}
	if !grids[0].ObservedAt.Equal(hours[0]) || !grids[1].ObservedAt.Equal(hours[1]) {
		t.Fatalf("unexpected valid times: %v, %v", grids[0].ObservedAt, grids[1].ObservedAt)
	}
	// Frame 1, row 0 must be the northern row: 284.15K -> 11C.
	if got := grids[1].Values[0]; math.Abs(float64(got)-11.0) > 1e-4 {
		t.Fatalf("expected north-west cell 11.0C, got %v", got)
	}
	// Frame 1's fill sentinel (southern row after the flip) must be NaN.
	if got := grids[1].Values[3]; !math.IsNaN(float64(got)) {
		t.Fatalf("expected fill cell to be NaN, got %v", got)
	}
}

func TestGridSeriesSoftMiss(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusUnprocessableEntity} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"not available"}`))
		}))

		grids, err := New(srv.URL, "f.grib2", "2t", 1).
			GridSeries(context.Background(), 19, 59, 32, 71, []time.Time{time.Now()})
		srv.Close()
		if err != nil {
			t.Fatalf("status %d: expected soft miss (nil error), got %v", status, err)
		}
		if len(grids) != 0 {
			t.Fatalf("status %d: expected no grids, got %d", status, len(grids))
		}
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
