package weather

import (
	"context"
	"sync"
	"testing"
	"time"
)

// countingTempSource records the hours requested and returns a fresh grid each
// call, counting how often it is hit so tests can assert cache warming reaches
// gribsvc exactly once per frame.
type countingTempSource struct {
	mu    sync.Mutex
	hours []time.Time
}

func (c *countingTempSource) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error) {
	c.mu.Lock()
	c.hours = append(c.hours, at)
	c.mu.Unlock()
	return &FieldGrid{
		Rows: 1, Cols: 1,
		Values:     []*float64{ptr(1.0)},
		ObservedAt: at,
	}, at, nil
}

func (c *countingTempSource) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hours)
}

func newWarmTestService(src GribTemperatureSource) *Service {
	return &Service{
		gribTemp:      src,
		gribGridCache: NewCache[*FieldGrid](gribGridCacheTTL),
	}
}

func TestWarmTemperatureGrids_PopulatesEveryFrame(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	// One extract per hourly frame from now through the horizon (inclusive).
	want := MapForecastHorizon + 1
	if got := src.count(); got != want {
		t.Fatalf("expected %d gribsvc calls, got %d", want, got)
	}

	// After warming, a client request for any of those hours must be a cache hit
	// (no additional upstream call).
	base := time.Now().UTC().Truncate(time.Hour)
	if grid := svc.gribTemperatureGrid(context.Background(), 19, 59, 32, 71, base.Add(3*time.Hour)); grid == nil {
		t.Fatal("expected warmed grid for +3h, got nil")
	}
	if got := src.count(); got != want {
		t.Fatalf("expected no extra upstream call after warm, calls went %d -> %d", want, src.count())
	}
}

func TestWarmTemperatureGrids_NoSourceIsNoop(t *testing.T) {
	svc := &Service{gribGridCache: NewCache[*FieldGrid](gribGridCacheTTL)}
	// Must not panic when the GRIB source is unconfigured.
	svc.WarmTemperatureGrids(context.Background())
}

func TestWarmTemperatureGrids_StopsOnCancel(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.WarmTemperatureGrids(ctx)

	if got := src.count(); got != 0 {
		t.Fatalf("expected no calls after cancel, got %d", got)
	}
}
