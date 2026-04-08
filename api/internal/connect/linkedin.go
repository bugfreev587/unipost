// linkedin.go implements Connector for LinkedIn using OAuth 2.0
// without PKCE. LinkedIn's standard authorization-code flow only
// uses `state` for CSRF — there's no PKCE challenge.
//
// Scopes (Sprint 3): w_member_social r_liteprofile openid profile email.
//
//   - w_member_social → post on the user's behalf
//   - r_liteprofile / openid / profile / email → read the user's
//     basic identity for the social_accounts row
//
// All four are part of LinkedIn's "Sign In with LinkedIn using
// OpenID Connect" product, which is instant-approval in the
// Developer Portal. Higher-tier scopes (Marketing Developer Platform
// etc.) require an approval review and are NOT requested here.
//
// Token lifetimes: access tokens are valid for 60 days, refresh
// tokens for ~1 year. Refresh tokens are NOT rotated — the same
// value stays valid for the row's lifetime, so Refresh() returns
// an empty RefreshToken to signal "keep the existing one."

package connect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	linkedinAuthorizeEndpoint = "https://www.linkedin.com/oauth/v2/authorization"
	linkedinTokenEndpoint     = "https://www.linkedin.com/oauth/v2/accessToken"
	linkedinUserinfoEndpoint  = "https://api.linkedin.com/v2/userinfo"

	linkedinScopes = "w_member_social r_liteprofile openid profile email"
)

// LinkedInConnector is the OAuth 2.0 Connector for LinkedIn.
type LinkedInConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	UserinfoEndpoint  string
}

// NewLinkedInConnector returns a ready Connector or nil if either
// credential is missing — same fail-fast behavior as TwitterConnector.
func NewLinkedInConnector(clientID, clientSecret, callbackBaseURL string) *LinkedInConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &LinkedInConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/linkedin",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: linkedinAuthorizeEndpoint,
		TokenEndpoint:     linkedinTokenEndpoint,
		UserinfoEndpoint:  linkedinUserinfoEndpoint,
	}
}

func (l *LinkedInConnector) Platform() string { return "linkedin" }

// AuthorizeURL builds the linkedin.com authorize URL. No PKCE.
func (l *LinkedInConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", l.clientID)
	q.Set("redirect_uri", l.redirectURI)
	q.Set("scope", linkedinScopes)
	q.Set("state", session.OAuthState)
	return l.AuthorizeEndpoint + "?" + q.Encode(), nil
}

// ExchangeCode trades the auth code for access + refresh tokens.
// LinkedIn requires client_id/secret in the form body, NOT basic
// auth — this is the one place LinkedIn's OAuth differs visibly
// from Twitter's.
func (l *LinkedInConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", l.redirectURI)
	form.Set("client_id", l.clientID)
	form.Set("client_secret", l.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", l.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linkedin token exchange %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken           string `json:"access_token"`
		ExpiresIn             int    `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
		Scope                 string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("linkedin token exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("linkedin token exchange returned empty access_token: %s", string(body))
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Fields(strings.ReplaceAll(raw.Scope, ",", " ")),
	}, nil
}

// FetchProfile reads /v2/userinfo (the OIDC-shaped endpoint that
// the openid scope unlocks). Returns the OIDC `sub` as the stable
// account id.
func (l *LinkedInConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", l.UserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin userinfo: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linkedin userinfo %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("linkedin userinfo decode: %w", err)
	}
	if raw.Sub == "" {
		return nil, fmt.Errorf("linkedin userinfo empty sub: %s", string(body))
	}
	return &Profile{
		ExternalAccountID: raw.Sub,
		Username:          raw.Email,
		DisplayName:       raw.Name,
		AvatarURL:         raw.Picture,
	}, nil
}

// Refresh swaps a refresh token for a new access token. LinkedIn does
// NOT rotate refresh tokens, so we leave RefreshToken empty in the
// returned TokenSet to signal "keep the existing one."
func (l *LinkedInConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", l.clientID)
	form.Set("client_secret", l.clientSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", l.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("linkedin refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linkedin refresh %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("linkedin refresh empty access_token")
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: "", // LinkedIn doesn't rotate — keep the existing one
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Fields(strings.ReplaceAll(raw.Scope, ",", " ")),
	}, nil
}
