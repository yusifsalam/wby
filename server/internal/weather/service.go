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
}

type ForecastFetcher interface {
	FetchForecast(ctx context.Context, lat, lon float64) ([]DailyForecast, error)
}

type Service struct {
	store         WeatherStore
	fmi           ForecastFetcher
	forecastCache *Cache[[]DailyForecast]
}

func NewService(store WeatherStore, fmiClient ForecastFetcher, forecastCacheTTL time.Duration) *Service {
	return &Service{
		store:         store,
		fmi:           fmiClient,
		forecastCache: NewCache[[]DailyForecast](forecastCacheTTL),
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

	return &WeatherResponse{
		Current: CurrentWeather{
			Station:     station,
			DistanceKM:  distKM,
			Observation: obs,
		},
		Forecast: forecast,
	}, nil
}

func (s *Service) getForecast(ctx context.Context, gridLat, gridLon float64) ([]DailyForecast, error) {
	cacheKey := fmt.Sprintf("%.2f,%.2f", gridLat, gridLon)

	if cached, ok := s.forecastCache.Get(cacheKey); ok {
		return cached, nil
	}

	forecasts, err := s.store.GetForecasts(ctx, gridLat, gridLon)
	if err == nil && len(forecasts) > 0 && isFresh(forecasts, 3*time.Hour) {
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
