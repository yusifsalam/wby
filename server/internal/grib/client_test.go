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
