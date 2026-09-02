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
		Values:     []float32{1},
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
			Values:     []float32{1},
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
		gribGridCache: NewCache[*FieldGrid](0),
	}
}

// seriesCalls is how many chunked GridSeries calls warming a window of n
// hourly frames takes.
func seriesCalls(frames int) int {
	return (frames + warmSeriesChunkHours - 1) / warmSeriesChunkHours
}

func TestWarmTemperatureGrids_PopulatesEveryFrameInChunkedCalls(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	want := seriesCalls(MapForecastHorizon + warmHorizonSlack + 1)
	if grid, series := src.counts(); series != want || grid != 0 {
		t.Fatalf("expected %d series calls and 0 per-hour calls, got %d/%d", want, series, grid)
	}

	// After warming, a client request for any hour through the horizon must be
	// a cache hit (no additional upstream call).
	base := time.Now().UTC().Truncate(time.Hour)
	for _, offset := range []int{0, 3, MapForecastHorizon, MapForecastHorizon + warmHorizonSlack} {
		if grid := svc.gribTemperatureGrid(base.Add(time.Duration(offset) * time.Hour)); grid == nil {
			t.Fatalf("expected warmed grid for +%dh, got nil", offset)
		}
	}
	if grid, series := src.counts(); series != want || grid != 0 {
		t.Fatalf("expected no extra upstream call after warm, got %d/%d", series, grid)
	}
}

func TestWarmTemperatureGrids_SkipsPerHourWhenSeriesTimesOut(t *testing.T) {
	src := &countingTempSource{seriesErr: context.DeadlineExceeded}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	if grid, _ := src.counts(); grid != 0 {
		t.Fatalf("expected no per-hour fallback after a series timeout, got %d", grid)
	}
}

func TestWarmTemperatureGrids_PrunesHoursOutsideWindowAndKeepsCurrent(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)
	base := time.Now().UTC().Truncate(time.Hour)
	stale := base.Add(-2 * time.Hour)
	svc.gribGridCache.Set(gribTempGridKey(stale), &FieldGrid{Rows: 1, Cols: 1, Values: []float32{9}})

	svc.WarmTemperatureGrids(context.Background())
	if grid := svc.gribTemperatureGrid(stale); grid != nil {
		t.Fatal("expected hour before the window to be pruned")
	}

	// A failed refresh must keep serving the previous grids.
	src.seriesErr = context.DeadlineExceeded
	svc.WarmTemperatureGrids(context.Background())
	if grid := svc.gribTemperatureGrid(base.Add(3 * time.Hour)); grid == nil {
		t.Fatal("expected previously warmed grid to survive a failed warm")
	}
}

func TestWarmTemperatureGrids_FallsBackWhenSeriesFails(t *testing.T) {
	src := &countingTempSource{seriesErr: errors.New("boom")}
	svc := newWarmTestService(src)

	svc.WarmTemperatureGrids(context.Background())

	// One extract per hourly frame from now through the horizon plus slack (inclusive).
	want := MapForecastHorizon + warmHorizonSlack + 1
	if grid, _ := src.counts(); grid != want {
		t.Fatalf("expected %d per-hour fallback calls, got %d", want, grid)
	}
	base := time.Now().UTC().Truncate(time.Hour)
	if grid := svc.gribTemperatureGrid(base.Add(3 * time.Hour)); grid == nil {
		t.Fatal("expected warmed grid for +3h, got nil")
	}
}

func TestGribTemperatureGrid_NeverExtractsOnMiss(t *testing.T) {
	src := &countingTempSource{}
	svc := newWarmTestService(src)

	at := time.Now().UTC().Truncate(time.Hour).Add(3 * time.Hour)
	if grid := svc.gribTemperatureGrid(at); grid != nil {
		t.Fatalf("expected nil for unwarmed hour, got %+v", grid)
	}
	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no upstream call on miss, got %d/%d", grid, series)
	}

	_, err := svc.GetTemperatureSamplesAt(context.Background(), at)
	if !errors.Is(err, ErrForecastGridUnavailable) {
		t.Fatalf("expected ErrForecastGridUnavailable, got %v", err)
	}
	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no upstream call from samples path, got %d/%d", grid, series)
	}
}

// orderedSource logs each series call under a layer name into a shared log,
// so a test can assert the order in which layers and chunks are warmed.
type orderedSource struct {
	name string
	log  *[]string
}

func (o orderedSource) Grid(ctx context.Context, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error) {
	return nil, time.Time{}, errors.New("per-hour path not expected")
}

func (o orderedSource) GridSeries(ctx context.Context, minLon, minLat, maxLon, maxLat float64, times []time.Time) ([]*FieldGrid, error) {
	*o.log = append(*o.log, o.name)
	grids := make([]*FieldGrid, 0, len(times))
	for _, at := range times {
		grids = append(grids, &FieldGrid{Rows: 1, Cols: 1, Values: []float32{1}, ObservedAt: at})
	}
	return grids, nil
}

