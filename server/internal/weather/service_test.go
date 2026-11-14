package weather

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"
)

type fakeStore struct {
	WeatherStore
	station Station
}

func (f *fakeStore) NearestStations(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]StationDistance, error) {
	return []StationDistance{{Station: f.station, DistanceKM: 1}}, nil
}

func (f *fakeStore) LatestObservations(ctx context.Context, fmisids []int) (map[int]Observation, error) {
	temp := 4.0
	return map[int]Observation{f.station.FMISID: {FMISID: f.station.FMISID, ObservedAt: time.Now(), Temperature: &temp}}, nil
}

func (f *fakeStore) ObservedTemperatureRange(ctx context.Context, fmisid int, from, to time.Time) (*float64, *float64, error) {
	return nil, nil, nil
}

func (f *fakeStore) GetForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time) ([]DailyForecast, error) {
	return nil, nil
}

func (f *fakeStore) UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error {
	return nil
}

func (f *fakeStore) GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time, limit int) ([]HourlyForecast, error) {
	return nil, nil
}

func (f *fakeStore) UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error {
	return nil
}

type fakeForecasts struct{}

func (fakeForecasts) FetchForecast(ctx context.Context, lat, lon float64) (ForecastData, error) {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	temp := 5.0
	var days []DailyForecast
	for i := range 3 {
		days = append(days, DailyForecast{GridLat: lat, GridLon: lon, Date: today.AddDate(0, 0, i), FetchedAt: time.Now(), TempAvg: &temp})
	}
	return ForecastData{Forecasts: days, Timezone: DefaultPlaceTimezone}, nil
}

func (fakeForecasts) FetchHourlyForecast(ctx context.Context, lat, lon float64, start time.Time, hours int) ([]HourlyForecast, error) {
	var out []HourlyForecast
	for i := range 48 {
		out = append(out, HourlyForecast{Time: start.UTC().Add(time.Duration(i) * time.Hour)})
	}
	return out, nil
}

func (fakeForecasts) FetchUVForecast(ctx context.Context, lat, lon float64, start time.Time) ([]UVDataPoint, error) {
	var out []UVDataPoint
	for i := range 48 {
		out = append(out, UVDataPoint{Time: start.UTC().Add(time.Duration(i) * time.Hour), UVCumulated: 2})
	}
	return out, nil
}

func TestGetWeatherDoesNotMutateCachedForecasts(t *testing.T) {
	svc := NewService(&fakeStore{station: Station{FMISID: 100971, Name: "Helsinki Kaisaniemi"}}, fakeForecasts{}, time.Minute)
	ctx := context.Background()
	const lat, lon = 60.17, 24.94

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				res, err := svc.GetWeather(ctx, lat, lon)
				if err != nil {
					t.Error(err)
					return
				}
				if len(res.Hourly) == 0 || res.Hourly[0].UVCumulated == nil {
					t.Error("hourly forecast in the response lacks UV")
					return
				}
				if len(res.Forecast) == 0 || res.Forecast[0].UVIndexAvg == nil {
					t.Error("daily forecast in the response lacks UV")
					return
				}
				if _, err := svc.GetHourlyForecast(ctx, lat, lon); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	hourly, ok := svc.hourlyCache.Get("60.17,24.94")
	if !ok {
		t.Fatal("expected a cached hourly forecast")
	}
	for _, h := range hourly {
		if h.UVCumulated != nil {
			t.Fatal("cached hourly forecast was mutated with UV")
		}
	}
	daily, ok := svc.forecastCache.Get("60.17,24.94")
	if !ok {
		t.Fatal("expected a cached daily forecast")
	}
	for _, d := range daily {
		if d.UVIndexAvg != nil {
			t.Fatal("cached daily forecast was mutated with UV")
		}
	}
}

func TestNonFiniteAndForeignCoordinatesAreOutOfCoverage(t *testing.T) {
	svc := NewService(&fakeStore{}, fakeForecasts{}, time.Minute)
	ctx := context.Background()
	points := [][2]float64{
		{math.NaN(), 24.94},
		{60.17, math.NaN()},
		{math.Inf(1), 24.94},
		{48.85, 2.35},
	}
	for _, p := range points {
		lat, lon := p[0], p[1]
		if _, err := svc.GetWeather(ctx, lat, lon); !errors.Is(err, ErrOutOfCoverage) {
			t.Errorf("GetWeather(%v, %v) error = %v, want ErrOutOfCoverage", lat, lon, err)
		}
		if _, err := svc.GetHourlyForecast(ctx, lat, lon); !errors.Is(err, ErrOutOfCoverage) {
			t.Errorf("GetHourlyForecast(%v, %v) error = %v, want ErrOutOfCoverage", lat, lon, err)
		}
		if res, err := svc.GetDailyClimateNormals(ctx, lat, lon, nil, time.Now()); res != nil || err != nil {
			t.Errorf("GetDailyClimateNormals(%v, %v) = %v, %v; want nil, nil", lat, lon, res, err)
		}
		if st, _, _, _, err := svc.GetClimateNormals(ctx, lat, lon, nil); st != nil || err != nil {
			t.Errorf("GetClimateNormals(%v, %v) = %v, %v; want nil, nil", lat, lon, st, err)
		}
	}
}
