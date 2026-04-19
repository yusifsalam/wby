package api

import (
	"encoding/json"
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
