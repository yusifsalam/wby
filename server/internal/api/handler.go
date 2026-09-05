package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"time"

	"wby/internal/weather"
)

type WeatherService interface {
	GetWeather(ctx context.Context, lat, lon float64) (*weather.WeatherResponse, error)
	GetTemperatureOverlay(ctx context.Context, req weather.MapOverlayRequest) (*weather.TemperatureOverlay, error)
	GetTemperatureSamples(ctx context.Context) (*weather.TemperatureSamplesResponse, error)
	GetTemperatureSamplesAt(ctx context.Context, at time.Time) (*weather.TemperatureSamplesResponse, error)
	GetPrecipitationOverlay(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationOverlay, error)
	GetPrecipitationForecastGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error)
	GetPrecipitationObservationGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error)
	GetPrecipitationNowcastGrid(ctx context.Context, req weather.PrecipitationOverlayRequest) (*weather.PrecipitationForecastGrid, error)
	GetClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64) (*weather.Station, float64, []weather.ClimateNormal, weather.InterpolatedNormal, error)
	GetDailyClimateNormals(ctx context.Context, lat, lon float64, currentTemp *float64, now time.Time) (*weather.DailyNormalsResult, error)
	GetLeaderboard(ctx context.Context, lat, lon float64, timeframe string) ([]weather.LeaderboardEntry, error)
}

type Handler struct {
	service WeatherService
}

func NewHandler(service WeatherService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/weather", h.getWeather)
	mux.HandleFunc("GET /v1/map/temperature", h.getTemperatureOverlay)
	mux.HandleFunc("GET /v1/map/temperature/samples", h.getTemperatureSamples)
	mux.HandleFunc("GET /v1/map/precipitation", h.getPrecipitationOverlay)
	mux.HandleFunc("GET /v1/map/precipitation/forecast", h.getPrecipitationForecastGrid)
	mux.HandleFunc("GET /v1/map/precipitation/observed", h.getPrecipitationObservedGrid)
	mux.HandleFunc("GET /v1/map/precipitation/nowcast", h.getPrecipitationNowcastGrid)
	mux.HandleFunc("GET /v1/map/frames", h.getMapFrames)
	mux.HandleFunc("GET /v1/climate-normals", h.getClimateNormals)
	mux.HandleFunc("GET /v1/climate-normals/daily", h.getDailyClimateNormals)
	mux.HandleFunc("GET /v1/leaderboard", h.getLeaderboard)
	mux.HandleFunc("GET /health", h.health)
}

type weatherJSON struct {
	Station  stationJSON          `json:"station"`
	Current  currentJSON          `json:"current"`
	Hourly   []hourlyForecastJSON `json:"hourly_forecast"`
	Forecast []dailyForecastJSON  `json:"daily_forecast"`
	Timezone string               `json:"timezone"`
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
	Date                       string   `json:"date"`
	High                       *float64 `json:"high"`
	Low                        *float64 `json:"low"`
	TempAvg                    *float64 `json:"temperature_avg"`
	Symbol                     *string  `json:"symbol"`
	WindSpeed                  *float64 `json:"wind_speed_avg"`
	WindDir                    *float64 `json:"wind_direction_avg"`
	Humidity                   *float64 `json:"humidity_avg"`
	PrecipMM                   *float64 `json:"precipitation_mm"`
	Precip1hSum                *float64 `json:"precipitation_1h_sum"`
	DewPointAvg                *float64 `json:"dew_point_avg"`
	FogIntensityAvg            *float64 `json:"fog_intensity_avg"`
	FrostProbabilityAvg        *float64 `json:"frost_probability_avg"`
	SevereFrostProbabilityAvg  *float64 `json:"severe_frost_probability_avg"`
	GeopHeightAvg              *float64 `json:"geop_height_avg"`
	PressureAvg                *float64 `json:"pressure_avg"`
	HighCloudCoverAvg          *float64 `json:"high_cloud_cover_avg"`
	LowCloudCoverAvg           *float64 `json:"low_cloud_cover_avg"`
	MediumCloudCoverAvg        *float64 `json:"medium_cloud_cover_avg"`
	MiddleAndLowCloudCoverAvg  *float64 `json:"middle_and_low_cloud_cover_avg"`
	TotalCloudCoverAvg         *float64 `json:"total_cloud_cover_avg"`
	HourlyMaximumGustMax       *float64 `json:"hourly_maximum_gust_max"`
	HourlyMaximumWindSpeedMax  *float64 `json:"hourly_maximum_wind_speed_max"`
	PoPAvg                     *float64 `json:"pop_avg"`
	ProbabilityThunderstormAvg *float64 `json:"probability_thunderstorm_avg"`
	PotentialPrecipitationForm *float64 `json:"potential_precipitation_form_mode"`
	PotentialPrecipitationType *float64 `json:"potential_precipitation_type_mode"`
	PrecipitationForm          *float64 `json:"precipitation_form_mode"`
	PrecipitationType          *float64 `json:"precipitation_type_mode"`
	RadiationGlobalAvg         *float64 `json:"radiation_global_avg"`
	RadiationLWAvg             *float64 `json:"radiation_lw_avg"`
	WeatherNumberMode          *float64 `json:"weather_number_mode"`
	WeatherSymbol3Mode         *float64 `json:"weather_symbol3_mode"`
	WindUMSAvg                 *float64 `json:"wind_ums_avg"`
	WindVMSAvg                 *float64 `json:"wind_vms_avg"`
	WindVectorMSAvg            *float64 `json:"wind_vector_ms_avg"`
	UVIndexAvg                 *float64 `json:"uv_index_avg"`
}

