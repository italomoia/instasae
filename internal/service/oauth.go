package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/italomoia/instasae/internal/domain"
	"github.com/italomoia/instasae/internal/model"
)

type OAuthService struct {
	accountRepo domain.AccountRepository
	cache       domain.Cache
	httpClient  *http.Client
	appID       string
	appSecret   string
	redirectURI string
	apiVersion  string
	logger      *slog.Logger
}

func NewOAuthService(
	accountRepo domain.AccountRepository,
	cache domain.Cache,
	httpClient *http.Client,
	appID string,
	appSecret string,
	redirectURI string,
	apiVersion string,
	logger *slog.Logger,
) *OAuthService {
	return &OAuthService{
		accountRepo: accountRepo,
		cache:       cache,
		httpClient:  httpClient,
		appID:       appID,
		appSecret:   appSecret,
		redirectURI: redirectURI,
		apiVersion:  apiVersion,
		logger:      logger,
	}
}

// GetAuthorizationURL builds the Instagram OAuth URL with state=accountID.
func (s *OAuthService) GetAuthorizationURL(accountID string) string {
	params := url.Values{
		"client_id":     {s.appID},
		"redirect_uri":  {s.redirectURI},
		"response_type": {"code"},
		"scope":         {"instagram_business_basic,instagram_business_manage_messages"},
		"state":         {accountID},
	}
	return "https://www.instagram.com/oauth/authorize?" + params.Encode()
}

// HandleCallback exchanges the OAuth code for tokens, fetches the profile,
// updates the account, and subscribes the page to webhooks.
func (s *OAuthService) HandleCallback(ctx context.Context, code string, accountID string) error {
	// Validate account exists
	id, err := uuid.Parse(accountID)
	if err != nil {
		return fmt.Errorf("invalid account ID: %w", err)
	}

	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("looking up account: %w", err)
	}
	if account == nil {
		return fmt.Errorf("account not found: %s", accountID)
	}

	// Strip trailing #_ that Instagram appends
	code = strings.TrimSuffix(code, "#_")

	// Step 1: Exchange code for short-lived token
	shortToken, userID, err := s.exchangeCodeForToken(ctx, code)
	if err != nil {
		return fmt.Errorf("exchanging code for token: %w", err)
	}

	// Step 2: Exchange short-lived for long-lived token
	longToken, expiresIn, err := s.exchangeForLongLivedToken(ctx, shortToken)
	if err != nil {
		return fmt.Errorf("exchanging for long-lived token: %w", err)
	}

	// Step 3: Get user profile
	username, name, err := s.getUserProfile(ctx, longToken)
	if err != nil {
		s.logger.Warn("failed to fetch profile during OAuth, using user_id", "user_id", userID, "error", err)
		username = userID
		name = userID
	}

	// Step 4: Update account
	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)
	pageName := username
	if name != "" {
		pageName = name
	}

	updateParams := model.UpdateAccountParams{
		IGPageID:       &userID,
		IGPageName:     &pageName,
		IGAccessToken:  &longToken,
		TokenExpiresAt: &expiresAt,
	}

	// Invalidate old cache if ig_page_id is changing
	if account.IGPageID != "" && account.IGPageID != userID {
		_ = s.cache.DeleteAccount(ctx, account.IGPageID)
	}

	if _, err := s.accountRepo.Update(ctx, id, updateParams); err != nil {
		return fmt.Errorf("updating account with OAuth credentials: %w", err)
	}

	// Invalidate new cache entry
	_ = s.cache.DeleteAccount(ctx, userID)

	// Step 5: Subscribe page to webhooks
	if err := s.subscribePage(ctx, userID, longToken); err != nil {
		s.logger.Error("failed to subscribe page to webhooks after OAuth", "page_id", userID, "error", err)
		// Non-blocking — account is already updated, admin can re-subscribe manually
	}

	s.logger.Info("OAuth flow completed", "account_id", accountID, "ig_page_id", userID, "username", username)
	return nil
}

// AccountExists checks if an account ID exists.
func (s *OAuthService) AccountExists(ctx context.Context, accountID string) (bool, error) {
	id, err := uuid.Parse(accountID)
	if err != nil {
		return false, nil
	}
	account, err := s.accountRepo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return account != nil, nil
}

type igTokenResponse struct {
	AccessToken string `json:"access_token"`
	UserID      int64  `json:"user_id"`
}

type igTokenDataResponse struct {
	Data []igTokenResponse `json:"data"`
}

func (s *OAuthService) exchangeCodeForToken(ctx context.Context, code string) (token string, userID string, err error) {
	form := url.Values{
		"client_id":     {s.appID},
		"client_secret": {s.appSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {s.redirectURI},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.instagram.com/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sending token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// Try data array format first (Business Login)
	var dataResp igTokenDataResponse
	if err := json.Unmarshal(body, &dataResp); err == nil && len(dataResp.Data) > 0 {
		return dataResp.Data[0].AccessToken, fmt.Sprintf("%d", dataResp.Data[0].UserID), nil
	}

	// Fall back to flat format
	var tokenResp igTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", fmt.Errorf("decoding token response: %w", err)
	}

	return tokenResp.AccessToken, fmt.Sprintf("%d", tokenResp.UserID), nil
}

type igLongLivedTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (s *OAuthService) exchangeForLongLivedToken(ctx context.Context, shortToken string) (token string, expiresIn int64, err error) {
	params := url.Values{
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {s.appSecret},
		"access_token":  {shortToken},
	}

	endpoint := "https://graph.instagram.com/access_token?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", 0, fmt.Errorf("creating long-lived token request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("sending long-lived token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("long-lived token exchange failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp igLongLivedTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("decoding long-lived token response: %w", err)
	}

	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

type igMeResponse struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

func (s *OAuthService) getUserProfile(ctx context.Context, token string) (username string, name string, err error) {
	endpoint := fmt.Sprintf("https://graph.instagram.com/%s/me?fields=user_id,username,name&access_token=%s", s.apiVersion, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", fmt.Errorf("creating profile request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("sending profile request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("profile fetch failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var profile igMeResponse
	if err := json.Unmarshal(body, &profile); err != nil {
		return "", "", fmt.Errorf("decoding profile response: %w", err)
	}

	return profile.Username, profile.Name, nil
}

func (s *OAuthService) subscribePage(ctx context.Context, pageID string, token string) error {
	endpoint := fmt.Sprintf("https://graph.instagram.com/%s/%s/subscribed_apps?subscribed_fields=messages&access_token=%s", s.apiVersion, pageID, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("creating subscribe request: %w", err)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending subscribe request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("subscribe failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
