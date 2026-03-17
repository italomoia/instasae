package service_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

func newTestInstagramService(
	accountRepo *mockAccountRepo,
	contactRepo *mockContactRepo,
	convRepo *mockConversationRepo,
	igClient *mockInstagramClient,
	cwClient *mockChatwootClient,
	cache *mockCache,
	media *mockMediaHandler,
	appSecret string,
) *service.InstagramService {
	logger := slog.Default()
	return service.NewInstagramService(
		accountRepo,
		contactRepo,
		convRepo,
		igClient,
		cwClient,
		cache,
		media,
		logger,
		appSecret,
	)
}

func makeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func sampleAccountForIG() *model.Account {
	return &model.Account{
		ID:                uuid.New(),
		IGPageID:          "17841451529395669",
		IGPageName:        "Test Page",
		IGAccessToken:     "test-ig-token",
		ChatwootBaseURL:   "https://chat.example.com",
		ChatwootAccountID: 1,
		ChatwootInboxID:   5,
		ChatwootAPIToken:  "test-cw-token",
		IsActive:          true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

func sampleContactForIG(accountID uuid.UUID) *model.Contact {
	username := "johndoe"
	name := "John Doe"
	pic := "https://pic.example.com/johndoe.jpg"
	return &model.Contact{
		ID:                      uuid.New(),
		AccountID:               accountID,
		IGSenderID:              "1282743370242395",
		ChatwootContactID:       42,
		ChatwootContactSourceID: "abc123-uuid",
		IGUsername:               &username,
		IGName:                  &name,
		IGProfilePic:            &pic,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
	}
}

func sampleConversationForIG(accountID, contactID uuid.UUID) *model.Conversation {
	return &model.Conversation{
		ID:                     uuid.New(),
		AccountID:              accountID,
		ContactID:              contactID,
		ChatwootConversationID: 45,
		LastCustomerMessageAt:  time.Now(),
		IsActive:               true,
		CreatedAt:              time.Now(),
		UpdatedAt:              time.Now(),
	}
}

func textWebhookPayload() model.IGWebhookPayload {
	return model.IGWebhookPayload{
		Object: "instagram",
		Entry: []model.IGEntry{
			{
				Time: 1773581417513,
				ID:   "17841451529395669",
				Messaging: []model.IGMessaging{
					{
						Sender:    model.IGParticipant{ID: "1282743370242395"},
						Recipient: model.IGParticipant{ID: "17841451529395669"},
						Timestamp: 1773581416309,
						Message: &model.IGMessage{
							MID:  "aWdfZAG1faXR_msg1",
							Text: "Hello!",
						},
					},
				},
			},
		},
	}
}

func imageWebhookPayload() model.IGWebhookPayload {
	return model.IGWebhookPayload{
		Object: "instagram",
		Entry: []model.IGEntry{
			{
				Time: 1773581417513,
				ID:   "17841451529395669",
				Messaging: []model.IGMessaging{
					{
						Sender:    model.IGParticipant{ID: "1282743370242395"},
						Recipient: model.IGParticipant{ID: "17841451529395669"},
						Timestamp: 1773581416309,
						Message: &model.IGMessage{
							MID: "aWdfZAG1faXR_img1",
							Attachments: []model.IGAttachment{
								{
									Type: "image",
									Payload: model.IGAttachmentPayload{
										URL: "https://lookaside.fbsbx.com/ig_messaging_cdn/test.jpg",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// Test 1: ValidateSignature — valid signature returns true
func TestInstagramService_ValidateSignature_Valid(t *testing.T) {
	appSecret := "test_app_secret_key"
	svc := newTestInstagramService(nil, nil, nil, nil, nil, nil, nil, appSecret)

	body := []byte(`{"object":"instagram"}`)
	sig := makeSignature(appSecret, body)

	if !svc.ValidateSignature(body, sig) {
		t.Error("expected valid signature to return true")
	}
}

// Test 2: ValidateSignature — invalid signature returns false
func TestInstagramService_ValidateSignature_Invalid(t *testing.T) {
	appSecret := "test_app_secret_key"
	svc := newTestInstagramService(nil, nil, nil, nil, nil, nil, nil, appSecret)

	body := []byte(`{"object":"instagram"}`)

	if svc.ValidateSignature(body, "sha256=invalid") {
		t.Error("expected invalid signature to return false")
	}

	if svc.ValidateSignature(body, "") {
		t.Error("expected empty signature to return false")
	}

	if svc.ValidateSignature(body, "not-sha256-prefix") {
		t.Error("expected missing sha256= prefix to return false")
	}
}

// Test 3: ProcessWebhook — object != "instagram" is ignored
func TestInstagramService_ProcessWebhook_WrongObject(t *testing.T) {
	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			t.Error("SetDedup should not be called for wrong object")
			return false, nil
		},
	}
	svc := newTestInstagramService(nil, nil, nil, nil, nil, cache, nil, "secret")

	payload := model.IGWebhookPayload{
		Object: "page",
		Entry:  []model.IGEntry{},
	}

	svc.ProcessWebhook(context.Background(), payload)
	// No panic, no calls to cache = success
}

// Test 4: ProcessWebhook — is_echo message is ignored
func TestInstagramService_ProcessWebhook_EchoIgnored(t *testing.T) {
	dedupCalled := false
	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			dedupCalled = true
			return false, nil // already exists = duplicate, but shouldn't even get here
		},
	}
	svc := newTestInstagramService(nil, nil, nil, nil, nil, cache, nil, "secret")

	payload := model.IGWebhookPayload{
		Object: "instagram",
		Entry: []model.IGEntry{
			{
				ID: "17841451529395669",
				Messaging: []model.IGMessaging{
					{
						Sender:    model.IGParticipant{ID: "17841451529395669"},
						Recipient: model.IGParticipant{ID: "1282743370242395"},
						Timestamp: 1773581416309,
						Message: &model.IGMessage{
							MID:    "echo_msg_1",
							Text:   "Reply from app",
							IsEcho: true,
						},
					},
				},
			},
		},
	}

	svc.ProcessWebhook(context.Background(), payload)
	if dedupCalled {
		t.Error("dedup should not be called for echo messages")
	}
}

// Test 5: ProcessWebhook — duplicate message is ignored
func TestInstagramService_ProcessWebhook_DuplicateIgnored(t *testing.T) {
	accountRepoCalled := false
	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return false, nil // false = key already existed = duplicate
		},
	}
	accountRepo := &mockAccountRepo{
		GetByIGPageIDFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			accountRepoCalled = true
			return nil, nil
		},
	}
	svc := newTestInstagramService(accountRepo, nil, nil, nil, nil, cache, nil, "secret")

	svc.ProcessWebhook(context.Background(), textWebhookPayload())

	if accountRepoCalled {
		t.Error("account repo should not be called for duplicate messages")
	}
}

// Test 6: ProcessWebhook — unknown account is skipped
func TestInstagramService_ProcessWebhook_UnknownAccount(t *testing.T) {
	contactRepoCalled := false
	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil // new message
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return nil, nil // not in cache
		},
		SetAccountFn: func(ctx context.Context, igPageID string, account *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	accountRepo := &mockAccountRepo{
		GetByIGPageIDFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return nil, nil // not found
		},
	}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			contactRepoCalled = true
			return nil, nil
		},
	}
	svc := newTestInstagramService(accountRepo, contactRepo, nil, nil, nil, cache, nil, "secret")

	svc.ProcessWebhook(context.Background(), textWebhookPayload())

	if contactRepoCalled {
		t.Error("contact repo should not be called for unknown accounts")
	}
}

// Test 7: Full flow — new contact + new conversation created
func TestInstagramService_ProcessWebhook_NewContactNewConversation(t *testing.T) {
	account := sampleAccountForIG()

	var createdContactInRepo bool
	var createdConvInRepo bool
	var cwContactCreated bool
	var cwConvCreated bool
	var cwMessageCreated bool
	var lastCustomerMsgUpdated bool

	newContact := sampleContactForIG(account.ID)
	newConv := sampleConversationForIG(account.ID, newContact.ID)

	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return account, nil // found in cache
		},
		SetAccountFn: func(ctx context.Context, igPageID string, acc *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	accountRepo := &mockAccountRepo{}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			return nil, nil // not found — new contact
		},
		CreateFn: func(ctx context.Context, contact *model.Contact) (*model.Contact, error) {
			createdContactInRepo = true
			return newContact, nil
		},
	}
	convRepo := &mockConversationRepo{
		GetActiveByContactFn: func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
			return nil, nil // not found — new conversation
		},
		CreateFn: func(ctx context.Context, conv *model.Conversation) (*model.Conversation, error) {
			createdConvInRepo = true
			return newConv, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			lastCustomerMsgUpdated = true
			return nil
		},
	}
	igClient := &mockInstagramClient{
		GetUserProfileFn: func(ctx context.Context, token string, userID string) (*model.IGUserProfile, error) {
			return &model.IGUserProfile{
				Name:       "John Doe",
				Username:   "johndoe",
				ProfilePic: "https://pic.example.com/johndoe.jpg",
				ID:         "1282743370242395",
			}, nil
		},
	}
	cwClient := &mockChatwootClient{
		CreateContactFn: func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error) {
			cwContactCreated = true
			return &model.CWCreateContactResponse{
				Payload: model.CWContactPayload{
					Contact: model.CWContact{
						ID: 42,
						ContactInboxes: []model.CWContactInbox{
							{SourceID: "abc123-uuid", Inbox: model.CWInbox{ID: 5}},
						},
					},
				},
			}, nil
		},
		CreateConversationFn: func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error) {
			cwConvCreated = true
			return &model.CWCreateConversationResponse{ID: 45}, nil
		},
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			cwMessageCreated = true
			if req.Content != "Hello!" {
				t.Errorf("expected message content 'Hello!', got '%s'", req.Content)
			}
			if req.MessageType != "incoming" {
				t.Errorf("expected message type 'incoming', got '%s'", req.MessageType)
			}
			return nil
		},
	}

	svc := newTestInstagramService(accountRepo, contactRepo, convRepo, igClient, cwClient, cache, nil, "secret")
	svc.ProcessWebhook(context.Background(), textWebhookPayload())

	// Give goroutine time to finish (ProcessWebhook processes async)
	time.Sleep(100 * time.Millisecond)

	if !createdContactInRepo {
		t.Error("expected contact to be created in repo")
	}
	if !createdConvInRepo {
		t.Error("expected conversation to be created in repo")
	}
	if !cwContactCreated {
		t.Error("expected contact to be created in Chatwoot")
	}
	if !cwConvCreated {
		t.Error("expected conversation to be created in Chatwoot")
	}
	if !cwMessageCreated {
		t.Error("expected message to be created in Chatwoot")
	}
	if !lastCustomerMsgUpdated {
		t.Error("expected last_customer_message_at to be updated")
	}
}

