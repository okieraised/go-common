package goroutineutils

import (
	"sync"
	"time"
)

// Debounce returns a function that postpones fn until d has elapsed since the last call.
// The returned Stop func cancels any pending call.
func Debounce(d time.Duration, fn func()) (call func(), stop func()) {
	var mu sync.Mutex
	var t *time.Timer
	call = func() {
		mu.Lock()
		defer mu.Unlock()
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(d, fn)
	}
	stop = func() {
		mu.Lock()
		if t != nil {
			t.Stop()
			t = nil
		}
		mu.Unlock()
	}
	return
}
