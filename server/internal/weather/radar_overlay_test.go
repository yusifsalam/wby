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

func TestObservationGridCachesPerFrame(t *testing.T) {
	s := newRadarTestService()
	at := time.Now().UTC().Truncate(radarFrameStep).Add(-2 * radarFrameStep)
	src := &fakeRadarSource{frames: map[string]*FieldGrid{
		RadarFrameFile(at): radarTestGrid(0.4),
	}}
	s.SetRadarPrecipitationSource(src)

	req := PrecipitationOverlayRequest{Time: at}
	if _, err := s.GetPrecipitationObservationGrid(context.Background(), req); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := s.GetPrecipitationObservationGrid(context.Background(), req); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if len(src.calls) != 1 {
		t.Fatalf("source called %d times, want 1 (cache miss only)", len(src.calls))
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
