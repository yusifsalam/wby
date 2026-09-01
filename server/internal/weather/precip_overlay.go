package weather

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// PrecipForecastHorizon is the furthest-future hour the precipitation forecast
// map exposes (the layer keeps its original `precipitation12h` API name for
// client compatibility). Both the frame manifest window and
// WarmPrecipitationGrids derive from it.
const PrecipForecastHorizon = 24

// precipGrid* is the fixed extraction extent for the precipitation forecast
// grid: the full GRIB download extent (GRIB_BBOX), a superset of every client's
// request bbox (web 10.2–37.4, iOS 18.8–32.2). The grid cache is keyed by hour
// only, so extraction must not depend on the request bbox — unlike temperature,
// precipitation renders unclipped, so a narrower first-requester extent would
// visibly shrink the raster for everyone else. Clients georeference the raster
// by the extent carried in the response, so the superset is safe.
const (
	precipGridMinLon = 10.0
	precipGridMinLat = 56.5
	precipGridMaxLon = 37.6
	precipGridMaxLat = 71.5
)

// PrecipitationForecastGrid is the Harmonie precipitation-rate raster (mm/h) at
// one hour. Clients upload it as a texture and render it with hardware bilinear,
// mirroring the temperature forecast grid.
type PrecipitationForecastGrid struct {
	DataTime time.Time
	Min      float64
	Max      float64
	Grid     *FieldGrid
}

// GetPrecipitationForecastGrid returns the precipitation-rate raster (mm/h) at
// the requested hour, always over the fixed full-Finland extent (the request
// bbox selects nothing; clients position the raster by the extent in the
// response). Time snaps to the hour; a zero time means the current hour.
// Results are cached per hour, so a grid warmed after a GRIB refresh satisfies
// every client. Returns ErrPrecipitationDisabled when the GRIB source is
// unconfigured or the field isn't available yet, so the handler can answer 404
// and the client keeps its previous frame.
func (s *Service) GetPrecipitationForecastGrid(ctx context.Context, req PrecipitationOverlayRequest) (*PrecipitationForecastGrid, error) {
	if s.gribPrecip == nil {
		return nil, ErrPrecipitationDisabled
	}

	target := req.Time.UTC().Truncate(time.Hour)
	if target.IsZero() {
		target = time.Now().UTC().Truncate(time.Hour)
	}

	cacheKey := gribPrecipGridKey(target)
	if cached, ok := s.gribPrecipCache.Get(cacheKey); ok {
		return cached, nil
	}

	grid, validTime, err := s.gribPrecip.Grid(ctx, precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat, target)
	if err != nil {
		return nil, fmt.Errorf("fetch precipitation grid: %w", err)
	}
	if grid == nil || len(grid.Values) == 0 {
		return nil, ErrPrecipitationDisabled
	}
	if validTime.IsZero() {
		validTime = target
	}

	minV, maxV := fieldGridRange(grid)
	out := &PrecipitationForecastGrid{
		DataTime: validTime.UTC().Truncate(time.Second),
		Min:      minV,
		Max:      maxV,
		Grid:     grid,
	}
	s.gribPrecipCache.Set(cacheKey, out)
	slog.Info("precipitation forecast grid",
		"target", target.UTC().Format(time.RFC3339),
		"valid_time", validTime.UTC().Format(time.RFC3339),
		"cells", len(grid.Values),
	)
	return out, nil
}

// WarmPrecipitationGrids pre-populates the precipitation forecast grid cache
// for every hourly frame (the current hour through PrecipForecastHorizon).
// Intended to run once per GRIB refresh cycle, alongside WarmTemperatureGrids,
// so the first client request for each scrubber frame is a cache hit rather
// than a cold gribsvc extract. All frames come from a single GridSeries call
// (one pass over the GRIB file); if that fails or returns nothing, warming
// falls back to one extract per hour. Best-effort: frames the file lacks are
// left for a client to fill lazily.
func (s *Service) WarmPrecipitationGrids(ctx context.Context) {
	if s.gribPrecip == nil || ctx.Err() != nil {
		return
	}

	start := time.Now()
	hours := warmHours(time.Now().UTC().Truncate(time.Hour), PrecipForecastHorizon)

	warmed := 0
	grids, err := s.gribPrecip.GridSeries(ctx, precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat, hours)
	if err != nil {
		slog.Warn("grib precipitation series failed, warming per hour", "err", err)
	}
	if len(grids) > 0 {
		for _, grid := range grids {
			if grid == nil || len(grid.Values) == 0 {
				continue
			}
			minV, maxV := fieldGridRange(grid)
			validTime := grid.ObservedAt.UTC()
			s.gribPrecipCache.Set(gribPrecipGridKey(validTime.Truncate(time.Hour)), &PrecipitationForecastGrid{
				DataTime: validTime.Truncate(time.Second),
				Min:      minV,
				Max:      maxV,
				Grid:     grid,
			})
			warmed++
		}
	} else {
		for _, at := range hours {
			if ctx.Err() != nil {
				return
			}
			if _, err := s.GetPrecipitationForecastGrid(ctx, PrecipitationOverlayRequest{Time: at}); err == nil {
				warmed++
			}
		}
	}
	slog.Info("warmed grib precipitation grids",
		"frames", warmed,
		"horizon_hours", PrecipForecastHorizon,
		"duration", time.Since(start),
	)
}

func gribPrecipGridKey(hour time.Time) string {
	return "gribprecipgrid:" + hour.Format(time.RFC3339)
}

// fieldGridRange returns the min and max over the grid's valid (non-nil) cells,
// or (0, 0) when the grid is entirely masked.
func fieldGridRange(grid *FieldGrid) (float64, float64) {
	minV, maxV := math.Inf(1), math.Inf(-1)
	for _, v := range grid.Values {
		if v == nil {
			continue
		}
		if *v < minV {
			minV = *v
		}
		if *v > maxV {
			maxV = *v
		}
	}
	if math.IsInf(minV, 1) {
		return 0, 0
	}
	return minV, maxV
}
