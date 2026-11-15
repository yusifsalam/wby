package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlersRejectInvalidCoordinates(t *testing.T) {
	h := NewHandler(weatherServiceStub{})
	handlers := map[string]http.HandlerFunc{
		"/v1/weather":               h.getWeather,
		"/v1/weather/hourly":        h.getHourlyForecast,
		"/v1/climate-normals":       h.getClimateNormals,
		"/v1/climate-normals/daily": h.getDailyClimateNormals,
		"/v1/leaderboard":           h.getLeaderboard,
	}
	queries := []string{
		"lat=NaN&lon=24.9",
		"lat=60.1&lon=NaN",
		"lat=Inf&lon=24.9",
		"lat=60.1&lon=-Inf",
		"lat=90.1&lon=24.9",
		"lat=60.1&lon=180.5",
		"lat=&lon=24.9",
		"lat=60.1",
		"lat=north&lon=24.9",
	}
	for path, handler := range handlers {
		for _, q := range queries {
			rr := httptest.NewRecorder()
			handler(rr, httptest.NewRequest(http.MethodGet, path+"?"+q, nil))
			if rr.Code != http.StatusBadRequest {
				t.Errorf("%s?%s: status = %d, want 400", path, q, rr.Code)
			}
		}
	}
}

func TestParseLatLonAcceptsValidCoordinates(t *testing.T) {
	rr := httptest.NewRecorder()
	lat, lon, ok := parseLatLon(rr, httptest.NewRequest(http.MethodGet, "/v1/weather?lat=-90&lon=180", nil))
	if !ok || lat != -90 || lon != 180 {
		t.Fatalf("parseLatLon = %v, %v, %v; want -90, 180, true", lat, lon, ok)
	}
}

func TestParseMapTemperatureRequestRejectsNaNBBox(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/map/precipitation/forecast?bbox=NaN,59,32,71&width=256&height=256", nil)
	if _, err := parseMapTemperatureRequest(req); err == nil {
		t.Fatal("expected an error for a NaN bbox edge")
	}
}
