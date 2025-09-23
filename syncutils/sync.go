package syncutils

import (
	"context"
	"errors"
	"sync"
	"time"
)

type LockKind string

const (
	ReadLock  LockKind = "read"
	WriteLock LockKind = "write"
)

type FailReason string

const (
	FailNotAcquired FailReason = "not_acquired"
	FailContextDone FailReason = "context_done"
)

var (
	defaultTryInterval   = 2 * time.Millisecond
	defaultWaitThreshold = 10 * time.Millisecond

	DefaultHooks Hooks = NoopHooks{}
)

type Hooks interface {
	// Called right before a potentially blocking acquisition attempt starts.
	// For polling loops (*Ctx methods), this is called once at the beginning.
	OnWaitStart(name string, kind LockKind)

	// Called when a lock is acquired. 'waited' includes any blocking time.
	OnAcquire(name string, kind LockKind, waited time.Duration)

	// Called after OnAcquire if waited >= threshold.
	OnSlowAcquire(name string, kind LockKind, waited, threshold time.Duration)

	// Called when acquisition fails (e.g., Try* immediate miss or ctx canceled).
	// 'waited' is how long we tried before giving up.
	OnFail(name string, kind LockKind, reason FailReason, waited time.Duration)
}

// NoopHooks implements Hooks but does nothing.
type NoopHooks struct{}

func (NoopHooks) OnWaitStart(string, LockKind)                                 {}
func (NoopHooks) OnAcquire(string, LockKind, time.Duration)                    {}
func (NoopHooks) OnSlowAcquire(string, LockKind, time.Duration, time.Duration) {}
func (NoopHooks) OnFail(string, LockKind, FailReason, time.Duration)           {}

type options struct {
	name          string
	hooks         Hooks
	tryInterval   time.Duration
	waitThreshold time.Duration
}

// Option configures an instrumented type.
type Option func(*options)

func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

func WithHooks(h Hooks) Option {
	return func(o *options) { o.hooks = h }
}

// WithTryInterval sets the retry interval used by *Ctx methods when they
// poll with TryLock/TryRLock. If unset or <=0, a small default is used.
func WithTryInterval(d time.Duration) Option {
	return func(o *options) { o.tryInterval = d }
}

// WithWaitThreshold sets the duration beyond which an acquisition is reported
// as "slow" via OnSlowAcquire. If unset or <=0, a default is used.
func WithWaitThreshold(d time.Duration) Option {
	return func(o *options) { o.waitThreshold = d }
}

func resolveOptions(opts []Option) options {
	o := options{
		name:          "",
		hooks:         DefaultHooks,
		tryInterval:   defaultTryInterval,
		waitThreshold: defaultWaitThreshold,
	}
	for _, fn := range opts {
		fn(&o)
	}
	if o.hooks == nil {
		o.hooks = DefaultHooks
	}
	if o.tryInterval <= 0 {
		o.tryInterval = defaultTryInterval
	}
	if o.waitThreshold <= 0 {
		o.waitThreshold = defaultWaitThreshold
	}
	return o
}

var ErrLockNotAcquired = errors.New("syncx: lock not acquired")

type RWMutexWrapper struct {
	mu            sync.RWMutex
	name          string
	hooks         Hooks
	tryInterval   time.Duration
	waitThreshold time.Duration
}

func NewRWMutexWrapper(opts ...Option) *RWMutexWrapper {
	o := resolveOptions(opts)
	return &RWMutexWrapper{
		name:          o.name,
		hooks:         o.hooks,
		tryInterval:   o.tryInterval,
		waitThreshold: o.waitThreshold,
	}
}

// WithRead acquires RLock, runs fn, and unlocks.
// Emits hooks (including "slow" if wait exceeds threshold).
func (rw *RWMutexWrapper) WithRead(fn func()) {
	rw.hooks.OnWaitStart(rw.name, ReadLock)
	t0 := time.Now()
	rw.mu.RLock()
	waited := time.Since(t0)
	rw.hooks.OnAcquire(rw.name, ReadLock, waited)
	if waited >= rw.waitThreshold {
		rw.hooks.OnSlowAcquire(rw.name, ReadLock, waited, rw.waitThreshold)
	}
	defer rw.mu.RUnlock()
	fn()
}

