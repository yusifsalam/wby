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
	Station  stationJSON          `json:"station"`
	Current  currentJSON          `json:"current"`
	Hourly   []hourlyForecastJSON `json:"hourly_forecast"`
	Forecast []dailyForecastJSON  `json:"daily_forecast"`
}

type stationJSON struct {
	Name       string  `json:"name"`
	DistanceKM float64 `json:"distance_km"`
}

type currentJSON struct {
	Temperature     *float64           `json:"temperature"`
	FeelsLike       *float64           `json:"feels_like"`
	WindSpeed       *float64           `json:"wind_speed"`
	WindGust        *float64           `json:"wind_gust"`
	WindDir         *float64           `json:"wind_direction"`
	Humidity        *float64           `json:"humidity"`
	DewPoint        *float64           `json:"dew_point"`
	Pressure        *float64           `json:"pressure"`
	Precip1h        *float64           `json:"precipitation_1h"`
	PrecipIntensity *float64           `json:"precipitation_intensity"`
	SnowDepth       *float64           `json:"snow_depth"`
	Visibility      *float64           `json:"visibility"`
	CloudCover      *float64           `json:"cloud_cover"`
	WeatherCode     *float64           `json:"weather_code"`
	Extra           map[string]float64 `json:"extra,omitempty"`
	ObservedAt      time.Time          `json:"observed_at"`
}

type dailyForecastJSON struct {
	Date                          string   `json:"date"`
	High                          *float64 `json:"high"`
	Low                           *float64 `json:"low"`
	TempAvg                       *float64 `json:"temperature_avg"`
	Symbol                        *string  `json:"symbol"`
	WindSpeed                     *float64 `json:"wind_speed_avg"`
	WindDir                       *float64 `json:"wind_direction_avg"`
	Humidity                      *float64 `json:"humidity_avg"`
	PrecipMM                      *float64 `json:"precipitation_mm"`
	Precip1hSum                   *float64 `json:"precipitation_1h_sum"`
	DewPointAvg                   *float64 `json:"dew_point_avg"`
	FogIntensityAvg               *float64 `json:"fog_intensity_avg"`
	FrostProbabilityAvg           *float64 `json:"frost_probability_avg"`
	SevereFrostProbabilityAvg     *float64 `json:"severe_frost_probability_avg"`
	GeopHeightAvg                 *float64 `json:"geop_height_avg"`
	PressureAvg                   *float64 `json:"pressure_avg"`
	HighCloudCoverAvg             *float64 `json:"high_cloud_cover_avg"`
	LowCloudCoverAvg              *float64 `json:"low_cloud_cover_avg"`
	MediumCloudCoverAvg           *float64 `json:"medium_cloud_cover_avg"`
	MiddleAndLowCloudCoverAvg     *float64 `json:"middle_and_low_cloud_cover_avg"`
	TotalCloudCoverAvg            *float64 `json:"total_cloud_cover_avg"`
	HourlyMaximumGustMax          *float64 `json:"hourly_maximum_gust_max"`
	HourlyMaximumWindSpeedMax     *float64 `json:"hourly_maximum_wind_speed_max"`
	PoPAvg                        *float64 `json:"pop_avg"`
	ProbabilityThunderstormAvg    *float64 `json:"probability_thunderstorm_avg"`
	PotentialPrecipitationForm    *float64 `json:"potential_precipitation_form_mode"`
	PotentialPrecipitationType    *float64 `json:"potential_precipitation_type_mode"`
	PrecipitationForm             *float64 `json:"precipitation_form_mode"`
	PrecipitationType             *float64 `json:"precipitation_type_mode"`
	RadiationGlobalAvg            *float64 `json:"radiation_global_avg"`
	RadiationLWAvg                *float64 `json:"radiation_lw_avg"`
	WeatherNumberMode             *float64 `json:"weather_number_mode"`
	WeatherSymbol3Mode            *float64 `json:"weather_symbol3_mode"`
	WindUMSAvg                    *float64 `json:"wind_ums_avg"`
	WindVMSAvg                    *float64 `json:"wind_vms_avg"`
	WindVectorMSAvg               *float64 `json:"wind_vector_ms_avg"`
}

