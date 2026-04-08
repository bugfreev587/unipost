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

func TestLinkedInAuthorizeURL(t *testing.T) {
	c := NewLinkedInConnector("client123", "secretXYZ", "https://api.example.com")
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	got, err := c.AuthorizeURL(SessionView{OAuthState: "state-abc"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Host != "www.linkedin.com" {
		t.Errorf("host: got %q", u.Host)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "client123" || q.Get("state") != "state-abc" {
		t.Errorf("missing required params: %v", q)
	}
	if q.Get("redirect_uri") != "https://api.example.com/v1/connect/callback/linkedin" {
		t.Errorf("redirect_uri: %q", q.Get("redirect_uri"))
	}
	if q.Get("scope") != linkedinScopes {
		t.Errorf("scope: got %q", q.Get("scope"))
	}
	// LinkedIn does NOT use PKCE.
	if q.Get("code_challenge") != "" || q.Get("code_challenge_method") != "" {
		t.Error("LinkedIn must not include PKCE params")
	}
}

func TestLinkedInExchangeCode_HappyPath(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "client123" || r.FormValue("client_secret") != "secretXYZ" {
			t.Errorf("creds in form: id=%q secret=%q", r.FormValue("client_id"), r.FormValue("client_secret"))
		}
		// LinkedIn requires creds in the form body, NOT basic auth.
		if _, _, ok := r.BasicAuth(); ok {
			t.Error("LinkedIn must NOT use HTTP basic auth")
		}
		_, _ = io.WriteString(w, `{"access_token":"AT-1","expires_in":5184000,"refresh_token":"RT-1","refresh_token_expires_in":31536000,"scope":"openid,profile,email,w_member_social"}`)
	}))
	defer mock.Close()

	c := NewLinkedInConnector("client123", "secretXYZ", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "auth-code-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "AT-1" || tokens.RefreshToken != "RT-1" {
		t.Errorf("tokens: %+v", tokens)
	}
	if len(tokens.Scopes) != 4 {
		t.Errorf("scopes (CSV-split): %v", tokens.Scopes)
	}
}

func TestLinkedInFetchProfile(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT-1" {
			t.Errorf("auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"sub":"linkedin-user-99","name":"Jane Doe","email":"jane@example.com","picture":"https://example.com/p.jpg"}`)
	}))
	defer mock.Close()

	c := NewLinkedInConnector("c", "s", "https://api.example.com")
	c.UserinfoEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "linkedin-user-99" || p.DisplayName != "Jane Doe" || p.Username != "jane@example.com" {
		t.Errorf("profile: %+v", p)
	}
}

func TestLinkedInRefresh_NoRotation(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		_, _ = io.WriteString(w, `{"access_token":"new-at","expires_in":5184000,"scope":"w_member_social"}`)
	}))
	defer mock.Close()

	c := NewLinkedInConnector("c", "s", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.Refresh(context.Background(), "old-rt")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "new-at" {
		t.Errorf("access_token: %q", tokens.AccessToken)
	}
	// LinkedIn doesn't rotate — RefreshToken must be empty so the
	// worker keeps the existing one.
	if tokens.RefreshToken != "" {
		t.Errorf("LinkedIn refresh should NOT return a new refresh token; got %q", tokens.RefreshToken)
	}
}

func TestLinkedInScopes_NoMarketingTier(t *testing.T) {
	// Sprint 3 risk #2: do NOT request ad-tier scopes.
	for _, banned := range []string{"r_ads", "rw_ads", "w_organization_social", "r_organization_social"} {
		if strings.Contains(linkedinScopes, banned) {
			t.Errorf("must not request gated scope %s — Sign In with LinkedIn OIDC tier only", banned)
		}
	}
	// Sprint 3 PR4 hotfix: r_liteprofile was the legacy v1 scope
	// before LinkedIn migrated this product to OIDC. Requesting it
	// now triggers "Scope r_liteprofile is not authorized for your
	// application". The replacement is the OIDC `profile` scope.
	if strings.Contains(linkedinScopes, "r_liteprofile") {
		t.Error("must not request r_liteprofile — use OIDC `profile` scope instead")
	}
	// Lock the exact scope set so future edits stay in sync with
	// what the OIDC product actually grants.
	wantScopes := "openid profile email w_member_social"
	if linkedinScopes != wantScopes {
		t.Errorf("linkedinScopes drift: got %q, want %q", linkedinScopes, wantScopes)
	}
}
