package utilities

import "time"

// RetryWithBackoff retries fn until it succeeds or maxRetry attempts are exhausted.
// The backoff doubles each time, up to maxBackoff.
func RetryWithBackoff(fn func() error, maxRetry int, startBackoff, maxBackoff time.Duration) {
	if maxRetry <= 0 {
		return
	}
	backoff := startBackoff
	for attempt := 0; attempt < maxRetry; attempt++ {
		if err := fn(); err == nil {
			return
		}
		if attempt == maxRetry-1 {
			return
		}
		time.Sleep(backoff)
		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

// Retry retries fn up to maxRetry times with no delay.
func Retry(fn func() error, maxRetry int) {
	if maxRetry <= 0 {
		return
	}
	for attempt := 0; attempt < maxRetry; attempt++ {
		if err := fn(); err == nil {
			return
		}
		if attempt == maxRetry-1 {
			return
		}
	}
}
