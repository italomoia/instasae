package model

// Inbound webhook payload from Chatwoot

type CWWebhookPayload struct {
	Event        string         `json:"event"`
	ID           int            `json:"id"`
	Content      string         `json:"content"`
	MessageType  string         `json:"message_type"`
	Private      bool           `json:"private"`
	ContentType  string         `json:"content_type"`
	Conversation CWConversation `json:"conversation"`
	Sender       CWSender       `json:"sender"`
	Inbox        CWInbox        `json:"inbox"`
	Account      CWAccount      `json:"account"`
	Attachments  []CWAttachment `json:"attachments,omitempty"`
}

type CWConversation struct {
	ID      int `json:"id"`
	InboxID int `json:"inbox_id"`
}

type CWSender struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type CWInbox struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CWAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type CWAttachment struct {
	DataURL  string `json:"data_url"`
	FileType string `json:"file_type"`
	ThumbURL string `json:"thumb_url"`
}

// Outbound API payloads to Chatwoot

type CWCreateContactRequest struct {
	InboxID          int               `json:"inbox_id"`
	Name             string            `json:"name"`
	Identifier       string            `json:"identifier"`
	AvatarURL        string            `json:"avatar_url,omitempty"`
	CustomAttributes map[string]string `json:"custom_attributes,omitempty"`
}

type CWCreateContactResponse struct {
	Payload CWContactPayload `json:"payload"`
}

type CWContactPayload struct {
	Contact CWContact `json:"contact"`
}

type CWContact struct {
	ID             int              `json:"id"`
	Name           string           `json:"name"`
	ContactInboxes []CWContactInbox `json:"contact_inboxes"`
}

type CWContactInbox struct {
	SourceID string  `json:"source_id"`
	Inbox    CWInbox `json:"inbox"`
}

type CWCreateConversationRequest struct {
	SourceID  string `json:"source_id"`
	InboxID   int    `json:"inbox_id"`
	ContactID int    `json:"contact_id"`
	Status    string `json:"status"`
}

type CWCreateConversationResponse struct {
	ID int `json:"id"`
}

type CWCreateMessageRequest struct {
	Content     string `json:"content"`
	MessageType string `json:"message_type"`
	Private     bool   `json:"private"`
}
