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

func TestYouTubeAuthorizeURL(t *testing.T) {
	c := NewYouTubeConnector("client123", "secretXYZ", "https://api.example.com")
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
	if u.Host != "accounts.google.com" {
		t.Errorf("host: got %q", u.Host)
	}
	q := u.Query()
	checks := map[string]string{
		"response_type": "code",
		"client_id":     "client123",
		"redirect_uri":  "https://api.example.com/v1/connect/callback/youtube",
		"scope":         youtubeScopes,
		"state":         "state-abc",
		"access_type":   "offline",
		"prompt":        "consent",
	}
	for k, want := range checks {
		if q.Get(k) != want {
			t.Errorf("%s: got %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestYouTubeExchangeCode_HappyPath(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "client123" || r.FormValue("client_secret") != "secretXYZ" {
			t.Errorf("creds in form: id=%q secret=%q", r.FormValue("client_id"), r.FormValue("client_secret"))
		}
		if _, _, ok := r.BasicAuth(); ok {
			t.Error("YouTube must NOT use HTTP basic auth")
		}
		_, _ = io.WriteString(w, `{"access_token":"AT-1","refresh_token":"RT-1","expires_in":3600,"scope":"https://www.googleapis.com/auth/youtube.upload https://www.googleapis.com/auth/youtube.readonly"}`)
	}))
	defer mock.Close()

	c := NewYouTubeConnector("client123", "secretXYZ", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "auth-code-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "AT-1" || tokens.RefreshToken != "RT-1" {
		t.Errorf("tokens: %+v", tokens)
	}
	if len(tokens.Scopes) != 2 {
		t.Errorf("scopes: %v", tokens.Scopes)
	}
}

func TestYouTubeFetchProfile(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT-1" {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"items":[{"id":"UC123","snippet":{"title":"My Channel","customUrl":"@mychannel","thumbnails":{"default":{"url":"https://example.com/avatar.jpg"}}}}]}`)
	}))
	defer mock.Close()

	c := NewYouTubeConnector("c", "s", "https://api.example.com")
	c.ChannelsEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "UC123" || p.Username != "@mychannel" || p.DisplayName != "My Channel" {
		t.Errorf("profile: %+v", p)
	}
}

func TestYouTubeRefresh_KeepsRefreshTokenWhenGoogleDoesNotRotate(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		_, _ = io.WriteString(w, `{"access_token":"new-at","expires_in":3600}`)
	}))
	defer mock.Close()

	c := NewYouTubeConnector("c", "s", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.Refresh(context.Background(), "old-rt")
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if tokens.AccessToken != "new-at" {
		t.Errorf("access token: %q", tokens.AccessToken)
	}
	if tokens.RefreshToken != "old-rt" {
		t.Errorf("refresh token: got %q, want old-rt", tokens.RefreshToken)
	}
}

func TestNewYouTubeConnector_NilOnMissingCreds(t *testing.T) {
	if c := NewYouTubeConnector("", "secret", "https://api.example.com"); c != nil {
		t.Error("expected nil when client_id is empty")
	}
	if c := NewYouTubeConnector("client", "", "https://api.example.com"); c != nil {
		t.Error("expected nil when client_secret is empty")
	}
}

func TestYouTubeScopes_LockExpectedSet(t *testing.T) {
	for _, want := range []string{
		"https://www.googleapis.com/auth/youtube.upload",
		"https://www.googleapis.com/auth/youtube.readonly",
	} {
		if !strings.Contains(youtubeScopes, want) {
			t.Errorf("youtubeScopes missing %q", want)
		}
	}
}
