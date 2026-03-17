package model

import (
	"time"

	"github.com/google/uuid"
)

type Contact struct {
	ID                      uuid.UUID `json:"id"`
	AccountID               uuid.UUID `json:"account_id"`
	IGSenderID              string    `json:"ig_sender_id"`
	ChatwootContactID       int       `json:"chatwoot_contact_id"`
	ChatwootContactSourceID string    `json:"chatwoot_contact_source_id"`
	IGUsername               *string   `json:"ig_username,omitempty"`
	IGName                  *string   `json:"ig_name,omitempty"`
	IGProfilePic            *string   `json:"ig_profile_pic,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}
