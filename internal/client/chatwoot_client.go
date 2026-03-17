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

var _ domain.ChatwootClient = (*CWClient)(nil)

type CWClient struct {
	httpClient *http.Client
}

func NewCWClient(httpClient *http.Client) *CWClient {
	return &CWClient{httpClient: httpClient}
}

func (c *CWClient) CreateContact(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateContactRequest) (*model.CWCreateContactResponse, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/%d/contacts", baseURL, accountID)

	respBody, err := c.doJSON(ctx, http.MethodPost, url, token, req)
	if err != nil {
		return nil, fmt.Errorf("create contact: %w", err)
	}

	var result model.CWCreateContactResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding create contact response: %w", err)
	}
	return &result, nil
}

func (c *CWClient) CreateConversation(ctx context.Context, baseURL string, accountID int, token string, req model.CWCreateConversationRequest) (*model.CWCreateConversationResponse, error) {
	url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations", baseURL, accountID)

	respBody, err := c.doJSON(ctx, http.MethodPost, url, token, req)
	if err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}

	var result model.CWCreateConversationResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("decoding create conversation response: %w", err)
	}
	return &result, nil
}

func (c *CWClient) CreateMessage(ctx context.Context, baseURL string, accountID int, token string, conversationID int, req model.CWCreateMessageRequest) error {
	url := fmt.Sprintf("%s/api/v1/accounts/%d/conversations/%d/messages", baseURL, accountID, conversationID)

	_, err := c.doJSON(ctx, http.MethodPost, url, token, req)
	if err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

func (c *CWClient) doJSON(ctx context.Context, method string, endpoint string, token string, payload any) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	var respBody []byte
	err = RetryDo(ctx, 3, 1*time.Second, func() error {
		req, reqErr := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("creating request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("api_access_token", token)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			return fmt.Errorf("executing request: %w", doErr)
		}
		defer resp.Body.Close()

		respBody, _ = io.ReadAll(resp.Body)

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &HTTPError{StatusCode: resp.StatusCode, Message: string(respBody)}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("chatwoot API: %w", err)
	}

	return respBody, nil
}
