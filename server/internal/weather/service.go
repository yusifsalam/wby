package weather

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

type WeatherStore interface {
	NearestStation(ctx context.Context, lat, lon float64) (Station, float64, error)
	LatestObservation(ctx context.Context, fmisid int) (Observation, error)
	GetForecasts(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error)
	UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error
	GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, limit int) ([]HourlyForecast, error)
	UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error
}

type ForecastFetcher interface {
	FetchForecast(ctx context.Context, lat, lon float64) ([]DailyForecast, error)
	FetchHourlyForecast(ctx context.Context, lat, lon float64, limit int) ([]HourlyForecast, error)
	FetchUVForecast(ctx context.Context, lat, lon float64) ([]UVDataPoint, error)
}

type Service struct {
	store         WeatherStore
	fmi           ForecastFetcher
	forecastCache *Cache[[]DailyForecast]
	hourlyCache   *Cache[[]HourlyForecast]
	uvCache       *Cache[[]UVDataPoint]
}

func NewService(store WeatherStore, fmiClient ForecastFetcher, forecastCacheTTL time.Duration) *Service {
	return &Service{
		store:         store,
		fmi:           fmiClient,
		forecastCache: NewCache[[]DailyForecast](forecastCacheTTL),
		hourlyCache:   NewCache[[]HourlyForecast](forecastCacheTTL),
		uvCache:       NewCache[[]UVDataPoint](forecastCacheTTL),
	}
}

func (s *Service) GetWeather(ctx context.Context, lat, lon float64) (*WeatherResponse, error) {
	station, distKM, err := s.store.NearestStation(ctx, lat, lon)
	if err != nil {
		return nil, fmt.Errorf("nearest station: %w", err)
	}

	obs, err := s.store.LatestObservation(ctx, station.FMISID)
	if err != nil {
		return nil, fmt.Errorf("latest observation: %w", err)
	}

	gridLat, gridLon := snapToGrid(lat, lon)
	forecast, err := s.getForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, fmt.Errorf("forecast: %w", err)
	}
	hourly, err := s.getHourlyForecast(ctx, gridLat, gridLon, 12)
	if err != nil {
		slog.Warn("hourly forecast unavailable", "err", err, "lat", gridLat, "lon", gridLon)
	}

	uvPoints := s.getUVData(ctx, gridLat, gridLon)
	if len(uvPoints) > 0 {
		applyUVToHourly(uvPoints, hourly)
		applyUVToDaily(uvPoints, forecast)
		if err := s.store.UpsertHourlyForecasts(ctx, gridLat, gridLon, hourly); err != nil {
			slog.Warn("failed to persist UV-enriched hourly forecasts", "err", err)
		}
		if err := s.store.UpsertForecasts(ctx, forecast); err != nil {
			slog.Warn("failed to persist UV-enriched daily forecasts", "err", err)
		}
	}

	return &WeatherResponse{
		Current: CurrentWeather{
			Station:     station,
			DistanceKM:  distKM,
			Observation: obs,
		},
		Hourly:   hourly,
		Forecast: forecast,
	}, nil
}

func (s *Service) getForecast(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f", gridLat, gridLon)

	if cached, ok := s.forecastCache.Get(cacheKey); ok {
		if hasExpandedForecastData(cached) {
			return cached, nil
		}
	}

	forecasts, err := s.store.GetForecasts(ctx, gridLat, gridLon)
	if err == nil && len(forecasts) > 0 && isFresh(forecasts, 3*time.Hour) && hasExpandedForecastData(forecasts) {
		s.forecastCache.Set(cacheKey, forecasts)
		return forecasts, nil
	}

	forecasts, err = s.fmi.FetchForecast(ctx, gridLat, gridLon)
	if err != nil {
		return nil, err
	}

	if storeErr := s.store.UpsertForecasts(ctx, forecasts); storeErr != nil {
		slog.Warn("failed to store forecasts", "err", storeErr)
	}
	s.forecastCache.Set(cacheKey, forecasts)

	return forecasts, nil
}