// WithWrite acquires Lock, runs fn, and unlocks.
func (rw *RWMutexWrapper) WithWrite(fn func()) {
	rw.hooks.OnWaitStart(rw.name, WriteLock)
	t0 := time.Now()
	rw.mu.Lock()
	waited := time.Since(t0)
	rw.hooks.OnAcquire(rw.name, WriteLock, waited)
	if waited >= rw.waitThreshold {
		rw.hooks.OnSlowAcquire(rw.name, WriteLock, waited, rw.waitThreshold)
	}
	defer rw.mu.Unlock()
	fn()
}

// TryWithRead attempts a non-blocking RLock; returns false if not acquired.
func (rw *RWMutexWrapper) TryWithRead(fn func()) bool {
	rw.hooks.OnWaitStart(rw.name, ReadLock)
	t0 := time.Now()
	if !rw.mu.TryRLock() {
		rw.hooks.OnFail(rw.name, ReadLock, FailNotAcquired, time.Since(t0))
		return false
	}
	waited := time.Since(t0)
	rw.hooks.OnAcquire(rw.name, ReadLock, waited)
	if waited >= rw.waitThreshold {
		rw.hooks.OnSlowAcquire(rw.name, ReadLock, waited, rw.waitThreshold)
	}
	defer rw.mu.RUnlock()
	fn()
	return true
}

// TryWithWrite attempts a non-blocking Lock; returns false if not acquired.
func (rw *RWMutexWrapper) TryWithWrite(fn func()) bool {
	rw.hooks.OnWaitStart(rw.name, WriteLock)
	t0 := time.Now()
	if !rw.mu.TryLock() {
		rw.hooks.OnFail(rw.name, WriteLock, FailNotAcquired, time.Since(t0))
		return false
	}
	waited := time.Since(t0)
	rw.hooks.OnAcquire(rw.name, WriteLock, waited)
	if waited >= rw.waitThreshold {
		rw.hooks.OnSlowAcquire(rw.name, WriteLock, waited, rw.waitThreshold)
	}
	defer rw.mu.Unlock()
	fn()
	return true
}

