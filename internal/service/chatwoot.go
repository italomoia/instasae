package service

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

const messagingWindow = 7 * 24 * time.Hour

type ChatwootService struct {
	accountRepo domain.AccountRepository
	contactRepo domain.ContactRepository
	convRepo    domain.ConversationRepository
	igClient    domain.InstagramClient
	cwClient    domain.ChatwootClient
	cache       domain.Cache
	logger      *slog.Logger
}

func NewChatwootService(
	accountRepo domain.AccountRepository,
	contactRepo domain.ContactRepository,
	convRepo domain.ConversationRepository,
	igClient domain.InstagramClient,
	cwClient domain.ChatwootClient,
	cache domain.Cache,
	logger *slog.Logger,
) *ChatwootService {
	return &ChatwootService{
		accountRepo: accountRepo,
		contactRepo: contactRepo,
		convRepo:    convRepo,
		igClient:    igClient,
		cwClient:    cwClient,
		cache:       cache,
		logger:      logger,
	}
}

// ProcessCallback processes a Chatwoot webhook callback.
func (s *ChatwootService) ProcessCallback(ctx context.Context, payload model.CWWebhookPayload) error {
	// BR-OUTBOUND-01: Only process outgoing non-private message_created events
	if payload.Event != "message_created" {
		return nil
	}
	if payload.MessageType != "outgoing" && payload.MessageType != "1" {
		return nil
	}
	if payload.Private {
		return nil
	}

	// Find conversation by Chatwoot conversation ID
	conv, err := s.convRepo.GetByChatwootID(ctx, payload.Conversation.ID)
	if err != nil {
		return fmt.Errorf("get conversation by chatwoot id: %w", err)
	}
	if conv == nil {
		s.logger.Warn("conversation not found for chatwoot callback", "chatwoot_conversation_id", payload.Conversation.ID)
		return nil
	}

	// Load account
	account, err := s.accountRepo.GetByID(ctx, conv.AccountID)
	if err != nil {
		return fmt.Errorf("get account: %w", err)
	}
	if account == nil {
		s.logger.Warn("account not found for conversation", "account_id", conv.AccountID)
		return nil
	}

	// Load contact
	contact, err := s.contactRepo.GetByID(ctx, conv.ContactID)
	if err != nil {
		return fmt.Errorf("get contact: %w", err)
	}
	if contact == nil {
		s.logger.Warn("contact not found for conversation", "contact_id", conv.ContactID)
		return nil
	}

	// BR-OUTBOUND-04: Check messaging window
	elapsed := time.Since(conv.LastCustomerMessageAt)
	if elapsed > messagingWindow {
		daysAgo := int(math.Round(elapsed.Hours() / 24))
		note := fmt.Sprintf("⚠️ Message not sent: messaging window expired (last customer message was %d days ago).", daysAgo)
		s.postPrivateNote(ctx, account, conv.ChatwootConversationID, note)
		return nil
	}

	// Send to Instagram
	s.sendToInstagram(ctx, account, contact, conv, payload)

	return nil
}

func (s *ChatwootService) sendToInstagram(ctx context.Context, account *model.Account, contact *model.Contact, conv *model.Conversation, payload model.CWWebhookPayload) {
	hasText := payload.Content != ""
	hasAttachment := len(payload.Attachments) > 0

	// BR-OUTBOUND-02: Composite messages are split — attachment first, text second
	if hasAttachment {
		att := payload.Attachments[0]
		_, err := s.igClient.SendAttachment(ctx, account.IGPageID, account.IGAccessToken, contact.IGSenderID, att.FileType, att.DataURL)
		if err != nil {
			if hasText {
				// BR-OUTBOUND-06: Will still try text, but note attachment failure? No — if attachment fails on composite, we note it.
				// Actually BR-OUTBOUND-05 says post private note on failure.
				note := fmt.Sprintf("⚠️ Message not delivered to Instagram: %s", err.Error())
				s.postPrivateNote(ctx, account, conv.ChatwootConversationID, note)
				return
			}
			note := fmt.Sprintf("⚠️ Message not delivered to Instagram: %s", err.Error())
			s.postPrivateNote(ctx, account, conv.ChatwootConversationID, note)
			return
		}
	}

	if hasText {
		_, err := s.igClient.SendTextMessage(ctx, account.IGPageID, account.IGAccessToken, contact.IGSenderID, payload.Content)
		if err != nil {
			if hasAttachment {
				// BR-OUTBOUND-06: Attachment succeeded but text failed
				note := fmt.Sprintf("⚠️ Attachment sent but text failed: %s", err.Error())
				s.postPrivateNote(ctx, account, conv.ChatwootConversationID, note)
				return
			}
			note := fmt.Sprintf("⚠️ Message not delivered to Instagram: %s", err.Error())
			s.postPrivateNote(ctx, account, conv.ChatwootConversationID, note)
			return
		}
	}
}

func (s *ChatwootService) postPrivateNote(ctx context.Context, account *model.Account, chatwootConvID int, message string) {
	if s.cwClient == nil {
		s.logger.Warn("cannot post private note: chatwoot client not configured")
		return
	}
	req := model.CWCreateMessageRequest{
		Content:     message,
		MessageType: "outgoing",
		Private:     true,
	}
	if err := s.cwClient.CreateMessage(ctx, account.ChatwootBaseURL, account.ChatwootAccountID, account.ChatwootAPIToken, chatwootConvID, req); err != nil {
		s.logger.Error("failed to post private note", "chatwoot_conversation_id", chatwootConvID, "error", err)
	}
}
