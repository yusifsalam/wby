package fetcher

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"
)

// Pauses between restarts of a supervised loop: doubling from the minimum up
// to the maximum, and back to the minimum once a run has lasted that long.
var (
	superviseMinBackoff = 5 * time.Second
	superviseMaxBackoff = 5 * time.Minute
)

// Supervise runs run until ctx is done, restarting it after a panic or an
// early return.
func Supervise(ctx context.Context, name string, run func(context.Context)) {
	backoff := superviseMinBackoff
	for {
		started := time.Now()
		runRecovering(ctx, name, run)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) >= superviseMaxBackoff {
			backoff = superviseMinBackoff
		}
		slog.Error("background loop exited, restarting", "loop", name, "restart_in", backoff.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, superviseMaxBackoff)
	}
}

func runRecovering(ctx context.Context, name string, run func(context.Context)) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("background loop panicked", "loop", name, "panic", fmt.Sprint(r), "stack", string(debug.Stack()))
		}
	}()
	run(ctx)
}
