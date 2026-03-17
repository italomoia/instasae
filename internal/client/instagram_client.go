package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

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
	endpoint := fmt.Sprintf("%s/%s/%s/messages", c.baseURL, c.apiVersion, pageID)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling send message request: %w", err)
	}

	var result model.IGSendMessageResponse
	err = RetryDo(ctx, 3, 1*time.Second, func() error {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("creating send message request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("sending message: %w", doErr)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &HTTPError{StatusCode: resp.StatusCode, Message: string(respBody)}
		}

		if unmarshalErr := json.Unmarshal(respBody, &result); unmarshalErr != nil {
			return fmt.Errorf("decoding send message response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("instagram send message: %w", err)
	}
	return &result, nil
}

func (c *IGClient) GetUserProfile(ctx context.Context, token string, userID string) (*model.IGUserProfile, error) {
	endpoint := fmt.Sprintf("%s/%s/%s?fields=name,username,profile_pic", c.baseURL, c.apiVersion, userID)

	var profile model.IGUserProfile
	err := RetryDo(ctx, 3, 1*time.Second, func() error {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if reqErr != nil {
			return fmt.Errorf("creating profile request: %w", reqErr)
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("getting user profile: %w", doErr)
		}
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &HTTPError{StatusCode: resp.StatusCode, Message: string(respBody)}
		}

		if unmarshalErr := json.Unmarshal(respBody, &profile); unmarshalErr != nil {
			return fmt.Errorf("decoding profile response: %w", unmarshalErr)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("instagram get profile: %w", err)
	}
	return &profile, nil
}
