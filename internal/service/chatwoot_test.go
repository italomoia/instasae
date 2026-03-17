package service_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

func newTestChatwootService(
	accountRepo *mockAccountRepo,
	contactRepo *mockContactRepo,
	convRepo *mockConversationRepo,
	igClient *mockInstagramClient,
	cwClient *mockChatwootClient,
	cache *mockCache,
) *service.ChatwootService {
	logger := slog.Default()
	return service.NewChatwootService(
		accountRepo,
		contactRepo,
		convRepo,
		igClient,
		cwClient,
		cache,
		logger,
	)
}

func sampleCWPayloadText(convID, inboxID int) model.CWWebhookPayload {
	return model.CWWebhookPayload{
		Event:       "message_created",
		ID:          123,
		Content:     "Hi there, how can I help?",
		MessageType: "outgoing",
		Private:     false,
		ContentType: "text",
		Conversation: model.CWConversation{
			ID:      convID,
			InboxID: inboxID,
		},
		Sender: model.CWSender{
			ID:   1,
			Name: "Agent Name",
			Type: "user",
		},
		Inbox: model.CWInbox{
			ID:   inboxID,
			Name: "Instagram DM",
		},
		Account: model.CWAccount{
			ID:   1,
			Name: "My Agency",
		},
	}
}

func sampleCWPayloadAttachment(convID, inboxID int) model.CWWebhookPayload {
	p := sampleCWPayloadText(convID, inboxID)
	p.Content = ""
	p.Attachments = []model.CWAttachment{
		{
			DataURL:  "https://chat.example.com/rails/active_storage/blobs/image.jpg",
			FileType: "image",
		},
	}
	return p
}

func sampleCWPayloadComposite(convID, inboxID int) model.CWWebhookPayload {
	p := sampleCWPayloadText(convID, inboxID)
	p.Attachments = []model.CWAttachment{
		{
			DataURL:  "https://chat.example.com/rails/active_storage/blobs/image.jpg",
			FileType: "image",
		},
	}
	return p
}

type cwTestFixtures struct {
	account *model.Account
	contact *model.Contact
	conv    *model.Conversation
}

func setupCWFixtures() cwTestFixtures {
	account := sampleAccountForIG()
	contact := sampleContactForIG(account.ID)
	conv := sampleConversationForIG(account.ID, contact.ID)
	conv.LastCustomerMessageAt = time.Now().Add(-1 * time.Hour) // 1 hour ago, within window
	return cwTestFixtures{account: account, contact: contact, conv: conv}
}

func cwDefaultConvRepo(f cwTestFixtures) *mockConversationRepo {
	return &mockConversationRepo{
		GetByChatwootIDFn: func(ctx context.Context, chatwootConvID int) (*model.Conversation, error) {
			if chatwootConvID == f.conv.ChatwootConversationID {
				return f.conv, nil
			}
			return nil, nil
		},
		UpdateLastCustomerMessageFn: func(ctx context.Context, id uuid.UUID, at time.Time) error {
			return nil
		},
	}
}

func cwDefaultAccountRepo(f cwTestFixtures) *mockAccountRepo {
	return &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			if id == f.account.ID {
				return f.account, nil
			}
			return nil, nil
		},
	}
}

func cwDefaultContactRepo(f cwTestFixtures) *mockContactRepo {
	return &mockContactRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Contact, error) {
			if id == f.contact.ID {
				return f.contact, nil
			}
			return nil, nil
		},
	}
}

