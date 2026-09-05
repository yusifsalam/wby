package store

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"wby/internal/weather"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://weather:weather@localhost:5432/weather?sslmode=disable"
	}
	s, err := New(context.Background(), dsn)
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertStations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	stations := []weather.Station{
		{FMISID: 100971, Name: "Helsinki Kaisaniemi", Lat: 60.17523, Lon: 24.94459, WMOCode: "2978"},
	}

	err := s.UpsertStations(ctx, stations)
	if err != nil {
		t.Fatal(err)
	}

	nearest, dist, err := s.NearestStation(ctx, 60.17, 24.94)
	if err != nil {
		t.Fatal(err)
	}
	if nearest.FMISID != 100971 {
		t.Errorf("expected station 100971, got %d", nearest.FMISID)
	}
	if dist > 1.0 {
		t.Errorf("expected distance < 1km, got %f", dist)
	}
}

func TestLatestObservationComposesAcrossRows(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const fmisid = 999001
	if err := s.UpsertStations(ctx, []weather.Station{{FMISID: fmisid, Name: "Test Composite", Lat: 69.9, Lon: 31.9}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM observations WHERE fmisid = $1`, fmisid)
		_, _ = s.pool.Exec(ctx, `DELETE FROM stations WHERE fmisid = $1`, fmisid)
	})

	f := func(v float64) *float64 { return &v }
	base := time.Date(2026, 8, 22, 11, 0, 0, 0, time.UTC)
	obs := []weather.Observation{
		// :00 carries the hourly precip and an older wind reading.
		{FMISID: fmisid, ObservedAt: base, Temperature: f(15), WindSpeed: f(3), Precip1h: f(0.4), Humidity: f(84)},
		// :10 has no wind or precip at all.
		{FMISID: fmisid, ObservedAt: base.Add(10 * time.Minute), Temperature: f(15.2), Humidity: f(86)},
		// :20 has fresh wind but still no precip.
		{FMISID: fmisid, ObservedAt: base.Add(20 * time.Minute), Temperature: f(15.4), WindSpeed: f(4.2), WindGust: f(6.5), Humidity: f(86)},
		// A stale value outside the window must not leak in.
		{FMISID: fmisid, ObservedAt: base.Add(-2 * time.Hour), SnowDepth: f(30)},
	}
	if err := s.UpsertObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}

	got, err := s.LatestObservation(ctx, fmisid)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ObservedAt.Equal(base.Add(20 * time.Minute)) {
		t.Errorf("ObservedAt = %v, want %v", got.ObservedAt, base.Add(20*time.Minute))
	}
	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("Temperature", got.Temperature, 15.4)
	check("WindSpeed", got.WindSpeed, 4.2)
	check("WindGust", got.WindGust, 6.5)
	check("Precip1h", got.Precip1h, 0.4)
	check("Humidity", got.Humidity, 86)
	if got.SnowDepth != nil {
		t.Errorf("SnowDepth = %v, want nil (outside window)", *got.SnowDepth)
	}
}

func TestObservedTemperatureRange(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const fmisid = 999002
	if err := s.UpsertStations(ctx, []weather.Station{{FMISID: fmisid, Name: "Test Range", Lat: 69.9, Lon: 31.8}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM observations WHERE fmisid = $1`, fmisid)
		_, _ = s.pool.Exec(ctx, `DELETE FROM stations WHERE fmisid = $1`, fmisid)
	})

	f := func(v float64) *float64 { return &v }
	day := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	obs := []weather.Observation{
		{FMISID: fmisid, ObservedAt: day.Add(-10 * time.Minute), Temperature: f(-5)},
		{FMISID: fmisid, ObservedAt: day, Temperature: f(4)},
		{FMISID: fmisid, ObservedAt: day.Add(6 * time.Hour), Temperature: f(2.5)},
		{FMISID: fmisid, ObservedAt: day.Add(12 * time.Hour), Temperature: f(17)},
		{FMISID: fmisid, ObservedAt: day.Add(13 * time.Hour)},
		{FMISID: fmisid, ObservedAt: day.Add(24 * time.Hour), Temperature: f(30)},
	}
	if err := s.UpsertObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}

	low, high, err := s.ObservedTemperatureRange(ctx, fmisid, day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if low == nil || *low != 2.5 || high == nil || *high != 17 {
		t.Errorf("range = %v..%v, want 2.5..17", low, high)
	}

	low, high, err = s.ObservedTemperatureRange(ctx, fmisid, day.Add(48*time.Hour), day.Add(72*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if low != nil || high != nil {
		t.Errorf("empty window must yield nil, got %v..%v", low, high)
	}
}

// A single-connection pool must still complete the hourly upsert: the batch
// connection has to be released before the trailing cleanup DELETE acquires
// one, or concurrent requests deadlock the pool waiting on each other.
func TestUpsertHourlyForecastsReleasesBatchConnBeforeCleanup(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://weather:weather@localhost:5432/weather?sslmode=disable"
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	s, err := New(context.Background(), dsn+sep+"pool_max_conns=1")
	if err != nil {
		t.Skipf("database not available: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// The context outlives the deadlock check: the old code only returned once
	// the cleanup DELETE's acquire gave up on ctx, which would mask the hang.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Hour)
	temp := 12.5
	hourly := []weather.HourlyForecast{
		{Time: now, FetchedAt: now, Temperature: &temp},
		{Time: now.Add(time.Hour), FetchedAt: now, Temperature: &temp},
	}
	done := make(chan error, 1)
	go func() { done <- s.UpsertHourlyForecasts(ctx, 89.99, 179.99, hourly) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("upsert hourly forecasts: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpsertHourlyForecasts deadlocked on a single-connection pool")
	}

	if _, err := s.pool.Exec(context.Background(),
		`DELETE FROM hourly_forecasts WHERE grid_lat = 89.99 AND grid_lon = 179.99`); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
}
