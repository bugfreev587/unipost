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

// TestThreadsAuthorizeURL — sanity check that the authorize URL is
// hosted on threads.net (NOT graph.threads.net — they're different
// domains and the consumer authorize lives on the bare domain).
func TestThreadsAuthorizeURL(t *testing.T) {
	c := NewThreadsConnector("th-client", "th-secret", "https://api.example.com")
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	got, err := c.AuthorizeURL(SessionView{OAuthState: "state-th"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "threads.net" {
		t.Errorf("host: got %q, want threads.net (NOT graph.threads.net)", u.Host)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "th-client" || q.Get("state") != "state-th" {
		t.Errorf("missing required params: %v", q)
	}
	if q.Get("redirect_uri") != "https://api.example.com/v1/connect/callback/threads" {
		t.Errorf("redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != threadsScopes {
		t.Errorf("scope: got %q", q.Get("scope"))
	}
	if q.Get("code_challenge") != "" {
		t.Error("Threads must not include PKCE params")
	}
}

// TestThreadsConstructor_NilOnMissingCreds — half-configured envs
// must produce nil so main.go's "skip if nil" path keeps the
// registry from including a broken Threads entry.
func TestThreadsConstructor_NilOnMissingCreds(t *testing.T) {
	if c := NewThreadsConnector("", "secret", "https://api.example.com"); c != nil {
		t.Error("expected nil with empty client_id")
	}
	if c := NewThreadsConnector("client", "", "https://api.example.com"); c != nil {
		t.Error("expected nil with empty client_secret")
	}
}

// TestThreadsExchangeCode_TwoStepSwap exercises the full Threads
// token dance. Critical that the long-lived swap uses
// grant_type=th_exchange_token (note the th_ prefix — Threads uses
// th_ where Instagram uses ig_, and a typo here would silently
// downgrade every Threads connection to a 1-hour lifespan).
func TestThreadsExchangeCode_TwoStepSwap(t *testing.T) {
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "th-client" || r.FormValue("client_secret") != "th-secret" {
			t.Errorf("creds: id=%q secret=%q", r.FormValue("client_id"), r.FormValue("client_secret"))
		}
		if r.FormValue("code") != "auth-code" {
			t.Errorf("code: %q", r.FormValue("code"))
		}
		_, _ = io.WriteString(w, `{"access_token":"SHORT-AT","user_id":98765}`)
	}))
	defer short.Close()

	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// CRITICAL: th_ prefix, not ig_.
		if q.Get("grant_type") != "th_exchange_token" {
			t.Errorf("grant_type: got %q, want th_exchange_token", q.Get("grant_type"))
		}
		if q.Get("access_token") != "SHORT-AT" {
			t.Errorf("short token not forwarded: %q", q.Get("access_token"))
		}
		if q.Get("client_secret") != "th-secret" {
			t.Errorf("client_secret missing on long-lived swap")
		}
		_, _ = io.WriteString(w, `{"access_token":"LONG-AT","token_type":"bearer","expires_in":5184000}`)
	}))
	defer long.Close()

	c := NewThreadsConnector("th-client", "th-secret", "https://api.example.com")
	c.TokenEndpoint = short.URL
	c.LongLivedEndpoint = long.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "LONG-AT" {
		t.Errorf("AccessToken: got %q want LONG-AT", tokens.AccessToken)
	}
	if tokens.RefreshToken != "LONG-AT" {
		t.Errorf("RefreshToken should mirror AccessToken; got %q", tokens.RefreshToken)
	}
	if tokens.ExpiresAt.IsZero() {
		t.Error("ExpiresAt should be set from expires_in")
	}
}

// TestThreadsExchangeCode_LongLivedFailIsFatal — like Instagram,
// Threads must NOT fall back to the short-lived token on long-lived
// swap failure. Locks the fail-loud decision against drift.
func TestThreadsExchangeCode_LongLivedFailIsFatal(t *testing.T) {
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"SHORT-AT","user_id":1}`)
	}))
	defer short.Close()
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "long-lived swap broke", http.StatusInternalServerError)
	}))
	defer long.Close()

	c := NewThreadsConnector("c", "s", "https://api.example.com")
	c.TokenEndpoint = short.URL
	c.LongLivedEndpoint = long.URL

	if _, err := c.ExchangeCode(context.Background(), SessionView{}, "code"); err == nil {
		t.Error("long-lived swap failure must return an error, not silently fall back")
	}
}

// TestThreadsFetchProfile_HandlesDifferentAvatarFieldName — Threads
// uses threads_profile_picture_url (NOT profile_picture_url like
// Instagram). Easy to typo when porting between the two; this test
// catches it.
func TestThreadsFetchProfile_HandlesDifferentAvatarFieldName(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !strings.Contains(q.Get("fields"), "threads_profile_picture_url") {
			t.Errorf("fields must request threads_profile_picture_url, got %q", q.Get("fields"))
		}
		_, _ = io.WriteString(w, `{"id":"th-77","username":"poster","threads_profile_picture_url":"https://example.com/p.jpg"}`)
	}))
	defer mock.Close()

	c := NewThreadsConnector("c", "s", "https://api.example.com")
	c.ProfileEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "th-77" || p.Username != "poster" || p.AvatarURL != "https://example.com/p.jpg" {
		t.Errorf("profile: %+v", p)
	}
}

// TestThreadsRefresh_ReusesAccessTokenSlot — Threads refresh returns
// ONE token that goes into both AccessToken and RefreshToken so the
// worker stores it consistently. Same shape as Instagram.
func TestThreadsRefresh_ReusesAccessTokenSlot(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		// CRITICAL: th_ prefix.
		if q.Get("grant_type") != "th_refresh_token" {
			t.Errorf("grant_type: got %q, want th_refresh_token", q.Get("grant_type"))
		}
		if q.Get("access_token") != "old-token" {
			t.Errorf("old token not forwarded: %q", q.Get("access_token"))
		}
		_, _ = io.WriteString(w, `{"access_token":"refreshed-token","expires_in":5184000}`)
	}))
	defer mock.Close()

	c := NewThreadsConnector("c", "s", "https://api.example.com")
	c.RefreshEndpoint = mock.URL

	tokens, err := c.Refresh(context.Background(), "old-token")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "refreshed-token" || tokens.RefreshToken != "refreshed-token" {
		t.Errorf("both slots should hold the new token; got AT=%q RT=%q", tokens.AccessToken, tokens.RefreshToken)
	}
}

// TestThreadsScopes_LockedToFullTier locks the scope set against
// drift. All scopes submitted together for Meta App Review:
// basic + publish + replies + insights.
func TestThreadsScopes_LockedToFullTier(t *testing.T) {
	wantScopes := "threads_basic,threads_content_publish,threads_manage_replies,threads_manage_insights,threads_read_replies"
	if threadsScopes != wantScopes {
		t.Errorf("threadsScopes drift: got %q, want %q", threadsScopes, wantScopes)
	}
	// Anti-regression: legacy scopes must not appear.
	for _, banned := range []string{"user_profile"} {
		if strings.Contains(threadsScopes, banned) {
			t.Errorf("must not request deprecated scope %s", banned)
		}
	}
}
