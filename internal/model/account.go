package model

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID                 uuid.UUID  `json:"id"`
	IGPageID           string     `json:"ig_page_id"`
	IGPageName         string     `json:"ig_page_name"`
	IGAccessToken      string     `json:"-"`
	ChatwootBaseURL    string     `json:"chatwoot_base_url"`
	ChatwootAccountID  int        `json:"chatwoot_account_id"`
	ChatwootInboxID    int        `json:"chatwoot_inbox_id"`
	ChatwootAPIToken   string     `json:"-"`
	WebhookVerifyToken string     `json:"-"`
	TokenExpiresAt     *time.Time `json:"token_expires_at,omitempty"`
	IsActive           bool       `json:"is_active"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CreateAccountParams struct {
	IGPageID           string     `json:"ig_page_id"`
	IGPageName         string     `json:"ig_page_name"`
	IGAccessToken      string     `json:"ig_access_token"`
	ChatwootBaseURL    string     `json:"chatwoot_base_url"`
	ChatwootAccountID  int        `json:"chatwoot_account_id"`
	ChatwootInboxID    int        `json:"chatwoot_inbox_id"`
	ChatwootAPIToken   string     `json:"chatwoot_api_token"`
	WebhookVerifyToken string     `json:"webhook_verify_token"`
	TokenExpiresAt     *time.Time `json:"token_expires_at,omitempty"`
}

type UpdateAccountParams struct {
	IGPageID           *string    `json:"ig_page_id,omitempty"`
	IGPageName         *string    `json:"ig_page_name,omitempty"`
	IGAccessToken      *string    `json:"ig_access_token,omitempty"`
	ChatwootBaseURL    *string    `json:"chatwoot_base_url,omitempty"`
	ChatwootAccountID  *int       `json:"chatwoot_account_id,omitempty"`
	ChatwootInboxID    *int       `json:"chatwoot_inbox_id,omitempty"`
	ChatwootAPIToken   *string    `json:"chatwoot_api_token,omitempty"`
	WebhookVerifyToken *string    `json:"webhook_verify_token,omitempty"`
	TokenExpiresAt     *time.Time `json:"token_expires_at,omitempty"`
	IsActive           *bool      `json:"is_active,omitempty"`
}