// Test 8: Existing contact + existing conversation reused
func TestInstagramService_ProcessWebhook_ExistingContactExistingConversation(t *testing.T) {
	account := sampleAccountForIG()
	contact := sampleContactForIG(account.ID)
	conv := sampleConversationForIG(account.ID, contact.ID)

	var contactCreateCalled bool
	var convCreateCalled bool
	var cwMessageCreated bool
	var lastCustomerMsgUpdated bool

	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return account, nil
		},
		SetAccountFn: func(ctx context.Context, igPageID string, acc *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			return contact, nil // existing
		},
		CreateFn: func(ctx context.Context, c *model.Contact) (*model.Contact, error) {
			contactCreateCalled = true
			return nil, nil
		},
	}
	convRepo := &mockConversationRepo{
		GetActiveByContactFn: func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
			return conv, nil // existing
		},
		CreateFn: func(ctx context.Context, c *model.Conversation) (*model.Conversation, error) {
			convCreateCalled = true
			return nil, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			lastCustomerMsgUpdated = true
			return nil
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			cwMessageCreated = true
			if conversationID != 45 {
				t.Errorf("expected conversation ID 45, got %d", conversationID)
			}
			return nil
		},
	}

	svc := newTestInstagramService(&mockAccountRepo{}, contactRepo, convRepo, nil, cwClient, cache, nil, "secret")
	svc.ProcessWebhook(context.Background(), textWebhookPayload())
	time.Sleep(100 * time.Millisecond)

	if contactCreateCalled {
		t.Error("should not create contact when existing")
	}
	if convCreateCalled {
		t.Error("should not create conversation when existing")
	}
	if !cwMessageCreated {
		t.Error("expected message to be sent to Chatwoot")
	}
	if !lastCustomerMsgUpdated {
		t.Error("expected last_customer_message_at to be updated")
	}
}

