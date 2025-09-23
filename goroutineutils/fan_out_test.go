package goroutineutils

import (
	"context"
	"slices"
	"testing"
	"time"
)

func makeIn[T any](vals ...T) chan T {
	ch := make(chan T, len(vals))
	for _, v := range vals {
		ch <- v
	}
	close(ch)
	return ch
}

func collect[T any](t *testing.T, ch <-chan T, timeout time.Duration) (out []T, closed bool) {
	t.Helper()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case v, ok := <-ch:
			if !ok {
				return out, true
			}
			out = append(out, v)
		case <-timer.C:
			return out, false
		}
	}
}

func TestFanOut_Reliable_DuplicatesAll(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	in := makeIn(1, 2, 3, 4)
	outs := FanOut[int](ctx, in, 3 /*buf*/, 8)

	got := make([][]int, 3)
	for i, ch := range outs {
		items, closed := collect(t, ch, time.Second)
		if !closed {
			t.Fatalf("out[%d] did not close", i)
		}
		got[i] = items
	}
	// All outputs should receive identical streams
	for i := 0; i < len(got); i++ {
		if !slices.Equal(got[i], []int{1, 2, 3, 4}) {
			t.Fatalf("out[%d]=%v, want [1 2 3 4]", i, got[i])
		}
	}
}

func TestFanOut_Reliable_RespectsContextCancel(t *testing.T) {
	parent, stop := context.WithCancel(context.Background())
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()

	in := make(chan int, 1)
	outs := FanOut[int](ctx, in, 2, 0)

	// Send one value then cancel parent; outs must close soon.
	in <- 7
	stop() // cancel parent

	for i, ch := range outs {
		select {
		case _, ok := <-ch:
			if !ok {
				// closed promptly (may or may not deliver 7 depending on timing)
				continue
			}
			// drain the rest until close to avoid flakiness
			select {
			case <-time.After(500 * time.Millisecond):
				t.Fatalf("out[%d] did not close after cancel", i)
			case _, ok = <-ch:
				if ok {
					t.Fatalf("out[%d] still open after cancel", i)
				}
			}
		case <-time.After(800 * time.Millisecond):
			t.Fatalf("out[%d] did not close after cancel", i)
		}
	}
}

func TestFanOut_ZeroOrNegativeN(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	in := makeIn("a", "b")
	outs := FanOut[string](ctx, in, 0, 0) // should create one output

	if len(outs) != 1 {
		t.Fatalf("expected 1 output, got %d", len(outs))
	}
	items, closed := collect(t, outs[0], time.Second)
	if !closed {
		t.Fatalf("output did not close")
	}
	if !slices.Equal(items, []string{"a", "b"}) {
		t.Fatalf("got %v, want [a b]", items)
	}
}

func TestFanOut_BufferPreventsHeadOfLineBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Producer pushes fast
	in := make(chan int, 4)
	go func() {
		defer close(in)
		for i := 0; i < 4; i++ {
			in <- i
		}
	}()

	// One slow consumer, one fast
	outs := FanOut[int](ctx, in, 2 /*buf*/, 4)

	// Slow consumer: read with delays
	doneSlow := make(chan struct{})
	go func() {
		defer close(doneSlow)
		for range outs[0] {
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Fast consumer: drain quickly
	var got []int
	for v := range outs[1] {
		got = append(got, v)
	}

	<-doneSlow

	// Fast consumer should still receive all values thanks to buffering
	if !slices.Equal(got, []int{0, 1, 2, 3}) {
		t.Fatalf("fast consumer got %v, want [0 1 2 3]", got)
	}
}

func TestFanOutLossy_DropsOnSlowConsumer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	in := make(chan int, 16)
	for i := 0; i < 16; i++ {
		in <- i
	}
	close(in)

	outs := FanOutLossy[int](ctx, in, 2 /*buf*/, 2)

	// Slow consumer
	go func() {
		for range outs[0] {
			time.Sleep(10 * time.Millisecond)
		}
	}()

	// Fast consumer collects most/all values (lossy allows progress)
	collected, closed := collect(t, outs[1], time.Second)
	if !closed {
		t.Fatalf("lossy output did not close")
	}
	if len(collected) == 0 {
		t.Fatalf("lossy fast consumer should get some values, got none")
	}
}
