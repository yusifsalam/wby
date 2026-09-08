package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wby/internal/weather"
)

func f(v float64) *float64 { return &v }

func testDailyCache(fetched *[]int, fail bool) yearCache[weather.DailyRecord] {
	return yearCache[weather.DailyRecord]{
		product: dailyProduct,
		year:    func(r weather.DailyRecord) int { return r.Date.UTC().Year() },
		read:    readDailyCSV,
		write:   writeDailyCSV,
		fetch: func(ctx context.Context, start, end time.Time) ([]weather.DailyRecord, error) {
			if fail {
				return nil, errors.New("unexpected fetch")
			}
			*fetched = append(*fetched, start.Year())
			var out []weather.DailyRecord
			for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
				out = append(out, weather.DailyRecord{Date: d, TempAvg: f(float64(d.YearDay()))})
			}
			return out, nil
		},
	}
}

func testHourlyCache(product string, fetched *[]int) yearCache[weather.HourlyRecord] {
	return yearCache[weather.HourlyRecord]{
		product: product,
		year:    func(r weather.HourlyRecord) int { return r.Time.UTC().Year() },
		read:    readHourlyCSV,
		write:   writeHourlyCSV,
		fetch: func(ctx context.Context, start, end time.Time) ([]weather.HourlyRecord, error) {
			*fetched = append(*fetched, start.Year())
			return []weather.HourlyRecord{{Time: start, Temp: f(1), WindSpeed: f(2)}}, nil
		},
	}
}

func TestLoadFetchesOnlyMissingYears(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	var fetched []int
	cache := testDailyCache(&fetched, false)

	first, err := cache.load(context.Background(), log, dir, 1, 1991, 1993)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 3 || fetched[0] != 1991 || fetched[2] != 1993 {
		t.Fatalf("fetched years = %v, want 1991..1993", fetched)
	}
	if want := 365 + 366 + 365; len(first) != want {
		t.Fatalf("records = %d, want %d", len(first), want)
	}

	fetched = nil
	second, err := cache.load(context.Background(), log, dir, 1, 1992, 1995)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 2 || fetched[0] != 1994 || fetched[1] != 1995 {
		t.Errorf("fetched years = %v, want 1994 1995 only", fetched)
	}
	if want := 366 + 365 + 365 + 365; len(second) != want {
		t.Errorf("records = %d, want %d", len(second), want)
	}
	if second[0].Date.Year() != 1992 || second[len(second)-1].Date.Year() != 1995 {
		t.Errorf("records span %v..%v", second[0].Date, second[len(second)-1].Date)
	}
}

func TestMissingYearsAndMarkEmpty(t *testing.T) {
	dir := t.TempDir()
	var fetched []int
	cache := testHourlyCache(instantHourlyProduct, &fetched)

	missing, err := cache.missingYears(dir, 7, 2000, 2002)
	if err != nil || len(missing) != 3 {
		t.Fatalf("missing = %v, err %v", missing, err)
	}
	if err := cache.markEmpty(dir, 7, missing); err != nil {
		t.Fatal(err)
	}
	missing, err = cache.missingYears(dir, 7, 2000, 2003)
	if err != nil || len(missing) != 1 || missing[0] != 2003 {
		t.Fatalf("after markEmpty missing = %v, err %v", missing, err)
	}
	records, err := cache.load(context.Background(), slog.New(slog.DiscardHandler), dir, 7, 2000, 2003)
	if err != nil {
		t.Fatal(err)
	}
	if len(fetched) != 1 || fetched[0] != 2003 || len(records) != 1 {
		t.Errorf("fetched = %v, records = %d; want only 2003 fetched", fetched, len(records))
	}
}

func TestMigrateLegacyCacheSplitsPeriodFiles(t *testing.T) {
	dir := t.TempDir()
	log := slog.New(slog.DiscardHandler)
	station := filepath.Join(dir, "5")

	var daily []weather.DailyRecord
	for d := time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() <= 1993; d = d.AddDate(0, 0, 1) {
		daily = append(daily, weather.DailyRecord{Date: d, TempAvg: f(1), PrecipMm: f(-1)})
	}
	if err := writeDailyCSV(filepath.Join(station, "daily-1991-1993.csv"), daily); err != nil {
		t.Fatal(err)
	}
	hourly := []weather.HourlyRecord{
		{Time: time.Date(1993, 6, 1, 12, 0, 0, 0, time.UTC), Temp: f(15), Humidity: f(60), WindSpeed: f(3)},
	}
	if err := writeHourlyCSV(filepath.Join(station, "hourly-1991-1993.csv"), hourly); err != nil {
		t.Fatal(err)
	}
	if err := writeHourlyCSV(filepath.Join(station, "hourly-instant-1991-1993.csv"), nil); err != nil {
		t.Fatal(err)
	}

	var fetched []int
	dailyCache := testDailyCache(&fetched, true)
	hourlyCache := testHourlyCache(hourlyProduct, &fetched)
	instantCache := testHourlyCache(instantHourlyProduct, &fetched)
	if err := migrateLegacyCache(log, dir, 5, dailyCache, hourlyCache, instantCache); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"daily-1991-1993.csv", "hourly-1991-1993.csv", "hourly-instant-1991-1993.csv"} {
		if _, err := os.Stat(filepath.Join(station, name)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still present (err %v)", name, err)
		}
	}
	for _, product := range []string{dailyProduct, hourlyProduct, instantHourlyProduct} {
		for y := 1991; y <= 1993; y++ {
			if _, err := os.Stat(yearCachePath(dir, 5, product, y)); err != nil {
				t.Errorf("%s/%d missing: %v", product, y, err)
			}
		}
	}

	got, err := dailyCache.load(context.Background(), log, dir, 5, 1991, 1993)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(daily) {
		t.Errorf("daily records after split = %d, want %d", len(got), len(daily))
	}
	if got[0].PrecipMm == nil || *got[0].PrecipMm != -1 {
		t.Errorf("first daily record lost its precip marker: %+v", got[0])
	}
	gotHourly, err := hourlyCache.load(context.Background(), log, dir, 5, 1991, 1993)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotHourly) != 1 || !gotHourly[0].Time.Equal(hourly[0].Time) || *gotHourly[0].WindSpeed != 3 {
		t.Errorf("hourly records after split = %+v", gotHourly)
	}
	gotInstant, err := instantCache.load(context.Background(), log, dir, 5, 1991, 1993)
	if err != nil {
		t.Fatal(err)
	}
	if len(gotInstant) != 0 || len(fetched) != 0 {
		t.Errorf("empty legacy instant file should yield empty years without fetching: records %d, fetched %v", len(gotInstant), fetched)
	}

	if err := migrateLegacyCache(log, dir, 5, dailyCache, hourlyCache, instantCache); err != nil {
		t.Errorf("second migration should be a no-op: %v", err)
	}
	if err := migrateLegacyCache(log, dir, 999, dailyCache, hourlyCache, instantCache); err != nil {
		t.Errorf("unknown station should be a no-op: %v", err)
	}
}
