// instagram.go implements Connector for Instagram using the
// "Instagram API with Instagram Login" flow (graph.instagram.com,
// no Facebook required). The end user logs in directly with their
// Instagram business / creator account — there is no Facebook Page
// hop and no multi-account picker, so the "auto-select first IG
// business account" decision (Sprint 5 PRD #6) is structurally
// moot for this path.
//
// Scopes (Sprint 5):
//   - instagram_business_basic            → read profile
//   - instagram_business_content_publish  → publish posts
//   - instagram_business_manage_insights  → analytics rollup
//
// All three are part of Meta's "Instagram API with Instagram Login"
// product, which is the modern replacement for the legacy Basic
// Display API (which was deprecated Dec 2024). The same trio is
// already requested by the existing platform.InstagramAdapter for
// the BYO/dashboard OAuth flow — the Connector here mirrors them
// so a managed Connect row and a BYO row can publish identically.
//
// Token lifetimes:
//   - Short-lived access token (1 hour) returned from
//     api.instagram.com/oauth/access_token
//   - Swapped immediately for a long-lived token (60 days) via
//     graph.instagram.com/access_token?grant_type=ig_exchange_token
//   - Long-lived tokens are refreshed by GET-ing
//     graph.instagram.com/refresh_access_token (the worker calls this
//     via Refresh() below) — Instagram does NOT issue a separate
//     refresh_token, so the same access token is reused as the
//     "refresh token" stored in the social_accounts row.
//
// Feature flag: registration of this connector lives in
// cmd/api/main.go behind CONNECT_INSTAGRAM_ENABLED. When the flag
// is unset, the platform isn't in the registry and any inbound
// connect_session for "instagram" gets "platform not supported" —
// so this code is dead weight in production until we flip the
// flag, by design (Sprint 5 PRD: feature-flagged ship).

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
	instagramAuthorizeEndpoint  = "https://api.instagram.com/oauth/authorize"
	instagramTokenEndpoint      = "https://api.instagram.com/oauth/access_token"
	instagramLongLivedEndpoint  = "https://graph.instagram.com/access_token"
	instagramRefreshEndpoint    = "https://graph.instagram.com/refresh_access_token"
	instagramProfileEndpoint    = "https://graph.instagram.com/v21.0/me"

	// Comma-separated per Instagram's API contract (LinkedIn uses
	// space-separated, Twitter uses space-separated — Meta is the
	// odd one out here, hence the explicit constant).
	instagramScopes = "instagram_business_basic,instagram_business_content_publish,instagram_business_manage_insights,instagram_business_manage_comments"
)

// InstagramConnector is the Connect Connector for Instagram.
type InstagramConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	LongLivedEndpoint string
	RefreshEndpoint   string
	ProfileEndpoint   string
}

// NewInstagramConnector returns a ready Connector or nil if either
// credential is missing — same fail-fast behavior as the Twitter and
// LinkedIn connectors. Callers in main.go skip appending nil so a
// half-configured env just doesn't have Instagram registered.
func NewInstagramConnector(clientID, clientSecret, callbackBaseURL string) *InstagramConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &InstagramConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/instagram",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: instagramAuthorizeEndpoint,
		TokenEndpoint:     instagramTokenEndpoint,
		LongLivedEndpoint: instagramLongLivedEndpoint,
		RefreshEndpoint:   instagramRefreshEndpoint,
		ProfileEndpoint:   instagramProfileEndpoint,
	}
}

func (c *InstagramConnector) Platform() string { return "instagram" }

