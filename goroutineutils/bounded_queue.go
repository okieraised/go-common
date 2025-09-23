package goroutineutils

import (
	"context"
	"sync/atomic"

	"github.com/okieraised/go-common/cerrors"
)

type OverflowPolicy int

const (
	// Block blocks when full (standard channel semantics).
	Block OverflowPolicy = iota
	// DropLatest drops the incoming item when full and returns cerrors.ErrQueueFullDropped.
	DropLatest
	// DropOldest drops one buffered (oldest) item when full, then enqueues the new one.
	DropOldest
)

// BoundedQueue provides a channel-like queue with a chosen overflow policy.
// It’s built on a buffered channel and requires no goroutine to run.
type BoundedQueue[T any] struct {
	ch      chan T
	policy  OverflowPolicy
	closed  atomic.Bool
	dropped atomic.Uint64
}

// NewBoundedQueue creates a queue with capacity > 0 and the given overflow policy.
func NewBoundedQueue[T any](capacity int, policy OverflowPolicy) *BoundedQueue[T] {
	if capacity <= 0 {
		capacity = 1
	}
	return &BoundedQueue[T]{
		ch:     make(chan T, capacity),
		policy: policy,
	}
}

// Send enqueues v. Behavior on full depends on policy.
// - Block: blocks until space or ctx done/closed
// - DropLatest: returns cerrors.ErrQueueFullDropped immediately if full
// - DropOldest: discards one buffered item if full, then enqueues
func (q *BoundedQueue[T]) Send(ctx context.Context, v T) error {
	if q.closed.Load() {
		return cerrors.ErrQueueClosed
	}
	switch q.policy {
	case Block:
		select {
		case q.ch <- v:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	case DropLatest:
		select {
		case q.ch <- v:
			return nil
		default:
			q.dropped.Add(1)
			return cerrors.ErrQueueFullDropped
		}
	case DropOldest:
		select {
		case q.ch <- v:
			return nil
		default:
			// queue is full: drop one oldest if present
			select {
			case <-q.ch:
				// space made; now try to send respecting ctx
				select {
				case q.ch <- v:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			default:
				// nothing to drop (race): treat as drop-latest
				q.dropped.Add(1)
				return cerrors.ErrQueueClosed
			}
		}
	default:
		// fallback to blocking
		select {
		case q.ch <- v:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// TrySend attempts a non-blocking send. Returns true on success.
// For DropOldest, it will drop one buffered item first if full.
func (q *BoundedQueue[T]) TrySend(v T) (ok bool) {
	if q.closed.Load() {
		return false
	}
	switch q.policy {
	case Block, DropLatest:
		select {
		case q.ch <- v:
			return true
		default:
			if q.policy == DropLatest {
				q.dropped.Add(1)
			}
			return false
		}
	case DropOldest:
		select {
		case q.ch <- v:
			return true
		default:
			select {
			case <-q.ch:
				select {
				case q.ch <- v:
					return true
				default:
					// extremely rare race: treat as drop
					q.dropped.Add(1)
					return false
				}
			default:
				q.dropped.Add(1)
				return false
			}
		}
	}
	return
}

// Recv receives one value. If ctx is done before a value arrives, returns ctx error.
// ok=false means the queue is closed and fully drained (like a channel).
func (q *BoundedQueue[T]) Recv(ctx context.Context) (zero T, v T, ok bool, err error) {
	select {
	case v, ok = <-q.ch:
		return zero, v, ok, nil
	case <-ctx.Done():
		var z T
		return z, z, false, ctx.Err()
	}
}

// Close closes the queue once. Further sends fail with cerrors.ErrQueueClosed.
// Receivers will drain remaining items until channel closes.
func (q *BoundedQueue[T]) Close() {
	if q.closed.CompareAndSwap(false, true) {
		close(q.ch)
	}
}

// Chan exposes the underlying receive-only channel for range loops.
func (q *BoundedQueue[T]) Chan() <-chan T { return q.ch }

// Stats returns (len, cap, droppedCount).
func (q *BoundedQueue[T]) Stats() (int, int, uint64) {
	return len(q.ch), cap(q.ch), q.dropped.Load()
}
