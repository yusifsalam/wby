package fetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"wby/internal/fmi"
	"wby/internal/weather"
)

// radarFrameStep is the composite publish cadence.
const radarFrameStep = 5 * time.Minute

// radarNodata is the uint16 fill GeoServer writes outside radar coverage.
const radarNodata = 65535

// RadarJob describes the rolling window of radar rain-rate frames to keep on
// disk for gribsvc: the frame extent/size to request and how far back to hold.
type RadarJob struct {
	DataDir string
	BBox    string // "minLon,minLat,maxLon,maxLat" (EPSG:4326 degrees)
	Width   int
	Height  int
	Span    time.Duration // how far back frames are kept and backfilled
}

// RunRadarLoop keeps the last Span of radar composite frames on disk: on each
// tick it fetches the newest published frame and backfills any gaps (e.g. after
// a restart), then prunes frames older than the window. Fetch once immediately,
// then on each tick until ctx is cancelled, mirroring RunGribLoop.
func (f *Fetcher) RunRadarLoop(ctx context.Context, radar *fmi.RadarClient, job RadarJob, onRefresh func(context.Context)) {
	slog.Info("radar fetcher starting", "dir", job.DataDir, "span", job.Span)

	f.fetchRadarWindow(ctx, radar, job, onRefresh)

	ticker := time.NewTicker(radarFrameStep)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("radar fetcher stopped")
			return
		case <-ticker.C:
			f.fetchRadarWindow(ctx, radar, job, onRefresh)
		}
	}
}

func (f *Fetcher) fetchRadarWindow(ctx context.Context, radar *fmi.RadarClient, job RadarJob, onRefresh func(context.Context)) {
	start := time.Now()
	latest := start.UTC().Truncate(radarFrameStep)
	fetched := 0

	for at := latest; !at.Before(latest.Add(-job.Span)); at = at.Add(-radarFrameStep) {
		if ctx.Err() != nil {
			return
		}
		path := filepath.Join(job.DataDir, weather.RadarFrameFile(at))
		if _, err := os.Stat(path); err == nil {
			continue
		}

		err := f.downloadRadarFrame(ctx, radar, job, at)
		if errors.Is(err, fmi.ErrRadarFrameUnavailable) {
			// The newest slot lags publication by a few minutes; older gaps
			// are frames FMI never published. Either way, move on quietly.
			continue
		}
		if err != nil {
			slog.Error("failed to fetch radar frame", "err", err, "frame", at.Format(time.RFC3339))
			continue
		}
		fetched++
	}

	f.pruneRadarFrames(job)

	if fetched > 0 {
		slog.Info("radar frames fetched", "frames", fetched, "duration", time.Since(start))
		if onRefresh != nil {
			onRefresh(ctx)
		}
	}
}

// downloadRadarFrame streams one GeoTIFF into a hidden temp file plus its JSON
// sidecar (georeference + units for gribsvc), then renames both into place so
// gribsvc never sees a frame without metadata: the sidecar lands first.
func (f *Fetcher) downloadRadarFrame(ctx context.Context, radar *fmi.RadarClient, job RadarJob, at time.Time) error {
	name := weather.RadarFrameFile(at)

	tmp, err := os.CreateTemp(job.DataDir, "."+name+".*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if err := radar.FetchFrame(ctx, at, job.BBox, job.Width, job.Height, tmp); err != nil {
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

	sidecar, err := radarSidecarJSON(job, at)
	if err != nil {
		return err
	}
	sidecarPath := filepath.Join(job.DataDir, strings.TrimSuffix(name, ".tif")+".json")
	if err := os.WriteFile(sidecarPath, sidecar, 0o644); err != nil {
		return err
	}

	return os.Rename(tmpName, filepath.Join(job.DataDir, name))
}

func radarSidecarJSON(job RadarJob, at time.Time) ([]byte, error) {
	parts := strings.Split(job.BBox, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("radar bbox %q: want minLon,minLat,maxLon,maxLat", job.BBox)
	}
	bbox := make([]json.Number, 4)
	for i, p := range parts {
		bbox[i] = json.Number(strings.TrimSpace(p))
	}
	return json.Marshal(map[string]any{
		"param":  "rr",
		"time":   at.UTC().Format(time.RFC3339),
		"bbox":   bbox,
		"scale":  0.01, // uint16 -> mm/h, the FMI radar GeoTIFF convention
		"nodata": radarNodata,
		"units":  "mm/h",
	})
}

// pruneRadarFrames removes frame tiffs (and their sidecars) older than the
// window, keyed by the timestamp embedded in the filename.
func (f *Fetcher) pruneRadarFrames(job RadarJob) {
	cutoff := time.Now().UTC().Add(-job.Span - 2*radarFrameStep)

	entries, err := os.ReadDir(job.DataDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "radar_rr_") {
			continue
		}
		stamp := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(name, "radar_rr_"), ".tif"), ".json")
		at, err := time.Parse("20060102T1504Z", stamp)
		if err != nil || !at.Before(cutoff) {
			continue
		}
		os.Remove(filepath.Join(job.DataDir, name))
	}
}
