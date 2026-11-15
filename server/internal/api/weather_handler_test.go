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

func TestForecastFetchedAtIsReportedWhenKnown(t *testing.T) {
	fetchedAt := time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)
	h := NewHandler(weatherServiceStub{
		weather: &weather.WeatherResponse{ForecastFetchedAt: fetchedAt},
		hourly:  &weather.HourlyForecastResponse{FetchedAt: fetchedAt},
	})
	handlers := map[string]http.HandlerFunc{
		"/v1/weather":        h.getWeather,
		"/v1/weather/hourly": h.getHourlyForecast,
	}
	for path, handler := range handlers {
		rr := httptest.NewRecorder()
		handler(rr, httptest.NewRequest(http.MethodGet, path+"?lat=60.1&lon=24.9", nil))
		var resp struct {
			ForecastFetchedAt *time.Time `json:"forecast_fetched_at"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("%s: decode response: %v", path, err)
		}
		if resp.ForecastFetchedAt == nil || !resp.ForecastFetchedAt.Equal(fetchedAt) {
			t.Errorf("%s: forecast_fetched_at = %v, want %v", path, resp.ForecastFetchedAt, fetchedAt)
		}
	}

	rr := httptest.NewRecorder()
	NewHandler(weatherServiceStub{}).getWeather(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, present := raw["forecast_fetched_at"]; present {
		t.Error("forecast_fetched_at should be omitted when unknown")
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

func (s weatherServiceStub) GetTemperatureSamples(ctx context.Context) (*weather.TemperatureSamplesResponse, error) {
	panic("not used in this test")
}

func (s weatherServiceStub) GetTemperatureSamplesAt(ctx context.Context, at time.Time) (*weather.TemperatureSamplesResponse, error) {
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

func TestGetWeather_NamesBorrowedValuesAndFeelsLike(t *testing.T) {
	feels := -5.0
	current := weather.CurrentWeather{
		Station:     weather.Station{Name: "Helsinki Kaisaniemi"},
		DistanceKM:  0.7,
		Observation: weather.Observation{ObservedAt: time.Date(2026, 11, 14, 14, 0, 0, 0, time.UTC)},
		FeelsLike:   &feels,
		Borrowed: []weather.BorrowedValues{{
			Station:    weather.Station{Name: "Helsinki Kumpula"},
			DistanceKM: 3.2,
			Fields:     []string{"wind_speed", "wind_gust"},
		}},
	}
	type response struct {
		Current struct {
			FeelsLike *float64 `json:"feels_like"`
			Borrowed  []struct {
				Station struct {
					Name       string  `json:"name"`
					DistanceKM float64 `json:"distance_km"`
				} `json:"station"`
				Fields []string `json:"fields"`
			} `json:"borrowed"`
		} `json:"current"`
	}
	get := func(current weather.CurrentWeather) (response, map[string]json.RawMessage) {
		t.Helper()
		h := NewHandler(weatherServiceStub{weather: &weather.WeatherResponse{Current: current, Timezone: "Europe/Helsinki"}})
		rr := httptest.NewRecorder()
		h.getWeather(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil))
		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		var resp response
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		var raw struct {
			Current map[string]json.RawMessage `json:"current"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
			t.Fatalf("decode raw response: %v", err)
		}
		return resp, raw.Current
	}

	resp, _ := get(current)
	if resp.Current.FeelsLike == nil || *resp.Current.FeelsLike != feels {
		t.Errorf("feels_like = %v, want %v", resp.Current.FeelsLike, feels)
	}
	if len(resp.Current.Borrowed) != 1 {
		t.Fatalf("borrowed = %+v, want one entry", resp.Current.Borrowed)
	}
	b := resp.Current.Borrowed[0]
	if b.Station.Name != "Helsinki Kumpula" || b.Station.DistanceKM != 3.2 || len(b.Fields) != 2 || b.Fields[0] != "wind_speed" || b.Fields[1] != "wind_gust" {
		t.Errorf("borrowed entry = %+v", b)
	}

	current.Borrowed = nil
	if _, raw := get(current); raw["borrowed"] != nil {
		t.Errorf("borrowed = %s, want the key omitted when nothing is borrowed", raw["borrowed"])
	}
}

func TestGetWeather_SendsEmptyForecastListsAsArrays(t *testing.T) {
	h := NewHandler(weatherServiceStub{
		weather: &weather.WeatherResponse{
			Current: weather.CurrentWeather{
				Observation: weather.Observation{ObservedAt: time.Date(2026, 11, 14, 14, 0, 0, 0, time.UTC)},
			},
			Timezone: "Europe/Helsinki",
		},
	})
	rr := httptest.NewRecorder()
	h.getWeather(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=60.1&lon=24.9", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"daily_forecast", "hourly_forecast", "uv_forecast"} {
		if got := string(resp[key]); got != "[]" {
			t.Errorf("%s = %s, want []", key, got)
		}
	}
}
