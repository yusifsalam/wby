package weather

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// countingTempSource records per-hour and series requests separately so tests
// can assert warming reaches gribsvc in one batch call, and falls back to the
// per-hour path only when the series fails.
type countingTempSource struct {
	mu          sync.Mutex
	hours       []time.Time
	seriesCalls int
	seriesErr   error
	seriesEmpty bool
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

func (c *countingTempSource) GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error) {
	c.mu.Lock()
	c.seriesCalls++
	c.mu.Unlock()
	if c.seriesErr != nil {
		return nil, c.seriesErr
	}
	if c.seriesEmpty {
		return nil, nil
	}
	grids := make([]*FieldGrid, 0, len(times))
	for _, at := range times {
		grids = append(grids, &FieldGrid{
			Rows: 1, Cols: 1,
			Values:     []*float64{ptr(1.0)},
			ObservedAt: at,
		})
	}
	return grids, nil
}

func (c *countingTempSource) counts() (grid, series int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hours), c.seriesCalls
}

func newWarmTestService(src GribTemperatureSource) *Service {
	return &Service{
		gribTemp:      src,
		gribGridCache: NewCache[*FieldGrid](gribGridCacheTTL),
	}
}

func TestWarmTemperatureGrids_PopulatesEveryFrameInOneCall(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	if grid, series := src.counts(); series != 1 || grid != 0 {
		t.Fatalf("expected 1 series call and 0 per-hour calls, got %d/%d", series, grid)
	}

	// After warming, a client request for any hour through the horizon must be
	// a cache hit (no additional upstream call).
	base := time.Now().UTC().Truncate(time.Hour)
	for _, offset := range []int{0, 3, MapForecastHorizon} {
		if grid := svc.gribTemperatureGrid(context.Background(), 19, 59, 32, 71, base.Add(time.Duration(offset)*time.Hour)); grid == nil {
			t.Fatalf("expected warmed grid for +%dh, got nil", offset)
		}
	}
	if grid, series := src.counts(); series != 1 || grid != 0 {
		t.Fatalf("expected no extra upstream call after warm, got %d/%d", series, grid)
	}
}

func TestWarmTemperatureGrids_FallsBackWhenSeriesFails(t *testing.T) {
	src := &countingTempSource{seriesErr: errors.New("boom")}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	// One extract per hourly frame from now through the horizon (inclusive).
	if grid, _ := src.counts(); grid != MapForecastHorizon+1 {
		t.Fatalf("expected %d per-hour fallback calls, got %d", MapForecastHorizon+1, grid)
	}
	base := time.Now().UTC().Truncate(time.Hour)
	if grid := svc.gribTemperatureGrid(context.Background(), 19, 59, 32, 71, base.Add(3*time.Hour)); grid == nil {
		t.Fatal("expected warmed grid for +3h, got nil")
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

	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no calls after cancel, got %d/%d", grid, series)
	}
}

// countingPrecipSource records the bboxes requested alongside the hours, so the
// precipitation warm tests can assert the fixed full-Finland extraction extent.
type countingPrecipSource struct {
	mu          sync.Mutex
	hours       []time.Time
	bboxes      [][4]float64
	seriesCalls int
	seriesEmpty bool
}

func (c *countingPrecipSource) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error) {
	c.mu.Lock()
	c.hours = append(c.hours, at)
	c.bboxes = append(c.bboxes, [4]float64{minLon, minLat, maxLon, maxLat})
	c.mu.Unlock()
	return &FieldGrid{
		Rows: 1, Cols: 1,
		Values:     []*float64{ptr(0.5)},
		ObservedAt: at,
	}, at, nil
}

func (c *countingPrecipSource) GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error) {
	c.mu.Lock()
	c.seriesCalls++
	c.bboxes = append(c.bboxes, [4]float64{minLon, minLat, maxLon, maxLat})
	c.mu.Unlock()
	if c.seriesEmpty {
		return nil, nil
	}
	grids := make([]*FieldGrid, 0, len(times))
	for _, at := range times {
		grids = append(grids, &FieldGrid{
			Rows: 1, Cols: 1,
			Values:     []*float64{ptr(0.5)},
			ObservedAt: at,
		})
	}
	return grids, nil
}

func (c *countingPrecipSource) counts() (grid, series int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.hours), c.seriesCalls
}

func TestWarmPrecipitationGrids_PopulatesEveryFrameInOneCall(t *testing.T) {
	src := &countingPrecipSource{}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](gribGridCacheTTL),
	}

	svc.WarmPrecipitationGrids(context.Background())

	if grid, series := src.counts(); series != 1 || grid != 0 {
		t.Fatalf("expected 1 series call and 0 per-hour calls, got %d/%d", series, grid)
	}

	// The series extract must use the fixed full-Finland extent.
	for _, b := range src.bboxes {
		if b != [4]float64{precipGridMinLon, precipGridMinLat, precipGridMaxLon, precipGridMaxLat} {
			t.Fatalf("expected fixed extraction extent, got %v", b)
		}
	}

	// After warming, a client request for any of those hours must be a cache hit
	// (no additional upstream call), whatever bbox it carries.
	base := time.Now().UTC().Truncate(time.Hour)
	got, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{
		MapOverlayRequest: MapOverlayRequest{MinLon: 19, MinLat: 59, MaxLon: 32, MaxLat: 71.5, Width: 100, Height: 100},
		Time:              base.Add(3 * time.Hour),
	})
	if err != nil || got == nil {
		t.Fatalf("expected warmed grid for +3h, got %v (err %v)", got, err)
	}
	if grid, series := src.counts(); series != 1 || grid != 0 {
		t.Fatalf("expected no extra upstream call after warm, got %d/%d", series, grid)
	}
}

func TestWarmPrecipitationGrids_FallsBackWhenSeriesEmpty(t *testing.T) {
	src := &countingPrecipSource{seriesEmpty: true}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](gribGridCacheTTL),
	}

	svc.WarmPrecipitationGrids(context.Background())

	// One extract per hourly frame from now through the horizon (inclusive).
	if grid, _ := src.counts(); grid != PrecipForecastHorizon+1 {
		t.Fatalf("expected %d per-hour fallback calls, got %d", PrecipForecastHorizon+1, grid)
	}
}

func TestWarmPrecipitationGrids_NoSourceIsNoop(t *testing.T) {
	svc := &Service{gribPrecipCache: NewCache[*PrecipitationForecastGrid](gribGridCacheTTL)}
	// Must not panic when the GRIB source is unconfigured.
	svc.WarmPrecipitationGrids(context.Background())
}

func TestWarmPrecipitationGrids_StopsOnCancel(t *testing.T) {
	src := &countingPrecipSource{}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](gribGridCacheTTL),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.WarmPrecipitationGrids(ctx)

	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no calls after cancel, got %d/%d", grid, series)
	}
}
