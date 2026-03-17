package service_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

// logRecord captures a single slog record for test assertions.
type logRecord struct {
	Level   slog.Level
	Message string
	Attrs   map[string]any
}

// testLogHandler captures log records for assertions.
type testLogHandler struct {
	mu      sync.Mutex
	records []logRecord
}

func (h *testLogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *testLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	rec := logRecord{Level: r.Level, Message: r.Message, Attrs: make(map[string]any)}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.Any()
		return true
	})
	h.records = append(h.records, rec)
	return nil
}

func (h *testLogHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *testLogHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *testLogHandler) getRecords() []logRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	cp := make([]logRecord, len(h.records))
	copy(cp, h.records)
	return cp
}

func TestTokenChecker_LogsExpiringTokens(t *testing.T) {
	expires := time.Now().Add(3 * 24 * time.Hour) // 3 days from now
	repo := &mockAccountRepo{
		ListActiveWithExpiringTokensFn: func(ctx context.Context, within time.Duration) ([]model.Account, error) {
			return []model.Account{
				{ID: uuid.New(), IGPageName: "Store A", TokenExpiresAt: &expires},
				{ID: uuid.New(), IGPageName: "Store B", TokenExpiresAt: &expires},
			}, nil
		},
	}

	handler := &testLogHandler{}
	logger := slog.New(handler)

	ctx, cancel := context.WithCancel(context.Background())
	checker := service.NewTokenChecker(repo, logger, 50*time.Millisecond, 7*24*time.Hour)
	checker.Start(ctx)

	// Wait for at least one check cycle
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	records := handler.getRecords()

	// Should have WARN logs for each expiring account
	warnCount := 0
	for _, r := range records {
		if r.Level == slog.LevelWarn && r.Message == "token expiring soon" {
			warnCount++
		}
	}
	if warnCount < 2 {
		t.Errorf("expected at least 2 warn logs for expiring tokens, got %d", warnCount)
	}

	// Should have INFO log with expiring count
	foundInfo := false
	for _, r := range records {
		if r.Level == slog.LevelInfo && r.Message == "token expiration check complete" {
			foundInfo = true
			break
		}
	}
	if !foundInfo {
		t.Error("expected INFO log for check completion")
	}
}

func TestTokenChecker_NoExpiringTokens(t *testing.T) {
	repo := &mockAccountRepo{
		ListActiveWithExpiringTokensFn: func(ctx context.Context, within time.Duration) ([]model.Account, error) {
			return []model.Account{}, nil
		},
	}

	handler := &testLogHandler{}
	logger := slog.New(handler)

	ctx, cancel := context.WithCancel(context.Background())
	checker := service.NewTokenChecker(repo, logger, 50*time.Millisecond, 7*24*time.Hour)
	checker.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(20 * time.Millisecond)

	records := handler.getRecords()

	// Should have INFO log with count 0
	found := false
	for _, r := range records {
		if r.Level == slog.LevelInfo && r.Message == "token expiration check complete" {
			if count, ok := r.Attrs["expiring_count"]; ok {
				if count.(int64) == 0 {
					found = true
				}
			}
			break
		}
	}
	if !found {
		t.Error("expected INFO log with expiring_count=0")
	}

	// No WARN logs
	for _, r := range records {
		if r.Level == slog.LevelWarn {
			t.Errorf("unexpected WARN log: %s", r.Message)
		}
	}
}

func TestTokenChecker_StopsOnContextCancel(t *testing.T) {
	callCount := 0
	repo := &mockAccountRepo{
		ListActiveWithExpiringTokensFn: func(ctx context.Context, within time.Duration) ([]model.Account, error) {
			callCount++
			return []model.Account{}, nil
		},
	}

	handler := &testLogHandler{}
	logger := slog.New(handler)

	ctx, cancel := context.WithCancel(context.Background())
	checker := service.NewTokenChecker(repo, logger, 1*time.Hour, 7*24*time.Hour)
	checker.Start(ctx)

	// Wait for the immediate first check
	time.Sleep(50 * time.Millisecond)
	countAfterFirst := callCount

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Should have been called at least once (immediate check)
	if countAfterFirst < 1 {
		t.Error("expected at least 1 call from immediate check")
	}

	// After cancel, no more calls should happen
	countAfterCancel := callCount
	time.Sleep(50 * time.Millisecond)
	if callCount != countAfterCancel {
		t.Error("checker should have stopped after context cancel")
	}
}
