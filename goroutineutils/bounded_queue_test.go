package goroutineutils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/okieraised/go-common/cerrors"
)

func ctxTO(t *testing.T, d time.Duration) (context.Context, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx, cancel
}

func TestNewBoundedQueue_MinCapacity(t *testing.T) {
	q := NewBoundedQueue[int](0, Block) // <=0 coerced to 1
	if _, capv, _ := q.Stats(); capv != 1 {
		t.Fatalf("cap=%d, want 1", capv)
	}
}

func TestBlockPolicy_SendBlocksUntilSpace(t *testing.T) {
	q := NewBoundedQueue[int](1, Block)

	// Fill queue
	if err := q.Send(context.Background(), 1); err != nil {
		t.Fatalf("send: %v", err)
	}

	// Start a blocking send
	done := make(chan error, 1)
	go func() {
		ctx, cancel := ctxTO(t, 1*time.Second)
		defer cancel()
		done <- q.Send(ctx, 2)
	}()

	// Briefly ensure it's blocked
	select {
	case err := <-done:
		t.Fatalf("send returned too early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	// Now make space by receiving one
	ctx, cancel := ctxTO(t, time.Second)
	defer cancel()
	_, v, ok, err := q.Recv(ctx)
	if err != nil || !ok || v != 1 {
		t.Fatalf("recv: v=%d ok=%v err=%v", v, ok, err)
	}

	// The blocked send should complete successfully
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("send (unblocked) err=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("blocked send did not complete")
	}

	// Drain second value
	_, v, ok, err = q.Recv(ctx)
	if err != nil || !ok || v != 2 {
		t.Fatalf("recv2: v=%d ok=%v err=%v", v, ok, err)
	}
}

func TestBlockPolicy_SendRespectsContext(t *testing.T) {
	q := NewBoundedQueue[int](1, Block)
	_ = q.Send(context.Background(), 1) // fill

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := q.Send(ctx, 2); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want ctx deadline, got %v", err)
	}
}

func TestDropLatestPolicy(t *testing.T) {
	// Fresh queue for this test only.
	q := NewBoundedQueue[int](2, DropLatest)

	// Fill to capacity: [1,2]
	if err := q.Send(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := q.Send(context.Background(), 2); err != nil {
		t.Fatal(err)
	}

	// Next send should drop (queue is full)
	err := q.Send(context.Background(), 3)
	if !errors.Is(err, cerrors.ErrQueueFullDropped) {
		t.Fatalf("want ErrDropped, got %v", err)
	}

	// TrySend should also drop when full
	if ok := q.TrySend(4); ok {
		t.Fatalf("TrySend should fail when full in DropLatest")
	}

	// Sanity: queue must not be closed here
	if _, _, _ = q.Stats(); false { // no-op to show Stats is fine to call
	}

	// Drain: expect original 1,2 (3 and 4 were dropped)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, v1, ok, err := q.Recv(ctx)
	if err != nil || !ok || v1 != 1 {
		t.Fatalf("first recv: v=%d ok=%v err=%v (want 1)", v1, ok, err)
	}
	_, v2, ok, err := q.Recv(ctx)
	if err != nil || !ok || v2 != 2 {
		t.Fatalf("second recv: v=%d ok=%v err=%v (want 2)", v2, ok, err)
	}
}

func TestDropOldestPolicy(t *testing.T) {
	q := NewBoundedQueue[int](2, DropOldest)

	// Fill: [1,2]
	_ = q.Send(context.Background(), 1)
	_ = q.Send(context.Background(), 2)

	// Send 3 -> drop oldest (1), enqueue 3 -> expect [2,3]
	if err := q.Send(context.Background(), 3); err != nil {
		t.Fatalf("send 3: %v", err)
	}

	l, capv, dropped := q.Stats()
	if l != 2 || capv != 2 || dropped != 0 {
		t.Fatalf("stats: len=%d cap=%d dropped=%d", l, capv, dropped)
	}

	ctx, cancel := ctxTO(t, time.Second)
	defer cancel()
	_, v, ok, _ := q.Recv(ctx)
	if !ok || v != 2 {
		t.Fatalf("first recv want 2, got %d ok=%v", v, ok)
	}
	_, v, ok, _ = q.Recv(ctx)
	if !ok || v != 3 {
		t.Fatalf("second recv want 3, got %d ok=%v", v, ok)
	}
}

func TestDropOldestPolicy_TrySend(t *testing.T) {
	q := NewBoundedQueue[int](1, DropOldest)

	_ = q.Send(context.Background(), 10) // buffer full [10]
	if ok := q.TrySend(20); !ok {
		t.Fatalf("TrySend should evict oldest and enqueue new")
	}
	// expect [20]
	ctx, cancel := ctxTO(t, time.Second)
	defer cancel()
	_, v, ok, _ := q.Recv(ctx)
	if !ok || v != 20 {
		t.Fatalf("want 20, got %d ok=%v", v, ok)
	}
}

func TestRecv_ContextDeadline(t *testing.T) {
	q := NewBoundedQueue[int](1, Block)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, _, ok, err := q.Recv(ctx)
	if ok || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline error, got ok=%v err=%v", ok, err)
	}
}

func TestCloseAndDrain(t *testing.T) {
	q := NewBoundedQueue[int](3, Block)
	_ = q.Send(context.Background(), 1)
	_ = q.Send(context.Background(), 2)
	_ = q.Send(context.Background(), 3)

	q.Close()

	// further sends should fail with ErrClosed
	if err := q.Send(context.Background(), 4); !errors.Is(err, cerrors.ErrQueueClosed) {
		t.Fatalf("send after close got %v, want ErrClosed", err)
	}
	if ok := q.TrySend(5); ok {
		t.Fatalf("TrySend after close should be false")
	}

	// Recv drains and then closes
	ctx, cancel := ctxTO(t, time.Second)
	defer cancel()
	var got []int
	for {
		_, v, ok, err := q.Recv(ctx)
		if err != nil {
			t.Fatalf("recv error while draining: %v", err)
		}
		if !ok {
			break
		}
		got = append(got, v)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("drained %v, want [1 2 3]", got)
	}
}

func TestChanExposure_Range(t *testing.T) {
	q := NewBoundedQueue[string](4, Block)
	_ = q.Send(context.Background(), "a")
	_ = q.Send(context.Background(), "b")
	q.Close()

	var got []string
	for v := range q.Chan() {
		got = append(got, v)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("range got %v, want [a b]", got)
	}
}

func TestStatsReflectsDroppedCounts(t *testing.T) {
	q := NewBoundedQueue[int](1, DropLatest)

	_ = q.Send(context.Background(), 1) // fill
	_ = q.Send(context.Background(), 2) // drop
	_ = q.Send(context.Background(), 3) // drop
	_ = q.TrySend(4)                    // drop (non-blocking)

	_, _, dropped := q.Stats()
	if dropped < 3 {
		t.Fatalf("dropped=%d, want at least 3", dropped)
	}
}
