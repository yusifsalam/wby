package weather

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStore struct {
	WeatherStore
	station      Station
	observedAt   time.Time
	noCandidates bool
	daily        []DailyForecast
	hourly       []HourlyForecast

	dailyUpserts, hourlyUpserts atomic.Int32
}

func (f *fakeStore) observation() Observation {
	at := f.observedAt
	if at.IsZero() {
		at = time.Now()
	}
	temp := 4.0
	return Observation{FMISID: f.station.FMISID, ObservedAt: at, Temperature: &temp}
}

func (f *fakeStore) NearestStations(ctx context.Context, lat, lon, radiusKm float64, limit int) ([]StationDistance, error) {
	if f.noCandidates {
		return nil, nil
	}
	return []StationDistance{{Station: f.station, DistanceKM: 1}}, nil
}

func (f *fakeStore) LatestObservations(ctx context.Context, fmisids []int) (map[int]Observation, error) {
	return map[int]Observation{f.station.FMISID: f.observation()}, nil
}

func (f *fakeStore) NearestStation(ctx context.Context, lat, lon float64) (Station, float64, error) {
	return f.station, 80, nil
}

func (f *fakeStore) LatestObservation(ctx context.Context, fmisid int) (Observation, error) {
	return f.observation(), nil
}

func (f *fakeStore) ObservedTemperatureRange(ctx context.Context, fmisid int, from, to time.Time) (*float64, *float64, error) {
	return nil, nil, nil
}

func (f *fakeStore) GetForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time) ([]DailyForecast, error) {
	return f.daily, nil
}

func (f *fakeStore) UpsertForecasts(ctx context.Context, forecasts []DailyForecast) error {
	f.dailyUpserts.Add(1)
	return nil
}

func (f *fakeStore) GetHourlyForecasts(ctx context.Context, gridLat, gridLon float64, from time.Time, limit int) ([]HourlyForecast, error) {
	return f.hourly, nil
}

func (f *fakeStore) UpsertHourlyForecasts(ctx context.Context, gridLat, gridLon float64, hourly []HourlyForecast) error {
	f.hourlyUpserts.Add(1)
	return nil
}

type fakeFMI struct {
	delay                         time.Duration
	forecastErr, hourlyErr, uvErr error
	noUV                          bool

	forecastCalls, hourlyCalls, uvCalls atomic.Int32
}

func testDailyForecasts(lat, lon float64, fetchedAt time.Time) []DailyForecast {
	today := time.Now().UTC().Truncate(24 * time.Hour)
	temp := 5.0
	var days []DailyForecast
	for i := range 3 {
		days = append(days, DailyForecast{GridLat: lat, GridLon: lon, Date: today.AddDate(0, 0, i), FetchedAt: fetchedAt, TempAvg: &temp})
	}
	return days
}

func testHourlyForecasts(start time.Time, fetchedAt time.Time) []HourlyForecast {
	var out []HourlyForecast
	for i := range 48 {
		out = append(out, HourlyForecast{Time: start.UTC().Add(time.Duration(i) * time.Hour), FetchedAt: fetchedAt})
	}
	return out
}

func (f *fakeFMI) FetchForecast(ctx context.Context, lat, lon float64) (ForecastData, error) {
	f.forecastCalls.Add(1)
	time.Sleep(f.delay)
	if f.forecastErr != nil {
		return ForecastData{}, f.forecastErr
	}
	return ForecastData{Forecasts: testDailyForecasts(lat, lon, time.Now()), Timezone: DefaultPlaceTimezone}, nil
}

func (f *fakeFMI) FetchHourlyForecast(ctx context.Context, lat, lon float64, start time.Time, hours int) ([]HourlyForecast, error) {
	f.hourlyCalls.Add(1)
	time.Sleep(f.delay)
	if f.hourlyErr != nil {
		return nil, f.hourlyErr
	}
	return testHourlyForecasts(start, time.Time{}), nil
}

func (f *fakeFMI) FetchUVForecast(ctx context.Context, lat, lon float64, start time.Time) ([]UVDataPoint, error) {
	f.uvCalls.Add(1)
	time.Sleep(f.delay)
	if f.uvErr != nil {
		return nil, f.uvErr
	}
	if f.noUV {
		return nil, nil
	}
	var out []UVDataPoint
	for i := range 48 {
		out = append(out, UVDataPoint{Time: start.UTC().Add(time.Duration(i) * time.Hour), UVCumulated: 2})
	}
	return out, nil
}

