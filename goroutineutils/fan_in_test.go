package goroutineutils

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"
)

func produce[T any](vals []T, delay time.Duration) <-chan T {
	ch := make(chan T)
	go func() {
		defer close(ch)
		for _, v := range vals {
			if delay > 0 {
				time.Sleep(delay)
			}
			ch <- v
		}
	}()
	return ch
}

func collectWithTimeout[T any](t *testing.T, ctx context.Context, ch <-chan T) (items []T, closed bool) {
	t.Helper()
	for {
		select {
		case <-ctx.Done():
			return items, false
		case v, ok := <-ch:
			if !ok {
				return items, true
			}
			items = append(items, v)
		}
	}
}

func TestFanIn_MergesAllValues(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	a := produce([]int{1, 3, 5, 7}, 2*time.Millisecond)
	b := produce([]int{2, 4, 6, 8}, 1*time.Millisecond)
	c := produce([]int{10, 20}, 3*time.Millisecond)

	out := FanIn(ctx, a, b, c)

	got, ok := collectWithTimeout(t, ctx, out)
	if !ok {
		t.Fatalf("output channel did not close before timeout; collected: %v", got)
	}

	want := []int{1, 2, 3, 4, 5, 6, 7, 8, 10, 20}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("merged values mismatch: got %v; want %v", got, want)
	}
}

func TestFanIn_HandlesNoChannels(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out := FanIn[int](ctx)
	select {
	case _, ok := <-out:
		if ok {
			t.Fatalf("expected closed channel immediately for zero inputs")
		}
	case <-ctx.Done():
		t.Fatalf("timed out waiting for closed channel")
	}
}

func TestFanIn_ContextCancelStopsForwarding(t *testing.T) {
	t.Parallel()

	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	// Slow infinite producer (until closed by ctx in goroutine below)
	slow := make(chan int)
	go func() {
		defer close(slow)
		for i := 0; i < 1_000_000; i++ {
			time.Sleep(2 * time.Millisecond)
			select {
			case slow <- i:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Another finite producer to ensure some initial traffic
	finite := produce([]int{42, 43, 44}, 1*time.Millisecond)

	out := FanIn(ctx, slow, finite)

	// Read a few values, then cancel parent to force FanIn to stop promptly.
	readSome := 0
	for readSome < 2 {
		select {
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("did not receive initial values in time")
		case _, ok := <-out:
			if !ok {
				t.Fatalf("output closed earlier than expected")
			}
			readSome++
		}
	}
	// Now cancel the *parent* to trigger shutdown
	stop()

	// After cancel, the output must eventually close (and soon)
	select {
	case _, ok := <-out:
		if ok {
			// Drain until closed or timeout
			// (cheap guard in case one value slipped through concurrently)
			select {
			case _, ok2 := <-out:
				if ok2 {
					t.Fatalf("output still open after cancel")
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("output not closed after cancel")
			}
		}
	case <-time.After(800 * time.Millisecond):
		t.Fatalf("timeout waiting for output close after cancel")
	}
}

func TestFanIn_ClosesWhenInputsClose(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	a := produce([]string{"a1", "a2"}, 1*time.Millisecond)
	b := produce([]string{"b1"}, 2*time.Millisecond)

	out := FanIn(ctx, a, b)

	var got []string
	for v := range out {
		got = append(got, v)
	}
	// Channel closed cleanly -> got should have exactly 3 elements (order unknown)
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(got), got)
	}
}

func TestFanIn_NoLeak_OnSlowConsumer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Many producers; ensure we can read slowly without deadlocking,
	// thanks to inner select { out <- v ; ctx.Done() }.
	var chans []<-chan int
	for i := 0; i < 5; i++ {
		chans = append(chans, produce([]int{i, i + 100, i + 200}, 1*time.Millisecond))
	}

	out := FanIn(ctx, chans...)

	var mu sync.Mutex
	var got []int
readLoop:
	for {
		select {
		case v, ok := <-out:
			if !ok {
				break readLoop
			}
			// Simulate slow consumer
			time.Sleep(2 * time.Millisecond)
			mu.Lock()
			got = append(got, v)
			mu.Unlock()
		case <-ctx.Done():
			t.Fatalf("timed out; possible leak or deadlock")
		}
	}
	if len(got) != 15 {
		t.Fatalf("expected 15 merged items, got %d", len(got))
	}
}
