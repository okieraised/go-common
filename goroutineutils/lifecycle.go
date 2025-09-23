package goroutineutils

import (
	"context"
	"sync"
)

// Lifecycle manages a set of long-running goroutines tied to a cancellable context.
// On the first error from any goroutine, Lifecycle cancels its context.
type Lifecycle struct {
	ctx    context.Context
	cancel context.CancelFunc

	wg   sync.WaitGroup
	mu   sync.Mutex
	err  error
	once sync.Once
}

// NewLifecycle creates a Lifecycle bound to parent.
func NewLifecycle(parent context.Context) *Lifecycle {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Lifecycle{ctx: ctx, cancel: cancel}
}

// Context returns the lifecycle context (canceled on first error or Stop()).
func (lc *Lifecycle) Context() context.Context { return lc.ctx }

// Go starts a managed goroutine. If fn returns an error, it is recorded once
// and the lifecycle context is canceled.
func (lc *Lifecycle) Go(fn func(ctx context.Context) error) {
	lc.wg.Add(1)
	go func() {
		defer lc.wg.Done()
		if err := fn(lc.ctx); err != nil {
			lc.setErr(err)
		}
	}()
}

// Stop cancels the lifecycle's context. It can be called multiple times.
func (lc *Lifecycle) Stop() { lc.cancel() }

// Wait waits for all managed goroutines to exit and returns the first error (if any).
func (lc *Lifecycle) Wait() error {
	// Ensure we only cancel once, and do it BEFORE waiting so any goroutines
	// blocked on <-ctx.Done() can exit.
	lc.once.Do(func() { lc.cancel() })

	lc.wg.Wait()

	lc.mu.Lock()
	defer lc.mu.Unlock()
	return lc.err
}

func (lc *Lifecycle) setErr(err error) {
	if err == nil {
		return
	}
	lc.mu.Lock()
	if lc.err == nil {
		lc.err = err
		lc.cancel()
	}
	lc.mu.Unlock()
}
