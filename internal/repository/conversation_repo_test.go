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

func newTestConversationRepo(t *testing.T) (*repository.ConversationRepo, *repository.ContactRepo, *repository.AccountRepo) {
	t.Helper()
	pool := setupTestDB(t)
	cleanDB(t, pool)

	enc, err := crypto.NewEncryptor(testEncKey)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}

	return repository.NewConversationRepo(pool), repository.NewContactRepo(pool), repository.NewAccountRepo(pool, enc)
}

func createTestContact(t *testing.T, contactRepo *repository.ContactRepo, accountID uuid.UUID) *model.Contact {
	t.Helper()
	ctx := context.Background()
	c, err := contactRepo.Create(ctx, &model.Contact{
		AccountID:               accountID,
		IGSenderID:              "sender_" + uuid.New().String()[:8],
		ChatwootContactID:       1,
		ChatwootContactSourceID: "src_" + uuid.New().String()[:8],
	})
	if err != nil {
		t.Fatalf("createTestContact: %v", err)
	}
	return c
}

func TestCreateConversation(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	conv := &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 100,
		LastCustomerMessageAt:  time.Now(),
	}

	created, err := convRepo.Create(ctx, conv)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Error("ID should not be nil")
	}
	if created.ChatwootConversationID != 100 {
		t.Errorf("ChatwootConversationID = %d, want 100", created.ChatwootConversationID)
	}
	if !created.IsActive {
		t.Error("new conversation should be active")
	}
}

func TestGetActiveByContact_Found(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	created, _ := convRepo.Create(ctx, &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 200,
		LastCustomerMessageAt:  time.Now(),
	})

	got, err := convRepo.GetActiveByContact(ctx, acc.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetActiveByContact: %v", err)
	}
	if got == nil {
		t.Fatal("should return conversation")
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
}

func TestGetActiveByContact_NotFound(t *testing.T) {
	convRepo, _, _ := newTestConversationRepo(t)
	ctx := context.Background()

	got, err := convRepo.GetActiveByContact(ctx, uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("GetActiveByContact: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestGetActiveByContact_InactiveSkipped(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	created, _ := convRepo.Create(ctx, &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 300,
		LastCustomerMessageAt:  time.Now(),
	})
	_ = convRepo.Deactivate(ctx, created.ID)

	got, err := convRepo.GetActiveByContact(ctx, acc.ID, contact.ID)
	if err != nil {
		t.Fatalf("GetActiveByContact: %v", err)
	}
	if got != nil {
		t.Error("inactive conversation should not be returned")
	}
}

func TestGetByChatwootID_Found(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	created, _ := convRepo.Create(ctx, &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 400,
		LastCustomerMessageAt:  time.Now(),
	})

	got, err := convRepo.GetByChatwootID(ctx, 400)
	if err != nil {
		t.Fatalf("GetByChatwootID: %v", err)
	}
	if got == nil {
		t.Fatal("should return conversation")
	}
	if got.ID != created.ID {
		t.Errorf("ID = %v, want %v", got.ID, created.ID)
	}
}

func TestGetByChatwootID_NotFound(t *testing.T) {
	convRepo, _, _ := newTestConversationRepo(t)
	ctx := context.Background()

	got, err := convRepo.GetByChatwootID(ctx, 99999)
	if err != nil {
		t.Fatalf("GetByChatwootID: %v", err)
	}
	if got != nil {
		t.Error("should return nil for not found")
	}
}

func TestUpdateLastCustomerMessage(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	original := time.Now().Add(-1 * time.Hour)
	created, _ := convRepo.Create(ctx, &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 500,
		LastCustomerMessageAt:  original,
	})

	newTime := time.Now()
	err := convRepo.UpdateLastCustomerMessage(ctx, created.ID, newTime)
	if err != nil {
		t.Fatalf("UpdateLastCustomerMessage: %v", err)
	}

	got, _ := convRepo.GetByChatwootID(ctx, 500)
	if got == nil {
		t.Fatal("should return conversation")
	}
	if got.LastCustomerMessageAt.Before(original.Add(30 * time.Minute)) {
		t.Error("LastCustomerMessageAt should have been updated")
	}
}

func TestDeactivate(t *testing.T) {
	convRepo, contactRepo, accountRepo := newTestConversationRepo(t)
	ctx := context.Background()

	acc := createTestAccount(t, accountRepo)
	contact := createTestContact(t, contactRepo, acc.ID)

	created, _ := convRepo.Create(ctx, &model.Conversation{
		AccountID:              acc.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: 600,
		LastCustomerMessageAt:  time.Now(),
	})

	err := convRepo.Deactivate(ctx, created.ID)
	if err != nil {
		t.Fatalf("Deactivate: %v", err)
	}

	// GetByChatwootID filters active only, so should return nil
	got, _ := convRepo.GetByChatwootID(ctx, 600)
	if got != nil {
		t.Error("deactivated conversation should not be returned by GetByChatwootID")
	}
}
