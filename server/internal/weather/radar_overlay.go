package weather

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// RadarObsSpan is how far back the radar observation grid is offered — matches
// the scrubber's past window plus a little slack for frame latency.
const RadarObsSpan = time.Hour

// radarFrameStep is the composite publish cadence.
const radarFrameStep = 5 * time.Minute

// RadarFrameFile is the deterministic on-disk name for the composite frame
// valid at the given instant. The fetcher writes frames under this name and
// this service addresses them by it, so neither needs a listing round-trip.
func RadarFrameFile(at time.Time) string {
	return "radar_rr_" + at.UTC().Format("20060102T1504Z") + ".tif"
}

// RadarPrecipitationSource fetches a rain-rate raster (mm/h, as a FieldGrid)
// from one per-timestamp radar frame file in gribsvc. Soft misses (frame file
// not on disk yet) return a nil grid and nil error. Optional.
type RadarPrecipitationSource interface {
	GridForFile(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error)
}

// SetRadarPrecipitationSource configures the keyless radar-composite source for
// the precipitation observation grid. A nil source (the default) keeps the
// observation overlay on the WMS tile path.
func (s *Service) SetRadarPrecipitationSource(src RadarPrecipitationSource) {
	s.radarPrecip = src
}

// GetPrecipitationObservationGrid returns the radar rain-rate raster (mm/h)
// nearest at-or-before the requested instant, over the full radar extent (the
// request bbox selects nothing; clients position the raster by the extent in
// the response, like the forecast grid). Time snaps to the 5-min frame cadence;
// a zero time means the newest available frame. Results are cached per frame,
// so a grid warmed after a radar refresh satisfies every client. Returns
// ErrPrecipitationDisabled when the source is unconfigured or no frame within
// tolerance exists, so the handler can answer 404 and the client falls back.
func (s *Service) GetPrecipitationObservationGrid(ctx context.Context, req PrecipitationOverlayRequest) (*PrecipitationForecastGrid, error) {
	if s.radarPrecip == nil {
		return nil, ErrPrecipitationDisabled
	}

	now := time.Now().UTC()
	target := req.Time.UTC().Truncate(radarFrameStep)
	if req.Time.IsZero() {
		target = now.Truncate(radarFrameStep)
	}
	// Reject targets outside the served window (future frames belong to the
	// forecast sources; older ones have been pruned).
	if target.After(now.Add(radarFrameStep)) || target.Before(now.Add(-RadarObsSpan-2*radarFrameStep)) {
		return nil, ErrPrecipitationDisabled
	}

	// Walk back a few frames from the target: the newest slot lags publication
	// by a couple of minutes, and individual frames can be missing.
	for i := 0; i < 3; i++ {
		at := target.Add(time.Duration(-i) * radarFrameStep)

		cacheKey := fmt.Sprintf("radarprecipgrid:%s", at.Format(time.RFC3339))
		if cached, ok := s.radarPrecipCache.Get(cacheKey); ok {
			return cached, nil
		}

		grid, validTime, err := s.radarPrecip.GridForFile(ctx, RadarFrameFile(at),
			precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat, time.Time{})
		if err != nil {
			return nil, fmt.Errorf("fetch radar grid: %w", err)
		}
		if grid == nil || len(grid.Values) == 0 {
			continue
		}
		if validTime.IsZero() {
			validTime = at
		}

		minV, maxV := fieldGridRange(grid)
		out := &PrecipitationForecastGrid{
			DataTime: validTime.UTC().Truncate(time.Second),
			Min:      minV,
			Max:      maxV,
			Grid:     grid,
		}
		s.radarPrecipCache.Set(cacheKey, out)
		slog.Info("precipitation observation grid",
			"target", target.Format(time.RFC3339),
			"frame", at.Format(time.RFC3339),
			"cells", len(grid.Values),
		)
		return out, nil
	}

	return nil, ErrPrecipitationDisabled
}

// WarmRadarGrids pre-populates the radar observation grid cache for every
// frame in the served window. Intended to run after each radar fetch cycle so
// scrubbing is cache hits. Best-effort: frames already cached are skipped via
// the per-frame cache, and misses are left for clients to fill lazily.
func (s *Service) WarmRadarGrids(ctx context.Context) {
	if s.radarPrecip == nil {
		return
	}

	start := time.Now()
	base := start.UTC().Truncate(radarFrameStep)
	frames := int(RadarObsSpan / radarFrameStep)
	warmed := 0
	for i := 0; i <= frames; i++ {
		if ctx.Err() != nil {
			return
		}
		at := base.Add(time.Duration(-i) * radarFrameStep)
		if _, err := s.GetPrecipitationObservationGrid(ctx, PrecipitationOverlayRequest{Time: at}); err == nil {
			warmed++
		}
	}
	slog.Info("warmed radar precipitation grids",
		"frames", warmed,
		"span", RadarObsSpan,
		"duration", time.Since(start),
	)
}
