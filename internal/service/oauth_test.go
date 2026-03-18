package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/model"
	"github.com/italomoia/instasae/internal/service"
)

func TestGetAuthorizationURL(t *testing.T) {
	repo := &mockAccountRepo{}
	cache := &mockCache{}
	svc := service.NewOAuthService(repo, cache, http.DefaultClient, "test_app_id", "test_secret", "https://example.com/oauth/callback", "v25.0", slog.Default())

	accountID := uuid.New().String()
	url := svc.GetAuthorizationURL(accountID)

	checks := []string{
		"https://www.instagram.com/oauth/authorize",
		"client_id=test_app_id",
		"redirect_uri=https%3A%2F%2Fexample.com%2Foauth%2Fcallback",
		"response_type=code",
		"scope=instagram_business_basic%2Cinstagram_business_manage_messages",
		"state=" + accountID,
	}
	for _, check := range checks {
		if !strings.Contains(url, check) {
			t.Errorf("URL missing %q\ngot: %s", check, url)
		}
	}
}

func TestHandleCallback_Success(t *testing.T) {
	accountID := uuid.New()

	// Mock Instagram OAuth endpoints
	igServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/oauth/access_token" && r.Method == http.MethodPost:
			// Short-lived token exchange
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "short_token_123",
				"user_id":      12345,
			})
		case r.URL.Path == "/access_token" && r.URL.Query().Get("grant_type") == "ig_exchange_token":
			// Long-lived token exchange
			json.NewEncoder(w).Encode(map[string]any{
				"access_token": "long_token_456",
				"token_type":   "bearer",
				"expires_in":   5183944,
			})
		case r.URL.Path == "/v25.0/me":
			// Profile fetch
			json.NewEncoder(w).Encode(map[string]string{
				"user_id":  "12345",
				"username": "testuser",
				"name":     "Test User",
			})
		case strings.Contains(r.URL.Path, "/subscribed_apps"):
			// Webhook subscription
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer igServer.Close()

	var updatedParams model.UpdateAccountParams
	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			if id == accountID {
				return &model.Account{
					ID:       accountID,
					IGPageID: "pending_oauth",
					IsActive: true,
				}, nil
			}
			return nil, nil
		},
		UpdateFn: func(ctx context.Context, id uuid.UUID, params model.UpdateAccountParams) (*model.Account, error) {
			updatedParams = params
			return &model.Account{
				ID:       id,
				IGPageID: *params.IGPageID,
				IsActive: true,
			}, nil
		},
	}

	cache := &mockCache{
		DeleteAccountFn: func(ctx context.Context, igPageID string) error { return nil },
	}

	// Create service with overridden URLs pointing to test server
	svc := newTestOAuthService(repo, cache, igServer.URL, "test_app_id", "test_secret")

	err := svc.HandleCallback(context.Background(), "auth_code_123#_", accountID.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if updatedParams.IGPageID == nil || *updatedParams.IGPageID != "12345" {
		t.Errorf("expected ig_page_id=12345, got %v", updatedParams.IGPageID)
	}
	if updatedParams.IGAccessToken == nil || *updatedParams.IGAccessToken != "long_token_456" {
		t.Errorf("expected ig_access_token=long_token_456, got %v", updatedParams.IGAccessToken)
	}
	if updatedParams.IGPageName == nil || *updatedParams.IGPageName != "Test User" {
		t.Errorf("expected ig_page_name=Test User, got %v", updatedParams.IGPageName)
	}
	if updatedParams.TokenExpiresAt == nil {
		t.Fatal("expected token_expires_at to be set")
	}
	// Token should expire approximately 60 days from now
	expectedExpiry := time.Now().Add(5183944 * time.Second)
	diff := updatedParams.TokenExpiresAt.Sub(expectedExpiry)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("token_expires_at off by %v", diff)
	}
}

func TestHandleCallback_CodeExchangeFails(t *testing.T) {
	accountID := uuid.New()

	igServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error_type":"OAuthException","code":400,"error_message":"Invalid code"}`)
	}))
	defer igServer.Close()

	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			return &model.Account{ID: accountID, IGPageID: "pending_oauth", IsActive: true}, nil
		},
	}
	cache := &mockCache{}

	svc := newTestOAuthService(repo, cache, igServer.URL, "app_id", "secret")

	err := svc.HandleCallback(context.Background(), "bad_code", accountID.String())
	if err == nil {
		t.Fatal("expected error for failed code exchange")
	}
	if !strings.Contains(err.Error(), "exchanging code for token") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHandleCallback_AccountNotFound(t *testing.T) {
	repo := &mockAccountRepo{
		GetByIDFn: func(ctx context.Context, id uuid.UUID) (*model.Account, error) {
			return nil, nil
		},
	}
	cache := &mockCache{}

	svc := newTestOAuthService(repo, cache, "http://unused", "app_id", "secret")

	err := svc.HandleCallback(context.Background(), "code", uuid.New().String())
	if err == nil {
		t.Fatal("expected error for missing account")
	}
	if !strings.Contains(err.Error(), "account not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// newTestOAuthService creates an OAuthService that uses a test server URL
// for all Instagram API calls. We use a custom HTTP client transport to
// redirect Instagram API hosts to the test server.
func newTestOAuthService(repo *mockAccountRepo, cache *mockCache, testServerURL string, appID, appSecret string) *service.OAuthService {
	transport := &rewriteTransport{baseURL: testServerURL}
	httpClient := &http.Client{Transport: transport, Timeout: 5 * time.Second}

	return service.NewOAuthService(repo, cache, httpClient, appID, appSecret, "https://example.com/oauth/callback", "v25.0", slog.Default())
}

// rewriteTransport rewrites all requests to point to the test server.
type rewriteTransport struct {
	baseURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Rewrite the URL to point to test server, keeping path and query
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.baseURL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}
