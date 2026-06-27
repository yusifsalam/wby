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
	_, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 24, MinLat: 60, MaxLon: 25, MaxLat: 61, Width: 100, Height: 100},
	})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled for missing grid, got %v", err)
	}
}

func TestGetPrecipitationForecastGrid_RangeAndCache(t *testing.T) {
	at := time.Date(2026, 6, 27, 14, 0, 0, 0, time.UTC)
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
}
