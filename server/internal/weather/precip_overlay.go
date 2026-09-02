package weather

import (
	"context"
	"fmt"
	"log/slog"
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
// Grids are served from the per-hour cache populated by WarmPrecipitationGrids.
// Returns ErrPrecipitationDisabled when the GRIB source is unconfigured or the
// hour isn't cached, so the handler can answer 404.
func (s *Service) GetPrecipitationForecastGrid(ctx context.Context, req PrecipitationOverlayRequest) (*PrecipitationForecastGrid, error) {
	if s.gribPrecip == nil {
		return nil, ErrPrecipitationDisabled
	}

	target := req.Time.UTC().Truncate(time.Hour)
	if target.IsZero() {
		target = time.Now().UTC().Truncate(time.Hour)
	}

	if cached, ok := s.gribPrecipCache.Get(gribPrecipGridKey(target)); ok {
		return cached, nil
	}
	return nil, ErrPrecipitationDisabled
}

// fetchPrecipitationGrid extracts one hour from gribsvc and caches it. A nil
// result with a nil error is a soft miss.
func (s *Service) fetchPrecipitationGrid(ctx context.Context, target time.Time) (*PrecipitationForecastGrid, error) {
	grid, validTime, err := s.gribPrecip.Grid(ctx, precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat, target)
	if err != nil {
		return nil, fmt.Errorf("fetch precipitation grid: %w", err)
	}
	if grid == nil || len(grid.Values) == 0 {
		return nil, nil
	}
	if validTime.IsZero() {
		validTime = target
	}

	minV, maxV := grid.GridRange()
	out := &PrecipitationForecastGrid{
		DataTime: validTime.UTC().Truncate(time.Second),
		Min:      minV,
		Max:      maxV,
		Grid:     grid,
	}
	s.gribPrecipCache.Set(gribPrecipGridKey(target), out)
	return out, nil
}

// WarmPrecipitationGrids pre-populates the precipitation forecast grid cache
// for every hourly frame (the current hour through PrecipForecastHorizon plus
// warmHorizonSlack). It is the only path that extracts from gribsvc; it runs
// at startup and after each GRIB download attempt, alongside
// WarmTemperatureGrids. All frames come from a single GridSeries call (one
// pass over the GRIB file); if that fails or returns nothing, warming falls
// back to one extract per hour.
func (s *Service) WarmPrecipitationGrids(ctx context.Context) {
	if s.gribPrecip == nil || ctx.Err() != nil {
		return
	}

	start := time.Now()
	hours := warmHours(time.Now().UTC().Truncate(time.Hour), PrecipForecastHorizon+warmHorizonSlack)
	warmed := s.warmPrecipitationHours(ctx, hours)
	pruneWarmCache(s.gribPrecipCache, gribPrecipGridKey, hours)
	slog.Info("warmed grib precipitation grids",
		"frames", warmed,
		"horizon_hours", PrecipForecastHorizon,
		"duration", time.Since(start),
	)
}

func (s *Service) warmPrecipitationHours(ctx context.Context, hours []time.Time) int {
	if s.gribPrecip == nil || len(hours) == 0 {
		return 0
	}
	return warmGridSeries(ctx, "precipitation", hours,
		func(ctx context.Context, times []time.Time) ([]*FieldGrid, error) {
			return s.gribPrecip.GridSeries(ctx, precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat, times)
		},
		func(grid *FieldGrid) {
			minV, maxV := grid.GridRange()
			validTime := grid.ObservedAt.UTC()
			s.gribPrecipCache.Set(gribPrecipGridKey(validTime.Truncate(time.Hour)), &PrecipitationForecastGrid{
				DataTime: validTime.Truncate(time.Second),
				Min:      minV,
				Max:      maxV,
				Grid:     grid,
			})
		},
		func(ctx context.Context, at time.Time) bool {
			grid, err := s.fetchPrecipitationGrid(ctx, at)
			return err == nil && grid != nil
		},
	)
}

func gribPrecipGridKey(hour time.Time) string {
	return "gribprecipgrid:" + hour.Format(time.RFC3339)
}