// Test 1: Text message — IGClient.SendTextMessage called with correct args
func TestProcessCallback_TextMessage(t *testing.T) {
	f := setupCWFixtures()
	var sentText string
	var sentRecipient string

	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			sentText = text
			sentRecipient = recipientID
			return &model.IGSendMessageResponse{RecipientID: recipientID, MessageID: "m_123"}, nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, nil, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadText(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentText != "Hi there, how can I help?" {
		t.Errorf("expected text 'Hi there, how can I help?', got '%s'", sentText)
	}
	if sentRecipient != f.contact.IGSenderID {
		t.Errorf("expected recipient '%s', got '%s'", f.contact.IGSenderID, sentRecipient)
	}
}

// Test 2: Attachment message — IGClient.SendAttachment called
func TestProcessCallback_AttachmentMessage(t *testing.T) {
	f := setupCWFixtures()
	var sentURL string
	var sentType string

	igClient := &mockInstagramClient{
		SendAttachmentFn: func(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error) {
			sentURL = url
			sentType = attachmentType
			return &model.IGSendMessageResponse{RecipientID: recipientID, MessageID: "m_124"}, nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, nil, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadAttachment(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sentURL != "https://chat.example.com/rails/active_storage/blobs/image.jpg" {
		t.Errorf("unexpected URL: %s", sentURL)
	}
	if sentType != "image" {
		t.Errorf("expected type 'image', got '%s'", sentType)
	}
}

// Test 3: Composite message — attachment first, text second (BR-OUTBOUND-02)
func TestProcessCallback_CompositeMessage(t *testing.T) {
	f := setupCWFixtures()
	var callOrder []string

	igClient := &mockInstagramClient{
		SendAttachmentFn: func(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error) {
			callOrder = append(callOrder, "attachment")
			return &model.IGSendMessageResponse{RecipientID: recipientID, MessageID: "m_125"}, nil
		},
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			callOrder = append(callOrder, "text")
			return &model.IGSendMessageResponse{RecipientID: recipientID, MessageID: "m_126"}, nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, nil, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadComposite(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(callOrder) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(callOrder))
	}
	if callOrder[0] != "attachment" {
		t.Errorf("expected attachment first, got '%s'", callOrder[0])
	}
	if callOrder[1] != "text" {
		t.Errorf("expected text second, got '%s'", callOrder[1])
	}
}

// Test 4: Incoming message ignored (BR-OUTBOUND-01)
func TestProcessCallback_IncomingIgnored(t *testing.T) {
	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			t.Error("SendTextMessage should not be called for incoming messages")
			return nil, nil
		},
	}

	svc := newTestChatwootService(nil, nil, nil, igClient, nil, nil)
	payload := sampleCWPayloadText(45, 5)
	payload.MessageType = "incoming"

	err := svc.ProcessCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test 5: Private message ignored (BR-OUTBOUND-01)
func TestProcessCallback_PrivateIgnored(t *testing.T) {
	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			t.Error("SendTextMessage should not be called for private messages")
			return nil, nil
		},
	}

	svc := newTestChatwootService(nil, nil, nil, igClient, nil, nil)
	payload := sampleCWPayloadText(45, 5)
	payload.Private = true

	err := svc.ProcessCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test 6: Non message_created event ignored (BR-OUTBOUND-01)
func TestProcessCallback_NonMessageEventIgnored(t *testing.T) {
	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			t.Error("SendTextMessage should not be called for non-message events")
			return nil, nil
		},
	}

	svc := newTestChatwootService(nil, nil, nil, igClient, nil, nil)
	payload := sampleCWPayloadText(45, 5)
	payload.Event = "conversation_updated"

	err := svc.ProcessCallback(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test 7: Window expired — private note posted, no IG send (BR-OUTBOUND-04)
func TestProcessCallback_WindowExpired(t *testing.T) {
	f := setupCWFixtures()
	f.conv.LastCustomerMessageAt = time.Now().Add(-8 * 24 * time.Hour) // 8 days ago

	var privateNotePosted bool
	var noteContent string
	igSendCalled := false

	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			igSendCalled = true
			return nil, nil
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			if req.Private {
				privateNotePosted = true
				noteContent = req.Content
			}
			return nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, cwClient, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadText(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if igSendCalled {
		t.Error("IG send should not be called when window expired")
	}
	if !privateNotePosted {
		t.Error("expected private note to be posted for expired window")
	}
	if noteContent == "" {
		t.Error("expected non-empty private note content")
	}
}

// Test 8: Send failure — private note posted (BR-OUTBOUND-05)
func TestProcessCallback_SendFailure_PrivateNote(t *testing.T) {
	f := setupCWFixtures()

	var privateNotePosted bool
	var noteContent string

	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			return nil, fmt.Errorf("token expired (code: 190)")
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			if req.Private {
				privateNotePosted = true
				noteContent = req.Content
			}
			return nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, cwClient, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadText(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !privateNotePosted {
		t.Error("expected private note on send failure")
	}
	if noteContent == "" {
		t.Error("expected non-empty private note content")
	}
}

// Test 9: Conversation not found — logged and skipped
func TestProcessCallback_ConversationNotFound(t *testing.T) {
	convRepo := &mockConversationRepo{
		GetByChatwootIDFn: func(ctx context.Context, chatwootConvID int) (*model.Conversation, error) {
			return nil, nil // not found
		},
	}
	igClient := &mockInstagramClient{
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			t.Error("SendTextMessage should not be called for unknown conversation")
			return nil, nil
		},
	}

	svc := newTestChatwootService(nil, nil, convRepo, igClient, nil, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadText(999, 5))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Test 10: Partial failure — attachment succeeds, text fails → private note (BR-OUTBOUND-06)
func TestProcessCallback_PartialFailure(t *testing.T) {
	f := setupCWFixtures()

	var privateNotePosted bool
	var noteContent string

	igClient := &mockInstagramClient{
		SendAttachmentFn: func(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error) {
			return &model.IGSendMessageResponse{RecipientID: recipientID, MessageID: "m_127"}, nil // success
		},
		SendTextMessageFn: func(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
			return nil, fmt.Errorf("rate limited (code: 429)") // failure
		},
	}
	cwClient := &mockChatwootClient{
		CreateMessageFn: func(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
			if req.Private {
				privateNotePosted = true
				noteContent = req.Content
			}
			return nil
		},
	}

	svc := newTestChatwootService(cwDefaultAccountRepo(f), cwDefaultContactRepo(f), cwDefaultConvRepo(f), igClient, cwClient, nil)
	err := svc.ProcessCallback(context.Background(), sampleCWPayloadComposite(f.conv.ChatwootConversationID, f.account.ChatwootInboxID))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !privateNotePosted {
		t.Error("expected private note on partial failure")
	}
	if noteContent == "" {
		t.Error("expected non-empty private note content for partial failure")
	}
}
