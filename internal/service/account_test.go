package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

func noopCache() *mockCache {
	return &mockCache{
		SetDedupFn:      func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) { return true, nil },
		GetAccountFn:    func(ctx context.Context, igPageID string) (*model.Account, error) { return nil, nil },
		SetAccountFn:    func(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error { return nil },
		DeleteAccountFn: func(ctx context.Context, igPageID string) error { return nil },
	}
}

func sampleAccount() *model.Account {
	return &model.Account{
		ID:                uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		IGPageID:          "page_123",
		IGPageName:        "Test Store",
		ChatwootBaseURL:   "https://chat.example.com",
		ChatwootAccountID: 1,
		ChatwootInboxID:   5,
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

func TestCreateAccount_Success(t *testing.T) {
	want := sampleAccount()
	repo := &mockAccountRepo{
		CreateFn: func(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
			return want, nil
		},
	}

	svc := service.NewAccountService(repo, noopCache())
	got, err := svc.Create(context.Background(), model.CreateAccountParams{IGPageID: "page_123"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.IGPageID != want.IGPageID {
		t.Errorf("IGPageID = %q, want %q", got.IGPageID, want.IGPageID)
	}
}

func TestCreateAccount_DuplicateError(t *testing.T) {
	repo := &mockAccountRepo{
		CreateFn: func(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
			return nil, fmt.Errorf("duplicate ig_page_id")
		},
	}

	svc := service.NewAccountService(repo, noopCache())
	_, err := svc.Create(context.Background(), model.CreateAccountParams{IGPageID: "dup"})
	if err == nil {
		t.Error("should return error on duplicate")
	}
}

func TestGetByID_Found(t *testing.T) {
	want := sampleAccount()
	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			return want, nil
		},
	}

	svc := service.NewAccountService(repo, noopCache())
	got, err := svc.GetByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("ID = %v, want %v", got.ID, want.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			return nil, nil
		},
	}

	svc := service.NewAccountService(repo, noopCache())
	got, err := svc.GetByID(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestList(t *testing.T) {
	accounts := []model.Account{*sampleAccount(), *sampleAccount()}
	accounts[1].IGPageID = "page_456"

	repo := &mockAccountRepo{
		ListFn: func(ctx context.Context) ([]model.Account, error) {
			return accounts, nil
		},
	}

	svc := service.NewAccountService(repo, noopCache())
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d accounts, want 2", len(got))
	}
}

func TestUpdate_Success(t *testing.T) {
	updated := sampleAccount()
	updated.IGPageName = "Updated Store"

	var cacheDeletedKey string
	cache := noopCache()
	cache.DeleteAccountFn = func(ctx context.Context, igPageID string) error {
		cacheDeletedKey = igPageID
		return nil
	}

	repo := &mockAccountRepo{
		UpdateFn: func(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error) {
			return updated, nil
		},
	}

	svc := service.NewAccountService(repo, cache)
	name := "Updated Store"
	got, err := svc.Update(context.Background(), updated.ID, model.UpdateAccountParams{IGPageName: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got.IGPageName != "Updated Store" {
		t.Errorf("IGPageName = %q, want %q", got.IGPageName, "Updated Store")
	}
	if cacheDeletedKey != updated.IGPageID {
		t.Errorf("cache deleted key = %q, want %q", cacheDeletedKey, updated.IGPageID)
	}
}

func TestSoftDelete_Success(t *testing.T) {
	acc := sampleAccount()

	var softDeleted bool
	var cacheDeletedKey string

	cache := noopCache()
	cache.DeleteAccountFn = func(ctx context.Context, igPageID string) error {
		cacheDeletedKey = igPageID
		return nil
	}

	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			return acc, nil
		},
		SoftDeleteFn: func(ctx context.Context, id uuid.UUID) error {
			softDeleted = true
			return nil
		},
	}

	svc := service.NewAccountService(repo, cache)
	err := svc.SoftDelete(context.Background(), acc.ID)
	if err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if !softDeleted {
		t.Error("repo.SoftDelete should have been called")
	}
	if cacheDeletedKey != acc.IGPageID {
		t.Errorf("cache deleted key = %q, want %q", cacheDeletedKey, acc.IGPageID)
	}
}
