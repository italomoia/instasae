package service_test

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
)

// B2Client mock

type mockB2Client struct {
	UploadFn func(ctx context.Context, key string, data io.Reader, contentType string) (string, error)
}

func (m *mockB2Client) Upload(ctx context.Context, key string, data io.Reader, contentType string) (string, error) {
	return m.UploadFn(ctx, key, data, contentType)
}

// InstagramClient mock

type mockInstagramClient struct {
	SendTextMessageFn func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error)
	SendAttachmentFn  func(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error)
	GetUserProfileFn  func(ctx context.Context, token string, userID string) (*model.IGUserProfile, error)
}

func (m *mockInstagramClient) SendTextMessage(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
	return m.SendTextMessageFn(ctx, pageID, token, recipientID, text)
}

func (m *mockInstagramClient) SendAttachment(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error) {
	return m.SendAttachmentFn(ctx, pageID, token, recipientID, attachmentType, url)
}

func (m *mockInstagramClient) GetUserProfile(ctx context.Context, token string, userID string) (*model.IGUserProfile, error) {
	return m.GetUserProfileFn(ctx, token, userID)
}

// ChatwootClient mock

type mockChatwootClient struct {
	CreateContactFn      func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error)
	CreateConversationFn func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error)
	CreateMessageFn      func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error
}

func (m *mockChatwootClient) CreateContact(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error) {
	return m.CreateContactFn(ctx, baseURL, accountID, token, req)
}

func (m *mockChatwootClient) CreateConversation(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error) {
	return m.CreateConversationFn(ctx, baseURL, accountID, token, req)
}

func (m *mockChatwootClient) CreateMessage(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
	return m.CreateMessageFn(ctx, baseURL, accountID, token, conversationID, req)
}

// AccountRepository mock

type mockAccountRepo struct {
	CreateFn                       func(ctx context.Context, params model.CreateAccountParams) (*model.Account, error)
	GetByIDFn                      func(ctx context.Context, id uuid.UUID) (*model.Account, error)
	GetByIGPageIDFn                func(ctx context.Context, igPageID string) (*model.Account, error)
	ListFn                         func(ctx context.Context) ([]model.Account, error)
	UpdateFn                       func(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error)
	SoftDeleteFn                   func(ctx context.Context, id uuid.UUID) error
	ListActiveWithExpiringTokensFn func(ctx context.Context, within time.Duration) ([]model.Account, error)
}

func (m *mockAccountRepo) Create(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
	return m.CreateFn(ctx, params)
}

func (m *mockAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	return m.GetByIDFn(ctx, id)
}

func (m *mockAccountRepo) GetByIGPageID(ctx context.Context, igPageID string) (*model.Account, error) {
	return m.GetByIGPageIDFn(ctx, igPageID)
}

func (m *mockAccountRepo) List(ctx context.Context) ([]model.Account, error) {
	return m.ListFn(ctx)
}

func (m *mockAccountRepo) Update(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error) {
	return m.UpdateFn(ctx, id, params)
}

func (m *mockAccountRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return m.SoftDeleteFn(ctx, id)
}

func (m *mockAccountRepo) ListActiveWithExpiringTokens(ctx context.Context, within time.Duration) ([]model.Account, error) {
	return m.ListActiveWithExpiringTokensFn(ctx, within)
}

// ContactRepository mock

type mockContactRepo struct {
	CreateFn                func(ctx context.Context, contact *model.Contact) (*model.Contact, error)
	GetByIDFn               func(ctx context.Context, id uuid.UUID) (*model.Contact, error)
	GetByAccountAndSenderFn func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error)
}

func (m *mockContactRepo) Create(ctx context.Context, contact *model.Contact) (*model.Contact, error) {
	return m.CreateFn(ctx, contact)
}

func (m *mockContactRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Contact, error) {
	return m.GetByIDFn(ctx, id)
}

func (m *mockContactRepo) GetByAccountAndSender(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
	return m.GetByAccountAndSenderFn(ctx, accountID, igSenderID)
}

// ConversationRepository mock

type mockConversationRepo struct {
	CreateFn                   func(ctx context.Context, conv *model.Conversation) (*model.Conversation, error)
	GetActiveByContactFn       func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error)
	GetByChatwootIDFn          func(ctx context.Context, chatwootConvID int) (*model.Conversation, error)
	UpdateLastCustomerMessageFn func(ctx context.Context, id uuid.UUID, at time.Time) error
	DeactivateFn               func(ctx context.Context, id uuid.UUID) error
}

func (m *mockConversationRepo) Create(ctx context.Context, conv *model.Conversation) (*model.Conversation, error) {
	return m.CreateFn(ctx, conv)
}

func (m *mockConversationRepo) GetActiveByContact(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
	return m.GetActiveByContactFn(ctx, accountID, contactID)
}

func (m *mockConversationRepo) GetByChatwootID(ctx context.Context, chatwootConvID int) (*model.Conversation, error) {
	return m.GetByChatwootIDFn(ctx, chatwootConvID)
}

func (m *mockConversationRepo) UpdateLastCustomerMessage(ctx context.Context, id uuid.UUID, at time.Time) error {
	return m.UpdateLastCustomerMessageFn(ctx, id, at)
}

func (m *mockConversationRepo) Deactivate(ctx context.Context, id uuid.UUID) error {
	return m.DeactivateFn(ctx, id)
}

// Cache mock

type mockCache struct {
	SetDedupFn      func(ctx context.Context, messageID string, ttl time.Duration) (bool, error)
	GetAccountFn    func(ctx context.Context, igPageID string) (*model.Account, error)
	SetAccountFn    func(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error
	DeleteAccountFn func(ctx context.Context, igPageID string) error
}

func (m *mockCache) SetDedup(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
	return m.SetDedupFn(ctx, messageID, ttl)
}

func (m *mockCache) GetAccount(ctx context.Context, igPageID string) (*model.Account, error) {
	return m.GetAccountFn(ctx, igPageID)
}

func (m *mockCache) SetAccount(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error {
	return m.SetAccountFn(ctx, igPageID, account, ttl)
}

func (m *mockCache) DeleteAccount(ctx context.Context, igPageID string) error {
	return m.DeleteAccountFn(ctx, igPageID)
}

// Encryptor mock

type mockEncryptor struct {
	EncryptFn func(plaintext string) (string, error)
	DecryptFn func(ciphertext string) (string, error)
}

func (m *mockEncryptor) Encrypt(plaintext string) (string, error) {
	return m.EncryptFn(plaintext)
}

func (m *mockEncryptor) Decrypt(ciphertext string) (string, error) {
	return m.DecryptFn(ciphertext)
}

// MediaHandler mock

type mockMediaHandler struct {
	DownloadAndUploadFn func(ctx context.Context, sourceURL string, accountID string, contentType string) (string, error)
}

func (m *mockMediaHandler) DownloadAndUpload(ctx context.Context, sourceURL string, accountID string, contentType string) (string, error) {
	return m.DownloadAndUploadFn(ctx, sourceURL, accountID, contentType)
}
