package goroutineutils

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
)

// GoSafe runs fn in a goroutine and recovers from panics.
// If onPanic != nil it receives the formatted panic message and stack trace.
func GoSafe(fn func(), onPanic func(err string)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if onPanic != nil {
					onPanic(fmt.Sprintf("panic: %v\n%s", r, string(debug.Stack())))
				}
			}
		}()
		fn()
	}()
}

type Group struct {
	sem    Semaphore
	wg     sync.WaitGroup
	cancel context.CancelFunc

	mu  sync.Mutex
	err error

	ctx context.Context
}

// NewGroup creates a group tied to ctx. If limit>0, concurrency is bounded.
func NewGroup(ctx context.Context, limit int) *Group {
	cctx, cancel := context.WithCancel(ctx)
	var sem Semaphore
	if limit > 0 {
		sem = NewSemaphore(limit)
	}
	return &Group{ctx: cctx, cancel: cancel, sem: sem}
}

// Go starts fn(ctx). On first non-nil error, it is recorded and ctx is canceled.
func (g *Group) Go(fn func(ctx context.Context) error) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if g.sem != nil {
			if err := g.sem.Acquire(g.ctx); err != nil {
				g.setErr(err)
				return
			}
			defer g.sem.Release()
		}
		if err := fn(g.ctx); err != nil {
			g.setErr(err)
		}
	}()
}

func (g *Group) setErr(err error) {
	if err == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err == nil {
		g.err = err
		if g.cancel != nil {
			g.cancel()
		}
	}
}

// Wait waits for all goroutines to finish and returns the first error.
func (g *Group) Wait() error {
	g.wg.Wait()
	if g.cancel != nil {
		g.cancel()
	}
	return g.err
}

// Context returns the group's context (canceled on first error).
func (g *Group) Context() context.Context { return g.ctx }

func ParallelMap[T any, R any](ctx context.Context, items []T, limit int, fn func(context.Context, T, int) (R, error)) ([]R, error) {
	if limit <= 0 {
		limit = 1
	}
	out := make([]R, len(items))
	g := NewGroup(ctx, limit)
	for i := range items {
		i := i
		g.Go(func(ctx context.Context) error {
			r, err := fn(ctx, items[i], i)
			if err != nil {
				return err
			}
			out[i] = r
			return nil
		})
	}
	return out, g.Wait()
}

func ForEach[T any](ctx context.Context, items []T, limit int, fn func(context.Context, T, int) error) error {
	_, err := ParallelMap[T, struct{}](ctx, items, limit, func(ctx context.Context, t T, i int) (struct{}, error) {
		return struct{}{}, fn(ctx, t, i)
	})
	return err
}
