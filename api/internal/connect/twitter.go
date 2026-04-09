// twitter.go implements Connector for Twitter / X using OAuth 2.0
// with PKCE. The flow:
//
//	1. AuthorizeURL → builds the twitter.com/i/oauth2/authorize URL
//	   with code_challenge = base64url(SHA256(verifier)).
//
//	2. ExchangeCode → POSTs to api.twitter.com/2/oauth2/token with
//	   grant_type=authorization_code, code, redirect_uri, code_verifier,
//	   plus HTTP basic auth using client id / secret.
//
//	3. FetchProfile → GET api.twitter.com/2/users/me with the bearer.
//
//	4. Refresh → POSTs to the same token endpoint with
//	   grant_type=refresh_token. Twitter rotates the refresh token
//	   on every refresh — we always return both.
//
// Sprint 3 PR3 ships text-only managed Twitter — the media.write
// scope is intentionally NOT requested per founder decision #2.
// The post validator (handler/validate.go branch) refuses any
// media on a managed Twitter account so the user fails fast
// instead of getting a 403 from Twitter.

package connect

import (
	"context"
	"crypto/sha256"
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
	twitterAuthorizeEndpoint = "https://twitter.com/i/oauth2/authorize"
	twitterTokenEndpoint     = "https://api.twitter.com/2/oauth2/token"
	twitterUsersMeEndpoint   = "https://api.twitter.com/2/users/me"

	// Sprint 4 PR1: managed Twitter now supports media. media.write
	// was added to the UniPost OAuth app's scope allowlist; this
	// constant requests it on every Connect handshake so newly-minted
	// tokens can call POST /1.1/media/upload. Existing tokens minted
	// before this change DON'T have the scope and need a re-Connect.
	twitterScopes = "tweet.read tweet.write users.read offline.access media.write"
)

// TwitterConnector is the OAuth 2.0 PKCE Connector for Twitter / X.
//
// Endpoint fields are exported so tests can swap them for an httptest
// server. Production code never touches them after construction.
type TwitterConnector struct {
	clientID     string
	clientSecret string
	redirectURI  string
	httpClient   *http.Client

	AuthorizeEndpoint string
	TokenEndpoint     string
	UsersMeEndpoint   string
}

// NewTwitterConnector reads credentials + the registered callback URL
// and returns a ready Connector. Both client id / secret are required;
// the constructor returns nil when either is missing so a misconfigured
// process fails to even register the platform.
func NewTwitterConnector(clientID, clientSecret, callbackBaseURL string) *TwitterConnector {
	if clientID == "" || clientSecret == "" {
		return nil
	}
	return &TwitterConnector{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       strings.TrimRight(callbackBaseURL, "/") + "/v1/connect/callback/twitter",
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		AuthorizeEndpoint: twitterAuthorizeEndpoint,
		TokenEndpoint:     twitterTokenEndpoint,
		UsersMeEndpoint:   twitterUsersMeEndpoint,
	}
}

func (t *TwitterConnector) Platform() string { return "twitter" }

// pkceChallenge derives the S256 challenge from a verifier per RFC 7636:
// challenge = base64url(SHA256(verifier))
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// AuthorizeURL builds the twitter.com authorize URL with all the
// required PKCE + scope + state params.
func (t *TwitterConnector) AuthorizeURL(session SessionView) (string, error) {
	if session.PKCEVerifier == "" {
		return "", fmt.Errorf("twitter PKCE flow requires a verifier on the session")
	}
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", t.clientID)
	q.Set("redirect_uri", t.redirectURI)
	q.Set("scope", twitterScopes)
	q.Set("state", session.OAuthState)
	q.Set("code_challenge", pkceChallenge(session.PKCEVerifier))
	q.Set("code_challenge_method", "S256")
	return t.AuthorizeEndpoint + "?" + q.Encode(), nil
}

// ExchangeCode trades the authorization code from the callback for
// access + refresh tokens. Twitter is one of the platforms that
// REQUIRES the same redirect_uri on this call as on the authorize
// call — we use t.redirectURI for both.
func (t *TwitterConnector) ExchangeCode(ctx context.Context, session SessionView, code string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", t.redirectURI)
	form.Set("code_verifier", session.PKCEVerifier)
	form.Set("client_id", t.clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", t.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// Confidential client → HTTP basic auth as well as client_id in body.
	req.SetBasicAuth(t.clientID, t.clientSecret)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitter token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter token exchange %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		TokenType    string `json:"token_type"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("twitter token exchange decode: %w", err)
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("twitter token exchange returned empty access_token: %s", string(body))
	}

	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Fields(raw.Scope),
	}, nil
}

// FetchProfile reads /2/users/me to get the user id + handle.
func (t *TwitterConnector) FetchProfile(ctx context.Context, accessToken string) (*Profile, error) {
	q := url.Values{}
	q.Set("user.fields", "profile_image_url,name,username")
	req, err := http.NewRequestWithContext(ctx, "GET", t.UsersMeEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitter users.me: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter users.me %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		Data struct {
			ID              string `json:"id"`
			Name            string `json:"name"`
			Username        string `json:"username"`
			ProfileImageURL string `json:"profile_image_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("twitter users.me decode: %w", err)
	}
	if raw.Data.ID == "" {
		return nil, fmt.Errorf("twitter users.me empty: %s", string(body))
	}
	return &Profile{
		ExternalAccountID: raw.Data.ID,
		Username:          raw.Data.Username,
		DisplayName:       raw.Data.Name,
		AvatarURL:         raw.Data.ProfileImageURL,
	}, nil
}

// Refresh exchanges a refresh token for a new access token. Twitter
// rotates the refresh token on every call so we always return both.
func (t *TwitterConnector) Refresh(ctx context.Context, refreshToken string) (*TokenSet, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)
	form.Set("client_id", t.clientID)

	req, err := http.NewRequestWithContext(ctx, "POST", t.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(t.clientID, t.clientSecret)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitter refresh: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitter refresh %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	if raw.AccessToken == "" {
		return nil, fmt.Errorf("twitter refresh empty access_token")
	}
	return &TokenSet{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
		Scopes:       strings.Fields(raw.Scope),
	}, nil
}