// Test 9: Text message sent to Chatwoot with correct content
func TestInstagramService_ProcessWebhook_TextMessage(t *testing.T) {
	account := sampleAccountForIG()
	contact := sampleContactForIG(account.ID)
	conv := sampleConversationForIG(account.ID, contact.ID)

	var receivedContent string
	var receivedMsgType string

	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return account, nil
		},
		SetAccountFn: func(ctx context.Context, igPageID string, acc *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			return contact, nil
		},
	}
	convRepo := &mockConversationRepo{
		GetActiveByContactFn: func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
			return conv, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			return nil
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			receivedContent = req.Content
			receivedMsgType = req.MessageType
			return nil
		},
	}

	svc := newTestInstagramService(&mockAccountRepo{}, contactRepo, convRepo, nil, cwClient, cache, nil, "secret")
	svc.ProcessWebhook(context.Background(), textWebhookPayload())
	time.Sleep(100 * time.Millisecond)

	if receivedContent != "Hello!" {
		t.Errorf("expected content 'Hello!', got '%s'", receivedContent)
	}
	if receivedMsgType != "incoming" {
		t.Errorf("expected message_type 'incoming', got '%s'", receivedMsgType)
	}
}

// Test 10: Image attachment — downloaded, uploaded to B2, sent to Chatwoot as multipart attachment
func TestInstagramService_ProcessWebhook_ImageAttachment(t *testing.T) {
	account := sampleAccountForIG()
	contact := sampleContactForIG(account.ID)
	conv := sampleConversationForIG(account.ID, contact.ID)

	var mediaDownloaded bool
	var attachmentSent bool
	var sentAttachmentURL string
	var sentFilename string

	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return account, nil
		},
		SetAccountFn: func(ctx context.Context, igPageID string, acc *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			return contact, nil
		},
	}
	convRepo := &mockConversationRepo{
		GetActiveByContactFn: func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
			return conv, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			return nil
		},
	}
	mediaHandler := &mockMediaHandler{
		DownloadAndUploadFn: func(ctx context.Context, sourceURL string, accountID string, attachmentType string) (string, error) {
			mediaDownloaded = true
			if sourceURL != "https://lookaside.fbsbx.com/ig_messaging_cdn/test.jpg" {
				t.Errorf("unexpected source URL: %s", sourceURL)
			}
			return "https://b2.example.com/instasae/test/image.jpg", nil
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageWithAttachmentFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, content string, attachmentURL string, filename string) error {
			attachmentSent = true
			sentAttachmentURL = attachmentURL
			sentFilename = filename
			return nil
		},
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			t.Error("CreateMessage should not be called for attachment-only message")
			return nil
		},
	}

	svc := newTestInstagramService(&mockAccountRepo{}, contactRepo, convRepo, nil, cwClient, cache, mediaHandler, "secret")
	svc.ProcessWebhook(context.Background(), imageWebhookPayload())
	time.Sleep(100 * time.Millisecond)

	if !mediaDownloaded {
		t.Error("expected media to be downloaded and uploaded")
	}
	if !attachmentSent {
		t.Error("expected attachment to be sent via CreateMessageWithAttachment")
	}
	if sentAttachmentURL != "https://b2.example.com/instasae/test/image.jpg" {
		t.Errorf("expected B2 URL as attachment URL, got '%s'", sentAttachmentURL)
	}
	if sentFilename != "attachment.jpg" {
		t.Errorf("expected filename 'attachment.jpg', got '%s'", sentFilename)
	}
}