// WithReadCtx keeps trying TryRLock until acquired or ctx done.
func (rw *RWMutexWrapper) WithReadCtx(ctx context.Context, interval time.Duration, fn func()) error {
	if interval <= 0 {
		interval = rw.tryInterval
	}
	rw.hooks.OnWaitStart(rw.name, ReadLock)
	t0 := time.Now()
	for {
		if rw.mu.TryRLock() {
			waited := time.Since(t0)
			rw.hooks.OnAcquire(rw.name, ReadLock, waited)
			if waited >= rw.waitThreshold {
				rw.hooks.OnSlowAcquire(rw.name, ReadLock, waited, rw.waitThreshold)
			}
			defer rw.mu.RUnlock()
			fn()
			return nil
		}
		select {
		case <-ctx.Done():
			rw.hooks.OnFail(rw.name, ReadLock, FailContextDone, time.Since(t0))
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// WithWriteCtx keeps trying TryLock until acquired or ctx done.
func (rw *RWMutexWrapper) WithWriteCtx(ctx context.Context, interval time.Duration, fn func()) error {
	if interval <= 0 {
		interval = rw.tryInterval
	}
	rw.hooks.OnWaitStart(rw.name, WriteLock)
	t0 := time.Now()
	for {
		if rw.mu.TryLock() {
			waited := time.Since(t0)
			rw.hooks.OnAcquire(rw.name, WriteLock, waited)
			if waited >= rw.waitThreshold {
				rw.hooks.OnSlowAcquire(rw.name, WriteLock, waited, rw.waitThreshold)
			}
			defer rw.mu.Unlock()
			fn()
			return nil
		}
		select {
		case <-ctx.Done():
			rw.hooks.OnFail(rw.name, WriteLock, FailContextDone, time.Since(t0))
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

type SafeStore[T any] struct {
	mu            sync.RWMutex
	name          string
	hooks         Hooks
	tryInterval   time.Duration
	waitThreshold time.Duration
	data          T
}

func NewSafeStore[T any](initial T, opts ...Option) *SafeStore[T] {
	o := resolveOptions(opts)
	return &SafeStore[T]{
		name:          o.name,
		hooks:         o.hooks,
		tryInterval:   o.tryInterval,
		waitThreshold: o.waitThreshold,
		data:          initial,
	}
}

func (s *SafeStore[T]) Get() T {
	s.hooks.OnWaitStart(s.name, ReadLock)
	t0 := time.Now()
	s.mu.RLock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, ReadLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, ReadLock, waited, s.waitThreshold)
	}
	defer s.mu.RUnlock()
	return s.data
}

func (s *SafeStore[T]) Set(val T) {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	s.mu.Lock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = val
}

func (s *SafeStore[T]) Update(f func(old T) T) {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	s.mu.Lock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = f(s.data)
}

func (s *SafeStore[T]) TryGet() (T, bool) {
	s.hooks.OnWaitStart(s.name, ReadLock)
	t0 := time.Now()
	if !s.mu.TryRLock() {
		var zero T
		s.hooks.OnFail(s.name, ReadLock, FailNotAcquired, time.Since(t0))
		return zero, false
	}
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, ReadLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, ReadLock, waited, s.waitThreshold)
	}
	defer s.mu.RUnlock()
	return s.data, true
}

func (s *SafeStore[T]) TrySet(val T) bool {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	if !s.mu.TryLock() {
		s.hooks.OnFail(s.name, WriteLock, FailNotAcquired, time.Since(t0))
		return false
	}
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = val
	return true
}

func (s *SafeStore[T]) UpdateCtx(ctx context.Context, interval time.Duration, f func(old T) T) error {
	if interval <= 0 {
		interval = s.tryInterval
	}
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	for {
		if s.mu.TryLock() {
			waited := time.Since(t0)
			s.hooks.OnAcquire(s.name, WriteLock, waited)
			if waited >= s.waitThreshold {
				s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
			}
			defer s.mu.Unlock()
			s.data = f(s.data)
			return nil
		}
		select {
		case <-ctx.Done():
			s.hooks.OnFail(s.name, WriteLock, FailContextDone, time.Since(t0))
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

type SafeMap[K comparable, V any] struct {
	mu            sync.RWMutex
	name          string
	hooks         Hooks
	tryInterval   time.Duration
	waitThreshold time.Duration
	data          map[K]V
}

func NewSafeMap[K comparable, V any](opts ...Option) *SafeMap[K, V] {
	o := resolveOptions(opts)
	return &SafeMap[K, V]{
		name:          o.name,
		hooks:         o.hooks,
		tryInterval:   o.tryInterval,
		waitThreshold: o.waitThreshold,
		data:          make(map[K]V),
	}
}

func (m *SafeMap[K, V]) Get(key K) (V, bool) {
	m.hooks.OnWaitStart(m.name, ReadLock)
	t0 := time.Now()
	m.mu.RLock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, ReadLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, ReadLock, waited, m.waitThreshold)
	}
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok
}

func (m *SafeMap[K, V]) Set(key K, val V) {
	m.hooks.OnWaitStart(m.name, WriteLock)
	t0 := time.Now()
	m.mu.Lock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, WriteLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, WriteLock, waited, m.waitThreshold)
	}
	defer m.mu.Unlock()
	m.data[key] = val
}

func (m *SafeMap[K, V]) Delete(key K) {
	m.hooks.OnWaitStart(m.name, WriteLock)
	t0 := time.Now()
	m.mu.Lock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, WriteLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, WriteLock, waited, m.waitThreshold)
	}
	defer m.mu.Unlock()
	delete(m.data, key)
}

