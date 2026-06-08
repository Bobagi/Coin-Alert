package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Google OAuth 2.0 / OpenID Connect endpoints.
const (
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint         = "https://oauth2.googleapis.com/token"
	googleUserInfoEndpoint      = "https://openidconnect.googleapis.com/v1/userinfo"
)

// ErrGoogleOAuthNotConfigured is returned when Google sign-in is requested but the server has no
// OAuth client configured.
var ErrGoogleOAuthNotConfigured = errors.New("google sign-in is not configured")

// GoogleUserInfo is the subset of the OIDC userinfo response we rely on.
type GoogleUserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

// GoogleOAuthService implements the OAuth 2.0 authorization-code flow against Google using only the
// standard library. The userinfo endpoint (reached over TLS with the access token) is treated as the
// source of truth, so we do not verify the id_token signature ourselves.
type GoogleOAuthService struct {
	clientID     string
	clientSecret string
	redirectURL  string
	httpClient   *http.Client
}

// NewGoogleOAuthService returns a configured service, or nil when any required setting is missing
// (in which case the feature is simply disabled and the UI hides the Google button).
func NewGoogleOAuthService(clientID string, clientSecret string, redirectURL string) *GoogleOAuthService {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" || strings.TrimSpace(redirectURL) == "" {
		return nil
	}
	return &GoogleOAuthService{
		clientID:     strings.TrimSpace(clientID),
		clientSecret: strings.TrimSpace(clientSecret),
		redirectURL:  strings.TrimSpace(redirectURL),
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthorizationURL builds the Google consent-screen URL the browser is redirected to. The opaque
// state is echoed back to the callback for CSRF protection.
func (service *GoogleOAuthService) AuthorizationURL(state string) string {
	query := url.Values{}
	query.Set("client_id", service.clientID)
	query.Set("redirect_uri", service.redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid email profile")
	query.Set("state", state)
	query.Set("access_type", "online")
	query.Set("prompt", "select_account")
	return googleAuthorizationEndpoint + "?" + query.Encode()
}

// ExchangeCodeForUserInfo trades an authorization code for an access token and returns the verified
// Google profile behind it.
func (service *GoogleOAuthService) ExchangeCodeForUserInfo(operationContext context.Context, authorizationCode string) (*GoogleUserInfo, error) {
	accessToken, exchangeError := service.exchangeCode(operationContext, authorizationCode)
	if exchangeError != nil {
		return nil, exchangeError
	}
	return service.fetchUserInfo(operationContext, accessToken)
}

func (service *GoogleOAuthService) exchangeCode(operationContext context.Context, authorizationCode string) (string, error) {
	form := url.Values{}
	form.Set("code", authorizationCode)
	form.Set("client_id", service.clientID)
	form.Set("client_secret", service.clientSecret)
	form.Set("redirect_uri", service.redirectURL)
	form.Set("grant_type", "authorization_code")

	request, requestError := http.NewRequestWithContext(operationContext, http.MethodPost, googleTokenEndpoint, strings.NewReader(form.Encode()))
	if requestError != nil {
		return "", requestError
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	response, responseError := service.httpClient.Do(request)
	if responseError != nil {
		return "", responseError
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google token exchange failed: status %d", response.StatusCode)
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if decodeError := json.Unmarshal(body, &tokenResponse); decodeError != nil {
		return "", decodeError
	}
	if tokenResponse.AccessToken == "" {
		return "", errors.New("google token exchange returned no access token")
	}
	return tokenResponse.AccessToken, nil
}

func (service *GoogleOAuthService) fetchUserInfo(operationContext context.Context, accessToken string) (*GoogleUserInfo, error) {
	request, requestError := http.NewRequestWithContext(operationContext, http.MethodGet, googleUserInfoEndpoint, nil)
	if requestError != nil {
		return nil, requestError
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")

	response, responseError := service.httpClient.Do(request)
	if responseError != nil {
		return nil, responseError
	}
	defer response.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo request failed: status %d", response.StatusCode)
	}

	userInfo := &GoogleUserInfo{}
	if decodeError := json.Unmarshal(body, userInfo); decodeError != nil {
		return nil, decodeError
	}
	if userInfo.Subject == "" || userInfo.Email == "" {
		return nil, errors.New("google userinfo response was incomplete")
	}
	return userInfo, nil
}