var testStation = Station{FMISID: 100971, Name: "Helsinki Kaisaniemi"}

const testLat, testLon = 60.17, 24.94

func TestGetWeatherDoesNotMutateCachedForecasts(t *testing.T) {
	svc := NewService(&fakeStore{station: testStation}, &fakeFMI{}, time.Minute)
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				res, err := svc.GetWeather(ctx, testLat, testLon)
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
				if _, err := svc.GetHourlyForecast(ctx, testLat, testLon); err != nil {
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
	svc := NewService(&fakeStore{}, &fakeFMI{}, time.Minute)
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

func TestConcurrentColdRequestsFetchEachUpstreamOnce(t *testing.T) {
	store := &fakeStore{station: testStation}
	fmi := &fakeFMI{delay: 50 * time.Millisecond}
	svc := NewService(store, fmi, time.Minute)

	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := svc.GetWeather(context.Background(), testLat, testLon)
			if err != nil {
				t.Error(err)
				return
			}
			if len(res.Forecast) != 3 || len(res.UV) == 0 {
				t.Errorf("got %d forecast days and %d UV points, want 3 and some", len(res.Forecast), len(res.UV))
			}
		}()
	}
	wg.Wait()

	if n := fmi.forecastCalls.Load(); n != 1 {
		t.Errorf("daily forecast fetched %d times, want 1", n)
	}
	if n := fmi.hourlyCalls.Load(); n != 1 {
		t.Errorf("hourly forecast fetched %d times, want 1", n)
	}
	if n := fmi.uvCalls.Load(); n != 1 {
		t.Errorf("UV forecast fetched %d times, want 1", n)
	}
	if n := store.dailyUpserts.Load(); n != 2 {
		t.Errorf("daily forecast stored %d times, want 2 (the fetch, then once with UV)", n)
	}
	if n := store.hourlyUpserts.Load(); n != 2 {
		t.Errorf("hourly forecast stored %d times, want 2 (the fetch, then once with UV)", n)
	}
}

func TestGetWeatherServesStaleForecastsWhenFMIFails(t *testing.T) {
	fetchedAt := time.Now().Add(-5 * time.Hour).Truncate(time.Second)
	store := &fakeStore{
		station: testStation,
		daily:   testDailyForecasts(testLat, testLon, fetchedAt),
		hourly:  testHourlyForecasts(time.Now().Truncate(time.Hour), fetchedAt),
	}
	down := errors.New("FMI returned 503")
	fmi := &fakeFMI{forecastErr: down, hourlyErr: down, uvErr: down}
	svc := NewService(store, fmi, time.Minute)

	for range 2 {
		res, err := svc.GetWeather(context.Background(), testLat, testLon)
		if err != nil {
			t.Fatalf("GetWeather failed although stale forecasts are stored: %v", err)
		}
		if len(res.Forecast) != 3 || len(res.Hourly) == 0 {
			t.Fatalf("got %d forecast days and %d hours, want the stored ones", len(res.Forecast), len(res.Hourly))
		}
		if !res.ForecastFetchedAt.Equal(fetchedAt) {
			t.Errorf("ForecastFetchedAt = %v, want %v", res.ForecastFetchedAt, fetchedAt)
		}
		if len(res.UV) != 0 {
			t.Errorf("got %d UV points from a failed fetch", len(res.UV))
		}
	}
	if n := fmi.forecastCalls.Load(); n != 1 {
		t.Errorf("daily forecast fetch attempted %d times, want 1 within the retry interval", n)
	}
	if n := fmi.hourlyCalls.Load(); n != 1 {
		t.Errorf("hourly forecast fetch attempted %d times, want 1 within the retry interval", n)
	}
	if n := fmi.uvCalls.Load(); n != 1 {
		t.Errorf("UV fetch attempted %d times, want 1 within the retry interval", n)
	}
}

