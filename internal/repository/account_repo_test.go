package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/italomoia/instasae/internal/crypto"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/repository"
)

const testEncKey = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func newTestAccountRepo(t *testing.T) *repository.AccountRepo {
	t.Helper()
	pool := setupTestDB(t)
	cleanDB(t, pool)

	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	return repository.NewAccountRepo(pool, enc)
}

func validCreateParams(igPageID string) model.CreateAccountParams {
	return model.CreateAccountParams{
		IGPageID:           igPageID,
		IGPageName:         "Test Store",
		IGAccessToken:      "ig_token_secret",
		ChatwootBaseURL:    "https://chat.example.com",
		ChatwootAccountID:  1,
		ChatwootInboxID:    5,
		ChatwootAPIToken:   "cw_token_secret",
		WebhookVerifyToken: "verify_123",
	}
}

func TestCreateAccount(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	acc, err := repo.Create(ctx, validCreateParams("page_create"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if acc.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if acc.IGPageID != "page_create" {
		t.Errorf("IGPageID = %q, want %q", acc.IGPageID, "page_create")
	}
	if acc.IGAccessToken != "ig_token_secret" {
		t.Errorf("IGAccessToken should be decrypted, got %q", acc.IGAccessToken)
	}
	if acc.ChatwootAPIToken != "cw_token_secret" {
		t.Errorf("ChatwootAPIToken should be decrypted, got %q", acc.ChatwootAPIToken)
	}
	if !acc.IsActive {
		t.Error("new account should be active")
	}
	if time.Since(acc.CreatedAt) > 5*time.Second {
		t.Error("CreatedAt should be recent")
	}
}

func TestCreateAccount_DuplicateIGPageID(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	_, err := repo.Create(ctx, validCreateParams("page_dup"))
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err = repo.Create(ctx, validCreateParams("page_dup"))
	if err == nil {
		t.Error("duplicate ig_page_id should fail")
	}
}

func TestGetByID(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_getid"))

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
	if got.IGAccessToken != "ig_token_secret" {
		t.Errorf("token should be decrypted, got %q", got.IGAccessToken)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	got, err := repo.GetByID(ctx, uuid.New())
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestGetByIGPageID(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_lookup"))

	got, err := repo.GetByIGPageID(ctx, "page_lookup")
	if err != nil {
		t.Fatalf("GetByIGPageID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByIGPageID returned nil")
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
}

func TestGetByIGPageID_NotFound(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	got, err := repo.GetByIGPageID(ctx, "nonexistent_page")
	if err != nil {
		t.Fatalf("GetByIGPageID: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestGetByIGPageID_InactiveSkipped(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_inactive"))
	_ = repo.SoftDelete(ctx, created.ID)

	got, err := repo.GetByIGPageID(ctx, "page_inactive")
	if err != nil {
		t.Fatalf("GetByIGPageID: %v", err)
	}
	if got != nil {
		t.Error("inactive account should not be returned by GetByIGPageID")
	}
}

func TestList(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	_, _ = repo.Create(ctx, validCreateParams("page_list_1"))
	_, _ = repo.Create(ctx, validCreateParams("page_list_2"))
	_, _ = repo.Create(ctx, validCreateParams("page_list_3"))

	accounts, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 3 {
		t.Errorf("got %d accounts, want 3", len(accounts))
	}
}

func TestUpdate(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_update"))

	newName := "Updated Store"
	newInbox := 10
	updated, err := repo.Update(ctx, created.ID, model.UpdateAccountParams{
		IGPageName:  &newName,
		ChatwootInboxID: &newInbox,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.IGPageName != "Updated Store" {
		t.Errorf("IGPageName = %q, want %q", updated.IGPageName, "Updated Store")
	}
	if updated.ChatwootInboxID != 10 {
		t.Errorf("ChatwootInboxID = %d, want 10", updated.ChatwootInboxID)
	}
	if updated.IGAccessToken != "ig_token_secret" {
		t.Error("unchanged token should still be decrypted correctly")
	}
}

func TestUpdate_Token(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_upd_token"))

	newToken := "new_ig_token"
	updated, err := repo.Update(ctx, created.ID, model.UpdateAccountParams{
		IGAccessToken: &newToken,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.IGAccessToken != "new_ig_token" {
		t.Errorf("IGAccessToken = %q, want %q", updated.IGAccessToken, "new_ig_token")
	}
}

func TestSoftDelete(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	created, _ := repo.Create(ctx, validCreateParams("page_delete"))

	err := repo.SoftDelete(ctx, created.ID)
	if err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID after delete: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID should still return soft-deleted account")
	}
	if got.IsActive {
		t.Error("soft-deleted account should have IsActive=false")
	}
}

func TestListActiveWithExpiringTokens(t *testing.T) {
	repo := newTestAccountRepo(t)
	ctx := context.Background()

	// Account with token expiring in 3 days
	expires3d := time.Now().Add(3 * 24 * time.Hour)
	p1 := validCreateParams("page_exp_3d")
	p1.TokenExpiresAt = &expires3d
	_, _ = repo.Create(ctx, p1)

	// Account with token expiring in 10 days
	expires10d := time.Now().Add(10 * 24 * time.Hour)
	p2 := validCreateParams("page_exp_10d")
	p2.TokenExpiresAt = &expires10d
	_, _ = repo.Create(ctx, p2)

	// Account with no expiration
	_, _ = repo.Create(ctx, validCreateParams("page_no_exp"))

	accounts, err := repo.ListActiveWithExpiringTokens(ctx, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("ListActiveWithExpiringTokens: %v", err)
	}
	if len(accounts) != 1 {
		t.Fatalf("got %d accounts, want 1", len(accounts))
	}
	if accounts[0].IGPageID != "page_exp_3d" {
		t.Errorf("IGPageID = %q, want %q", accounts[0].IGPageID, "page_exp_3d")
	}
}
