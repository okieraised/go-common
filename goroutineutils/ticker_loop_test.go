package goroutineutils

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestTickerLoop_RunsImmediately_ThenTicks_UntilCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var calls atomic.Int64
	started := make(chan struct{}, 1)

	fn := func(ctx context.Context) error {
		if calls.Add(1) == 1 {
			// immediate run
			started <- struct{}{}
		}
		// Cancel after 3 total invocations (1 immediate + 2 ticks)
		if calls.Load() >= 3 {
			cancel()
		}
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- TickerLoop(ctx, 20*time.Millisecond, 3*time.Millisecond, fn)
	}()

	// Should signal first call promptly
	waitOrFail(t, started, 200*time.Millisecond, "TickerLoop did not run immediately")

	// Should exit with ctx error after we cancel
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("TickerLoop returned %v, want context cancellation", err)
		}
	default:
		// wait a bit for the 2 ticks and cancel to propagate
		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("TickerLoop returned %v, want context cancellation", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("TickerLoop did not exit after cancel")
		}
	}

	if got := calls.Load(); got < 3 {
		t.Fatalf("calls=%d, want at least 3 (immediate + ticks)", got)
	}
}

func TestTickerLoop_PropagatesError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var calls atomic.Int64
	sentinel := errors.New("boom")

	fn := func(ctx context.Context) error {
		if calls.Add(1) == 2 {
			return sentinel // fail on second invocation
		}
		return nil
	}

	start := time.Now()
	err := TickerLoop(ctx, 15*time.Millisecond, 0, fn)
	if !errors.Is(err, sentinel) {
		t.Fatalf("TickerLoop err=%v, want sentinel", err)
	}
	// Should fail relatively soon after start (not wait for context)
	if time.Since(start) > time.Second {
		t.Fatalf("TickerLoop took too long to propagate error")
	}
}

func TestTickerLoop_RespectsCancelDuringJitter(t *testing.T) {
	// Use a larger jitter to make it more likely we're inside the jitter sleep.
	ctx, cancel := context.WithCancel(context.Background())

	var calls atomic.Int64
	fn := func(ctx context.Context) error {
		if calls.Add(1) >= 2 {
			// cancel on/around a tick; the loop should stop promptly
			cancel()
		}
		return nil
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- TickerLoop(ctx, 15*time.Millisecond, 5*time.Millisecond, fn)
	}()

	// Should exit with context cancellation.
	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("TickerLoop err=%v, want context.Canceled", err)
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("TickerLoop did not stop after cancel (possibly stuck in jitter)")
	}
}
