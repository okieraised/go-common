package goroutineutils

import "context"

type Semaphore chan struct{}

func NewSemaphore(n int) Semaphore {
	if n <= 0 {
		n = 1
	}
	return make(Semaphore, n)
}

func (s Semaphore) Acquire(ctx context.Context) error {
	select {
	case s <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s Semaphore) TryAcquire() bool {
	select {
	case s <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s Semaphore) Release() {
	select {
	case <-s:
	default:
	}
}
