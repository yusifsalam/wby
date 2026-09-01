package fetcher

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
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

// GribJob describes one GRIB2 product to keep fresh on disk: which FMI producer
// and params to download, the bbox to bound it, and where to write it.
type GribJob struct {
	DataDir  string
	Filename string
	Producer string
	Params   string
	BBox     string
}

// RunGribLoop periodically downloads a GRIB2 grid from FMI into the shared data
// directory that gribsvc reads. It mirrors RunObservationLoop: fetch once
// immediately, then on each tick until ctx is cancelled. After each successful
// download onRefresh (if non-nil) is invoked so the caller can warm any caches
// derived from the new file (e.g. the GRIB grid cache) before clients ask.
func (f *Fetcher) RunGribLoop(ctx context.Context, job GribJob, interval time.Duration, onRefresh func(context.Context)) {
	slog.Info("grib fetcher starting", "interval", interval, "dir", job.DataDir, "file", job.Filename)

	if onRefresh != nil {
		onRefresh(ctx)
	}
	f.fetchGrib(ctx, job, onRefresh)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("grib fetcher stopped")
			return
		case <-ticker.C:
			f.fetchGrib(ctx, job, onRefresh)
		}
	}
}

func (f *Fetcher) fetchGrib(ctx context.Context, job GribJob, onRefresh func(context.Context)) {
	start := time.Now()
	if err := f.downloadGrib(ctx, job); err != nil {
		slog.Error("failed to fetch grib from FMI", "err", err, "producer", job.Producer)
	} else {
		slog.Info("grib fetched", "producer", job.Producer, "file", job.Filename, "duration", time.Since(start))
	}

	if onRefresh != nil && ctx.Err() == nil {
		onRefresh(ctx)
	}
}

// downloadGrib streams the GRIB2 file into a hidden temp file, then atomically
// renames it into place so gribsvc's directory scan never sees a partial file.
func (f *Fetcher) downloadGrib(ctx context.Context, job GribJob) error {
	tmp, err := os.CreateTemp(job.DataDir, "."+job.Filename+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if err := f.fmi.FetchGRIB(ctx, job.Producer, job.Params, job.BBox, tmp); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, filepath.Join(job.DataDir, job.Filename))
}