type hourlyForecastJSON struct {
	Time        time.Time `json:"time"`
	Temperature *float64  `json:"temperature"`
	WindSpeed   *float64  `json:"wind_speed"`
	WindDir     *float64  `json:"wind_direction"`
	Humidity    *float64  `json:"humidity"`
	Precip1h    *float64  `json:"precipitation_1h"`
	Symbol      *string   `json:"symbol"`
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
			Temperature:     result.Current.Observation.Temperature,
			FeelsLike:       computeFeelsLike(result.Current.Observation.Temperature, result.Current.Observation.WindSpeed),
			WindSpeed:       result.Current.Observation.WindSpeed,
			WindGust:        result.Current.Observation.WindGust,
			WindDir:         result.Current.Observation.WindDir,
			Humidity:        result.Current.Observation.Humidity,
			DewPoint:        result.Current.Observation.DewPoint,
			Pressure:        result.Current.Observation.Pressure,
			Precip1h:        result.Current.Observation.Precip1h,
			PrecipIntensity: result.Current.Observation.PrecipIntensity,
			SnowDepth:       result.Current.Observation.SnowDepth,
			Visibility:      result.Current.Observation.Visibility,
			CloudCover:      result.Current.Observation.TotalCloudCover,
			WeatherCode:     result.Current.Observation.WeatherCode,
			Extra:           result.Current.Observation.ExtraNumericParams,
			ObservedAt:      result.Current.Observation.ObservedAt,
		},
	}

	for _, f := range result.Forecast {
		resp.Forecast = append(resp.Forecast, dailyForecastJSON{
			Date:                       f.Date.Format("2006-01-02"),
			High:                       f.TempHigh,
			Low:                        f.TempLow,
			TempAvg:                    f.TempAvg,
			Symbol:                     f.Symbol,
			WindSpeed:                  f.WindSpeed,
			WindDir:                    f.WindDir,
			Humidity:                   f.HumidityAvg,
			PrecipMM:                   f.PrecipMM,
			Precip1hSum:                f.Precip1hSum,
			DewPointAvg:                f.DewPointAvg,
			FogIntensityAvg:            f.FogIntensityAvg,
			FrostProbabilityAvg:        f.FrostProbabilityAvg,
			SevereFrostProbabilityAvg:  f.SevereFrostProbabilityAvg,
			GeopHeightAvg:              f.GeopHeightAvg,
			PressureAvg:                f.PressureAvg,
			HighCloudCoverAvg:          f.HighCloudCoverAvg,
			LowCloudCoverAvg:           f.LowCloudCoverAvg,
			MediumCloudCoverAvg:        f.MediumCloudCoverAvg,
			MiddleAndLowCloudCoverAvg:  f.MiddleAndLowCloudCoverAvg,
			TotalCloudCoverAvg:         f.TotalCloudCoverAvg,
			HourlyMaximumGustMax:       f.HourlyMaximumGustMax,
			HourlyMaximumWindSpeedMax:  f.HourlyMaximumWindSpeedMax,
			PoPAvg:                     f.PoPAvg,
			ProbabilityThunderstormAvg: f.ProbabilityThunderstormAvg,
			PotentialPrecipitationForm: f.PotentialPrecipitationFormMode,
			PotentialPrecipitationType: f.PotentialPrecipitationTypeMode,
			PrecipitationForm:          f.PrecipitationFormMode,
			PrecipitationType:          f.PrecipitationTypeMode,
			RadiationGlobalAvg:         f.RadiationGlobalAvg,
			RadiationLWAvg:             f.RadiationLWAvg,
			WeatherNumberMode:          f.WeatherNumberMode,
			WeatherSymbol3Mode:         f.WeatherSymbol3Mode,
			WindUMSAvg:                 f.WindUMSAvg,
			WindVMSAvg:                 f.WindVMSAvg,
			WindVectorMSAvg:            f.WindVectorMSAvg,
		})
	}
	for _, hfc := range result.Hourly {
		resp.Hourly = append(resp.Hourly, hourlyForecastJSON{
			Time:        hfc.Time,
			Temperature: hfc.Temperature,
			WindSpeed:   hfc.WindSpeed,
			WindDir:     hfc.WindDir,
			Humidity:    hfc.Humidity,
			Precip1h:    hfc.Precip1h,
			Symbol:      hfc.Symbol,
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
