package client

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRetry_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetry_SuccessOnSecondAttempt(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		if calls == 1 {
			return &HTTPError{StatusCode: 500, Message: "server error"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetry_AllAttemptsFail(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return &HTTPError{StatusCode: 503, Message: "service unavailable"}
	})
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}

func TestRetry_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	err := RetryDo(ctx, 5, 50*time.Millisecond, func() error {
		calls++
		return &HTTPError{StatusCode: 429, Message: "rate limited"}
	})

	if err == nil {
		t.Fatal("expected error when context canceled")
	}
	if calls >= 5 {
		t.Errorf("expected fewer than 5 calls due to context cancel, got %d", calls)
	}
}

func TestRetry_NonRetryableError_NoRetry(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return &HTTPError{StatusCode: 400, Message: "bad request"}
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for 400), got %d", calls)
	}
}

func TestRetry_NonHTTPError_NoRetry(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		return fmt.Errorf("some non-HTTP error")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for non-HTTP errors), got %d", calls)
	}
}

func TestRetry_429IsRetryable(t *testing.T) {
	calls := 0
	err := RetryDo(context.Background(), 3, 10*time.Millisecond, func() error {
		calls++
		if calls < 3 {
			return &HTTPError{StatusCode: 429, Message: "rate limited"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
}
