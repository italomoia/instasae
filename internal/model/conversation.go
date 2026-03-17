package model

import (
	"time"

	"github.com/google/uuid"
)

type Conversation struct {
	ID                     uuid.UUID `json:"id"`
	AccountID              uuid.UUID `json:"account_id"`
	ContactID              uuid.UUID `json:"contact_id"`
	ChatwootConversationID int       `json:"chatwoot_conversation_id"`
	LastCustomerMessageAt  time.Time `json:"last_customer_message_at"`
	IsActive               bool      `json:"is_active"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}
