package goroutineutils

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestParallelMap_OrderPreserved(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5, 6}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := ParallelMap[int, int](ctx, items, 4, func(ctx context.Context, v int, i int) (int, error) {
		// Vary timing to try to shuffle execution order;
		// result should still land at the correct index.
		time.Sleep(time.Duration((len(items)-i)%3) * 5 * time.Millisecond)
		return v * v, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != len(items) {
		t.Fatalf("len(out)=%d want %d", len(out), len(items))
	}
	for i, v := range items {
		if out[i] != v*v {
			t.Fatalf("out[%d]=%d want %d", i, out[i], v*v)
		}
	}
}

func TestParallelMap_RespectsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	items := make([]int, 20)
	for i := range items {
		items[i] = i
	}

	var current, maxSeen int32
	limit := 3

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := ParallelMap[int, struct{}](ctx, items, limit, func(ctx context.Context, v int, i int) (struct{}, error) {
		cur := atomic.AddInt32(&current, 1)
		for {
			if curMax := atomic.LoadInt32(&maxSeen); cur > curMax {
				if atomic.CompareAndSwapInt32(&maxSeen, curMax, int32(cur)) {
					break
				}
			} else {
				break
			}
		}
		// Simulate work
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt32(&maxSeen) > int32(limit) {
		t.Fatalf("max concurrency=%d exceeded limit=%d", maxSeen, limit)
	}
}

func TestParallelMap_ErrorPropagation(t *testing.T) {
	t.Parallel()

	items := []int{0, 1, 2, 3, 4, 5, 6}
	sentinel := errors.New("boom")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := ParallelMap[int, string](ctx, items, 8, func(ctx context.Context, v int, i int) (string, error) {
		// Fail on a specific index.
		if i == 3 {
			return "", sentinel
		}
		time.Sleep(5 * time.Millisecond)
		return strconv.Itoa(v), nil
	})

	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v, want sentinel", err)
	}
	if len(out) != len(items) {
		t.Fatalf("len(out)=%d want %d", len(out), len(items))
	}
	// The failing index should be the zero value.
	if out[3] != "" {
		t.Fatalf("out[3]=%q want zero value", out[3])
	}
	// Other indices may or may not be set depending on scheduling/cancellation,
	// so we don't assert further here.
}

func TestParallelMap_EmptyInput(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	out, err := ParallelMap[int, int](ctx, nil, 4, func(ctx context.Context, v int, i int) (int, error) {
		return v, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("len(out)=%d want 0", len(out))
	}
}

func TestParallelMap_LimitNonPositiveMeansSerial(t *testing.T) {
	t.Parallel()

	items := []int{1, 2, 3, 4, 5}
	var concurrent, maxSeen int32

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := ParallelMap[int, struct{}](ctx, items, 0, func(ctx context.Context, v int, i int) (struct{}, error) {
		cur := atomic.AddInt32(&concurrent, 1)
		if cur > maxSeen {
			atomic.StoreInt32(&maxSeen, cur)
		}
		// Very short work
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&concurrent, -1)
		return struct{}{}, nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Implementation coerces limit<=0 to 1, so maxSeen should be 1.
	if maxSeen > 1 {
		t.Fatalf("max concurrency=%d, want <=1 for limit<=0", maxSeen)
	}
}

func TestForEach_Basic(t *testing.T) {
	t.Parallel()

	items := []string{"a", "b", "c", "d"}
	var count int32

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ForEach[string](ctx, items, 2, func(ctx context.Context, s string, i int) error {
		time.Sleep(5 * time.Millisecond)
		atomic.AddInt32(&count, 1)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := atomic.LoadInt32(&count); got != int32(len(items)) {
		t.Fatalf("processed=%d want=%d", got, len(items))
	}
}

func TestForEach_Error(t *testing.T) {
	t.Parallel()

	items := []int{10, 20, 30, 40}
	sentinel := errors.New("stop")

	var started int32

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := ForEach[int](ctx, items, 4, func(ctx context.Context, v int, i int) error {
		atomic.AddInt32(&started, 1)
		if i == 2 {
			return sentinel
		}
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error=%v, want sentinel", err)
	}

	// Depending on your NewGroup implementation (first error cancels context),
	// not all tasks may run. At least one must have started.
	if atomic.LoadInt32(&started) == 0 {
		t.Fatalf("expected at least one task to start")
	}
}