func (m *SafeMap[K, V]) Len() int {
	m.hooks.OnWaitStart(m.name, ReadLock)
	t0 := time.Now()
	m.mu.RLock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, ReadLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, ReadLock, waited, m.waitThreshold)
	}
	defer m.mu.RUnlock()
	return len(m.data)
}

func (m *SafeMap[K, V]) Keys() []K {
	m.hooks.OnWaitStart(m.name, ReadLock)
	t0 := time.Now()
	m.mu.RLock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, ReadLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, ReadLock, waited, m.waitThreshold)
	}
	defer m.mu.RUnlock()
	out := make([]K, 0, len(m.data))
	for k := range m.data {
		out = append(out, k)
	}
	return out
}

func (m *SafeMap[K, V]) Values() []V {
	m.hooks.OnWaitStart(m.name, ReadLock)
	t0 := time.Now()
	m.mu.RLock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, ReadLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, ReadLock, waited, m.waitThreshold)
	}
	defer m.mu.RUnlock()
	out := make([]V, 0, len(m.data))
	for _, v := range m.data {
		out = append(out, v)
	}
	return out
}

// Upsert updates key with f(old, exists) atomically and returns the new value.
func (m *SafeMap[K, V]) Upsert(key K, f func(old V, exists bool) V) V {
	m.hooks.OnWaitStart(m.name, WriteLock)
	t0 := time.Now()
	m.mu.Lock()
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, WriteLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, WriteLock, waited, m.waitThreshold)
	}
	defer m.mu.Unlock()
	old, ok := m.data[key]
	nv := f(old, ok)
	m.data[key] = nv
	return nv
}

// TryGet is a non-blocking Get. (v, ok, acquired)
func (m *SafeMap[K, V]) TryGet(key K) (V, bool, bool) {
	m.hooks.OnWaitStart(m.name, ReadLock)
	t0 := time.Now()
	if !m.mu.TryRLock() {
		var zero V
		m.hooks.OnFail(m.name, ReadLock, FailNotAcquired, time.Since(t0))
		return zero, false, false
	}
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, ReadLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, ReadLock, waited, m.waitThreshold)
	}
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	return v, ok, true
}

// TrySet is a non-blocking Set. returns acquired=false if not acquired.
func (m *SafeMap[K, V]) TrySet(key K, val V) (acquired bool) {
	m.hooks.OnWaitStart(m.name, WriteLock)
	t0 := time.Now()
	if !m.mu.TryLock() {
		m.hooks.OnFail(m.name, WriteLock, FailNotAcquired, time.Since(t0))
		return false
	}
	waited := time.Since(t0)
	m.hooks.OnAcquire(m.name, WriteLock, waited)
	if waited >= m.waitThreshold {
		m.hooks.OnSlowAcquire(m.name, WriteLock, waited, m.waitThreshold)
	}
	defer m.mu.Unlock()
	m.data[key] = val
	return true
}

// UpsertCtx retries TryLock until ctx done.
func (m *SafeMap[K, V]) UpsertCtx(ctx context.Context, interval time.Duration, key K, f func(old V, exists bool) V) error {
	if interval <= 0 {
		interval = m.tryInterval
	}
	m.hooks.OnWaitStart(m.name, WriteLock)
	t0 := time.Now()
	for {
		if m.mu.TryLock() {
			waited := time.Since(t0)
			m.hooks.OnAcquire(m.name, WriteLock, waited)
			if waited >= m.waitThreshold {
				m.hooks.OnSlowAcquire(m.name, WriteLock, waited, m.waitThreshold)
			}
			defer m.mu.Unlock()
			old, ok := m.data[key]
			m.data[key] = f(old, ok)
			return nil
		}
		select {
		case <-ctx.Done():
			m.hooks.OnFail(m.name, WriteLock, FailContextDone, time.Since(t0))
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

type SafeSlice[T any] struct {
	mu            sync.RWMutex
	name          string
	hooks         Hooks
	tryInterval   time.Duration
	waitThreshold time.Duration
	data          []T
}

func NewSafeSlice[T any](opts ...Option) *SafeSlice[T] {
	o := resolveOptions(opts)
	return &SafeSlice[T]{
		name:          o.name,
		hooks:         o.hooks,
		tryInterval:   o.tryInterval,
		waitThreshold: o.waitThreshold,
		data:          make([]T, 0),
	}
}

func (s *SafeSlice[T]) Append(v T) {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	s.mu.Lock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = append(s.data, v)
}

func (s *SafeSlice[T]) Len() int {
	s.hooks.OnWaitStart(s.name, ReadLock)
	t0 := time.Now()
	s.mu.RLock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, ReadLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, ReadLock, waited, s.waitThreshold)
	}
	defer s.mu.RUnlock()
	return len(s.data)
}

// GetAll returns a snapshot copy of the underlying slice.
func (s *SafeSlice[T]) GetAll() []T {
	s.hooks.OnWaitStart(s.name, ReadLock)
	t0 := time.Now()
	s.mu.RLock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, ReadLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, ReadLock, waited, s.waitThreshold)
	}
	defer s.mu.RUnlock()
	out := make([]T, len(s.data))
	copy(out, s.data)
	return out
}

