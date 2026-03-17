package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

const dedupTTL = 5 * time.Minute

type InstagramService struct {
	accountRepo  domain.AccountRepository
	contactRepo  domain.ContactRepository
	convRepo     domain.ConversationRepository
	igClient     domain.InstagramClient
	cwClient     domain.ChatwootClient
	cache        domain.Cache
	media        domain.MediaHandler
	logger       *slog.Logger
	appSecret    string
}

func NewInstagramService(
	accountRepo domain.AccountRepository,
	contactRepo domain.ContactRepository,
	convRepo domain.ConversationRepository,
	igClient domain.InstagramClient,
	cwClient domain.ChatwootClient,
	cache domain.Cache,
	media domain.MediaHandler,
	logger *slog.Logger,
	appSecret string,
) *InstagramService {
	return &InstagramService{
		accountRepo: accountRepo,
		contactRepo: contactRepo,
		convRepo:    convRepo,
		igClient:    igClient,
		cwClient:    cwClient,
		cache:       cache,
		media:       media,
		logger:      logger,
		appSecret:   appSecret,
	}
}

// ValidateSignature validates the X-Hub-Signature-256 header against the request body.
func (s *InstagramService) ValidateSignature(body []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	expectedSig := signature[7:] // strip "sha256="

	mac := hmac.New(sha256.New, []byte(s.appSecret))
	mac.Write(body)
	computedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expectedSig), []byte(computedSig))
}

// ProcessWebhook processes an Instagram webhook payload.
// BR-WEBHOOK-06: Only "instagram" object is processed.
func (s *InstagramService) ProcessWebhook(ctx context.Context, payload model.IGWebhookPayload) {
	s.logger.Debug("processing webhook", "object", payload.Object, "entries", len(payload.Entry))

	if payload.Object != "instagram" {
		s.logger.Warn("ignoring non-instagram webhook object", "object", payload.Object)
		return
	}

	for _, entry := range payload.Entry {
		for _, messaging := range entry.Messaging {
			s.processMessage(ctx, entry.ID, messaging)
		}
	}
}

func (s *InstagramService) processMessage(ctx context.Context, igPageID string, messaging model.IGMessaging) {
	s.logger.Debug("processing message", "ig_page_id", igPageID, "sender", messaging.Sender.ID, "has_message", messaging.Message != nil)

	msg := messaging.Message
	if msg == nil {
		return
	}

	// BR-WEBHOOK-03: Echo messages are always ignored
	if msg.IsEcho {
		s.logger.Debug("ignoring echo message", "mid", msg.MID)
		return
	}

	// BR-WEBHOOK-04: Webhook deduplication via message ID
	isNew, err := s.cache.SetDedup(ctx, msg.MID, dedupTTL)
	if err != nil {
		s.logger.Error("dedup check failed", "mid", msg.MID, "error", err)
		return
	}
	if !isNew {
		s.logger.Debug("ignoring duplicate message", "mid", msg.MID)
		return
	}

	// BR-WEBHOOK-05: Unknown accounts are silently skipped
	account, err := s.findOrCacheAccount(ctx, igPageID)
	if err != nil {
		s.logger.Error("account lookup failed", "ig_page_id", igPageID, "error", err)
		return
	}
	if account == nil {
		s.logger.Warn("unknown account, skipping", "ig_page_id", igPageID)
		return
	}

	senderID := messaging.Sender.ID

	// Find or create contact
	contact, err := s.findOrCreateContact(ctx, account, senderID)
	if err != nil {
		s.logger.Error("contact lookup/creation failed", "sender_id", senderID, "error", err)
		return
	}

	// Find or create conversation
	conv, err := s.findOrCreateConversation(ctx, account, contact)
	if err != nil {
		s.logger.Error("conversation lookup/creation failed", "contact_id", contact.ID, "error", err)
		return
	}

	// BR-INBOUND-04: Update last_customer_message_at
	msgTime := time.Unix(messaging.Timestamp/1000, (messaging.Timestamp%1000)*int64(time.Millisecond))
	if err := s.convRepo.UpdateLastCustomerMessage(ctx, conv.ID, msgTime); err != nil {
		s.logger.Error("failed to update last_customer_message_at", "conv_id", conv.ID, "error", err)
	}

	// Send to Chatwoot
	s.sendToChatwoot(ctx, account, conv, msg)
}

