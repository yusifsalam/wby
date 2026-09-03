// import-daily-normals fetches a station's daily and hourly observation
// history from the FMI open-data WFS, computes day-of-year climate normals for
// a period, and upserts them into daily_climate_normals.
//
//	DATABASE_URL=... go run ./cmd/import-daily-normals -fmisid 100971 -period 1991-2020
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"wby/internal/fmi"
	"wby/internal/store"
	"wby/internal/weather"
)

func main() {
	fmisids := flag.String("fmisid", "100971", "comma-separated station FMISIDs")
	period := flag.String("period", "1991-2020", "normal period as YYYY-YYYY")
	withHourly := flag.Bool("hourly", true, "also fetch hourly temperatures for the hour-of-day curve")
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

	for _, raw := range strings.Split(*fmisids, ",") {
		fmisid, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			slog.Error("invalid fmisid", "value", raw)
			os.Exit(1)
		}
		log := slog.With("fmisid", fmisid, "period", *period)

		t0 := time.Now()
		daily, err := client.FetchDailyObservations(ctx, fmisid, start, end)
		if err != nil {
			log.Error("fetch daily observations", "err", err)
			os.Exit(1)
		}
		log.Info("fetched daily observations", "records", len(daily), "took", time.Since(t0).Round(time.Second))

		var hourly []weather.HourlyRecord
		if *withHourly {
			t1 := time.Now()
			hourly, err = client.FetchHourlyObservations(ctx, fmisid, start, end)
			if err != nil {
				log.Error("fetch hourly observations", "err", err)
				os.Exit(1)
			}
			log.Info("fetched hourly observations", "records", len(hourly), "took", time.Since(t1).Round(time.Second))
		}

		normals, err := weather.ComputeDailyNormals(fmisid, *period, daily, hourly)
		if err != nil {
			log.Error("compute normals", "err", err)
			os.Exit(1)
		}
		withTemp, withHours := 0, 0
		for _, n := range normals {
			if n.TempAvg != nil {
				withTemp++
			}
			if n.TempHourly != nil {
				withHours++
			}
		}
		if err := db.UpsertDailyClimateNormals(ctx, normals); err != nil {
			log.Error("upsert normals", "err", err)
			os.Exit(1)
		}
		log.Info("imported daily normals", "days", len(normals), "with_temperature", withTemp, "with_hourly", withHours)
	}
}
