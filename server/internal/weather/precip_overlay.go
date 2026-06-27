package weather

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// precipForecastLayer is the cache/log label for the GRIB-backed precipitation
// forecast field (distinct from the WMS observation/forecast layers).
const precipForecastLayer = "harmonie:precipitationRate"

// PrecipitationForecastGrid is the Harmonie precipitation-rate raster (mm/h) at
// one hour. Clients upload it as a texture and render it with hardware bilinear,
// mirroring the temperature forecast grid.
type PrecipitationForecastGrid struct {
	DataTime time.Time
	Min      float64
	Max      float64
	Grid     *FieldGrid
}

// GetPrecipitationForecastGrid returns the precipitation-rate raster (mm/h) over
// the bbox at the requested hour. Time snaps to the hour; a zero time means the
// current hour. Returns ErrPrecipitationDisabled when the GRIB source is
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

	cacheKey := precipCacheKey(precipForecastLayer, target, req.MapOverlayRequest)
	if cached, ok := s.gribPrecipCache.Get(cacheKey); ok {
		return cached, nil
	}

	grid, validTime, err := s.gribPrecip.Grid(ctx, req.MinLon, req.MinLat, req.MaxLon, req.MaxLat, target)
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
