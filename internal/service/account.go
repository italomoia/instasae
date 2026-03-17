package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

type AccountService struct {
	repo  domain.AccountRepository
	cache domain.Cache
}

func NewAccountService(repo domain.AccountRepository, cache domain.Cache) *AccountService {
	return &AccountService{repo: repo, cache: cache}
}

func (s *AccountService) Create(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
	acc, err := s.repo.Create(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("creating account: %w", err)
	}
	return acc, nil
}

func (s *AccountService) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *AccountService) List(ctx context.Context) ([]model.Account, error) {
	return s.repo.List(ctx)
}

func (s *AccountService) Update(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error) {
	// If ig_page_id is changing, get the old one to invalidate its cache
	var oldIGPageID string
	if params.IGPageID != nil {
		existing, err := s.repo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("getting account for cache invalidation: %w", err)
		}
		if existing != nil {
			oldIGPageID = existing.IGPageID
		}
	}

	acc, err := s.repo.Update(ctx, id, params)
	if err != nil {
		return nil, fmt.Errorf("updating account: %w", err)
	}
	if acc != nil {
		_ = s.cache.DeleteAccount(ctx, acc.IGPageID)
		// Invalidate old ig_page_id cache if it changed
		if oldIGPageID != "" && oldIGPageID != acc.IGPageID {
			_ = s.cache.DeleteAccount(ctx, oldIGPageID)
		}
	}
	return acc, nil
}

func (s *AccountService) SoftDelete(ctx context.Context, id uuid.UUID) error {
	acc, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting account for delete: %w", err)
	}
	if acc == nil {
		return fmt.Errorf("account not found")
	}

	if err := s.repo.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("soft deleting account: %w", err)
	}

	_ = s.cache.DeleteAccount(ctx, acc.IGPageID)
	return nil
}
