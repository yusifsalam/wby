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

// TestMigrateRealLegacyCache splits a copy of a real station directory from
// the observation cache named by FMI_OBS_CACHE_DIR (../../../data/fmi-observations from this package)
// and checks nothing is lost or fetched. Skipped when the variable is unset.
func TestMigrateRealLegacyCache(t *testing.T) {
	src := os.Getenv("FMI_OBS_CACHE_DIR")
	if src == "" {
		t.Skip("FMI_OBS_CACHE_DIR not set")
	}
	const fmisid = 100971
	entries, err := os.ReadDir(filepath.Join(src, "100971"))
	if err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	legacy := map[string]int{}
	for _, e := range entries {
		m := legacyCachePattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, "100971", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "100971"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "100971", e.Name()), data, 0o644); err != nil {
			t.Fatal(err)
		}
		header := hourlyHeader
		if m[1] == dailyProduct {
			header = dailyHeader
		}
		rows, err := readCSV(filepath.Join(dir, "100971", e.Name()), header)
		if err != nil {
			t.Fatal(err)
		}
		legacy[e.Name()] = len(rows)
	}
	if len(legacy) == 0 {
		t.Skip("no legacy period files left to migrate")
	}

	fail := func(ctx context.Context, start, end time.Time) ([]weather.HourlyRecord, error) {
		return nil, errors.New("unexpected fetch")
	}
	caches := newStationCaches(nil, fmisid)
	caches.daily.fetch = func(ctx context.Context, start, end time.Time) ([]weather.DailyRecord, error) {
		return nil, errors.New("unexpected fetch")
	}
	caches.hourly.fetch, caches.instant.fetch = fail, fail
	log := slog.New(slog.DiscardHandler)
	if err := migrateLegacyCache(log, dir, fmisid, caches.daily, caches.hourly, caches.instant); err != nil {
		t.Fatal(err)
	}

	for name, rows := range legacy {
		m := legacyCachePattern.FindStringSubmatch(name)
		startYear, endYear := atoi(m[2]), atoi(m[3])
		var got int
		switch m[1] {
		case dailyProduct:
			recs, err := caches.daily.load(context.Background(), log, dir, fmisid, startYear, endYear)
			if err != nil {
				t.Fatal(err)
			}
			got = len(recs)
		case hourlyProduct:
			recs, err := caches.hourly.load(context.Background(), log, dir, fmisid, startYear, endYear)
			if err != nil {
				t.Fatal(err)
			}
			got = len(recs)
		case instantHourlyProduct:
			recs, err := caches.instant.load(context.Background(), log, dir, fmisid, startYear, endYear)
			if err != nil {
				t.Fatal(err)
			}
			got = len(recs)
		}
		if got != rows {
			t.Errorf("%s: %d records after split, legacy file had %d", name, got, rows)
		}
		t.Logf("%s: legacy %d rows, per-year load %d records", name, rows, got)
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}
