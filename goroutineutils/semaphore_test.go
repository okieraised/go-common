package goroutineutils

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSemaphore_AcquireRelease_AllowsUpToN(t *testing.T) {
	sem := NewSemaphore(2)

	// Acquire twice should succeed immediately.
	if err := sem.Acquire(context.Background()); err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if err := sem.Acquire(context.Background()); err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}

	// Third acquire should block until one is released.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := sem.Acquire(ctx); err != nil {
			t.Errorf("third acquire should have waited until release: %v", err)
		}
	}()

	// Release one after a short delay to unblock third.
	time.Sleep(10 * time.Millisecond)
	sem.Release()

	select {
	case <-done:
		// Good, it unblocked.
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for third acquire to unblock after release")
	}
}

func TestSemaphore_TryAcquire(t *testing.T) {
	sem := NewSemaphore(1)

	if ok := sem.TryAcquire(); !ok {
		t.Fatalf("TryAcquire should succeed when semaphore not full")
	}
	if ok := sem.TryAcquire(); ok {
		t.Fatalf("TryAcquire should fail when semaphore is full")
	}

	sem.Release()

	if ok := sem.TryAcquire(); !ok {
		t.Fatalf("TryAcquire should succeed again after release")
	}
}

func TestSemaphore_Acquire_ContextCancel(t *testing.T) {
	sem := NewSemaphore(1)

	// Fill the semaphore so Acquire will block
	if !sem.TryAcquire() {
		t.Fatalf("failed to acquire initial slot")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := sem.Acquire(ctx)
	if err == nil {
		t.Fatalf("expected Acquire to fail due to context timeout")
	}
	if time.Since(start) < 20*time.Millisecond {
		t.Fatalf("Acquire returned too early: %v", err)
	}
}

func TestSemaphore_ReleaseWithoutAcquireSafe(t *testing.T) {
	sem := NewSemaphore(1)

	// Releasing when semaphore is empty should not panic or block.
	sem.Release()

	// We should still be able to acquire (capacity should not be exceeded).
	if ok := sem.TryAcquire(); !ok {
		t.Fatalf("semaphore should still allow one acquire after extra release")
	}
}

func TestSemaphore_AllowsParallelismLimit(t *testing.T) {
	const n = 3
	sem := NewSemaphore(n)

	var inFlight int32
	var maxSeen int32

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := sem.Acquire(ctx); err != nil {
				t.Errorf("acquire failed: %v", err)
				return
			}
			cur := atomic.AddInt32(&inFlight, 1)
			if cur > atomic.LoadInt32(&maxSeen) {
				atomic.StoreInt32(&maxSeen, cur)
			}
			time.Sleep(10 * time.Millisecond) // simulate work
			atomic.AddInt32(&inFlight, -1)
			sem.Release()
		}()
	}
	wg.Wait()

	if max := atomic.LoadInt32(&maxSeen); max > n {
		t.Fatalf("saw %d concurrent acquires, want <= %d", max, n)
	}
}

func TestNewSemaphore_NonPositiveCreatesCapacityOne(t *testing.T) {
	sem := NewSemaphore(0) // should fallback to capacity=1
	if !sem.TryAcquire() {
		t.Fatalf("expected acquire to succeed with capacity=1 semaphore")
	}
}