func TestGetWeatherFailsWithoutAnyForecast(t *testing.T) {
	fmi := &fakeFMI{forecastErr: errors.New("FMI returned 503")}
	svc := NewService(&fakeStore{station: testStation}, fmi, time.Minute)
	if _, err := svc.GetWeather(context.Background(), testLat, testLon); err == nil {
		t.Fatal("expected an error with FMI down and nothing stored")
	}
}

func TestEmptyUVForecastIsNotRefetchedEveryRequest(t *testing.T) {
	fmi := &fakeFMI{noUV: true}
	svc := NewService(&fakeStore{station: testStation}, fmi, time.Minute)
	for range 3 {
		if _, err := svc.GetWeather(context.Background(), testLat, testLon); err != nil {
			t.Fatal(err)
		}
	}
	if n := fmi.uvCalls.Load(); n != 1 {
		t.Errorf("UV fetched %d times, want 1 within the retry interval", n)
	}
}

func TestGetHourlyForecastSurvivesDailyForecastFailure(t *testing.T) {
	fmi := &fakeFMI{forecastErr: errors.New("FMI returned 503")}
	svc := NewService(&fakeStore{station: testStation}, fmi, time.Minute)

	before := time.Now()
	res, err := svc.GetHourlyForecast(context.Background(), testLat, testLon)
	if err != nil {
		t.Fatalf("GetHourlyForecast failed on a daily forecast error: %v", err)
	}
	if len(res.Hourly) == 0 || res.Timezone != DefaultPlaceTimezone {
		t.Fatalf("got %d hours in %q, want some in %q", len(res.Hourly), res.Timezone, DefaultPlaceTimezone)
	}
	if res.FetchedAt.Before(before) {
		t.Errorf("FetchedAt = %v, want the time of this fetch", res.FetchedAt)
	}
}

func TestCurrentConditionsOlderThanExpiryCarryNoValues(t *testing.T) {
	observedAt := time.Now().Add(-5 * time.Hour).Truncate(time.Second)
	for _, noCandidates := range []bool{false, true} {
		store := &fakeStore{station: testStation, observedAt: observedAt, noCandidates: noCandidates}
		svc := NewService(store, &fakeFMI{}, time.Minute)

		res, err := svc.GetWeather(context.Background(), testLat, testLon)
		if err != nil {
			t.Fatal(err)
		}
		obs := res.Current.Observation
		if obs.Temperature != nil {
			t.Errorf("noCandidates=%v: temperature %v from a 5-hour-old observation served as current", noCandidates, *obs.Temperature)
		}
		if !obs.ObservedAt.Equal(observedAt) || res.Current.Station.FMISID != testStation.FMISID {
			t.Errorf("noCandidates=%v: station %d observed at %v, want %d at %v",
				noCandidates, res.Current.Station.FMISID, obs.ObservedAt, testStation.FMISID, observedAt)
		}
	}
}

type fakePrecipHistory struct {
	fakeFMI
	err   error
	calls atomic.Int32
}

func (f *fakePrecipHistory) FetchHourlyPrecipitationNear(ctx context.Context, lat, lon float64, start, end time.Time) (PrecipitationObservations, error) {
	f.calls.Add(1)
	if f.err != nil {
		return PrecipitationObservations{}, f.err
	}
	return PrecipitationObservations{Station: testStation, DistanceKM: 2}, nil
}

func TestObservedPrecipitationFetchIsNotRepeatedAfterAFailure(t *testing.T) {
	loc, err := time.LoadLocation(DefaultPlaceTimezone)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for name, fetchErr := range map[string]error{
		"upstream failure": errors.New("FMI returned 503"),
		"no gauge nearby":  ErrNoStation,
		"success":          nil,
	} {
		history := &fakePrecipHistory{err: fetchErr}
		svc := NewService(&fakeStore{station: testStation}, history, time.Minute)

		var last *PrecipitationObservations
		for range 3 {
			last = svc.observedPrecipitationThisMonth(context.Background(), testLat, testLon, now, loc)
		}
		if n := history.calls.Load(); n != 1 {
			t.Errorf("%s: fetched %d times for 3 requests, want 1", name, n)
		}
		if (last != nil) != (fetchErr == nil) {
			t.Errorf("%s: result = %v", name, last)
		}
	}
}
