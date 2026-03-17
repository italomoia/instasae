package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

var _ domain.InstagramClient = (*IGClient)(nil)

type IGClient struct {
	httpClient *http.Client
	baseURL    string
	apiVersion string
}

func NewIGClient(httpClient *http.Client, apiVersion string) *IGClient {
	return &IGClient{
		httpClient: httpClient,
		baseURL:    "https://graph.instagram.com",
		apiVersion: apiVersion,
	}
}

func NewIGClientWithBaseURL(httpClient *http.Client, apiVersion string, baseURL string) *IGClient {
	return &IGClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		apiVersion: apiVersion,
	}
}

func (c *IGClient) SendTextMessage(ctx context.Context, pageID string, token string, recipientID string, text string) (*model.IGSendMessageResponse, error) {
	req := model.IGSendMessageRequest{
		Recipient: model.IGParticipant{ID: recipientID},
		Message:   model.IGSendMessage{Text: text},
		Tag:       "HUMAN_AGENT",
	}
	return c.sendMessage(ctx, pageID, token, req)
}

func (c *IGClient) SendAttachment(ctx context.Context, pageID string, token string, recipientID string, attachmentType string, url string) (*model.IGSendMessageResponse, error) {
	req := model.IGSendMessageRequest{
		Recipient: model.IGParticipant{ID: recipientID},
		Message: model.IGSendMessage{
			Attachment: &model.IGSendAttachment{
				Type:    attachmentType,
				Payload: model.IGSendAttachmentPayload{URL: url},
			},
		},
		Tag: "HUMAN_AGENT",
	}
	return c.sendMessage(ctx, pageID, token, req)
}

func (c *IGClient) sendMessage(ctx context.Context, pageID string, token string, payload model.IGSendMessageRequest) (*model.IGSendMessageResponse, error) {
	url := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, pageID)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling send message request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating send message request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("instagram API error: status %d, body: %s", resp.StatusCode, respBody)
	}

	var result model.IGSendMessageResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding send message response: %w", err)
	}
	return &result, nil
}

func (c *IGClient) GetUserProfile(ctx context.Context, token string, userID string) (*model.IGUserProfile, error) {
	url := fmt.Sprintf("%s/%s/%s?fields=name,username,profile_pic", c.baseURL, c.apiVersion, userID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating profile request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("getting user profile: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("instagram API error: status %d, body: %s", resp.StatusCode, respBody)
	}

	var profile model.IGUserProfile
	if err := json.Unmarshal(respBody, &profile); err != nil {
		return nil, fmt.Errorf("decoding profile response: %w", err)
	}
	return &profile, nil
}