func TestWarmGribGrids_FirstChunksFirstPrecipAheadOfTemperature(t *testing.T) {
	var log []string
	svc := &Service{
		gribTemp:        orderedSource{"temperature", &log},
		gribGridCache:   NewCache[*FieldGrid](0),
		gribPrecip:      orderedSource{"precipitation", &log},
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](0),
	}

	svc.WarmGribGrids(context.Background())

	precipCalls := seriesCalls(PrecipForecastHorizon + warmHorizonSlack + 1)
	tempCalls := seriesCalls(MapForecastHorizon + warmHorizonSlack + 1)
	if len(log) != precipCalls+tempCalls {
		t.Fatalf("expected %d series calls, got %d: %v", precipCalls+tempCalls, len(log), log)
	}
	if log[0] != "precipitation" || log[1] != "temperature" {
		t.Fatalf("expected first chunks of precipitation then temperature, got %v", log[:2])
	}
	for i := 2; i < 1+precipCalls; i++ {
		if log[i] != "precipitation" {
			t.Fatalf("expected remaining precipitation chunks before temperature, got %v", log)
		}
	}
	for i := 1 + precipCalls; i < len(log); i++ {
		if log[i] != "temperature" {
			t.Fatalf("expected temperature chunks last, got %v", log)
		}
	}

	base := time.Now().UTC().Truncate(time.Hour)
	if svc.gribTemperatureGrid(base.Add(time.Duration(MapForecastHorizon)*time.Hour)) == nil {
		t.Fatal("expected temperature horizon warmed")
	}
	if _, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{Time: base.Add(time.Duration(PrecipForecastHorizon) * time.Hour)}); err != nil {
		t.Fatalf("expected precipitation horizon warmed, got %v", err)
	}
}

func TestWarmTemperatureGrids_NoSourceIsNoop(t *testing.T) {
	svc := &Service{gribGridCache: NewCache[*FieldGrid](0)}
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
		Values:     []float32{0.5},
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
			Values:     []float32{0.5},
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

func TestWarmPrecipitationGrids_PopulatesEveryFrameInChunkedCalls(t *testing.T) {
	src := &countingPrecipSource{}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](0),
	}

	svc.WarmPrecipitationGrids(context.Background())

	want := seriesCalls(PrecipForecastHorizon + warmHorizonSlack + 1)
	if grid, series := src.counts(); series != want || grid != 0 {
		t.Fatalf("expected %d series calls and 0 per-hour calls, got %d/%d", want, series, grid)
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
	if grid, series := src.counts(); series != want || grid != 0 {
		t.Fatalf("expected no extra upstream call after warm, got %d/%d", series, grid)
	}
}

func TestWarmPrecipitationGrids_FallsBackWhenSeriesEmpty(t *testing.T) {
	src := &countingPrecipSource{seriesEmpty: true}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](0),
	}

	svc.WarmPrecipitationGrids(context.Background())

	// One extract per hourly frame from now through the horizon plus slack (inclusive).
	want := PrecipForecastHorizon + warmHorizonSlack + 1
	if grid, _ := src.counts(); grid != want {
		t.Fatalf("expected %d per-hour fallback calls, got %d", want, grid)
	}
}

func TestGetPrecipitationForecastGrid_NeverExtractsOnMiss(t *testing.T) {
	src := &countingPrecipSource{}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](0),
	}

	at := time.Now().UTC().Truncate(time.Hour).Add(3 * time.Hour)
	_, err := svc.GetPrecipitationForecastGrid(context.Background(), PrecipitationOverlayRequest{Time: at})
	if !errors.Is(err, ErrPrecipitationDisabled) {
		t.Fatalf("expected ErrPrecipitationDisabled for unwarmed hour, got %v", err)
	}
	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no upstream call on miss, got %d/%d", grid, series)
	}
}

func TestWarmPrecipitationGrids_NoSourceIsNoop(t *testing.T) {
	svc := &Service{gribPrecipCache: NewCache[*PrecipitationForecastGrid](0)}
	// Must not panic when the GRIB source is unconfigured.
	svc.WarmPrecipitationGrids(context.Background())
}

func TestWarmPrecipitationGrids_StopsOnCancel(t *testing.T) {
	src := &countingPrecipSource{}
	svc := &Service{
		gribPrecip:      src,
		gribPrecipCache: NewCache[*PrecipitationForecastGrid](0),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.WarmPrecipitationGrids(ctx)

	if grid, series := src.counts(); grid != 0 || series != 0 {
		t.Fatalf("expected no calls after cancel, got %d/%d", grid, series)
	}
}
