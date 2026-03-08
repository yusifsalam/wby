package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"wby/internal/fmi"
	"wby/internal/store"
)

func main() {
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

	fmiBaseURL := os.Getenv("FMI_BASE_URL")
	if fmiBaseURL == "" {
		fmiBaseURL = "https://opendata.fmi.fi/wfs"
	}

	stationIDs, err := db.AllStationFMISIDs(ctx)
	if err != nil {
		slog.Error("list stations", "err", err)
		os.Exit(1)
	}
	slog.Info("found stations", "count", len(stationIDs))

	client := fmi.NewClient(fmiBaseURL, "", "")
	batchSize := 20
	total := 0

	for i := 0; i < len(stationIDs); i += batchSize {
		end := i + batchSize
		if end > len(stationIDs) {
			end = len(stationIDs)
		}
		batch := stationIDs[i:end]

		fmisidStrs := make([]string, len(batch))
		for j, id := range batch {
			fmisidStrs[j] = fmt.Sprintf("%d", id)
		}

		data, err := client.FetchClimateNormals(ctx, strings.Join(fmisidStrs, ","))
		if err != nil {
			slog.Warn("fetch batch", "start", i, "err", err)
			continue
		}

		normals, err := fmi.ParseClimateNormals(data)
		if err != nil {
			slog.Warn("parse batch", "start", i, "err", err)
			continue
		}

		if err := db.UpsertClimateNormals(ctx, normals); err != nil {
			slog.Warn("upsert batch", "start", i, "err", err)
			continue
		}

		total += len(normals)
		slog.Info("imported batch", "stations", len(batch), "normals", len(normals), "total", total)

		// Rate limit: be nice to FMI API
		time.Sleep(1 * time.Second)
	}

	slog.Info("import complete", "total_normals", total)
}
