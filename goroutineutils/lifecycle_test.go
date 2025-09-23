package goroutineutils

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycle_AllGoroutinesExit_NoError(t *testing.T) {
	t.Parallel()

	lc := NewLifecycle(context.Background())

	var ran int64
	for i := 0; i < 5; i++ {
		lc.Go(func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt64(&ran, 1)
			return nil
		})
	}

	if err := lc.Wait(); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}
	if got := atomic.LoadInt64(&ran); got != 5 {
		t.Fatalf("ran=%d want=5", got)
	}
}

func TestLifecycle_FirstErrorCancelsContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 3*time.Second)
	defer cancel()

	lc := NewLifecycle(ctx)

	errSentinel := errors.New("boom")
	var sawCancel int64

	// Long runner that should see cancellation after the error.
	lc.Go(func(c context.Context) error {
		<-c.Done()
		atomic.AddInt64(&sawCancel, 1)
		return nil
	})

	// The one that fails first.
	lc.Go(func(c context.Context) error {
		time.Sleep(20 * time.Millisecond)
		return errSentinel
	})

	// More goroutines that should get cancelled.
	for i := 0; i < 3; i++ {
		lc.Go(func(c context.Context) error {
			<-c.Done()
			return nil
		})
	}

	err := lc.Wait()
	if !errors.Is(err, errSentinel) {
		t.Fatalf("Wait()=%v; want sentinel", err)
	}
	if atomic.LoadInt64(&sawCancel) == 0 {
		t.Fatalf("expected at least one goroutine to observe cancellation")
	}
}

func TestLifecycle_StopCancels_NoError(t *testing.T) {
	t.Parallel()

	lc := NewLifecycle(context.Background())

	done := make(chan struct{}, 1)
	lc.Go(func(ctx context.Context) error {
		<-ctx.Done()
		done <- struct{}{}
		return nil
	})

	// Stop should cancel context; Wait should return nil since no task errored.
	lc.Stop()
	select {
	case <-done:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("goroutine did not observe Stop() cancellation")
	}

	if err := lc.Wait(); err != nil {
		t.Fatalf("Wait() error after Stop(): %v", err)
	}
}

func TestLifecycle_StopIsIdempotent(t *testing.T) {
	t.Parallel()

	lc := NewLifecycle(context.Background())
	for i := 0; i < 3; i++ {
		lc.Stop()
	}
	// No goroutines started; Wait should still succeed.
	if err := lc.Wait(); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}
}

func TestLifecycle_WaitCancelsIfNotStopped(t *testing.T) {
	t.Parallel()

	lc := NewLifecycle(context.Background())

	seen := make(chan struct{}, 1)
	lc.Go(func(ctx context.Context) error {
		<-ctx.Done()
		seen <- struct{}{}
		return nil
	})

	// Give the goroutine time to start and block on Done().
	time.Sleep(10 * time.Millisecond)

	if err := lc.Wait(); err != nil {
		t.Fatalf("Wait() error: %v", err)
	}

	select {
	case <-seen:
		// ok, context was cancelled by Wait()
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("goroutine did not observe cancellation from Wait()")
	}
}

func TestLifecycle_ParentCancelPropagates(t *testing.T) {
	t.Parallel()

	parent, stop := context.WithCancel(context.Background())
	lc := NewLifecycle(parent)

	observed := make(chan struct{}, 1)
	lc.Go(func(ctx context.Context) error {
		<-ctx.Done()
		observed <- struct{}{}
		return nil
	})

	stop() // cancel parent
	if err := lc.Wait(); err != nil {
		t.Fatalf("Wait() error after parent cancel: %v", err)
	}

	select {
	case <-observed:
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("managed goroutine did not observe parent cancel")
	}
}

func TestLifecycle_RecordsOnlyFirstError(t *testing.T) {
	t.Parallel()

	lc := NewLifecycle(context.Background())
	err1 := errors.New("first")
	err2 := errors.New("second")

	lc.Go(func(ctx context.Context) error { return err1 })
	lc.Go(func(ctx context.Context) error {
		<-ctx.Done()
		return err2
	})

	got := lc.Wait()
	if !errors.Is(got, err1) {
		t.Fatalf("Wait()=%v; want first error", got)
	}
}
