package domain

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
)

// Repository interfaces

type AccountRepository interface {
	Create(ctx context.Context, params model.CreateAccountParams) (*model.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error)
	GetByIGPageID(ctx context.Context, igPageID string) (*model.Account, error)
	List(ctx context.Context) ([]model.Account, error)
	Update(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListActiveWithExpiringTokens(ctx context.Context, within time.Duration) ([]model.Account, error)
}

type ContactRepository interface {
	Create(ctx context.Context, contact *model.Contact) (*model.Contact, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Contact, error)
	GetByAccountAndSender(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error)
}

type ConversationRepository interface {
	Create(ctx context.Context, conv *model.Conversation) (*model.Conversation, error)
	GetActiveByContact(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error)
	GetByChatwootID(ctx context.Context, chatwootConvID int) (*model.Conversation, error)
	UpdateLastCustomerMessage(ctx context.Context, id uuid.UUID, at time.Time) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

// Client interfaces

type InstagramClient interface {
	SendTextMessage(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error)
	SendAttachment(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error)
	GetUserProfile(ctx context.Context, token string, userID string) (*model.IGUserProfile, error)
}

type ChatwootClient interface {
	CreateContact(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error)
	CreateConversation(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error)
	CreateMessage(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error
}

type B2Client interface {
	Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
}

// Service interfaces

type MediaHandler interface {
	DownloadAndUpload(ctx context.Context, sourceURL string, accountID string, attachmentType string) (string, error)
}

// Cache interface

type Cache interface {
	SetDedup(ctx context.Context, messageID string, ttl time.Duration) (bool, error)
	GetAccount(ctx context.Context, igPageID string) (*model.Account, error)
	SetAccount(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error
	DeleteAccount(ctx context.Context, igPageID string) error
}

// Crypto interface

type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}