// AuthorizeURL builds the api.instagram.com authorize URL. No PKCE.
// Instagram's authorize endpoint requires response_type=code and the
// redirect_uri to match the one registered in the Meta App dashboard
// exactly (down to the trailing slash).
func (c *InstagramConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", instagramScopes)
	q.Set("state", session.OAuthState)
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

// ExchangeCode performs Instagram's two-step token swap:
//
//  1. POST to api.instagram.com/oauth/access_token to trade the
//     authorization code for a SHORT-lived token (1 hour) plus the
//     numeric Instagram user id.
//  2. GET graph.instagram.com/access_token?grant_type=ig_exchange_token
//     to swap the short-lived token for a LONG-lived one (60 days).
//
// We always do both steps inline because the dashboard / publish
// path expects a long-lived token. Step 2 failure is fatal — we
// don't fall back to the short-lived token because it'd expire
// in an hour and the customer would lose the connection
// immediately.
func (c *InstagramConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
	// Step 1: short-lived token.
	form := url.Values{}
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("redirect_uri", c.redirectURI)
	form.Set("code", code)

	req, err := http.NewRequestWithContext(ctx, "POST", c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram token exchange %d: %s", resp.StatusCode, string(body))
	}

	var short struct {
		AccessToken string `json:"access_token"`
		UserID      int64  `json:"user_id"`
	}
	if err := json.Unmarshal(body, &short); err != nil {
		return nil, fmt.Errorf("instagram token exchange decode: %w", err)
	}
	if short.AccessToken == "" {
		return nil, fmt.Errorf("instagram token exchange returned empty access_token: %s", string(body))
	}

	// Step 2: long-lived swap.
	long, expiresIn, err := c.exchangeLongLived(ctx, short.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("instagram long-lived swap: %w", err)
	}

	return &TokenSet{
		AccessToken: long,
		// Instagram doesn't issue a separate refresh_token. The same
		// long-lived access token is what gets passed to
		// graph.instagram.com/refresh_access_token to extend it. We
		// store it in BOTH slots so the existing refresh worker
		// (which reads RefreshToken) Just Works.
		RefreshToken: long,
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		Scopes:       strings.Split(instagramScopes, ","),
	}, nil
}

// exchangeLongLived swaps the short-lived token for a 60-day token.
// Returns (long_token, expires_in_seconds, error).
func (c *InstagramConnector) exchangeLongLived(ctx context.Context, shortToken string) (string, int, error) {
	q := url.Values{}
	q.Set("grant_type", "ig_exchange_token")
	q.Set("client_secret", c.clientSecret)
	q.Set("access_token", shortToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.LongLivedEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return "", 0, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("long-lived swap %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", 0, err
	}
	if raw.AccessToken == "" {
		return "", 0, fmt.Errorf("long-lived swap empty access_token")
	}
	return raw.AccessToken, raw.ExpiresIn, nil
}

// FetchProfile reads /v21.0/me?fields=id,username,profile_picture_url
// to populate the social_accounts row. Instagram's id field is the
// stable account identifier — username can change, id never does.
func (c *InstagramConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("fields", "id,username,profile_picture_url")
	q.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.ProfileEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram profile: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram profile %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID                string `json:"id"`
		Username          string `json:"username"`
		ProfilePictureURL string `json:"profile_picture_url"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("instagram profile decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("instagram profile empty id: %s", string(body))
	}
	return &Profile{
		ExternalAccountID: raw.ID,
		Username:          raw.Username,
		DisplayName:       raw.Username, // IG has no separate display name in this endpoint
		AvatarURL:         raw.ProfilePictureURL,
	}, nil
}

// Refresh extends a long-lived token via
// GET graph.instagram.com/refresh_access_token. Per Meta docs the
// token must be at least 24 hours old to be refreshable; the worker
// calls this when token_expires_at is within 30 minutes (Sprint 3
// PR7), which is well past the 24-hour minimum so we don't need to
// special-case it here.
//
// Like LinkedIn, Instagram doesn't rotate refresh tokens — there's
// only one token, and refreshing returns a new value for the SAME
// slot. We populate both AccessToken and RefreshToken in the
// returned TokenSet so the worker stores the new value in both
// places consistently.
func (c *InstagramConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	q := url.Values{}
	q.Set("grant_type", "ig_refresh_token")
	q.Set("access_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.RefreshEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("instagram refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("instagram refresh %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("instagram refresh empty access_token")
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Split(instagramScopes, ","),
	}, nil
}
