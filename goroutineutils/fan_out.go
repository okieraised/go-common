package goroutineutils

import "context"

// FanOut duplicates every value from `in` to `n` outputs.
// It guarantees delivery to every output (no dropping).
// A slow consumer can still stall the broadcaster once its buffer fills.
// Use `buf` to provide per-output buffering (>=0).
func FanOut[T any](ctx context.Context, in <-chan T, n, buf int) []<-chan T {
	if n <= 0 {
		n = 1
	}
	if buf < 0 {
		buf = 0
	}
	outs := make([]chan T, n)
	rs := make([]<-chan T, n)
	for i := 0; i < n; i++ {
		outs[i] = make(chan T, buf)
		rs[i] = outs[i]
	}

	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				// broadcast v to all outputs (reliable)
				for _, ch := range outs {
					select {
					case <-ctx.Done():
						return
					case ch <- v:
					}
				}
			}
		}
	}()

	return rs
}

// FanOutLossy duplicates values from `in` to `n` outputs but never blocks:
// if an output channel's buffer is full, the value for that output is dropped.
// Great for pub/sub-ish telemetry where freshness > completeness.
func FanOutLossy[T any](ctx context.Context, in <-chan T, n, buf int) []<-chan T {
	if n <= 0 {
		n = 1
	}
	if buf < 0 {
		buf = 0
	}
	outs := make([]chan T, n)
	rs := make([]<-chan T, n)
	for i := 0; i < n; i++ {
		outs[i] = make(chan T, buf)
		rs[i] = outs[i]
	}

	go func() {
		defer func() {
			for _, ch := range outs {
				close(ch)
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				for _, ch := range outs {
					select {
					case ch <- v:
					case <-ctx.Done():
						return
					default:
						// drop for this output
					}
				}
			}
		}
	}()

	return rs
}
