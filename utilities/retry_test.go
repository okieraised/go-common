package utilities

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRetry_SuccessOnFirstTry(t *testing.T) {
	calls := 0
	Retry(func() error {
		calls++
		return nil
	}, 5)
	require.Equal(t, 1, calls, "should stop after first success")
}

func TestRetry_FailAllAttempts(t *testing.T) {
	calls := 0
	Retry(func() error {
		calls++
		return errors.New("fail")
	}, 3)
	require.Equal(t, 3, calls, "should try exactly maxRetry times")
}

func TestRetryWithBackoff_SuccessAfterRetries(t *testing.T) {
	calls := 0
	start := time.Now()

	RetryWithBackoff(func() error {
		calls++
		if calls < 3 {
			return errors.New("fail")
		}
		return nil
	}, 5, 10*time.Millisecond, 40*time.Millisecond)

	elapsed := time.Since(start)
	require.GreaterOrEqual(t, elapsed, 10*time.Millisecond+20*time.Millisecond,
		"should wait at least backoff between retries")
	require.Equal(t, 3, calls, "should stop after success")
}

func TestRetryWithBackoff_ClampsToMaxBackoff(t *testing.T) {
	calls := 0
	backoffs := []time.Duration{}
	start := time.Now()

	RetryWithBackoff(func() error {
		calls++
		if calls < 4 {
			backoffs = append(backoffs, time.Since(start))
			return errors.New("fail")
		}
		return nil
	}, 4, 5*time.Millisecond, 6*time.Millisecond)

	require.Equal(t, 4, calls, "should stop after max retries")
	for i := 1; i < len(backoffs); i++ {
		require.LessOrEqual(t, backoffs[i]-backoffs[i-1], 10*time.Millisecond)
	}
}

func TestRetryWithBackoff_ZeroRetriesDoesNothing(t *testing.T) {
	calls := 0
	RetryWithBackoff(func() error {
		calls++
		return errors.New("fail")
	}, 0, 10*time.Millisecond, 10*time.Millisecond)
	require.Equal(t, 0, calls, "should not call fn when maxRetry=0")
}

func TestRetry_ZeroRetriesDoesNothing(t *testing.T) {
	calls := 0
	Retry(func() error {
		calls++
		return errors.New("fail")
	}, 0)
	require.Equal(t, 0, calls, "should not call fn when maxRetry=0")
}
