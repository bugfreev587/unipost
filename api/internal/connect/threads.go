// threads.go implements Connector for Threads (Meta) using the
// "Threads API" OAuth flow (graph.threads.net). Structurally a clone
// of instagram.go — same two-step token swap, same long-lived
// extension model, same "refresh re-uses the access token slot"
// design — because Meta deliberately shipped Threads with the
// Instagram-API shape so existing IG integrators could plug in with
// minimal work.
//
// Scopes (Sprint 5):
//   - threads_basic            → read profile
//   - threads_content_publish  → publish posts
//
// We keep the scope list identical to platform.ThreadsAdapter so a
// managed Connect row and a BYO row publish through the same code
// path with the same permissions. Adding threads_manage_insights for
// analytics is a follow-up — Sprint 5 PR4 ships only the publish
// scopes that platform.ThreadsAdapter already requires, so the
// analytics rollup endpoint sees identical row shapes from both
// connection types and the per-account quota counts agree.
//
// Token lifetimes mirror Instagram:
//   - Short-lived access token (1 hour) from
//     graph.threads.net/oauth/access_token
//   - Swapped immediately for a long-lived token (60 days) via
//     graph.threads.net/access_token?grant_type=th_exchange_token
//   - Long-lived tokens are extended by GET-ing
//     graph.threads.net/refresh_access_token (Refresh() below).
//     Threads does NOT issue a separate refresh_token, so the same
//     access token is reused as the "refresh token" stored in the
//     social_accounts row.
//
// Feature flag: registration of this connector lives in
// cmd/api/main.go behind CONNECT_THREADS_ENABLED. When the flag is
// unset, the platform isn't in the registry — same fail-safe shape
// as the Sprint 5 PR3 Instagram gate.

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
	threadsAuthorizeEndpoint  = "https://threads.net/oauth/authorize"
	threadsTokenEndpoint      = "https://graph.threads.net/oauth/access_token"
	threadsLongLivedEndpoint  = "https://graph.threads.net/access_token"
	threadsRefreshEndpoint    = "https://graph.threads.net/refresh_access_token"
	threadsProfileEndpoint    = "https://graph.threads.net/v1.0/me"

	// Comma-separated like Instagram. Threads inherits the IG-style
	// scope serialization since the API was shipped as a thin layer
	// over the IG infrastructure.
	threadsScopes = "threads_basic,threads_content_publish"
)

// ThreadsConnector is the Connect Connector for Threads.
type ThreadsConnector struct {
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

// NewThreadsConnector returns a ready Connector or nil if either
// credential is missing — same fail-fast nil contract as the
// Twitter, LinkedIn, and Instagram constructors.
func NewThreadsConnector(clientID, clientSecret, callbackBaseURL string) *ThreadsConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &ThreadsConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/threads",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: threadsAuthorizeEndpoint,
		TokenEndpoint:     threadsTokenEndpoint,
		LongLivedEndpoint: threadsLongLivedEndpoint,
		RefreshEndpoint:   threadsRefreshEndpoint,
		ProfileEndpoint:   threadsProfileEndpoint,
	}
}

func (c *ThreadsConnector) Platform() string { return "threads" }

// AuthorizeURL builds the threads.net authorize URL. No PKCE.
// Threads' authorize endpoint is hosted on threads.net (the consumer
// domain), not graph.threads.net (the API domain) — different host
// from every other endpoint in this file, so don't try to "clean up"
// the constants by collapsing them.
func (c *ThreadsConnector) AuthorizeURL(session SessionView) (string, error) {
	q := url.Values{}
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", c.redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", threadsScopes)
	q.Set("state", session.OAuthState)
	return c.AuthorizeEndpoint + "?" + q.Encode(), nil
}

// ExchangeCode performs Threads' two-step token swap, structurally
// identical to the Instagram flow:
//
//  1. POST graph.threads.net/oauth/access_token to trade the
//     authorization code for a SHORT-lived token (1 hour) plus the
//     numeric Threads user id.
//  2. GET graph.threads.net/access_token?grant_type=th_exchange_token
//     to swap the short-lived token for a LONG-lived one (60 days).
//
// Step 2 failure is fatal — we don't fall back to the short-lived
// token because it'd expire in an hour and the customer would lose
// the connection immediately. Same decision as the IG path; same
// reasoning.
func (c *ThreadsConnector) ExchangeCode(ctx context.Context, _ SessionView, code string) (*TokenSet, error) {
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
		return nil, fmt.Errorf("threads token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads token exchange %d: %s", resp.StatusCode, string(body))
	}

	var short struct {
		AccessToken string `json:"access_token"`
		UserID      int64  `json:"user_id"`
	}
	if err := json.Unmarshal(body, &short); err != nil {
		return nil, fmt.Errorf("threads token exchange decode: %w", err)
	}
	if short.AccessToken == "" {
		return nil, fmt.Errorf("threads token exchange returned empty access_token: %s", string(body))
	}

	long, expiresIn, err := c.exchangeLongLived(ctx, short.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("threads long-lived swap: %w", err)
	}

	return &TokenSet{
		AccessToken:  long,
		RefreshToken: long, // Threads has no separate refresh token; mirror IG pattern.
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
		Scopes:       strings.Split(threadsScopes, ","),
	}, nil
}

// exchangeLongLived swaps the short-lived token for a 60-day token.
// The grant_type here is th_exchange_token (note the th_ prefix —
// Threads uses th_ where Instagram uses ig_ for the same operation).
func (c *ThreadsConnector) exchangeLongLived(ctx context.Context, shortToken string) (string, int, error) {
	q := url.Values{}
	q.Set("grant_type", "th_exchange_token")
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

// FetchProfile reads /v1.0/me?fields=id,username,threads_profile_picture_url
// to populate the social_accounts row. Threads' field name for the
// avatar is threads_profile_picture_url (NOT profile_picture_url like
// Instagram) — easy to typo when porting between the two.
func (c *ThreadsConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("fields", "id,username,threads_profile_picture_url")
	q.Set("access_token", accessToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.ProfileEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads profile: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads profile %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		ID                       string `json:"id"`
		Username                 string `json:"username"`
		ThreadsProfilePictureURL string `json:"threads_profile_picture_url"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("threads profile decode: %w", err)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("threads profile empty id: %s", string(body))
	}
	return &Profile{
		ExternalAccountID: raw.ID,
		Username:          raw.Username,
		DisplayName:       raw.Username,
		AvatarURL:         raw.ThreadsProfilePictureURL,
	}, nil
}

// Refresh extends a long-lived token via
// GET graph.threads.net/refresh_access_token. Per Meta docs the
// token must be at least 24 hours old to be refreshable; the worker
// calls this when token_expires_at is within 30 minutes (Sprint 3
// PR7), well past the 24-hour minimum.
//
// Like Instagram, Threads doesn't rotate refresh tokens — there's
// only one token, and refreshing returns a new value for the SAME
// slot. Both AccessToken and RefreshToken in the returned TokenSet
// hold the new value so the worker stores it consistently.
func (c *ThreadsConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	q := url.Values{}
	q.Set("grant_type", "th_refresh_token")
	q.Set("access_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, "GET", c.RefreshEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("threads refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("threads refresh %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("threads refresh empty access_token")
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.AccessToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Split(threadsScopes, ","),
	}, nil
}
