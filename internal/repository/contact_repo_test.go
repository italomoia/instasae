package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/italomoia/instasae/internal/crypto"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/repository"
)

func createTestAccount(t *testing.T, repo *repository.AccountRepo) *model.Account {
	t.Helper()
	ctx := context.Background()
	acc, err := repo.Create(ctx, validCreateParams("page_"+uuid.New().String()[:8]))
	if err != nil {
		t.Fatalf("createTestAccount: %v", err)
	}
	return acc
}

func newTestContactRepo(t *testing.T) (*repository.ContactRepo, *repository.AccountRepo) {
	t.Helper()
	pool := setupTestDB(t)
	cleanDB(t, pool)

	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	return repository.NewContactRepo(pool), repository.NewAccountRepo(pool, enc)
}

func TestCreateContact(t *testing.T) {
	contactRepo, accountRepo := newTestContactRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)

	username := "johndoe"
	name := "John Doe"
	contact := &model.Contact{
		AccountID:               acc.ID,
		IGSenderID:              "sender_001",
		ChatwootContactID:       42,
		ChatwootContactSourceID: "source_abc",
		IGUsername:               &username,
		IGName:                  &name,
	}

	created, err := contactRepo.Create(ctx, contact)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if created.IGSenderID != "sender_001" {
		t.Errorf("IGSenderID = %q, want %q", created.IGSenderID, "sender_001")
	}
	if created.ChatwootContactID != 42 {
		t.Errorf("ChatwootContactID = %d, want 42", created.ChatwootContactID)
	}
	if created.IGUsername == nil || *created.IGUsername != "johndoe" {
		t.Error("IGUsername should be set")
	}
}

func TestGetByAccountAndSender_Found(t *testing.T) {
	contactRepo, accountRepo := newTestContactRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := &model.Contact{
		AccountID:               acc.ID,
		IGSenderID:              "sender_find",
		ChatwootContactID:       10,
		ChatwootContactSourceID: "src_find",
	}
	created, _ := contactRepo.Create(ctx, contact)

	got, err := contactRepo.GetByAccountAndSender(ctx, acc.ID, "sender_find")
	if err != nil {
		t.Fatalf("GetByAccountAndSender: %v", err)
	}
	if got == nil {
		t.Fatal("should return contact")
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
}

func TestGetByAccountAndSender_NotFound(t *testing.T) {
	contactRepo, _ := newTestContactRepo(t)
	ctx := context.Background()

	got, err := contactRepo.GetByAccountAndSender(ctx, uuid.New(), "nonexistent")
	if err != nil {
		t.Fatalf("GetByAccountAndSender: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestCreateContact_DuplicateSender(t *testing.T) {
	contactRepo, accountRepo := newTestContactRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := &model.Contact{
		AccountID:               acc.ID,
		IGSenderID:              "sender_dup",
		ChatwootContactID:       1,
		ChatwootContactSourceID: "src_1",
	}
	_, _ = contactRepo.Create(ctx, contact)

	contact2 := &model.Contact{
		AccountID:               acc.ID,
		IGSenderID:              "sender_dup",
		ChatwootContactID:       2,
		ChatwootContactSourceID: "src_2",
	}
	_, err := contactRepo.Create(ctx, contact2)
	if err == nil {
		t.Error("duplicate (account_id, ig_sender_id) should fail")
	}
}
