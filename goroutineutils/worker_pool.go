package goroutineutils

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
)

type Task func(context.Context) error

type WorkerPool struct {
	wg     sync.WaitGroup
	ctx    context.Context
	cancel context.CancelFunc

	tasks   chan Task
	once    sync.Once
	stopped atomic.Bool

	errMu sync.Mutex
	err   error
}

var ErrPoolClosed = errors.New("worker pool is closed")

// NewWorkerPool creates a pool with n workers.
// Queue capacity controls back-pressure; 0 means unbuffered.
func NewWorkerPool(parent context.Context, n, queue int) *WorkerPool {
	if n <= 0 {
		n = 1
	}
	ctx, cancel := context.WithCancel(parent)
	p := &WorkerPool{
		ctx:    ctx,
		cancel: cancel,
		tasks:  make(chan Task, queue),
	}
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go p.worker()
	}
	return p
}

func (p *WorkerPool) worker() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case t, ok := <-p.tasks:
			if !ok {
				return
			}
			if err := t(p.ctx); err != nil {
				p.setErr(err)
			}
		}
	}
}

// Submit enqueues a task or returns ctx error if pool is shutting down.
// It never panics, even if Close/Wait races with Submit.
func (p *WorkerPool) Submit(t Task) error {
	if p.stopped.Load() {
		select {
		case <-p.ctx.Done():
			return p.ctx.Err()
		default:
			return ErrPoolClosed
		}
	}

	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case p.tasks <- t:
		return nil
	default:
	}

	select {
	case <-p.ctx.Done():
		return p.ctx.Err()
	case p.tasks <- t:
		return nil
	}
}

// Close stops accepting tasks and lets workers drain the queue.
// Idempotent and safe under races with Submit.
func (p *WorkerPool) Close() {
	p.once.Do(func() {
		p.stopped.Store(true)
		close(p.tasks)
	})
}

// Wait waits for all workers and returns the first task error (if any).
func (p *WorkerPool) Wait() error {
	p.Close()
	p.wg.Wait()
	p.cancel()
	return p.err
}

func (p *WorkerPool) setErr(err error) {
	p.errMu.Lock()
	if p.err == nil {
		p.err = err
		p.cancel()
	}
	p.errMu.Unlock()
}
