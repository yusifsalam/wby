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

func TestGetHourlyForecast(t *testing.T) {
	temp := 12.5
	h := NewHandler(weatherServiceStub{
		hourly: &weather.HourlyForecastResponse{
			Hourly: []weather.HourlyForecast{
				{Time: time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC), Temperature: &temp},
			},
			Timezone: "Europe/Helsinki",
		},
	})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/weather/hourly?lat=60.1&lon=24.9", nil)
	h.getHourlyForecast(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
	var resp struct {
		Hourly []struct {
			Time        string   `json:"time"`
			Temperature *float64 `json:"temperature"`
		} `json:"hourly_forecast"`
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Timezone != "Europe/Helsinki" {
		t.Fatalf("expected timezone Europe/Helsinki, got %q", resp.Timezone)
	}
	if len(resp.Hourly) != 1 || resp.Hourly[0].Temperature == nil || *resp.Hourly[0].Temperature != temp {
		t.Fatalf("unexpected hourly payload: %+v", resp.Hourly)
	}

	rr = httptest.NewRecorder()
	h.getHourlyForecast(rr, httptest.NewRequest(http.MethodGet, "/v1/weather/hourly?lat=x&lon=24.9", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for bad lat, got %d", rr.Code)
	}
}

type weatherServiceStub struct {
	weather *weather.WeatherResponse
	hourly  *weather.HourlyForecastResponse
	err     error
}

func (s weatherServiceStub) GetHourlyForecast(ctx context.Context, lat, lon float64) (*weather.HourlyForecastResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.hourly != nil {
		return s.hourly, nil
	}
	return &weather.HourlyForecastResponse{}, nil
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

func (s weatherServiceStub) GetTemperatureSamplesAt(ctx context.Context, at time.Time) (*weather.TemperatureSamplesResponse, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetPrecipitationOverlay(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationOverlay, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetPrecipitationForecastGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetPrecipitationObservationGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetPrecipitationNowcastGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*weather.Station, float64, []weather.ClimateNormal, weather.InterpolatedNormal, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetDailyClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64, now time.Time) (*weather.DailyNormalsResult, error) {
	return nil, nil
}

func (s weatherServiceStub) GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]weather.LeaderboardEntry, error) {
	panic("not used in this test")
}
