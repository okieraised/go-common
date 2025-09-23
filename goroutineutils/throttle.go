package goroutineutils

import (
	"sync"
	"time"
)

// ThrottleLeadingTrailing ensures fn runs at most once per window d,
// executing immediately on the first call (leading edge) and scheduling
// exactly one additional execution (trailing edge) if more calls happen
// during the window.
// ThrottleLeadingTrailing ensures fn runs at most once per window d,
// executing immediately on the first call (leading edge) and scheduling
// exactly one additional execution (trailing edge) if more calls happen
// during the window.
func ThrottleLeadingTrailing(d time.Duration, fn func()) func() {
	if d <= 0 {
		return fn
	}

	var mu sync.Mutex
	var next time.Time
	var trailingScheduled bool
	var timer *time.Timer

	call := func() {
		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		if now.Before(next) {
			// Inside the current window: schedule trailing call if not already
			if !trailingScheduled {
				delay := time.Until(next)
				if delay < 0 {
					delay = 0
				}
				trailingScheduled = true
				if timer != nil {
					timer.Stop()
				}
				timer = time.AfterFunc(delay, func() {
					mu.Lock()
					trailingScheduled = false
					next = time.Now().Add(d) // start a new window
					mu.Unlock()
					fn()
				})
			}
			return
		}

		// We're past the window: execute immediately (leading)
		next = now.Add(d)
		fn()
	}
	return call
}