type hourlyForecastJSON struct {
	Time        time.Time `json:"time"`
	Temperature *float64  `json:"temperature"`
	FeelsLike   *float64  `json:"feels_like"`
	WindSpeed   *float64  `json:"wind_speed"`
	WindDir     *float64  `json:"wind_direction"`
	Humidity    *float64  `json:"humidity"`
	Precip1h    *float64  `json:"precipitation_1h"`
	Symbol      *string   `json:"symbol"`
	UVCumulated *float64  `json:"uv_cumulated"`
	WindGust    *float64  `json:"wind_gust"`
	Pressure    *float64  `json:"pressure"`
	CloudCover  *float64  `json:"cloud_cover"`
	PoP         *float64  `json:"pop"`
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
		if errors.Is(err, weather.ErrOutOfCoverage) {
			writeJSONError(w, "no weather coverage for this location", http.StatusNotFound)
			return
		}
		if isClientCanceled(r, err) {
			return
		}
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
			FeelsLike:       weather.FeelsLike(result.Current.Observation.Temperature, result.Current.Observation.WindSpeed),
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
			CloudCover:      oktasToPercent(result.Current.Observation.TotalCloudCover),
			WeatherCode:     result.Current.Observation.WeatherCode,
			Extra:           result.Current.Observation.ExtraNumericParams,
			ObservedAt:      result.Current.Observation.ObservedAt,
		},
		Timezone: result.Timezone,
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
			UVIndexAvg:                 f.UVIndexAvg,
		})
	}
	for _, hfc := range result.Hourly {
		resp.Hourly = append(resp.Hourly, hourlyForecastJSON{
			Time:        hfc.Time,
			Temperature: hfc.Temperature,
			FeelsLike:   hfc.FeelsLike,
			WindSpeed:   hfc.WindSpeed,
			WindDir:     hfc.WindDir,
			Humidity:    hfc.Humidity,
			Precip1h:    hfc.Precip1h,
			Symbol:      hfc.Symbol,
			UVCumulated: hfc.UVCumulated,
			WindGust:    hfc.WindGust,
			Pressure:    hfc.Pressure,
			CloudCover:  hfc.CloudCover,
			PoP:         hfc.PoP,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	json.NewEncoder(w).Encode(resp)
}

func writeJSONError(w http.ResponseWriter, msg string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// isClientCanceled reports whether the request's own context is done, i.e. the
// client closed the connection. A context error from an upstream call whose
// deadline elapsed while the client is still waiting is not a cancel.
func isClientCanceled(r *http.Request, err error) bool {
	return r.Context().Err() != nil
}

// upstreamStatus maps an upstream failure to a response status: 504 when the
// upstream call timed out, 502 otherwise.
func upstreamStatus(err error) int {
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	return http.StatusBadGateway
}

type climateNormalsJSON struct {
	Station stationJSON            `json:"station"`
	Period  string                 `json:"period"`
	Today   interpolatedNormalJSON `json:"today"`
	Monthly []monthlyNormalJSON    `json:"monthly"`
}

type interpolatedNormalJSON struct {
	TempAvg     *float64 `json:"temp_avg"`
	TempHigh    *float64 `json:"temp_high"`
	TempLow     *float64 `json:"temp_low"`
	PrecipMmDay *float64 `json:"precip_mm_day"`
	TempDiff    *float64 `json:"temp_diff"`
}

type monthlyNormalJSON struct {
	Month    int      `json:"month"`
	TempAvg  *float64 `json:"temp_avg"`
	TempHigh *float64 `json:"temp_high"`
	TempLow  *float64 `json:"temp_low"`
	PrecipMm *float64 `json:"precip_mm"`
}

type dailyNormalsJSON struct {
	Station       stationJSON             `json:"station"`
	HourlyStation *stationJSON            `json:"hourly_station,omitempty"`
	Period        string                  `json:"period"`
	Today         dailyNormalTodayJSON    `json:"today"`
	Precipitation precipitationToDateJSON `json:"precipitation"`
	Daily         []dailyNormalJSON       `json:"daily"`
}

type precipitationToDateJSON struct {
	Station               *stationJSON `json:"station"`
	TodayObservedMm       *float64     `json:"today_observed_mm"`
	TodayNormalMm         *float64     `json:"today_normal_mm"`
	MonthToDateObservedMm *float64     `json:"month_to_date_observed_mm"`
	MonthToDateNormalMm   *float64     `json:"month_to_date_normal_mm"`
	MonthNormalMm         *float64     `json:"month_normal_mm"`
	ObservedThrough       *time.Time   `json:"observed_through"`
}

type dailyNormalJSON struct {
	Month         int      `json:"month"`
	Day           int      `json:"day"`
	TempAvg       *float64 `json:"temp_avg"`
	TempHigh      *float64 `json:"temp_high"`
	TempLow       *float64 `json:"temp_low"`
	FeelsLikeAvg  *float64 `json:"feels_like_avg"`
	FeelsLikeHigh *float64 `json:"feels_like_high"`
	FeelsLikeLow  *float64 `json:"feels_like_low"`
	WindAvg       *float64 `json:"wind_avg"`
	WindGust      *float64 `json:"wind_gust"`
	HumidityAvg   *float64 `json:"humidity_avg"`
	PrecipMm      *float64 `json:"precip_mm"`
	PrecipDaysPct *float64 `json:"precip_days_pct"`
	SnowCm        *float64 `json:"snow_cm"`
}

type dailyNormalTodayJSON struct {
	dailyNormalJSON
	TempHourly         []float64 `json:"temp_hourly"`
	TempHourlyP10      []float64 `json:"temp_hourly_p10"`
	TempHourlyP90      []float64 `json:"temp_hourly_p90"`
	FeelsLikeHourly    []float64 `json:"feels_like_hourly"`
	WindHourly         []float64 `json:"wind_hourly"`
	HumidityHourly     []float64 `json:"humidity_hourly"`
	TempNowNormal      *float64  `json:"temp_now_normal"`
	TempDiff           *float64  `json:"temp_diff"`
	FeelsLikeNowNormal *float64  `json:"feels_like_now_normal"`
	WindNowNormal      *float64  `json:"wind_now_normal"`
	HumidityNowNormal  *float64  `json:"humidity_now_normal"`
}

func (h *Handler) getDailyClimateNormals(w http.ResponseWriter, r *http.Request) {
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
	var currentTemp *float64
	if ct := r.URL.Query().Get("current_temp"); ct != "" {
		if v, err := strconv.ParseFloat(ct, 64); err == nil {
			currentTemp = &v
		}
	}

	res, err := h.service.GetDailyClimateNormals(r.Context(), lat, lon, currentTemp, time.Now())
	if err != nil {
		slog.Error("daily climate normals", "err", err)
		writeJSONError(w, "failed to get daily climate normals", http.StatusInternalServerError)
		return
	}
	if res == nil {
		writeJSONError(w, "no daily climate normals available for this location", http.StatusNotFound)
		return
	}

	toJSON := func(n weather.DailyClimateNormal) dailyNormalJSON {
		return dailyNormalJSON{
			Month: n.Month, Day: n.Day,
			TempAvg: n.TempAvg, TempHigh: n.TempHigh, TempLow: n.TempLow,
			FeelsLikeAvg: n.FeelsLikeAvg, FeelsLikeHigh: n.FeelsLikeHigh, FeelsLikeLow: n.FeelsLikeLow,
			WindAvg: n.WindAvg, WindGust: n.WindGust, HumidityAvg: n.HumidityAvg,
			PrecipMm: n.PrecipMm, PrecipDaysPct: n.PrecipDaysPct, SnowCm: n.SnowCm,
		}
	}
	daily := make([]dailyNormalJSON, len(res.Daily))
	for i, n := range res.Daily {
		daily[i] = toJSON(n)
	}
	var precipStation *stationJSON
	if st := res.Precipitation.Station; st != nil {
		precipStation = &stationJSON{Name: st.Name, DistanceKM: res.Precipitation.StationDistanceKM}
	}
	var hourlyStation *stationJSON
	if st := res.HourlyStation; st != nil {
		hourlyStation = &stationJSON{Name: st.Name, DistanceKM: res.HourlyDistanceKM}
	}
	resp := dailyNormalsJSON{
		Station:       stationJSON{Name: res.Station.Name, DistanceKM: res.DistanceKM},
		HourlyStation: hourlyStation,
		Period:        res.Period,
		Today: dailyNormalTodayJSON{
			dailyNormalJSON:    toJSON(res.Today),
			TempHourly:         res.Today.TempHourly,
			TempHourlyP10:      res.Today.TempHourlyP10,
			TempHourlyP90:      res.Today.TempHourlyP90,
			FeelsLikeHourly:    res.Today.FeelsLikeHourly,
			WindHourly:         res.Today.WindHourly,
			HumidityHourly:     res.Today.HumidityHourly,
			TempNowNormal:      res.TempNowNormal,
			TempDiff:           res.TempDiff,
			FeelsLikeNowNormal: res.FeelsLikeNowNormal,
			WindNowNormal:      res.WindNowNormal,
			HumidityNowNormal:  res.HumidityNowNormal,
		},
		Precipitation: precipitationToDateJSON{
			Station:               precipStation,
			TodayObservedMm:       res.Precipitation.TodayObservedMm,
			TodayNormalMm:         res.Precipitation.TodayNormalMm,
			MonthToDateObservedMm: res.Precipitation.MonthToDateObservedMm,
			MonthToDateNormalMm:   res.Precipitation.MonthToDateNormalMm,
			MonthNormalMm:         res.Precipitation.MonthNormalMm,
			ObservedThrough:       res.Precipitation.ObservedThrough,
		},
		Daily: daily,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) getClimateNormals(w http.ResponseWriter, r *http.Request) {
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

	// Optionally accept current_temp for computing temp_diff
	var currentTemp *float64
	if ct := r.URL.Query().Get("current_temp"); ct != "" {
		if v, err := strconv.ParseFloat(ct, 64); err == nil {
			currentTemp = &v
		}
	}

	station, distKm, normals, today, err := h.service.GetClimateNormals(r.Context(), lat, lon, currentTemp)
	if err != nil {
		slog.Error("climate normals", "err", err)
		writeJSONError(w, "failed to get climate normals", http.StatusInternalServerError)
		return
	}

	if station == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "no climate normals available for this location"})
		return
	}

	monthly := make([]monthlyNormalJSON, len(normals))
	for i, n := range normals {
		monthly[i] = monthlyNormalJSON{
			Month:    n.Month,
			TempAvg:  n.TempAvg,
			TempHigh: n.TempHigh,
			TempLow:  n.TempLow,
			PrecipMm: n.PrecipMm,
		}
	}

	resp := climateNormalsJSON{
		Station: stationJSON{Name: station.Name, DistanceKM: distKm},
		Period:  "1991-2020",
		Today: interpolatedNormalJSON{
			TempAvg:     today.TempAvg,
			TempHigh:    today.TempHigh,
			TempLow:     today.TempLow,
			PrecipMmDay: today.PrecipMmDay,
			TempDiff:    today.TempDiff,
		},
		Monthly: monthly,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// oktasToPercent converts an FMI station cloud-cover observation (oktas,
// 0–8; 9 = sky obscured) to a percentage, matching the forecast fields.
func oktasToPercent(oktas *float64) *float64 {
	if oktas == nil {
		return nil
	}
	pct := math.Min(*oktas, 8) * 12.5
	return &pct
}
