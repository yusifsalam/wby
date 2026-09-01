package weather

import (
	"context"
	"testing"
	"time"
)

type fakeRadarSource struct {
	// frames maps gribsvc file names to grids; absent names are soft misses.
	frames map[string]*FieldGrid
	calls  []string
}

func (f *fakeRadarSource) GridForFile(ctx context.Context, file string, minLon, minLat, maxLon, maxLat float64, at time.Time) (*FieldGrid, time.Time, error) {
	f.calls = append(f.calls, file)
	grid, ok := f.frames[file]
	if !ok {
		return nil, time.Time{}, nil
	}
	return grid, time.Time{}, nil
}

func radarTestGrid(v float64) *FieldGrid {
	return &FieldGrid{Rows: 1, Cols: 2, Values: []*float64{&v, nil}}
}

func newRadarTestService() *Service {
	return NewService(nil, nil, time.Minute)
}

func TestRadarFrameFileName(t *testing.T) {
	at := time.Date(2026, 9, 1, 16, 5, 0, 0, time.UTC)
	if got := RadarFrameFile(at); got != "radar_rr_20260901T1605Z.tif" {
		t.Fatalf("RadarFrameFile = %q", got)
	}
}

func TestObservationGridDisabledWithoutSource(t *testing.T) {
	s := newRadarTestService()
	if _, err := s.GetPrecipitationObservationGrid(context.Background(), PrecipitationOverlayRequest{}); err != ErrPrecipitationDisabled {
		t.Fatalf("want ErrPrecipitationDisabled, got %v", err)
	}
}

func TestObservationGridWalksBackToNewestFrame(t *testing.T) {
	s := newRadarTestService()
	now := time.Now().UTC().Truncate(radarFrameStep)
	// Latest slot not yet published; the previous one is.
	prev := now.Add(-radarFrameStep)
	src := &fakeRadarSource{frames: map[string]*FieldGrid{
		RadarFrameFile(prev): radarTestGrid(1.5),
	}}
	s.SetRadarPrecipitationSource(src)
	s.WarmRadarGrids(context.Background())

	out, err := s.GetPrecipitationObservationGrid(context.Background(), PrecipitationOverlayRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.DataTime.Equal(prev) {
		t.Fatalf("DataTime = %s, want %s", out.DataTime, prev)
	}
	if out.Max != 1.5 {
		t.Fatalf("Max = %v, want 1.5", out.Max)
	}
}

func TestObservationGridServesCacheOnly(t *testing.T) {
	s := newRadarTestService()
	at := time.Now().UTC().Truncate(radarFrameStep).Add(-2 * radarFrameStep)
	src := &fakeRadarSource{frames: map[string]*FieldGrid{
		RadarFrameFile(at): radarTestGrid(0.4),
	}}
	s.SetRadarPrecipitationSource(src)

	req := PrecipitationOverlayRequest{Time: at}
	if _, err := s.GetPrecipitationObservationGrid(context.Background(), req); err != ErrPrecipitationDisabled {
		t.Fatalf("before warm: want ErrPrecipitationDisabled, got %v", err)
	}
	if len(src.calls) != 0 {
		t.Fatalf("source called %d times before warm, want 0", len(src.calls))
	}

	s.WarmRadarGrids(context.Background())
	calls := len(src.calls)
	if _, err := s.GetPrecipitationObservationGrid(context.Background(), req); err != nil {
		t.Fatalf("after warm: %v", err)
	}
	if len(src.calls) != calls {
		t.Fatalf("request reached the source (%d calls, want %d)", len(src.calls), calls)
	}

	// A second warm pass skips frames already cached.
	s.WarmRadarGrids(context.Background())
	if len(src.calls) != calls+(calls-1) {
		t.Fatalf("second warm re-fetched cached frame: %d calls", len(src.calls))
	}
}

func TestNowcastGridServesCacheOnly(t *testing.T) {
	s := newRadarTestService()
	at := time.Now().UTC().Truncate(radarFrameStep).Add(2 * radarFrameStep)
	src := &fakeRadarSource{frames: map[string]*FieldGrid{
		NowcastFrameFile(at): radarTestGrid(0.7),
	}}
	s.SetRadarPrecipitationSource(src)

	req := PrecipitationOverlayRequest{Time: at}
	if _, err := s.GetPrecipitationNowcastGrid(context.Background(), req); err != ErrPrecipitationDisabled {
		t.Fatalf("before warm: want ErrPrecipitationDisabled, got %v", err)
	}
	if len(src.calls) != 0 {
		t.Fatalf("source called %d times before warm, want 0", len(src.calls))
	}

	s.WarmNowcastGrids(context.Background())
	calls := len(src.calls)
	out, err := s.GetPrecipitationNowcastGrid(context.Background(), req)
	if err != nil {
		t.Fatalf("after warm: %v", err)
	}
	if out.Max != 0.7 || len(src.calls) != calls {
		t.Fatalf("Max = %v, calls = %d (want %d)", out.Max, len(src.calls), calls)
	}
}

func TestObservationGridRejectsOutOfWindowTargets(t *testing.T) {
	s := newRadarTestService()
	s.SetRadarPrecipitationSource(&fakeRadarSource{frames: map[string]*FieldGrid{}})

	now := time.Now().UTC()
	for _, target := range []time.Time{
		now.Add(time.Hour),                 // future: forecast territory
		now.Add(-RadarObsSpan - time.Hour), // pruned past
	} {
		if _, err := s.GetPrecipitationObservationGrid(context.Background(), PrecipitationOverlayRequest{Time: target}); err != ErrPrecipitationDisabled {
			t.Fatalf("target %s: want ErrPrecipitationDisabled, got %v", target, err)
		}
	}
}
