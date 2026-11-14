package fetcher

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"wby/internal/fmi"
	"wby/internal/weather"
)

// FMISource is the part of the FMI client the fetcher polls.
type FMISource interface {
	FetchObservations(ctx context.Context) (*fmi.ObservationResult, error)
	FetchGRIB(ctx context.Context, producer, params, bbox string, w io.Writer) error
}

// ObservationStore persists what the observation poll returns.
type ObservationStore interface {
	UpsertStations(ctx context.Context, stations []weather.Station) error
	UpsertObservations(ctx context.Context, observations []weather.Observation) error
}

type Fetcher struct {
	fmi   FMISource
	store ObservationStore

	mu     sync.Mutex
	status Status
}

// Status is when each background fetch last succeeded; a zero time means not
// since the process started.
type Status struct {
	// ObservationsFetchedAt is when observations were last fetched and stored.
	ObservationsFetchedAt time.Time
	// NewestObservationAt is the latest observation time in that fetch.
	NewestObservationAt time.Time
	// GribFetchedAt is when the GRIB file was last downloaded.
	GribFetchedAt time.Time
	// NewestRadarFrameAt is the valid time of the newest radar frame on disk.
	NewestRadarFrameAt time.Time
}

func New(fmiClient FMISource, store ObservationStore) *Fetcher {
	return &Fetcher{fmi: fmiClient, store: store}
}

func (f *Fetcher) Status() Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

func (f *Fetcher) updateStatus(update func(*Status)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	update(&f.status)
}

// cycle runs one pass of a polling loop, bounded by the loop's interval.
func cycle(ctx context.Context, interval time.Duration, run func(context.Context)) {
	ctx, cancel := context.WithTimeout(ctx, interval)
	defer cancel()
	run(ctx)
}

func (f *Fetcher) RunObservationLoop(ctx context.Context, interval time.Duration) {
	slog.Info("observation fetcher starting", "interval", interval)

	cycle(ctx, interval, f.fetchObservations)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("observation fetcher stopped")
			return
		case <-ticker.C:
			cycle(ctx, interval, f.fetchObservations)
		}
	}
}

func (f *Fetcher) fetchObservations(ctx context.Context) {
	start := time.Now()
	result, err := f.fmi.FetchObservations(ctx)
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		slog.Error("failed to fetch observations from FMI", "err", err)
		return
	}
	if result.SkippedMembers > 0 {
		slog.Warn("observation fetch skipped series without a usable station", "series", result.SkippedMembers)
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

	var newest time.Time
	for _, o := range result.Observations {
		if o.ObservedAt.After(newest) {
			newest = o.ObservedAt
		}
	}
	f.updateStatus(func(s *Status) {
		s.ObservationsFetchedAt = time.Now()
		s.NewestObservationAt = newest
	})

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

	cycle(ctx, interval, func(ctx context.Context) {
		if onRefresh != nil {
			onRefresh(ctx)
		}
		f.fetchGrib(ctx, job, onRefresh)
	})

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("grib fetcher stopped")
			return
		case <-ticker.C:
			cycle(ctx, interval, func(ctx context.Context) { f.fetchGrib(ctx, job, onRefresh) })
		}
	}
}

func (f *Fetcher) fetchGrib(ctx context.Context, job GribJob, onRefresh func(context.Context)) {
	start := time.Now()
	err := fmi.Retry(ctx, func() error { return f.downloadGrib(ctx, job) })
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		slog.Error("failed to fetch grib from FMI", "err", err, "producer", job.Producer)
	} else {
		f.updateStatus(func(s *Status) { s.GribFetchedAt = time.Now() })
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