func (s *InstagramService) sendToChatwoot(ctx context.Context, account *model.Account, conv *model.Conversation, msg *model.IGMessage) {
	baseURL := account.ChatwootBaseURL
	acctID := account.ChatwootAccountID
	token := account.ChatwootAPIToken
	convID := conv.ChatwootConversationID

	// Handle attachments
	if len(msg.Attachments) > 0 {
		att := msg.Attachments[0]
		publicURL, err := s.media.DownloadAndUpload(ctx, att.Payload.URL, account.ID.String(), att.Type)
		if err != nil {
			s.logger.Error("media download/upload failed", "url", att.Payload.URL, "error", err)
			// Fall back to text-only below
		} else {
			// Derive filename from the B2 URL extension
			ext := path.Ext(publicURL) // e.g. ".jpg"
			if ext == "" {
				ext = ".bin"
			}
			filename := "attachment" + ext

			// Send attachment via multipart
			if err := s.cwClient.CreateMessageWithAttachment(ctx, baseURL, acctID, token, convID, "", publicURL, filename); err != nil {
				s.logger.Error("failed to send attachment to Chatwoot, falling back to URL", "error", err)
				// Fallback: send URL as text
				cwReq := model.CWCreateMessageRequest{Content: publicURL, MessageType: "incoming"}
				if msgErr := s.cwClient.CreateMessage(ctx, baseURL, acctID, token, convID, cwReq); msgErr != nil {
					s.logger.Error("failed to send fallback URL to Chatwoot", "error", msgErr)
				}
			}

			// If there's also text, send it as a separate message
			if msg.Text != "" {
				cwReq := model.CWCreateMessageRequest{Content: msg.Text, MessageType: "incoming"}
				if err := s.cwClient.CreateMessage(ctx, baseURL, acctID, token, convID, cwReq); err != nil {
					s.logger.Error("failed to send text to Chatwoot", "error", err)
				}
			}
			return
		}
	}

	// Text-only message (or attachment download failed)
	if msg.Text != "" {
		cwReq := model.CWCreateMessageRequest{Content: msg.Text, MessageType: "incoming"}
		if err := s.cwClient.CreateMessage(ctx, baseURL, acctID, token, convID, cwReq); err != nil {
			s.logger.Error("failed to send message to Chatwoot", "conv_id", convID, "error", err)
		}
	}
}

func (s *InstagramService) findOrCacheAccount(ctx context.Context, igPageID string) (*model.Account, error) {
	// Check cache first
	account, err := s.cache.GetAccount(ctx, igPageID)
	if err != nil {
		s.logger.Warn("cache get failed, falling back to DB", "ig_page_id", igPageID, "error", err)
	}
	if account != nil {
		return account, nil
	}

	// Fall back to database
	account, err = s.accountRepo.GetByIGPageID(ctx, igPageID)
	if err != nil {
		return nil, fmt.Errorf("get account by ig_page_id: %w", err)
	}
	if account == nil {
		return nil, nil
	}

	// Cache for next time
	if err := s.cache.SetAccount(ctx, igPageID, account, 5*time.Minute); err != nil {
		s.logger.Warn("cache set failed", "ig_page_id", igPageID, "error", err)
	}

	return account, nil
}