// Test 11: Profile fetch failure is non-blocking
func TestInstagramService_ProcessWebhook_ProfileFetchFailure(t *testing.T) {
	account := sampleAccountForIG()

	var createdContactName string
	var cwMessageCreated bool

	newContact := sampleContactForIG(account.ID)
	newContact.IGUsername = nil
	newContact.IGName = nil
	newContact.IGProfilePic = nil
	newConv := sampleConversationForIG(account.ID, newContact.ID)

	cache := &mockCache{
		SetDedupFn: func(ctx context.Context, messageID string, ttl time.Duration) (bool, error) {
			return true, nil
		},
		GetAccountFn: func(ctx context.Context, igPageID string) (*model.Account, error) {
			return account, nil
		},
		SetAccountFn: func(ctx context.Context, igPageID string, acc *model.Account, ttl time.Duration) error {
			return nil
		},
	}
	contactRepo := &mockContactRepo{
		GetByAccountAndSenderFn: func(ctx context.Context, accountID uuid.UUID, igSenderID string) (*model.Contact, error) {
			return nil, nil // not found
		},
		CreateFn: func(ctx context.Context, contact *model.Contact) (*model.Contact, error) {
			return newContact, nil
		},
	}
	convRepo := &mockConversationRepo{
		GetActiveByContactFn: func(ctx context.Context, accountID uuid.UUID, contactID uuid.UUID) (*model.Conversation, error) {
			return nil, nil
		},
		CreateFn: func(ctx context.Context, conv *model.Conversation) (*model.Conversation, error) {
			return newConv, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			return nil
		},
	}
	igClient := &mockInstagramClient{
		GetUserProfileFn: func(ctx context.Context, token string, userID string) (*model.IGUserProfile, error) {
			return nil, fmt.Errorf("rate limited")
		},
	}
	cwClient := &mockChatwootClient{
		CreateContactFn: func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error) {
			createdContactName = req.Name
			return &model.CWCreateContactResponse{
				Payload: model.CWContactPayload{
					Contact: model.CWContact{
						ID: 42,
						ContactInboxes: []model.CWContactInbox{
							{SourceID: "abc123-uuid", Inbox: model.CWInbox{ID: 5}},
						},
					},
				},
			}, nil
		},
		CreateConversationFn: func(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error) {
			return &model.CWCreateConversationResponse{ID: 45}, nil
		},
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			cwMessageCreated = true
			return nil
		},
	}

	svc := newTestInstagramService(&mockAccountRepo{}, contactRepo, convRepo, igClient, cwClient, cache, nil, "secret")
	svc.ProcessWebhook(context.Background(), textWebhookPayload())
	time.Sleep(100 * time.Millisecond)

	if !cwMessageCreated {
		t.Error("message should still be sent despite profile fetch failure")
	}
	expectedName := "IG User 1282743370242395"
	if createdContactName != expectedName {
		t.Errorf("expected placeholder name '%s', got '%s'", expectedName, createdContactName)
	}
}
