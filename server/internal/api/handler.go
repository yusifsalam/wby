package api

import (
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"wby/internal/weather"
)

type Handler struct {
	service *weather.Service
}

func NewHandler(service *weather.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/weather", h.getWeather)
	mux.HandleFunc("GET /health", h.health)
}

type weatherJSON struct {
	Station  stationJSON         `json:"station"`
	Current  currentJSON         `json:"current"`
	Forecast []dailyForecastJSON `json:"daily_forecast"`
}

type stationJSON struct {
	Name       string  `json:"name"`
	DistanceKM float64 `json:"distance_km"`
}

type currentJSON struct {
	Temperature *float64  `json:"temperature"`
	FeelsLike   *float64  `json:"feels_like"`
	WindSpeed   *float64  `json:"wind_speed"`
	WindDir     *float64  `json:"wind_direction"`
	Humidity    *float64  `json:"humidity"`
	Pressure    *float64  `json:"pressure"`
	ObservedAt  time.Time `json:"observed_at"`
}

type dailyForecastJSON struct {
	Date      string   `json:"date"`
	High      *float64 `json:"high"`
	Low       *float64 `json:"low"`
	Symbol    *string  `json:"symbol"`
	WindSpeed *float64 `json:"wind_speed_avg"`
	PrecipMM  *float64 `json:"precipitation_mm"`
}

func (h *Handler) getWeather(w http.ResponseWriter, r *http.Request) {
	lat, err := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	if err != nil {
		writeJSONError(w, "invalid lat parameter", http.StatusBadRequest)
		return
	}
	lon, err := strconv.ParseFloat(r.URL.Query().Get("lon"), 64)
	if err != nil {
		writeJSONError(w, "invalid lon parameter", http.StatusBadRequest)
		return
	}

	result, err := h.service.GetWeather(r.Context(), lat, lon)
	if err != nil {
		slog.Error("get weather failed", "err", err, "lat", lat, "lon", lon)
		writeJSONError(w, "internal server error", http.StatusInternalServerError)
		return
	}

	resp := weatherJSON{
		Station: stationJSON{
			Name:       result.Current.Station.Name,
			DistanceKM: result.Current.DistanceKM,
		},
		Current: currentJSON{
			Temperature: result.Current.Observation.Temperature,
			FeelsLike:   computeFeelsLike(result.Current.Observation.Temperature, result.Current.Observation.WindSpeed),
			WindSpeed:   result.Current.Observation.WindSpeed,
			WindDir:     result.Current.Observation.WindDir,
			Humidity:    result.Current.Observation.Humidity,
			Pressure:    result.Current.Observation.Pressure,
			ObservedAt:  result.Current.Observation.ObservedAt,
		},
	}

	for _, f := range result.Forecast {
		resp.Forecast = append(resp.Forecast, dailyForecastJSON{
			Date:      f.Date.Format("2006-01-02"),
			High:      f.TempHigh,
			Low:       f.TempLow,
			Symbol:    f.Symbol,
			WindSpeed: f.WindSpeed,
			PrecipMM:  f.PrecipMM,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(resp)
}

func computeFeelsLike(temp, wind *float64) *float64 {
	if temp == nil || wind == nil {
		return temp
	}
	t := *temp
	w := *wind * 3.6
	if t > 10 || w < 4.8 {
		return temp
	}
	fl := 13.12 + 0.6215*t - 11.37*math.Pow(w, 0.16) + 0.3965*t*math.Pow(w, 0.16)
	return &fl
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
