package weather

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// flightTimeout bounds one shared call, which runs detached from the context
// of the request that started it.
const flightTimeout = 60 * time.Second

// flightGroup runs one call per key at a time and hands its result to every
// caller that asked for that key meanwhile. The zero value is ready to use.
type flightGroup[V any] struct {
	mu    sync.Mutex
	calls map[string]*flightCall[V]
}

type flightCall[V any] struct {
	done chan struct{}
	val  V
	err  error
}

// Do returns the result of fn for key, starting fn unless a call for key is
// already running. leader is true for the caller that started it. fn keeps
// running when its callers give up; a caller whose ctx ends first gets ctx's
// error.
func (g *flightGroup[V]) Do(ctx context.Context, key string, fn func(context.Context) (V, error)) (val V, leader bool, err error) {
	g.mu.Lock()
	call, running := g.calls[key]
	if !running {
		call = &flightCall[V]{done: make(chan struct{})}
		if g.calls == nil {
			g.calls = make(map[string]*flightCall[V])
		}
		g.calls[key] = call
		go g.run(context.WithoutCancel(ctx), key, call, fn)
	}
	g.mu.Unlock()

	select {
	case <-call.done:
		return call.val, !running, call.err
	case <-ctx.Done():
		var zero V
		return zero, false, ctx.Err()
	}
}

func (g *flightGroup[V]) run(ctx context.Context, key string, call *flightCall[V], fn func(context.Context) (V, error)) {
	ctx, cancel := context.WithTimeout(ctx, flightTimeout)
	defer func() {
		if r := recover(); r != nil {
			call.err = fmt.Errorf("panic in shared call %q: %v", key, r)
		}
		cancel()
		g.mu.Lock()
		delete(g.calls, key)
		g.mu.Unlock()
		close(call.done)
	}()
	call.val, call.err = fn(ctx)
}
