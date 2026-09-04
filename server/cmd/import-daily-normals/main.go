// import-daily-normals fetches a station's daily and hourly observation
// history from the FMI open-data WFS, computes day-of-year climate normals for
// a period, and upserts them into daily_climate_normals. Raw observations are
// cached as CSV under -cache-dir (data/fmi-observations by default) so reruns
// recompute without refetching.
//
//	DATABASE_URL=... go run ./cmd/import-daily-normals -fmisid 100971 -period 1991-2020
//	DATABASE_URL=... go run ./cmd/import-daily-normals -from-climate-normals -period 1991-2020
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"wby/internal/fmi"
	"wby/internal/store"
	"wby/internal/weather"
)

// Stations with fewer temperature days than this are skipped rather than
// stored, so the API never selects a station with holes in its year.
const minTemperatureDays = 360

func main() {
	fmisids := flag.String("fmisid", "", "comma-separated station FMISIDs")
	fromNormals := flag.Bool("from-climate-normals", false, "import every station that has official monthly normals for the period")
	period := flag.String("period", "1991-2020", "normal period as YYYY-YYYY")
	withHourly := flag.Bool("hourly", true, "also fetch hourly temperature, humidity and wind for the hour-of-day curves and feels-like")
	cacheDir := flag.String("cache-dir", "../data/fmi-observations", "directory of per-station CSV observation caches; fetched data is written here and reused on later runs")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		slog.Error("DATABASE_URL not set")
		os.Exit(1)
	}
	ctx := context.Background()
	db, err := store.New(ctx, dsn)
	if err != nil {
		slog.Error("connect to database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	fmiBaseURL := os.Getenv("FMI_BASE_URL")
	if fmiBaseURL == "" {
		fmiBaseURL = "https://opendata.fmi.fi/wfs"
	}
	client := fmi.NewClient(fmiBaseURL, "", "")

	years := strings.SplitN(*period, "-", 2)
	if len(years) != 2 {
		slog.Error("period must be YYYY-YYYY", "period", *period)
		os.Exit(1)
	}
	startYear, err1 := strconv.Atoi(years[0])
	endYear, err2 := strconv.Atoi(years[1])
	if err1 != nil || err2 != nil {
		slog.Error("period must be YYYY-YYYY", "period", *period)
		os.Exit(1)
	}
	start := time.Date(startYear, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(endYear, 12, 31, 23, 0, 0, 0, time.UTC)

	var ids []int
	if *fromNormals {
		ids, err = db.StationsWithClimateNormals(ctx, *period)
		if err != nil {
			slog.Error("list stations with climate normals", "err", err)
			os.Exit(1)
		}
	}
	if *fmisids != "" {
		for _, raw := range strings.Split(*fmisids, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				slog.Error("invalid fmisid", "value", raw)
				os.Exit(1)
			}
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		slog.Error("no stations: pass -fmisid or -from-climate-normals")
		os.Exit(1)
	}

	failed, skipped := 0, 0
	t0 := time.Now()
	for i, fmisid := range ids {
		log := slog.With("fmisid", fmisid, "period", *period, "station", i+1, "of", len(ids))
		switch err := importStation(ctx, client, db, log, fmisid, *period, start, end, *withHourly, *cacheDir); {
		case err == errSkipped:
			skipped++
		case err != nil:
			log.Error("import failed", "err", err)
			failed++
		}
	}
	slog.Info("batch done", "stations", len(ids), "failed", failed, "skipped", skipped, "took", time.Since(t0).Round(time.Second))
	if failed > 0 {
		os.Exit(1)
	}
}

type sentinel string

func (s sentinel) Error() string { return string(s) }

const errSkipped = sentinel("skipped")

func importStation(ctx context.Context, client *fmi.Client, db *store.Store, log *slog.Logger, fmisid int, period string, start, end time.Time, withHourly bool, cacheDir string) error {
	daily, err := loadDaily(ctx, client, log, cacheDir, fmisid, period, start, end)
	if err != nil {
		return err
	}

	var hourly []weather.HourlyRecord
	if withHourly {
		hourly, err = loadHourly(ctx, client, log, cacheDir, fmisid, period, start, end)
		if err != nil {
			log.Warn("fetch hourly observations failed, continuing with daily only", "err", err)
			hourly = nil
		}
	}

	normals, err := weather.ComputeDailyNormals(fmisid, period, daily, hourly)
	if err != nil {
		return err
	}
	withTemp, withHours, withFeels, withWind := 0, 0, 0, 0
	for _, n := range normals {
		if n.TempAvg != nil {
			withTemp++
		}
		if n.TempHourly != nil {
			withHours++
		}
		if n.FeelsLikeAvg != nil {
			withFeels++
		}
		if n.WindAvg != nil {
			withWind++
		}
	}
	if withTemp < minTemperatureDays {
		log.Warn("skipping station: incomplete daily record", "with_temperature", withTemp, "daily_records", len(daily))
		return errSkipped
	}
	if err := db.UpsertDailyClimateNormals(ctx, normals); err != nil {
		return err
	}
	log.Info("imported daily normals", "days", len(normals), "with_temperature", withTemp, "with_hourly", withHours, "with_feels_like", withFeels, "with_wind", withWind)
	return nil
}

func loadDaily(ctx context.Context, client *fmi.Client, log *slog.Logger, cacheDir string, fmisid int, period string, start, end time.Time) ([]weather.DailyRecord, error) {
	path := dailyCachePath(cacheDir, fmisid, period)
	if records, ok, err := readDailyCSV(path); err != nil {
		return nil, err
	} else if ok {
		log.Info("loaded daily observations from cache", "records", len(records), "path", path)
		return records, nil
	}
	t0 := time.Now()
	records, err := client.FetchDailyObservations(ctx, fmisid, start, end)
	if err != nil {
		return nil, err
	}
	log.Info("fetched daily observations", "records", len(records), "took", time.Since(t0).Round(time.Second))
	if err := writeDailyCSV(path, records); err != nil {
		return nil, fmt.Errorf("write daily cache: %w", err)
	}
	return records, nil
}

func loadHourly(ctx context.Context, client *fmi.Client, log *slog.Logger, cacheDir string, fmisid int, period string, start, end time.Time) ([]weather.HourlyRecord, error) {
	path := hourlyCachePath(cacheDir, fmisid, period)
	if records, ok, err := readHourlyCSV(path); err != nil {
		return nil, err
	} else if ok {
		log.Info("loaded hourly observations from cache", "records", len(records), "path", path)
		return records, nil
	}
	t0 := time.Now()
	records, err := client.FetchHourlyObservations(ctx, fmisid, start, end)
	if err != nil {
		return nil, err
	}
	log.Info("fetched hourly observations", "records", len(records), "took", time.Since(t0).Round(time.Second))
	if err := writeHourlyCSV(path, records); err != nil {
		return nil, fmt.Errorf("write hourly cache: %w", err)
	}
	return records, nil
}
