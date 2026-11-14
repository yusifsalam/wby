package store

import (
	"context"
	"os"
	"slices"
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

func TestForecastFeelsLikeRoundTrip(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const lat, lon = 89.98, 179.98
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM forecasts WHERE grid_lat = $1 AND grid_lon = $2`, lat, lon)
	})

	f := func(v float64) *float64 { return &v }
	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	in := weather.DailyForecast{
		GridLat: lat, GridLon: lon, Date: day, FetchedAt: time.Now(),
		TempHigh: f(12), TempLow: f(6), TempAvg: f(9),
		FeelsLikeHigh: f(8.6), FeelsLikeLow: f(1.9), FeelsLikeAvg: f(5.2),
	}
	if err := s.UpsertForecasts(ctx, []weather.DailyForecast{in}); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetForecasts(ctx, lat, lon, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d forecasts, want 1", len(got))
	}
	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	check("FeelsLikeHigh", got[0].FeelsLikeHigh, 8.6)
	check("FeelsLikeLow", got[0].FeelsLikeLow, 1.9)
	check("FeelsLikeAvg", got[0].FeelsLikeAvg, 5.2)
}

func TestPruneHourlyForecastsRemovesOnlyExpiredHours(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const lat, lon = 89.99, 179.99
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM hourly_forecasts WHERE grid_lat = $1 AND grid_lon = $2`, lat, lon)
	})

	now := time.Now().UTC().Truncate(time.Hour)
	expired, kept := now.Add(-4*24*time.Hour), now.Add(-2*24*time.Hour)
	temp := 12.5
	hourly := []weather.HourlyForecast{
		{Time: expired, FetchedAt: now, Temperature: &temp},
		{Time: kept, FetchedAt: now, Temperature: &temp},
		{Time: now, FetchedAt: now, Temperature: &temp},
	}
	if err := s.UpsertHourlyForecasts(ctx, lat, lon, hourly); err != nil {
		t.Fatal(err)
	}
	stored := func() []time.Time {
		t.Helper()
		rows, err := s.GetHourlyForecasts(ctx, lat, lon, expired, 10)
		if err != nil {
			t.Fatal(err)
		}
		var times []time.Time
		for _, h := range rows {
			times = append(times, h.Time.UTC())
		}
		return times
	}
	if got := stored(); len(got) != 3 {
		t.Fatalf("stored %d hours after the upsert, want 3: storing forecasts no longer prunes", len(got))
	}

	deleted, err := s.PruneHourlyForecasts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted < 1 {
		t.Errorf("pruned %d rows, want at least the expired one", deleted)
	}
	if got := stored(); !slices.Equal(got, []time.Time{kept, now}) {
		t.Errorf("hours after pruning = %v, want %v", got, []time.Time{kept, now})
	}
}

