package api

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"wby/internal/weather"
)

func TestGetTemperatureSamples_OK(t *testing.T) {
	dataTime := time.Date(2026, 4, 19, 12, 40, 0, 0, time.UTC)
	h := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: dataTime,
			MinTemp:  -8.2,
			MaxTemp:  13.4,
			Samples: []weather.TemperatureSample{
				{Lat: 60.17, Lon: 24.94, Temperature: 7.1, ObservedAt: dataTime},
				{Lat: 61.50, Lon: 23.77, Temperature: 5.0, ObservedAt: dataTime},
			},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples", nil)
	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("unexpected content type: %s", got)
	}
	if rr.Header().Get("ETag") == "" {
		t.Fatal("expected ETag header")
	}

	var body struct {
		DataTime time.Time `json:"data_time"`
		MinTemp  float64   `json:"min_temp"`
		MaxTemp  float64   `json:"max_temp"`
		Samples  []struct {
			Lat  float64 `json:"lat"`
			Lon  float64 `json:"lon"`
			Temp float64 `json:"temp"`
		} `json:"samples"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.DataTime.Equal(dataTime) {
		t.Fatalf("unexpected data_time: %v", body.DataTime)
	}
	if body.MinTemp != -8.2 || body.MaxTemp != 13.4 {
		t.Fatalf("unexpected min/max: %f %f", body.MinTemp, body.MaxTemp)
	}
	if len(body.Samples) != 2 {
		t.Fatalf("expected 2 samples, got %d", len(body.Samples))
	}
}

func TestGetTemperatureSamples_NotModified(t *testing.T) {
	dataTime := time.Date(2026, 4, 19, 12, 40, 0, 0, time.UTC)
	h := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: dataTime,
			MinTemp:  -1,
			MaxTemp:  1,
			Samples:  []weather.TemperatureSample{{}, {}, {}},
		},
	})

	first := httptest.NewRecorder()
	h.getTemperatureSamples(first, httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples", nil))
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("expected ETag header")
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples", nil)
	req.Header.Set("If-None-Match", etag)

	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusNotModified {
		t.Fatalf("expected status 304, got %d", rr.Code)
	}
	if rr.Body.Len() != 0 {
		t.Fatalf("expected empty body on 304")
	}
}

func TestGetTemperatureSamples_AtParam_Invalid(t *testing.T) {
	h := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: time.Now().UTC(),
			Samples:  []weather.TemperatureSample{{}, {}, {}},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples?at=not-a-date", nil)
	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed at, got %d", rr.Code)
	}
}

func TestGetTemperatureSamples_AtParam_FrameMissingIs404(t *testing.T) {
	h := NewHandler(fakeWeatherService{err: weather.ErrForecastGridUnavailable})

	rr := httptest.NewRecorder()
	at := time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples?at="+at, nil)
	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unwarmed frame, got %d", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=30" {
		t.Fatalf("expected short cache on miss, got %q", got)
	}
}

func TestGetTemperatureSamples_UpstreamTimeoutIs504(t *testing.T) {
	h := NewHandler(fakeWeatherService{err: fmt.Errorf("call gribsvc: %w", context.DeadlineExceeded)})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples?at=2026-05-03T08:00:00Z", nil)
	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected 504 for upstream timeout with a live client, got %d", rr.Code)
	}
	if rr.Body.Len() == 0 {
		t.Fatal("expected an error body")
	}
}

func TestGetTemperatureSamples_ClientCanceled_NoBody(t *testing.T) {
	h := NewHandler(fakeWeatherService{err: context.Canceled})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples?at=2026-05-03T08:00:00Z", nil).WithContext(ctx)
	h.getTemperatureSamples(rr, req)

	if rr.Body.Len() != 0 {
		t.Fatalf("expected no body for a canceled client, got %q", rr.Body.String())
	}
}

func TestGetTemperatureSamples_AtParam_LongerCache(t *testing.T) {
	h := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: time.Now().UTC(),
			Samples:  []weather.TemperatureSample{{}, {}, {}},
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples?at=2026-05-03T08:00:00Z", nil)
	h.getTemperatureSamples(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Cache-Control"); got != "public, max-age=300, stale-while-revalidate=900" {
		t.Fatalf("expected longer cache for historical query, got %q", got)
	}
}

func TestGetTemperatureSamples_ETagChangesWhenSamplesChange(t *testing.T) {
	dataTime := time.Date(2026, 4, 19, 12, 40, 0, 0, time.UTC)
	base := httptest.NewRecorder()
	h1 := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: dataTime,
			MinTemp:  1,
			MaxTemp:  5,
			Samples: []weather.TemperatureSample{
				{Lat: 60.1, Lon: 24.9, Temperature: 4, ObservedAt: dataTime},
			},
		},
	})
	h1.getTemperatureSamples(base, httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples", nil))
	etag1 := base.Header().Get("ETag")
	if etag1 == "" {
		t.Fatal("expected first etag")
	}

	changed := httptest.NewRecorder()
	h2 := NewHandler(fakeWeatherService{
		samples: &weather.TemperatureSamplesResponse{
			DataTime: dataTime,
			MinTemp:  1,
			MaxTemp:  6,
			Samples: []weather.TemperatureSample{
				{Lat: 60.1, Lon: 24.9, Temperature: 6, ObservedAt: dataTime},
			},
		},
	})
	h2.getTemperatureSamples(changed, httptest.NewRequest(http.MethodGet, "/v1/map/temperature/samples", nil))
	etag2 := changed.Header().Get("ETag")
	if etag2 == "" {
		t.Fatal("expected second etag")
	}
	if etag1 == etag2 {
		t.Fatalf("expected ETag to change when payload changes; got %s", etag1)
	}
}

func TestFieldGridJSON_ValuesWireFormat(t *testing.T) {
	grid := &weather.FieldGrid{
		Rows: 2, Cols: 2,
		MinLat: 60, MaxLat: 60.1, MinLon: 24, MaxLon: 24.1,
		Values: []float32{1.25, float32(math.NaN()), -3.14, 0},
	}
	body, err := json.Marshal(buildFieldGridJSON(grid))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rows":2,"cols":2,"min_lat":60,"max_lat":60.1,"min_lon":24,"max_lon":24.1,"values":[1.3,null,-3.1,0]}`
	if string(body) != want {
		t.Fatalf("got %s\nwant %s", body, want)
	}
}
