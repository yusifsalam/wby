package fetcher

import (
	"context"
	"log/slog"
	"time"

	"wby/internal/fmi"
	"wby/internal/store"
)

type Fetcher struct {
	fmi   *fmi.Client
	store *store.Store
}

func New(fmiClient *fmi.Client, store *store.Store) *Fetcher {
	return &Fetcher{fmi: fmiClient, store: store}
}

func (f *Fetcher) RunObservationLoop(ctx context.Context, interval time.Duration) {
	slog.Info("observation fetcher starting", "interval", interval)

	f.fetchObservations(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("observation fetcher stopped")
			return
		case <-ticker.C:
			f.fetchObservations(ctx)
		}
	}
}

func (f *Fetcher) fetchObservations(ctx context.Context) {
	start := time.Now()
	result, err := f.fmi.FetchObservations(ctx)
	if err != nil {
		slog.Error("failed to fetch observations from FMI", "err", err)
		return
	}
	if len(result.Stations) == 0 {
		slog.Warn("observation fetch returned no stations")
		return
	}

	if err := f.store.UpsertStations(ctx, result.Stations); err != nil {
		slog.Error("failed to upsert stations", "err", err)
		return
	}

	if err := f.store.UpsertObservations(ctx, result.Observations); err != nil {
		slog.Error("failed to upsert observations", "err", err)
		return
	}

	slog.Info("observations fetched",
		"stations", len(result.Stations),
		"observations", len(result.Observations),
		"duration", time.Since(start),
	)
}
