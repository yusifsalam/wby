package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"wby/internal/weather"
)

func TestParseMapTemperatureRequest_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=24.7,60.1,25.2,60.4&width=390&height=844", nil)

	got, err := parseMapTemperatureRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.MinLon != 24.7 || got.MinLat != 60.1 || got.MaxLon != 25.2 || got.MaxLat != 60.4 {
		t.Fatalf("unexpected bbox: %+v", got)
	}
	if got.Width != 390 || got.Height != 844 {
		t.Fatalf("unexpected dimensions: %+v", got)
	}
}

func TestParseMapTemperatureRequest_InvalidBBox(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=bad&width=390&height=844", nil)
	_, err := parseMapTemperatureRequest(req)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGetTemperatureOverlay_OK(t *testing.T) {
	h := NewHandler(fakeWeatherService{
		overlay: &weather.TemperatureOverlay{
			PNG:      []byte{0x89, 0x50, 0x4e, 0x47},
			DataTime: time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
			MinTemp:  -4.5,
			MaxTemp:  2.8,
		},
	})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=24.7,60.1,25.2,60.4&width=390&height=844", nil)

	h.getTemperatureOverlay(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("unexpected content type: %s", got)
	}
	if rr.Header().Get("X-Temp-Min") == "" || rr.Header().Get("X-Temp-Max") == "" || rr.Header().Get("X-Data-Time") == "" {
		t.Fatalf("expected metadata headers")
	}
}

func TestGetTemperatureOverlay_BadRequest(t *testing.T) {
	h := NewHandler(fakeWeatherService{})
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/map/temperature?bbox=oops&width=390&height=844", nil)

	h.getTemperatureOverlay(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

type fakeWeatherService struct {
	overlay *weather.TemperatureOverlay
	err     error
}

func (f fakeWeatherService) GetWeather(ctx context.Context, lat, lon float64) (*weather.WeatherResponse, error) {
	panic("not used in this test")
}

func (f fakeWeatherService) GetTemperatureOverlay(ctx context.Context, req weather.MapOverlayRequest) (*weather.TemperatureOverlay, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.overlay != nil {
		return f.overlay, nil
	}
	return &weather.TemperatureOverlay{
		PNG:      []byte{0x89, 0x50, 0x4e, 0x47},
		DataTime: time.Now().UTC(),
		MinTemp:  0,
		MaxTemp:  1,
	}, nil
}