func TestLeaderboardFindsExtremesAcrossStations(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const cold, hot = 999004, 999005
	stations := []weather.Station{
		{FMISID: cold, Name: "Test Cold", Lat: 69.9, Lon: 31.6},
		{FMISID: hot, Name: "Test Hot And Windy", Lat: 69.8, Lon: 31.6},
	}
	if err := s.UpsertStations(ctx, stations); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM observations WHERE fmisid IN ($1, $2)`, cold, hot)
		_, _ = s.pool.Exec(ctx, `DELETE FROM stations WHERE fmisid IN ($1, $2)`, cold, hot)
	})

	f := func(v float64) *float64 { return &v }
	now := time.Now().UTC().Truncate(time.Minute)
	obs := []weather.Observation{
		// Latest rows: what "now" ranks.
		{FMISID: cold, ObservedAt: now.Add(-10 * time.Minute), Temperature: f(-80), WindSpeed: f(1)},
		{FMISID: hot, ObservedAt: now.Add(-10 * time.Minute), Temperature: f(80), WindSpeed: f(2)},
		// Six hours back: more extreme, inside 24h but outside 1h and "now".
		{FMISID: cold, ObservedAt: now.Add(-6 * time.Hour), Temperature: f(-90)},
		{FMISID: hot, ObservedAt: now.Add(-6 * time.Hour), Temperature: f(90), WindSpeed: f(150)},
		// Five days back: the most extreme, inside 7d only.
		{FMISID: cold, ObservedAt: now.Add(-5 * 24 * time.Hour), Temperature: f(-95)},
		// A row without values must not hide the station's latest reading.
		{FMISID: cold, ObservedAt: now.Add(-30 * time.Minute), WindSpeed: f(1)},
	}
	if err := s.UpsertObservations(ctx, obs); err != nil {
		t.Fatal(err)
	}

	type extreme struct {
		station string
		value   float64
	}
	cases := map[string]map[string]extreme{
		"now": {"coldest": {"Test Cold", -80}, "warmest": {"Test Hot And Windy", 80}},
		"1h":  {"coldest": {"Test Cold", -80}, "warmest": {"Test Hot And Windy", 80}},
		"24h": {"coldest": {"Test Cold", -90}, "warmest": {"Test Hot And Windy", 90}, "windiest": {"Test Hot And Windy", 150}},
		"3d":  {"coldest": {"Test Cold", -90}, "warmest": {"Test Hot And Windy", 90}, "windiest": {"Test Hot And Windy", 150}},
		"7d":  {"coldest": {"Test Cold", -95}, "warmest": {"Test Hot And Windy", 90}, "windiest": {"Test Hot And Windy", 150}},
	}
	for timeframe, want := range cases {
		entries, err := s.GetLeaderboard(ctx, 60.17, 24.94, timeframe)
		if err != nil {
			t.Fatalf("%s: %v", timeframe, err)
		}
		got := map[string]extreme{}
		for _, e := range entries {
			got[e.StatType] = extreme{e.StationName, e.Value}
			if e.DistanceKM < 900 || e.DistanceKM > 1300 {
				t.Errorf("%s %s: distance %.0f km from Helsinki to the far north-east, want about 1100", timeframe, e.StatType, e.DistanceKM)
			}
		}
		for stat, w := range want {
			if got[stat] != w {
				t.Errorf("%s %s = %+v, want %+v", timeframe, stat, got[stat], w)
			}
		}
	}
}

func TestUpsertObservationsKeepsValuesALaterFetchLacks(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const fmisid = 999003
	if err := s.UpsertStations(ctx, []weather.Station{{FMISID: fmisid, Name: "Test Refetch", Lat: 69.9, Lon: 31.7}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM observations WHERE fmisid = $1`, fmisid)
		_, _ = s.pool.Exec(ctx, `DELETE FROM stations WHERE fmisid = $1`, fmisid)
	})

	f := func(v float64) *float64 { return &v }
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	upsert := func(o weather.Observation) {
		t.Helper()
		o.FMISID, o.ObservedAt = fmisid, at
		if err := s.UpsertObservations(ctx, []weather.Observation{o}); err != nil {
			t.Fatal(err)
		}
	}
	type row struct {
		temp, wind, humidity *float64
		extra                map[string]float64
		version              string
	}
	read := func() row {
		t.Helper()
		var r row
		var extraRaw []byte
		err := s.pool.QueryRow(ctx,
			`SELECT temperature, wind_speed, humidity, extra, xmin::text FROM observations WHERE fmisid = $1 AND observed_at = $2`,
			fmisid, at,
		).Scan(&r.temp, &r.wind, &r.humidity, &extraRaw, &r.version)
		if err != nil {
			t.Fatal(err)
		}
		r.extra = decodeNumericExtras(extraRaw)
		return r
	}
	check := func(name string, got *float64, want float64) {
		t.Helper()
		if got == nil || *got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}

	upsert(weather.Observation{Temperature: f(5), WindSpeed: f(3), ExtraNumericParams: map[string]float64{"glob_1min": 120}})
	upsert(weather.Observation{WindSpeed: f(4), Humidity: f(80), ExtraNumericParams: map[string]float64{"tg_1min": 2}})

	merged := read()
	check("temperature kept", merged.temp, 5)
	check("wind updated", merged.wind, 4)
	check("humidity filled in", merged.humidity, 80)
	if merged.extra["glob_1min"] != 120 || merged.extra["tg_1min"] != 2 {
		t.Errorf("extra = %v, want both keys merged", merged.extra)
	}

	upsert(weather.Observation{Temperature: f(5), WindSpeed: f(4), ExtraNumericParams: map[string]float64{"tg_1min": 2}})
	upsert(weather.Observation{})
	if again := read(); again.version != merged.version {
		t.Errorf("row was rewritten by an upsert that changed nothing (xmin %s -> %s)", merged.version, again.version)
	}
}

func TestUpsertForecastsDropsStoredDaysPastTheNewLastDay(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	const lat, lon = 89.97, 179.97
	const otherLat, otherLon = 89.96, 179.96
	t.Cleanup(func() {
		_, _ = s.pool.Exec(ctx, `DELETE FROM forecasts WHERE (grid_lat, grid_lon) IN (($1, $2), ($3, $4))`, lat, lon, otherLat, otherLon)
	})

	day := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	days := func(lat, lon float64, n int) []weather.DailyForecast {
		var out []weather.DailyForecast
		for i := range n {
			temp := float64(i)
			out = append(out, weather.DailyForecast{GridLat: lat, GridLon: lon, Date: day.AddDate(0, 0, i), FetchedAt: time.Now(), TempAvg: &temp})
		}
		return out
	}
	count := func(lat, lon float64) int {
		t.Helper()
		got, err := s.GetForecasts(ctx, lat, lon, day)
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}

	if err := s.UpsertForecasts(ctx, slices.Concat(days(lat, lon, 5), days(otherLat, otherLon, 5))); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertForecasts(ctx, days(lat, lon, 3)); err != nil {
		t.Fatal(err)
	}
	if got := count(lat, lon); got != 3 {
		t.Errorf("stored days = %d, want 3 after a shorter forecast replaced a longer one", got)
	}
	if got := count(otherLat, otherLon); got != 5 {
		t.Errorf("another cell's stored days = %d, want 5 untouched", got)
	}
}
