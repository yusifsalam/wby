package weather

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePrecipSource struct {
	grid      *FieldGrid
	validTime time.Time
	err       error
}

func (f fakePrecipSource) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error) {
	return f.grid, f.validTime, f.err
}

func (f fakePrecipSource) GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error) {
	if f.err != nil || f.grid == nil {
		return nil, f.err
	}
	return []*FieldGrid{f.grid}, nil
}

func newPrecipTestService(src GribPrecipitationSource) *Service {
	return &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](time.Minute),
	}
}

func TestGetPrecipitationForecastGrid_Disabled(t *testing.T) {
	svc := &Service{gribPrecipCache: NewCache[*PrecipitationForecastGrid](time.Minute)}
	_, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 100, Height: 100},
	})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled, got %v", err)
	}
}

func TestGetPrecipitationForecastGrid_SoftMissOnEmpty(t *testing.T) {
	svc := newPrecipTestService(fakePrecipSource{grid: nil})
	svc.WarmPrecipitationGrids(context.Background())
	_, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 100, Height: 100},
	})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled for missing grid, got %v", err)
	}
}

func TestGetPrecipitationForecastGrid_RangeAndCache(t *testing.T) {
	at := time.Now().UTC().Truncate(time.Hour).Add(3 * time.Hour)
	src := fakePrecipSource{
		grid: &FieldGrid{
			Rows: 2, Cols: 2,
			MinLat: 60, MaxLat: 60.1, MinLon: 24, MaxLon: 24.1,
			Values:     []*float64{ptr(0.0), ptr(3.2), nil, ptr(0.6)},
			ObservedAt: at,
		},
		validTime: at,
	}
	svc := newPrecipTestService(src)
	svc.WarmPrecipitationGrids(context.Background())
	req := PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25.5, MaxLat: 61, Width: 120, Height: 120},
		Time:              at,
	}
	got, err := svc.GetPrecipitationForecastGrid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Min != 0.0 || got.Max != 3.2 {
		t.Fatalf("unexpected range: min=%v max=%v", got.Min, got.Max)
	}
	if got.Grid == nil || len(got.Grid.Values) != 4 {
		t.Fatalf("unexpected grid: %+v", got.Grid)
	}
	// Second call with the same key hits the cache (same pointer).
	got2, err := svc.GetPrecipitationForecastGrid(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if got2 != got {
		t.Fatal("expected cached pointer on second call")
	}

	// The cache is keyed by hour only, so a different bbox at the same hour
	// must also hit (web and iOS request different extents; both are served
	// the warmed full-Finland grid).
	otherBBox := PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 18.8, MinLat: 58.8, MaxLon: 32.2, MaxLat: 71.7, Width: 120, Height: 120},
		Time:              at,
	}
	got3, err := svc.GetPrecipitationForecastGrid(context.Background(), otherBBox)
	if err != nil {
		t.Fatalf("unexpected error on other-bbox call: %v", err)
	}
	if got3 != got {
		t.Fatal("expected hour-only cache key to serve the same grid for a different bbox")
	}
}
