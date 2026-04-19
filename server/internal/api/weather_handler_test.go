package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"wby/internal/weather"
)

func TestGetWeather_IncludesTimezoneFromService(t *testing.T) {
	h := NewHandler(weatherServiceStub{
		weather: &weather.WeatherResponse{
			Current: weather.CurrentWeather{
				Station: weather.Station{
					Name: "Helsinki Kaisaniemi",
				},
				DistanceKM: 1.2,
				Observation: weather.Observation{
					ObservedAt: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
				},
			},
			Timezone: "Europe/Helsinki",
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil)
	h.getWeather(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp struct {
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Timezone != "Europe/Helsinki" {
		t.Fatalf("expected timezone Europe/Helsinki, got %q", resp.Timezone)
	}
}

type weatherServiceStub struct {
	weather *weather.WeatherResponse
	err     error
}

func (s weatherServiceStub) GetWeather(ctx context.Context, lat, lon float64) (*weather.WeatherResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.weather != nil {
		return s.weather, nil
	}
	return &weather.WeatherResponse{}, nil
}

func (s weatherServiceStub) GetTemperatureOverlay(ctx context.Context, req weather.MapOverlayRequest) (*weather.TemperatureOverlay, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetTemperatureSamples(ctx context.Context) (*weather.TemperatureSamplesResponse, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*weather.Station, float64, []weather.ClimateNormal, weather.InterpolatedNormal, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]weather.LeaderboardEntry, error) {
	panic("not used in this test")
}
