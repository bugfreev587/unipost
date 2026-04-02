package platform

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// OAuthConfig holds the OAuth 2.0 configuration for a platform.
type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	RedirectURL  string
	Scopes       []string
}

// OAuthAdapter extends PlatformAdapter with OAuth 2.0 flow methods.
type OAuthAdapter interface {
	PlatformAdapter

	// DefaultOAuthConfig returns the default OAuth config using UniPost's own credentials.
	// baseRedirectURL is the base callback URL (e.g., "https://api.unipost.dev").
	DefaultOAuthConfig(baseRedirectURL string) OAuthConfig

	// GetAuthURL constructs the authorization URL the user should be redirected to.
	GetAuthURL(config OAuthConfig, state string) string

	// ExchangeCode exchanges an authorization code for tokens and account info.
	ExchangeCode(ctx context.Context, config OAuthConfig, code string) (*ConnectResult, error)
}

// GenerateState creates a cryptographically random state string for CSRF protection.
func GenerateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// BuildAuthURL is a helper to construct a standard OAuth 2.0 authorization URL.
func BuildAuthURL(authURL, clientID, redirectURL, state string, scopes []string) string {
	params := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {redirectURL},
		"response_type": {"code"},
		"state":         {state},
		"scope":         {strings.Join(scopes, " ")},
	}
	return authURL + "?" + params.Encode()
}
