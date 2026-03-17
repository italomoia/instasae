package model

// Inbound webhook payloads

type IGWebhookPayload struct {
	Object string    `json:"object"`
	Entry  []IGEntry `json:"entry"`
}

type IGEntry struct {
	Time      int64         `json:"time"`
	ID        string        `json:"id"`
	Messaging []IGMessaging `json:"messaging"`
}

type IGMessaging struct {
	Sender    IGParticipant `json:"sender"`
	Recipient IGParticipant `json:"recipient"`
	Timestamp int64         `json:"timestamp"`
	Message   *IGMessage    `json:"message,omitempty"`
}

type IGParticipant struct {
	ID string `json:"id"`
}

type IGMessage struct {
	MID         string         `json:"mid"`
	Text        string         `json:"text,omitempty"`
	IsEcho      bool           `json:"is_echo,omitempty"`
	Attachments []IGAttachment `json:"attachments,omitempty"`
}

type IGAttachment struct {
	Type    string              `json:"type"`
	Payload IGAttachmentPayload `json:"payload"`
}

type IGAttachmentPayload struct {
	URL string `json:"url"`
}

// Outbound send payloads

type IGSendMessageRequest struct {
	Recipient IGParticipant `json:"recipient"`
	Message   IGSendMessage `json:"message"`
	Tag       string        `json:"tag,omitempty"`
}

type IGSendMessage struct {
	Text       string            `json:"text,omitempty"`
	Attachment *IGSendAttachment `json:"attachment,omitempty"`
}

type IGSendAttachment struct {
	Type    string                  `json:"type"`
	Payload IGSendAttachmentPayload `json:"payload"`
}

type IGSendAttachmentPayload struct {
	URL string `json:"url"`
}

type IGSendMessageResponse struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

// User profile

type IGUserProfile struct {
	Name       string `json:"name"`
	Username   string `json:"username"`
	ProfilePic string `json:"profile_pic"`
	ID         string `json:"id"`
}
