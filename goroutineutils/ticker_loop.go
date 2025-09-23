package goroutineutils

import (
	"context"
	"time"
)

// TickerLoop runs fn every interval until ctx is done.
// If jitter>0, each tick is delayed by an additional random-ish sub-millisecond skew
// derived from time.Now().Nanosecond() mod jitter to avoid lockstep.
func TickerLoop(ctx context.Context, interval time.Duration, jitter time.Duration, fn func(context.Context) error) error {
	if interval <= 0 {
		interval = time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()

	// Run once immediately.
	if err := fn(ctx); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if jitter > 0 {
				ns := time.Now().Nanosecond()
				d := time.Duration(ns%int(jitter)) * time.Nanosecond
				timer := time.NewTimer(d)
				select {
				case <-ctx.Done():
					timer.Stop()
					return ctx.Err()
				case <-timer.C:
				}
			}
			if err := fn(ctx); err != nil {
				return err
			}
		}
	}
}
