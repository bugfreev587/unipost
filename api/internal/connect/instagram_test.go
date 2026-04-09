package connect

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestInstagramAuthorizeURL — sanity check that the authorize URL
// has the right host, all required query params, and NO PKCE.
// Instagram's authorize endpoint takes the same shape as LinkedIn's
// (no PKCE, OAuth 2.0 plain code flow).
func TestInstagramAuthorizeURL(t *testing.T) {
	c := NewInstagramConnector("ig-client", "ig-secret", "https://api.example.com")
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	got, err := c.AuthorizeURL(SessionView{OAuthState: "state-xyz"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "api.instagram.com" {
		t.Errorf("host: got %q", u.Host)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "ig-client" || q.Get("state") != "state-xyz" {
		t.Errorf("missing required params: %v", q)
	}
	if q.Get("redirect_uri") != "https://api.example.com/v1/connect/callback/instagram" {
		t.Errorf("redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != instagramScopes {
		t.Errorf("scope: got %q", q.Get("scope"))
	}
	// No PKCE on Instagram's authorize URL.
	if q.Get("code_challenge") != "" || q.Get("code_challenge_method") != "" {
		t.Error("Instagram must not include PKCE params")
	}
}

// TestInstagramConstructor_NilOnMissingCreds — half-configured envs
// must produce nil so main.go's "skip if nil" path keeps the registry
// from including a broken Instagram entry.
func TestInstagramConstructor_NilOnMissingCreds(t *testing.T) {
	if c := NewInstagramConnector("", "secret", "https://api.example.com"); c != nil {
		t.Error("expected nil with empty client_id")
	}
	if c := NewInstagramConnector("client", "", "https://api.example.com"); c != nil {
		t.Error("expected nil with empty client_secret")
	}
}

// TestInstagramExchangeCode_TwoStepSwap exercises the full Instagram
// token dance: short-lived token from api.instagram.com, then a
// long-lived swap from graph.instagram.com. We mock both endpoints
// because both are non-trivial and a regression in either would
// silently leave customers with 1-hour tokens.
func TestInstagramExchangeCode_TwoStepSwap(t *testing.T) {
	// Step 1: short-lived token endpoint (form POST).
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "ig-client" || r.FormValue("client_secret") != "ig-secret" {
			t.Errorf("creds: id=%q secret=%q", r.FormValue("client_id"), r.FormValue("client_secret"))
		}
		if r.FormValue("code") != "auth-code" {
			t.Errorf("code: %q", r.FormValue("code"))
		}
		_, _ = io.WriteString(w, `{"access_token":"SHORT-AT","user_id":12345}`)
	}))
	defer short.Close()

	// Step 2: long-lived swap endpoint (GET with query string).
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("grant_type") != "ig_exchange_token" {
			t.Errorf("grant_type: %q", q.Get("grant_type"))
		}
		if q.Get("access_token") != "SHORT-AT" {
			t.Errorf("short token not forwarded: %q", q.Get("access_token"))
		}
		if q.Get("client_secret") != "ig-secret" {
			t.Errorf("client_secret missing on long-lived swap")
		}
		_, _ = io.WriteString(w, `{"access_token":"LONG-AT","token_type":"bearer","expires_in":5184000}`)
	}))
	defer long.Close()

	c := NewInstagramConnector("ig-client", "ig-secret", "https://api.example.com")
	c.TokenEndpoint = short.URL
	c.LongLivedEndpoint = long.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "LONG-AT" {
		t.Errorf("AccessToken: got %q want LONG-AT", tokens.AccessToken)
	}
	// Instagram has no separate refresh token — RefreshToken must
	// equal AccessToken so the existing refresh worker can extend it.
	if tokens.RefreshToken != "LONG-AT" {
		t.Errorf("RefreshToken should mirror AccessToken; got %q", tokens.RefreshToken)
	}
	if tokens.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set from expires_in")
	}
}

// TestInstagramExchangeCode_LongLivedFailIsFatal — if step 2 fails,
// the whole exchange must fail. We deliberately do NOT fall back to
// the short-lived token because it would expire in 1 hour and the
// customer would lose the connection immediately. Locks the
// "fail-loud" decision in the connector against future drift.
func TestInstagramExchangeCode_LongLivedFailIsFatal(t *testing.T) {
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"SHORT-AT","user_id":1}`)
	}))
	defer short.Close()
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "long-lived swap broke", http.StatusInternalServerError)
	}))
	defer long.Close()

	c := NewInstagramConnector("c", "s", "https://api.example.com")
	c.TokenEndpoint = short.URL
	c.LongLivedEndpoint = long.URL

	if _, err := c.ExchangeCode(context.Background(), SessionView{}, "code"); err == nil {
		t.Error("long-lived swap failure must return an error, not silently fall back")
	}
}

// TestInstagramFetchProfile — happy path. The id field is the
// stable identifier; username is the human-readable handle.
func TestInstagramFetchProfile(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("access_token") != "AT-1" {
			t.Errorf("access_token: %q", q.Get("access_token"))
		}
		if !strings.Contains(q.Get("fields"), "username") {
			t.Errorf("fields missing username: %q", q.Get("fields"))
		}
		_, _ = io.WriteString(w, `{"id":"ig-99","username":"shipper","profile_picture_url":"https://example.com/p.jpg"}`)
	}))
	defer mock.Close()

	c := NewInstagramConnector("c", "s", "https://api.example.com")
	c.ProfileEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "ig-99" || p.Username != "shipper" {
		t.Errorf("profile: %+v", p)
	}
}

// TestInstagramRefresh_ReusesAccessTokenSlot — Instagram's refresh
// endpoint returns ONE token that goes into both AccessToken and
// RefreshToken so the worker stores it consistently. Catches a
// regression where someone splits the slots and the next refresh
// passes the wrong token.
func TestInstagramRefresh_ReusesAccessTokenSlot(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("grant_type") != "ig_refresh_token" {
			t.Errorf("grant_type: %q", q.Get("grant_type"))
		}
		if q.Get("access_token") != "old-token" {
			t.Errorf("old token not forwarded: %q", q.Get("access_token"))
		}
		_, _ = io.WriteString(w, `{"access_token":"refreshed-token","expires_in":5184000}`)
	}))
	defer mock.Close()

	c := NewInstagramConnector("c", "s", "https://api.example.com")
	c.RefreshEndpoint = mock.URL

	tokens, err := c.Refresh(context.Background(), "old-token")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "refreshed-token" || tokens.RefreshToken != "refreshed-token" {
		t.Errorf("both slots should hold the new token; got AT=%q RT=%q", tokens.AccessToken, tokens.RefreshToken)
	}
}

// TestInstagramScopes_LockedToBusinessAPI locks the scope set against
// drift. We must request only the three scopes Meta's "Instagram API
// with Instagram Login" product grants without manual review:
// instagram_business_basic, instagram_business_content_publish,
// instagram_business_manage_insights.
func TestInstagramScopes_LockedToBusinessAPI(t *testing.T) {
	wantScopes := "instagram_business_basic,instagram_business_content_publish,instagram_business_manage_insights"
	if instagramScopes != wantScopes {
		t.Errorf("instagramScopes drift: got %q, want %q", instagramScopes, wantScopes)
	}
	// Anti-regression: legacy Basic Display API scopes (deprecated
	// Dec 2024) must NEVER come back into this string.
	for _, banned := range []string{"user_profile", "user_media", "instagram_basic"} {
		if strings.Contains(instagramScopes, banned) {
			t.Errorf("must not request deprecated/Basic Display scope %s", banned)
		}
	}
}
