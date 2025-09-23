package goroutineutils

import (
	"sync/atomic"
	"testing"
	"time"
)

func waitOrFail(t *testing.T, ch <-chan struct{}, d time.Duration, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatalf("timeout: %s", msg)
	}
}

func TestThrottleLeadingTrailing(t *testing.T) {
	var calls int32
	th := ThrottleLeadingTrailing(80*time.Millisecond, func() {
		atomic.AddInt32(&calls, 1)
	})

	// Burst of calls: expect one leading + one trailing
	for i := 0; i < 5; i++ {
		th()
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(120 * time.Millisecond) // allow trailing to fire

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("got %d calls, want 2 (leading+trailing)", got)
	}
}
