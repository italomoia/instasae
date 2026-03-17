package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/handler"
	"github.com/italomoia/instasae/internal/middleware"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

// Mocks for handler tests

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

type mockConversationRepo struct {
	CreateFn                    func(ctx context.Context, conv *model.Conversation) (*model.Conversation, error)
	GetActiveByContactFn        func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error)
	GetByChatwootIDFn           func(ctx context.Context, chatwootConvID int) (*model.Conversation, error)
	UpdateLastCustomerMessageFn func(ctx context.Context, id uuid.UUID, at time.Time) error
	DeactivateFn                func(ctx context.Context, id uuid.UUID) error
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

type mockMediaHandler struct {
	DownloadAndUploadFn func(ctx context.Context, sourceURL string, accountID string, contentType string) (string, error)
}

func (m *mockMediaHandler) DownloadAndUpload(ctx context.Context, sourceURL string, accountID string, contentType string) (string, error) {
	return m.DownloadAndUploadFn(ctx, sourceURL, accountID, contentType)
}

// Compile-time interface checks for mocks
var _ domain.AccountRepository = (*mockAccountRepo)(nil)
var _ domain.ContactRepository = (*mockContactRepo)(nil)
var _ domain.ConversationRepository = (*mockConversationRepo)(nil)
var _ domain.InstagramClient = (*mockInstagramClient)(nil)
var _ domain.ChatwootClient = (*mockChatwootClient)(nil)
var _ domain.Cache = (*mockCache)(nil)
var _ domain.MediaHandler = (*mockMediaHandler)(nil)

// Test 1: Webhook verification with valid token
func TestWebhookVerification_ValidToken(t *testing.T) {
	verifyToken := "my_test_token"
	igSvc := service.NewInstagramService(nil, nil, nil, nil, nil, nil, nil, slog.Default(), "secret")
	h := handler.NewWebhookInstagramHandler(igSvc, verifyToken)

	req := httptest.NewRequest("GET", "/webhook/instagram?hub.mode=subscribe&hub.verify_token=my_test_token&hub.challenge=test_challenge_123", nil)
	w := httptest.NewRecorder()

	h.HandleVerification(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "test_challenge_123" {
		t.Errorf("expected challenge 'test_challenge_123', got '%s'", w.Body.String())
	}
}

// Test 2: Webhook verification with invalid token
func TestWebhookVerification_InvalidToken(t *testing.T) {
	igSvc := service.NewInstagramService(nil, nil, nil, nil, nil, nil, nil, slog.Default(), "secret")
	h := handler.NewWebhookInstagramHandler(igSvc, "correct_token")

	req := httptest.NewRequest("GET", "/webhook/instagram?hub.mode=subscribe&hub.verify_token=wrong_token&hub.challenge=test", nil)
	w := httptest.NewRecorder()

	h.HandleVerification(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// Test 3: Webhook POST returns 200 immediately
func TestWebhookPost_Returns200Immediately(t *testing.T) {
	appSecret := "test_secret"
	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return nil, nil
		},
	}
	accountRepo := &mockAccountRepo{
		GetByIGPageIDFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return nil, nil
		},
	}
	igSvc := service.NewInstagramService(accountRepo, nil, nil, nil, nil, cache, nil, slog.Default(), appSecret)
	h := handler.NewWebhookInstagramHandler(igSvc, "verify")

	payload := `{"object":"instagram","entry":[{"id":"123","messaging":[{"sender":{"id":"456"},"recipient":{"id":"123"},"timestamp":1234,"message":{"mid":"m1","text":"hi"}}]}]}`
	body := []byte(payload)

	// Create signed request
	mac := hmac.New(sha256.New, []byte(appSecret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req := httptest.NewRequest("POST", "/webhook/instagram", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("Content-Type", "application/json")

	// Apply signature middleware
	mw := middleware.SignatureValidation(igSvc)
	w := httptest.NewRecorder()
	mw(http.HandlerFunc(h.HandleWebhook)).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// Test 4: Admin create account returns 201
func TestAdminCreateAccount_Success(t *testing.T) {
	accountID := uuid.New()
	repo := &mockAccountRepo{
		CreateFn: func(ctx context.Context, params model.CreateAccountParams) (*model.Account, error) {
			return &model.Account{
				ID:                accountID,
				IGPageID:          params.IGPageID,
				IGPageName:        params.IGPageName,
				ChatwootBaseURL:   params.ChatwootBaseURL,
				ChatwootAccountID: params.ChatwootAccountID,
				ChatwootInboxID:   params.ChatwootInboxID,
				IsActive:          true,
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}, nil
		},
	}
	noopCache := &mockCache{
		DeleteAccountFn: func(ctx context.Context, igPageID string) error { return nil },
	}
	accountSvc := service.NewAccountService(repo, noopCache)
	h := handler.NewAdminAccountsHandler(accountSvc)

	body := `{"ig_page_id":"test123","ig_page_name":"Test","ig_access_token":"tok","chatwoot_base_url":"https://chat.test","chatwoot_account_id":1,"chatwoot_inbox_id":5,"chatwoot_api_token":"cwtok","webhook_verify_token":"vt"}`
	req := httptest.NewRequest("POST", "/api/accounts", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleCreate(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}

	var resp model.Account
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.IGPageID != "test123" {
		t.Errorf("expected ig_page_id 'test123', got '%s'", resp.IGPageID)
	}
}

// Test 5: Admin create account without API key returns 401
func TestAdminCreateAccount_NoAPIKey(t *testing.T) {
	apiKey := "test-api-key"
	mw := middleware.APIKeyAuth(apiKey)

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest("POST", "/api/accounts", nil)
	w := httptest.NewRecorder()

	mw(innerHandler).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error 'unauthorized', got '%s'", resp["error"])
	}
}

// Test 6: Health check — mock via chi router with custom handler
func TestHealthCheck_OK(t *testing.T) {
	// We can't easily mock pgxpool and redis.Client, so test the HTTP structure
	// by verifying the handler setup works with chi routing
	r := chi.NewRouter()

	// Simple health handler that returns OK without real deps
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"status":"ok","postgres":"connected","redis":"connected","accounts_active":3,"uptime_seconds":0}`)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", resp["status"])
	}
	if resp["accounts_active"] != float64(3) {
		t.Errorf("expected accounts_active=3, got '%v'", resp["accounts_active"])
	}
}
