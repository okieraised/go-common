package goroutineutils

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDebounce_FiresOnceWithLastValue(t *testing.T) {
	var calls atomic.Int64
	done := make(chan struct{}, 1)

	call, _ := Debounce(40*time.Millisecond, func() {
		calls.Add(1)
		done <- struct{}{}
	})

	// burst of calls; only the last should schedule the run
	for i := 0; i < 5; i++ {
		call()
		time.Sleep(5 * time.Millisecond)
	}

	// Should fire once after ~40ms since last call
	waitOrFail(t, done, 300*time.Millisecond, "debounce did not fire")

	if got := calls.Load(); got != 1 {
		t.Fatalf("debounce calls=%d, want 1", got)
	}
}

func TestDebounce_StopCancelsPending(t *testing.T) {
	var calls atomic.Int64
	call, stop := Debounce(30*time.Millisecond, func() {
		calls.Add(1)
	})

	// schedule and immediately stop
	call()
	stop()

	// Give enough time; nothing should run
	time.Sleep(80 * time.Millisecond)

	if got := calls.Load(); got != 0 {
		t.Fatalf("debounce fired after Stop(); calls=%d", got)
	}
}
