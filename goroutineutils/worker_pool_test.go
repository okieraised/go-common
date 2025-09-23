package goroutineutils

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func withTimeout(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx, cancel
}

func TestWorkerPool_AllTasksProcessed_NoError(t *testing.T) {
	t.Parallel()
	const nTasks = 100

	ctx, cancel := withTimeout(t, 5*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 3, 16)

	var done int64
	for i := 0; i < nTasks; i++ {
		err := p.Submit(func(ctx context.Context) error {
			time.Sleep(1 * time.Millisecond)
			atomic.AddInt64(&done, 1)
			return nil
		})
		if err != nil {
			t.Fatalf("submit failed unexpectedly: %v", err)
		}
	}

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait() unexpected error: %v", err)
	}
	if got := atomic.LoadInt64(&done); got != nTasks {
		t.Fatalf("processed=%d want=%d", got, nTasks)
	}
}

func TestWorkerPool_FirstErrorCancelsContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 5*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 2, 16)

	errSentinel := errors.New("boom")
	var cancelledObserved int64

	// Task that should be cancelled when the error happens.
	longTask := func(ctx context.Context) error {
		<-ctx.Done()
		atomic.AddInt64(&cancelledObserved, 1)
		return nil
	}

	if err := p.Submit(longTask); err != nil {
		t.Fatalf("submit long task: %v", err)
	}

	if err := p.Submit(func(ctx context.Context) error { return errSentinel }); err != nil {
		t.Fatalf("submit error task: %v", err)
	}

	for i := 0; i < 5; i++ {
		_ = p.Submit(longTask)
	}

	got := p.Wait()
	if got == nil || !errors.Is(got, errSentinel) {
		t.Fatalf("Wait() got=%v; want sentinel error", got)
	}

	if atomic.LoadInt64(&cancelledObserved) == 0 {
		t.Fatalf("expected at least one task to observe cancellation")
	}
}

func TestWorkerPool_SubmitAfterCancelReturnsError(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 5*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 1, 8)

	errSentinel := errors.New("fail fast")
	if err := p.Submit(func(ctx context.Context) error { return errSentinel }); err != nil {
		t.Fatalf("submit: %v", err)
	}

	_ = p.Wait()

	if err := p.Submit(func(ctx context.Context) error { return nil }); err == nil {
		t.Fatalf("expected submit error after cancel; got nil")
	}
}

func TestWorkerPool_BackpressureWithUnbufferedQueue(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 5*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 1, 0)

	unblock := make(chan struct{})
	if err := p.Submit(func(ctx context.Context) error {
		<-unblock
		return nil
	}); err != nil {
		t.Fatalf("submit 1: %v", err)
	}

	doneSubmit := make(chan error, 1)
	go func() {
		doneSubmit <- p.Submit(func(ctx context.Context) error { return nil })
	}()

	select {
	case err := <-doneSubmit:
		t.Fatalf("second submit returned too early (backpressure broken), err=%v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(unblock)

	select {
	case err := <-doneSubmit:
		if err != nil {
			t.Fatalf("second submit error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("second submit did not complete in time after unblock")
	}

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait error: %v", err)
	}
}

func TestWorkerPool_WaitWithoutTasks_IsOK(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 2*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 3, 4)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait() error with no tasks: %v", err)
	}
}

func TestWorkerPool_CloseIdempotent(t *testing.T) {
	t.Parallel()

	ctx, cancel := withTimeout(t, 2*time.Second)
	defer cancel()

	p := NewWorkerPool(ctx, 1, 1)

	_ = p.Submit(func(ctx context.Context) error { return nil })

	p.Close()
	p.Close()
	p.Close()

	if err := p.Wait(); err != nil {
		t.Fatalf("Wait error after multiple Close(): %v", err)
	}
}

func TestWorkerPool_PropagatesParentCancel(t *testing.T) {
	t.Parallel()

	parent, stop := withTimeout(t, 5*time.Second)
	defer stop()

	p := NewWorkerPool(parent, 2, 8)

	observed := make(chan struct{}, 1)
	_ = p.Submit(func(ctx context.Context) error {
		<-ctx.Done()
		observed <- struct{}{}
		return nil
	})

	stop()

	select {
	case <-observed:
	case <-time.After(1 * time.Second):
		t.Fatalf("task did not observe parent cancellation")
	}

	_ = p.Wait()
}
