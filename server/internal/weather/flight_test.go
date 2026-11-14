package weather

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlightGroupSharesOneCallPerKey(t *testing.T) {
	var g flightGroup[int]
	var calls, leaders atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{})

	const callers = 10
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, leader, err := g.Do(context.Background(), "k", func(context.Context) (int, error) {
				if calls.Add(1) == 1 {
					close(started)
				}
				<-release
				return 42, nil
			})
			if err != nil || val != 42 {
				t.Errorf("Do = %d, %v; want 42, nil", val, err)
			}
			if leader {
				leaders.Add(1)
			}
		}()
	}
	<-started
	waitFor(t, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return len(g.calls) == 1
	})
	time.Sleep(20 * time.Millisecond)
	close(release)
	wg.Wait()

	if calls.Load() != 1 {
		t.Errorf("fn ran %d times, want 1", calls.Load())
	}
	if leaders.Load() != 1 {
		t.Errorf("%d callers were leader, want 1", leaders.Load())
	}

	if _, leader, _ := g.Do(context.Background(), "k", func(context.Context) (int, error) { return 1, nil }); !leader {
		t.Error("a call after the shared one finished should start its own")
	}
}

func TestFlightGroupJoinerGetsResultAfterLeaderLeaves(t *testing.T) {
	var g flightGroup[string]
	release := make(chan struct{})
	fnCtxErr := make(chan error, 1)

	leaderCtx, cancel := context.WithCancel(context.Background())
	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		_, _, err := g.Do(leaderCtx, "k", func(ctx context.Context) (string, error) {
			<-release
			fnCtxErr <- ctx.Err()
			return "done", nil
		})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("canceled starter got %v, want context.Canceled", err)
		}
	}()
	waitFor(t, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		return len(g.calls) == 1
	})
	cancel()
	<-leaderDone

	joined := make(chan string, 1)
	go func() {
		val, leader, err := g.Do(context.Background(), "k", func(context.Context) (string, error) {
			return "own call", nil
		})
		if leader || err != nil {
			t.Errorf("joiner: leader=%v err=%v, want false, nil", leader, err)
		}
		joined <- val
	}()
	time.Sleep(20 * time.Millisecond)
	close(release)

	if got := <-joined; got != "done" {
		t.Errorf("joiner got %q, want the running call's result", got)
	}
	if err := <-fnCtxErr; err != nil {
		t.Errorf("shared call saw ctx error %v after its starter left, want nil", err)
	}
}

func TestFlightGroupTurnsPanicIntoError(t *testing.T) {
	var g flightGroup[int]
	_, _, err := g.Do(context.Background(), "k", func(context.Context) (int, error) {
		panic("boom")
	})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("err = %v, want one naming the panic", err)
	}
	if val, _, err := g.Do(context.Background(), "k", func(context.Context) (int, error) { return 7, nil }); err != nil || val != 7 {
		t.Fatalf("after a panic: Do = %d, %v; want 7, nil", val, err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}
