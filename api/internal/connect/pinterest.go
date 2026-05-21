// pinterest.go implements Connector for Pinterest's OAuth 2.0 flow.
//
// The existing platform adapter already supports dashboard/BYO OAuth.
// This connector mirrors that token/profile contract for hosted
// Connect Sessions so managed accounts can use the same publish path.

package connect

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	pinterestAuthorizeEndpoint = "https://www.pinterest.com/oauth/"
	pinterestTokenEndpoint     = "https://api.pinterest.com/v5/oauth/token"
	pinterestProfileEndpoint   = "https://api.pinterest.com/v5/user_account"

	pinterestScopes = "boards:read,boards:write,pins:read,pins:write,user_accounts:read"
)

// PinterestConnector is the Connect Connector for Pinterest.
type PinterestConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	ProfileEndpoint   string
}

// NewPinterestConnector returns a ready Connector or nil if either
// credential is missing.
func NewPinterestConnector(clientID, clientSecret, callbackBaseURL string) *PinterestConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &PinterestConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/pinterest",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: pinterestAuthorizeEndpoint,
		TokenEndpoint:     pinterestTokenEndpoint,
		ProfileEndpoint:   pinterestProfileEndpoint,
	}
}

func (c *PinterestConnector) Platform() string { return "pinterest" }

func (c *PinterestConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("consumer_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("refreshable", "true")
	q.Set("scope", pinterestScopes)
	q.Set("state", session.OAuthState)
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

func (c *PinterestConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", c.redirectURI)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.clientID+":"+c.clientSecret)))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest token exchange %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pinterest token exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("pinterest token exchange returned empty access_token")
	}

	expiresAt := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	scopes := splitPinterestScopes(raw.Scope)
	if len(scopes) == 0 {
		scopes = splitPinterestScopes(pinterestScopes)
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    expiresAt,
		Scopes:       scopes,
	}, nil
}

func (c *PinterestConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.ProfileEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest user_account: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest user_account %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID           string `json:"id"`
		Username     string `json:"username"`
		AccountType  string `json:"account_type"`
		ProfileImage string `json:"profile_image"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pinterest user_account decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("pinterest user_account returned empty id")
	}
	return &Profile{
		ExternalAccountID: raw.ID,
		Username:          firstNonEmpty(raw.Username, raw.AccountType, raw.ID),
		DisplayName:       raw.Username,
		AvatarURL:         raw.ProfileImage,
	}, nil
}

func (c *PinterestConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.clientID+":"+c.clientSecret)))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("pinterest refresh token: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pinterest refresh token %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("pinterest refresh token decode: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("pinterest refresh token returned empty access_token")
	}
	if raw.RefreshToken == "" {
		raw.RefreshToken = refreshToken
	}
	expiresAt := time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second)
	if raw.ExpiresIn == 0 {
		expiresAt = time.Now().Add(30 * 24 * time.Hour)
	}
	scopes := splitPinterestScopes(raw.Scope)
	if len(scopes) == 0 {
		scopes = splitPinterestScopes(pinterestScopes)
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    expiresAt,
		Scopes:       scopes,
	}, nil
}

func splitPinterestScopes(scope string) []string {
	parts := strings.FieldsFunc(scope, func(r rune) bool {
		return r == ',' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}