// ReplaceAll swaps the entire slice atomically.
func (s *SafeSlice[T]) ReplaceAll(newData []T) {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	s.mu.Lock()
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = make([]T, len(newData))
	copy(s.data, newData)
}

// TryAppend attempts a non-blocking Append.
func (s *SafeSlice[T]) TryAppend(v T) bool {
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	if !s.mu.TryLock() {
		s.hooks.OnFail(s.name, WriteLock, FailNotAcquired, time.Since(t0))
		return false
	}
	waited := time.Since(t0)
	s.hooks.OnAcquire(s.name, WriteLock, waited)
	if waited >= s.waitThreshold {
		s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
	}
	defer s.mu.Unlock()
	s.data = append(s.data, v)
	return true
}

// AppendCtx retries TryLock until ctx done.
func (s *SafeSlice[T]) AppendCtx(ctx context.Context, interval time.Duration, v T) error {
	if interval <= 0 {
		interval = s.tryInterval
	}
	s.hooks.OnWaitStart(s.name, WriteLock)
	t0 := time.Now()
	for {
		if s.mu.TryLock() {
			waited := time.Since(t0)
			s.hooks.OnAcquire(s.name, WriteLock, waited)
			if waited >= s.waitThreshold {
				s.hooks.OnSlowAcquire(s.name, WriteLock, waited, s.waitThreshold)
			}
			defer s.mu.Unlock()
			s.data = append(s.data, v)
			return nil
		}
		select {
		case <-ctx.Done():
			s.hooks.OnFail(s.name, WriteLock, FailContextDone, time.Since(t0))
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

//var ErrLockNotAcquired = errors.New("syncx: lock not acquired")
//
//// backoffWait is the default interval between TryLock attempts in *Ctx methods.
//const backoffWait = 2 * time.Millisecond
//
//
//type RWMutexWrapper struct {
//	mu sync.RWMutex
//}
//
//// WithRead acquires RLock, runs fn, and unlocks.
//func (rw *RWMutexWrapper) WithRead(fn func()) {
//	rw.mu.RLock()
//	defer rw.mu.RUnlock()
//	fn()
//}
//
//// WithWrite acquires Lock, runs fn, and unlocks.
//func (rw *RWMutexWrapper) WithWrite(fn func()) {
//	rw.mu.Lock()
//	defer rw.mu.Unlock()
//	fn()
//}
//
//// TryWithRead attempts a non-blocking RLock; returns false if not acquired.
//func (rw *RWMutexWrapper) TryWithRead(fn func()) bool {
//	if !rw.mu.TryRLock() {
//		return false
//	}
//	defer rw.mu.RUnlock()
//	fn()
//	return true
//}
//
//// TryWithWrite attempts a non-blocking Lock; returns false if not acquired.
//func (rw *RWMutexWrapper) TryWithWrite(fn func()) bool {
//	if !rw.mu.TryLock() {
//		return false
//	}
//	defer rw.mu.Unlock()
//	fn()
//	return true
//}
//
//// WithReadCtx tries RLock periodically until acquired or ctx done.
//// If interval<=0, a small default backoff is used.
//func (rw *RWMutexWrapper) WithReadCtx(ctx context.Context, interval time.Duration, fn func()) error {
//	if interval <= 0 {
//		interval = backoffWait
//	}
//	for {
//		if rw.mu.TryRLock() {
//			defer rw.mu.RUnlock()
//			fn()
//			return nil
//		}
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		case <-time.After(interval):
//		}
//	}
//}
//
//// WithWriteCtx tries Lock periodically until acquired or ctx done.
//// If interval<=0, a small default backoff is used.
//func (rw *RWMutexWrapper) WithWriteCtx(ctx context.Context, interval time.Duration, fn func()) error {
//	if interval <= 0 {
//		interval = backoffWait
//	}
//	for {
//		if rw.mu.TryLock() {
//			defer rw.mu.Unlock()
//			fn()
//			return nil
//		}
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		case <-time.After(interval):
//		}
//	}
//}
//
//
//
//type SafeStore[T any] struct {
//	mu   sync.RWMutex
//	data T
//}
//
//func NewSafeStore[T any](initial T) *SafeStore[T] {
//	return &SafeStore[T]{data: initial}
//}
//
//func (s *SafeStore[T]) Get() T {
//	s.mu.RLock()
//	defer s.mu.RUnlock()
//	return s.data
//}
//
//func (s *SafeStore[T]) Set(val T) {
//	s.mu.Lock()
//	defer s.mu.Unlock()
//	s.data = val
//}
//
//// Update applies f to the current value atomically.
//func (s *SafeStore[T]) Update(f func(old T) T) {
//	s.mu.Lock()
//	defer s.mu.Unlock()
//	s.data = f(s.data)
//}
//
//// TryGet returns (value, true) if RLock succeeds immediately.
//func (s *SafeStore[T]) TryGet() (T, bool) {
//	if !s.mu.TryRLock() {
//		var zero T
//		return zero, false
//	}
//	defer s.mu.RUnlock()
//	return s.data, true
//}
//
//// TrySet sets val if Lock succeeds immediately.
//func (s *SafeStore[T]) TrySet(val T) bool {
//	if !s.mu.TryLock() {
//		return false
//	}
//	defer s.mu.Unlock()
//	s.data = val
//	return true
//}
//
//// UpdateCtx keeps trying to Lock until ctx done.
//func (s *SafeStore[T]) UpdateCtx(ctx context.Context, interval time.Duration, f func(old T) T) error {
//	if interval <= 0 {
//		interval = backoffWait
//	}
//	for {
//		if s.mu.TryLock() {
//			defer s.mu.Unlock()
//			s.data = f(s.data)
//			return nil
//		}
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		case <-time.After(interval):
//		}
//	}
//}

//
//type SafeMap[K comparable, V any] struct {
//	mu   sync.RWMutex
//	data map[K]V
//}
//
//func NewSafeMap[K comparable, V any]() *SafeMap[K, V] {
//	return &SafeMap[K, V]{data: make(map[K]V)}
//}
//
//func (m *SafeMap[K, V]) Get(key K) (V, bool) {
//	m.mu.RLock()
//	defer m.mu.RUnlock()
//	v, ok := m.data[key]
//	return v, ok
//}
//
//func (m *SafeMap[K, V]) Set(key K, val V) {
//	m.mu.Lock()
//	defer m.mu.Unlock()
//	m.data[key] = val
//}
//
//func (m *SafeMap[K, V]) Delete(key K) {
//	m.mu.Lock()
//	defer m.mu.Unlock()
//	delete(m.data, key)
//}
//
//func (m *SafeMap[K, V]) Len() int {
//	m.mu.RLock()
//	defer m.mu.RUnlock()
//	return len(m.data)
//}
//
//func (m *SafeMap[K, V]) Keys() []K {
//	m.mu.RLock()
//	defer m.mu.RUnlock()
//	out := make([]K, 0, len(m.data))
//	for k := range m.data {
//		out = append(out, k)
//	}
//	return out
//}
//
//func (m *SafeMap[K, V]) Values() []V {
//	m.mu.RLock()
//	defer m.mu.RUnlock()
//	out := make([]V, 0, len(m.data))
//	for _, v := range m.data {
//		out = append(out, v)
//	}
//	return out
//}
//
//// Upsert updates key with f(old, exists) atomically and returns the new value.
//func (m *SafeMap[K, V]) Upsert(key K, f func(old V, exists bool) V) V {
//	m.mu.Lock()
//	defer m.mu.Unlock()
//	old, ok := m.data[key]
//	nv := f(old, ok)
//	m.data[key] = nv
//	return nv
//}
//
//// TryGet is a non-blocking Get. ok=false if lock couldn't be acquired immediately.
//func (m *SafeMap[K, V]) TryGet(key K) (V, bool, bool) {
//	if !m.mu.TryRLock() {
//		var zero V
//		return zero, false, false
//	}
//	defer m.mu.RUnlock()
//	v, ok := m.data[key]
//	return v, ok, true
//}
//
//// TrySet is a non-blocking Set. returns acquired=false if lock not acquired.
//func (m *SafeMap[K, V]) TrySet(key K, val V) (acquired bool) {
//	if !m.mu.TryLock() {
//		return false
//	}
//	defer m.mu.Unlock()
//	m.data[key] = val
//	return true
//}
//
//// UpsertCtx retries TryLock until ctx done.
//func (m *SafeMap[K, V]) UpsertCtx(ctx context.Context, interval time.Duration, key K, f func(old V, exists bool) V) error {
//	if interval <= 0 {
//		interval = backoffWait
//	}
//	for {
//		if m.mu.TryLock() {
//			defer m.mu.Unlock()
//			old, ok := m.data[key]
//			m.data[key] = f(old, ok)
//			return nil
//		}
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		case <-time.After(interval):
//		}
//	}
//}
//
//type SafeSlice[T any] struct {
//	mu   sync.RWMutex
//	data []T
//}
//
//func NewSafeSlice[T any]() *SafeSlice[T] {
//	return &SafeSlice[T]{data: make([]T, 0)}
//}
//
//func (s *SafeSlice[T]) Append(v T) {
//	s.mu.Lock()
//	defer s.mu.Unlock()
//	s.data = append(s.data, v)
//}
//
//func (s *SafeSlice[T]) Len() int {
//	s.mu.RLock()
//	defer s.mu.RUnlock()
//	return len(s.data)
//}
//
//// GetAll returns a snapshot copy of the underlying slice.
//func (s *SafeSlice[T]) GetAll() []T {
//	s.mu.RLock()
//	defer s.mu.RUnlock()
//	out := make([]T, len(s.data))
//	copy(out, s.data)
//	return out
//}
//
//// ReplaceAll swaps the entire slice atomically.
//func (s *SafeSlice[T]) ReplaceAll(newData []T) {
//	s.mu.Lock()
//	defer s.mu.Unlock()
//	s.data = make([]T, len(newData))
//	copy(s.data, newData)
//}
//
//// TryAppend attempts a non-blocking Append.
//func (s *SafeSlice[T]) TryAppend(v T) bool {
//	if !s.mu.TryLock() {
//		return false
//	}
//	defer s.mu.Unlock()
//	s.data = append(s.data, v)
//	return true
//}
//
//// AppendCtx retries TryLock until ctx done.
//func (s *SafeSlice[T]) AppendCtx(ctx context.Context, interval time.Duration, v T) error {
//	if interval <= 0 {
//		interval = backoffWait
//	}
//	for {
//		if s.mu.TryLock() {
//			defer s.mu.Unlock()
//			s.data = append(s.data, v)
//			return nil
//		}
//		select {
//		case <-ctx.Done():
//			return ctx.Err()
//		case <-time.After(interval):
//		}
//	}
//}
