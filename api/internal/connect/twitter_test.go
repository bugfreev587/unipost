package connect

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestPKCEChallenge — locks the S256 derivation to RFC 7636.
func TestPKCEChallenge(t *testing.T) {
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	want := base64.RawURLEncoding.EncodeToString(func() []byte { s := sha256.Sum256([]byte(verifier)); return s[:] }())
	if got := pkceChallenge(verifier); got != want {
		t.Errorf("pkceChallenge: got %q, want %q", got, want)
	}
}

// TestTwitterAuthorizeURL — checks all the required params land on
// the URL with correct values.
func TestTwitterAuthorizeURL(t *testing.T) {
	c := NewTwitterConnector("client123", "secretXYZ", "https://api.example.com")
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	got, err := c.AuthorizeURL(SessionView{
		OAuthState:   "state-abc",
		PKCEVerifier: "verifier-xyz",
	})
	if err != nil {
		t.Fatalf("AuthorizeURL err: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "twitter.com" {
		t.Errorf("host: got %q, want twitter.com", u.Host)
	}
	q := u.Query()
	checks := map[string]string{
		"response_type":         "code",
		"client_id":             "client123",
		"redirect_uri":          "https://api.example.com/v1/connect/callback/twitter",
		"scope":                 twitterScopes,
		"state":                 "state-abc",
		"code_challenge_method": "S256",
		"code_challenge":        pkceChallenge("verifier-xyz"),
	}
	for k, v := range checks {
		if q.Get(k) != v {
			t.Errorf("%s: got %q, want %q", k, q.Get(k), v)
		}
	}
}

// TestTwitterAuthorizeURL_NoMediaWriteScope — Sprint 3 PR3 ships
// text-only managed Twitter. Lock the scope set so a future change
// can't accidentally request media.write before the post-pipeline
// guard is removed.
func TestTwitterAuthorizeURL_NoMediaWriteScope(t *testing.T) {
	if strings.Contains(twitterScopes, "media.write") {
		t.Error("media.write must NOT be in twitterScopes until Sprint 4 unlocks managed Twitter media")
	}
}

// TestTwitterExchangeCode_HappyPath — exercises the token endpoint
// against a fake server.
func TestTwitterExchangeCode_HappyPath(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the form payload Twitter expects.
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: got %q", r.FormValue("grant_type"))
		}
		if r.FormValue("code") != "authcode-1" {
			t.Errorf("code: got %q", r.FormValue("code"))
		}
		if r.FormValue("code_verifier") != "verifier-xyz" {
			t.Errorf("code_verifier: got %q", r.FormValue("code_verifier"))
		}
		// Basic auth using client id / secret.
		user, pass, ok := r.BasicAuth()
		if !ok || user != "client123" || pass != "secretXYZ" {
			t.Errorf("basic auth: got %q:%q ok=%v", user, pass, ok)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"token_type":"bearer",
			"access_token":"AT-1",
			"refresh_token":"RT-1",
			"expires_in":7200,
			"scope":"tweet.read tweet.write users.read offline.access"
		}`)
	}))
	defer mock.Close()

	c := NewTwitterConnector("client123", "secretXYZ", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{PKCEVerifier: "verifier-xyz"}, "authcode-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "AT-1" || tokens.RefreshToken != "RT-1" {
		t.Errorf("tokens: %+v", tokens)
	}
	if len(tokens.Scopes) != 4 {
		t.Errorf("scopes: %v", tokens.Scopes)
	}
	if tokens.ExpiresAt.IsZero() {
		t.Error("ExpiresAt not set")
	}
}

// TestTwitterFetchProfile — happy path against a fake users.me.
func TestTwitterFetchProfile(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT-1" {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"data":{"id":"1234","name":"Test User","username":"testuser","profile_image_url":"https://example.com/x.jpg"}}`)
	}))
	defer mock.Close()

	c := NewTwitterConnector("c", "s", "https://api.example.com")
	c.UsersMeEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "1234" || p.Username != "testuser" || p.DisplayName != "Test User" {
		t.Errorf("profile: %+v", p)
	}
}

// TestTwitterRefresh — happy path against a fake token endpoint.
func TestTwitterRefresh(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old-rt" {
			t.Errorf("refresh_token: %q", r.FormValue("refresh_token"))
		}
		_, _ = io.WriteString(w, `{"access_token":"new-at","refresh_token":"new-rt","expires_in":7200,"scope":"tweet.read"}`)
	}))
	defer mock.Close()

	c := NewTwitterConnector("c", "s", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.Refresh(context.Background(), "old-rt")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "new-at" || tokens.RefreshToken != "new-rt" {
		t.Errorf("tokens: %+v", tokens)
	}
}

// TestNewTwitterConnector_NilOnMissingCreds — guards against
// half-configured environments. The constructor returns nil so the
// platform simply isn't registered, which is the right failure mode.
func TestNewTwitterConnector_NilOnMissingCreds(t *testing.T) {
	if c := NewTwitterConnector("", "secret", "https://api.example.com"); c != nil {
		t.Error("expected nil when client_id is empty")
	}
	if c := NewTwitterConnector("client", "", "https://api.example.com"); c != nil {
		t.Error("expected nil when client_secret is empty")
	}
}
