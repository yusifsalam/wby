package fetcher

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"wby/internal/fmi"
	"wby/internal/weather"
)

type fakeFMI struct {
	result *fmi.ObservationResult
	err    error
}

func (f *fakeFMI) FetchObservations(ctx context.Context) (*fmi.ObservationResult, error) {
	return f.result, f.err
}

func (f *fakeFMI) FetchGRIB(ctx context.Context, producer, params, bbox string, w io.Writer) error {
	return errors.New("not used")
}

type fakeStore struct {
	upsertErr    error
	observations int
}

func (s *fakeStore) UpsertStations(ctx context.Context, stations []weather.Station) error {
	return nil
}

func (s *fakeStore) UpsertObservations(ctx context.Context, observations []weather.Observation) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.observations += len(observations)
	return nil
}

func TestFetchObservationsRecordsStatusOnlyOnSuccess(t *testing.T) {
	newest := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	source := &fakeFMI{result: &fmi.ObservationResult{
		Stations: []weather.Station{{FMISID: 100971}},
		Observations: []weather.Observation{
			{FMISID: 100971, ObservedAt: newest.Add(-10 * time.Minute)},
			{FMISID: 100971, ObservedAt: newest},
		},
	}}
	store := &fakeStore{}
	f := New(source, store)

	if got := f.Status(); !got.ObservationsFetchedAt.IsZero() || !got.NewestObservationAt.IsZero() {
		t.Fatalf("status before any fetch = %+v, want zero", got)
	}

	before := time.Now()
	f.fetchObservations(context.Background())
	got := f.Status()
	if got.ObservationsFetchedAt.Before(before) || !got.NewestObservationAt.Equal(newest) {
		t.Fatalf("status = %+v, want a fetch time from now and newest %v", got, newest)
	}
	if store.observations != 2 {
		t.Fatalf("stored %d observations, want 2", store.observations)
	}

	source.err = errors.New("FMI unreachable")
	f.fetchObservations(context.Background())
	if after := f.Status(); after != got {
		t.Errorf("a failed fetch changed the status: %+v -> %+v", got, after)
	}

	source.err = nil
	store.upsertErr = errors.New("database unreachable")
	f.fetchObservations(context.Background())
	if after := f.Status(); after != got {
		t.Errorf("a failed upsert changed the status: %+v -> %+v", got, after)
	}
}

func TestCycleBoundsOnePassByTheInterval(t *testing.T) {
	var deadline time.Duration
	cycle(context.Background(), 50*time.Millisecond, func(ctx context.Context) {
		d, ok := ctx.Deadline()
		if !ok {
			t.Fatal("a cycle should run under a deadline")
		}
		deadline = time.Until(d)
		<-ctx.Done()
	})
	if deadline <= 0 || deadline > 50*time.Millisecond {
		t.Errorf("deadline in %v, want within the 50ms interval", deadline)
	}
}

func fastRestarts(t *testing.T) {
	t.Helper()
	savedMin, savedMax := superviseMinBackoff, superviseMaxBackoff
	superviseMinBackoff, superviseMaxBackoff = time.Millisecond, 4*time.Millisecond
	t.Cleanup(func() { superviseMinBackoff, superviseMaxBackoff = savedMin, savedMax })
}

func TestSuperviseRestartsALoopThatPanics(t *testing.T) {
	fastRestarts(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var runs atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		Supervise(ctx, "test", func(ctx context.Context) {
			switch runs.Add(1) {
			case 1:
				panic("boom")
			case 2:
				return
			default:
				cancel()
				<-ctx.Done()
			}
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Supervise did not return after its context ended")
	}
	if got := runs.Load(); got != 3 {
		t.Errorf("loop ran %d times, want 3: once to panic, once to return early, once until canceled", got)
	}
}

func TestSuperviseStopsWhileWaitingToRestart(t *testing.T) {
	savedMin := superviseMinBackoff
	superviseMinBackoff = time.Hour
	defer func() { superviseMinBackoff = savedMin }()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		Supervise(ctx, "test", func(context.Context) { panic("boom") })
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Supervise kept waiting to restart after its context ended")
	}
}