func (s *InstagramService) findOrCreateContact(ctx context.Context, account *model.Account, senderID string) (*model.Contact, error) {
	// Check if contact already exists
	contact, err := s.contactRepo.GetByAccountAndSender(ctx, account.ID, senderID)
	if err != nil {
		return nil, fmt.Errorf("get contact: %w", err)
	}
	if contact != nil {
		return contact, nil
	}

	// BR-INBOUND-01: Auto-create contact on first message
	// Fetch profile (BR-INBOUND-05: failure is non-blocking)
	var name, username, avatarURL string
	profile, err := s.igClient.GetUserProfile(ctx, account.IGAccessToken, senderID)
	if err != nil {
		s.logger.Warn("profile fetch failed, using placeholder", "sender_id", senderID, "error", err)
		name = fmt.Sprintf("IG User %s", senderID)
	} else {
		name = profile.Name
		username = profile.Username
		avatarURL = profile.ProfilePic
	}

	// Create contact in Chatwoot
	cwReq := model.CWCreateContactRequest{
		InboxID:    account.ChatwootInboxID,
		Name:       name,
		Identifier: fmt.Sprintf("ig_%s", senderID),
		AvatarURL:  avatarURL,
		CustomAttributes: map[string]string{
			"instagram_username": username,
			"instagram_id":      senderID,
		},
	}
	cwResp, err := s.cwClient.CreateContact(ctx, account.ChatwootBaseURL, account.ChatwootAccountID, account.ChatwootAPIToken, cwReq)
	if err != nil {
		return nil, fmt.Errorf("create chatwoot contact: %w", err)
	}

	// Find the source_id for the correct inbox
	var sourceID string
	for _, ci := range cwResp.Payload.Contact.ContactInboxes {
		if ci.Inbox.ID == account.ChatwootInboxID {
			sourceID = ci.SourceID
			break
		}
	}
	if sourceID == "" && len(cwResp.Payload.Contact.ContactInboxes) > 0 {
		sourceID = cwResp.Payload.Contact.ContactInboxes[0].SourceID
	}

	// Store in local DB
	newContact := &model.Contact{
		AccountID:               account.ID,
		IGSenderID:              senderID,
		ChatwootContactID:       cwResp.Payload.Contact.ID,
		ChatwootContactSourceID: sourceID,
	}
	if profile != nil {
		newContact.IGUsername = strPtr(profile.Username)
		newContact.IGName = strPtr(profile.Name)
		newContact.IGProfilePic = strPtr(profile.ProfilePic)
	}

	created, err := s.contactRepo.Create(ctx, newContact)
	if err != nil {
		return nil, fmt.Errorf("create contact in DB: %w", err)
	}

	return created, nil
}

func (s *InstagramService) findOrCreateConversation(ctx context.Context, account *model.Account, contact *model.Contact) (*model.Conversation, error) {
	// BR-INBOUND-03: Reuse existing active conversation
	conv, err := s.convRepo.GetActiveByContact(ctx, account.ID, contact.ID)
	if err != nil {
		return nil, fmt.Errorf("get active conversation: %w", err)
	}
	if conv != nil {
		return conv, nil
	}

	// BR-INBOUND-02: Auto-create conversation on first message
	cwReq := model.CWCreateConversationRequest{
		SourceID:  contact.ChatwootContactSourceID,
		InboxID:   account.ChatwootInboxID,
		ContactID: contact.ChatwootContactID,
		Status:    "open",
	}
	cwResp, err := s.cwClient.CreateConversation(ctx, account.ChatwootBaseURL, account.ChatwootAccountID, account.ChatwootAPIToken, cwReq)
	if err != nil {
		return nil, fmt.Errorf("create chatwoot conversation: %w", err)
	}

	newConv := &model.Conversation{
		AccountID:              account.ID,
		ContactID:              contact.ID,
		ChatwootConversationID: cwResp.ID,
		LastCustomerMessageAt:  time.Now(),
		IsActive:               true,
	}
	created, err := s.convRepo.Create(ctx, newConv)
	if err != nil {
		return nil, fmt.Errorf("create conversation in DB: %w", err)
	}

	return created, nil
}

func strPtr(s string) *string {
	return &s
}