func (s *Service) getHourlyForecast(ctx context.Context, gridLat, gridLon float64, limit int) ([]HourlyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f:%d", gridLat, gridLon, limit)
	if cached, ok := s.hourlyCache.Get(cacheKey); ok {
		return cached, nil
	}

	persistedHourly, storeErr := s.store.GetHourlyForecasts(ctx, gridLat, gridLon, limit)
	if storeErr == nil && len(persistedHourly) > 0 && isHourlyFresh(persistedHourly, 90*time.Minute) {
		s.hourlyCache.Set(cacheKey, persistedHourly)
		return persistedHourly, nil
	}

	hourly, err := s.fmi.FetchHourlyForecast(ctx, gridLat, gridLon, limit)
	if err != nil {
		if len(persistedHourly) > 0 {
			slog.Warn("using stale persisted hourly forecast", "err", err, "lat", gridLat, "lon", gridLon)
			s.hourlyCache.Set(cacheKey, persistedHourly)
			return persistedHourly, nil
		}
		return nil, err
	}

	fetchedAt := time.Now()
	for i := range hourly {
		hourly[i].FetchedAt = fetchedAt
	}

	if upsertErr := s.store.UpsertHourlyForecasts(ctx, gridLat, gridLon, hourly); upsertErr != nil {
		slog.Warn("failed to store hourly forecasts", "err", upsertErr)
	}
	s.hourlyCache.Set(cacheKey, hourly)
	return hourly, nil
}

func snapToGrid(lat, lon float64) (float64, float64) {
	return math.Round(lat*100) / 100, math.Round(lon*100) / 100
}

func isFresh(forecasts []DailyForecast, maxAge time.Duration) bool {
	oldest := forecasts[0].FetchedAt
	for _, f := range forecasts[1:] {
		if f.FetchedAt.Before(oldest) {
			oldest = f.FetchedAt
		}
	}
	return time.Since(oldest) < maxAge
}

func hasExpandedForecastData(forecasts []DailyForecast) bool {
	for _, f := range forecasts {
		// TempAvg is derived from Temperature and should be present for any day with temp data.
		// If it's nil across all days, rows are likely from pre-migration/pre-rollout data.
		if f.TempAvg != nil {
			return true
		}
	}
	return false
}

func (s *Service) getUVData(ctx context.Context, gridLat, gridLon float64) []UVDataPoint {
	cacheKey := fmt.Sprintf("uv:%.2f,%.2f", gridLat, gridLon)
	if cached, ok := s.uvCache.Get(cacheKey); ok {
		return cached
	}

	points, err := s.fmi.FetchUVForecast(ctx, gridLat, gridLon)
	if err != nil {
		slog.Warn("UV forecast fetch failed", "err", err)
		return nil
	}
	slog.Info("fetched UV forecast from FMI", "lat", gridLat, "lon", gridLon, "points", len(points), "data", points)
	if len(points) > 0 {
		s.uvCache.Set(cacheKey, points)
	}
	return points
}

func applyUVToHourly(uvPoints []UVDataPoint, hourly []HourlyForecast) {
	uvByHour := make(map[int64]float64, len(uvPoints))
	for _, p := range uvPoints {
		uvByHour[p.Time.Truncate(time.Hour).Unix()] = p.UVCumulated
	}
	for i := range hourly {
		if uv, ok := uvByHour[hourly[i].Time.Truncate(time.Hour).Unix()]; ok {
			hourly[i].UVCumulated = &uv
		}
	}
}

func applyUVToDaily(uvPoints []UVDataPoint, forecasts []DailyForecast) {
	type dailyUV struct {
		sum   float64
		count int
	}
	byDate := make(map[string]*dailyUV)
	for _, p := range uvPoints {
		date := p.Time.UTC().Format("2006-01-02")
		d, ok := byDate[date]
		if !ok {
			d = &dailyUV{}
			byDate[date] = d
		}
		d.sum += p.UVCumulated
		d.count++
	}
	for i := range forecasts {
		date := forecasts[i].Date.UTC().Format("2006-01-02")
		if d, ok := byDate[date]; ok && d.count > 0 {
			avg := d.sum / float64(d.count)
			forecasts[i].UVIndexAvg = &avg
		}
	}
}

func isHourlyFresh(hourly []HourlyForecast, maxAge time.Duration) bool {
	oldest := hourly[0].FetchedAt
	if oldest.IsZero() {
		return false
	}
	for _, h := range hourly[1:] {
		if h.FetchedAt.IsZero() {
			return false
		}
		if h.FetchedAt.Before(oldest) {
			oldest = h.FetchedAt
		}
	}
	return time.Since(oldest) < maxAge
}
