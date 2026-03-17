package service

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/italomoia/instasae/internal/domain"
)

type TokenChecker struct {
	repo          domain.AccountRepository
	logger        *slog.Logger
	interval      time.Duration
	warningWindow time.Duration
}

func NewTokenChecker(repo domain.AccountRepository, logger *slog.Logger, interval time.Duration, warningWindow time.Duration) *TokenChecker {
	return &TokenChecker{
		repo:          repo,
		logger:        logger,
		interval:      interval,
		warningWindow: warningWindow,
	}
}

func (tc *TokenChecker) Start(ctx context.Context) {
	go tc.run(ctx)
}

func (tc *TokenChecker) run(ctx context.Context) {
	// Immediate first check
	tc.check(ctx)

	ticker := time.NewTicker(tc.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tc.check(ctx)
		}
	}
}

func (tc *TokenChecker) check(ctx context.Context) {
	accounts, err := tc.repo.ListActiveWithExpiringTokens(ctx, tc.warningWindow)
	if err != nil {
		tc.logger.Error("failed to check expiring tokens", "error", err)
		return
	}

	for _, acc := range accounts {
		daysRemaining := 0
		if acc.TokenExpiresAt != nil {
			daysRemaining = int(math.Round(time.Until(*acc.TokenExpiresAt).Hours() / 24))
		}
		tc.logger.Warn("token expiring soon",
			"account_id", acc.ID,
			"ig_page_name", acc.IGPageName,
			"days_remaining", daysRemaining,
		)
	}

	tc.logger.Info("token expiration check complete",
		"expiring_count", int64(len(accounts)),
	)
}
