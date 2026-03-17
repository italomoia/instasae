package client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// HTTPError represents an HTTP error with a status code.
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// isRetryable returns true if the error is an HTTPError with a retryable status code.
func isRetryable(err error) bool {
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		return false
	}
	if httpErr.StatusCode == 429 {
		return true
	}
	if httpErr.StatusCode >= 500 && httpErr.StatusCode < 600 {
		return true
	}
	return false
}

// RetryDo executes fn with exponential backoff retry on transient HTTP errors.
// Only retries on 429 and 5xx status codes. baseDelay is doubled on each attempt.
func RetryDo(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt < maxAttempts; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if !isRetryable(lastErr) {
			return lastErr
		}

		if attempt == maxAttempts-1 {
			break
		}

		delay := baseDelay << uint(attempt) // baseDelay * 2^attempt
		slog.Warn("retrying after transient error",
			"attempt", attempt+1,
			"max_attempts", maxAttempts,
			"delay", delay,
			"error", lastErr,
		)

		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(delay):
		}
	}

	return lastErr
}
