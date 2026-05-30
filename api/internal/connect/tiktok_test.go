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

func TestTikTokAuthorizeURL(t *testing.T) {
	t.Setenv("FEATURE_TIKTOK_ANALYTICS_SCOPES", "false")

	c := NewTikTokConnector("client-key", "secretXYZ", "https://api.example.com")
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
	if u.Host != "www.tiktok.com" {
		t.Errorf("host: got %q", u.Host)
	}
	q := u.Query()
	checks := map[string]string{
		"response_type": "code",
		"client_key":    "client-key",
		"redirect_uri":  "https://api.example.com/v1/connect/callback/tiktok",
		"scope":         "video.publish,video.upload,user.info.basic",
		"state":         "state-abc",
	}
	for k, want := range checks {
		if q.Get(k) != want {
			t.Errorf("%s: got %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestTikTokAuthorizeURL_AppReviewSessionIgnoresAnalyticsScopes(t *testing.T) {
	t.Setenv("FEATURE_TIKTOK_ANALYTICS_SCOPES", "true")

	c := NewTikTokConnector("client-key", "secretXYZ", "https://api.example.com")
	if c == nil {
		t.Fatal("constructor returned nil")
	}
	got, err := c.AuthorizeURL(SessionView{OAuthState: "state-abc", ExternalUserID: "app-review:rvjob_1"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if scope := u.Query().Get("scope"); scope != "video.publish,video.upload,user.info.basic" {
		t.Fatalf("scope = %q", scope)
	}
}

func TestTikTokExchangeCode_HappyPath(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("grant_type") != "authorization_code" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_key") != "client-key" || r.FormValue("client_secret") != "secretXYZ" {
			t.Errorf("creds in form: key=%q secret=%q", r.FormValue("client_key"), r.FormValue("client_secret"))
		}
		if r.FormValue("redirect_uri") != "https://api.example.com/v1/connect/callback/tiktok" {
			t.Errorf("redirect_uri: %q", r.FormValue("redirect_uri"))
		}
		_, _ = io.WriteString(w, `{"data":{"access_token":"AT-1","refresh_token":"RT-1","expires_in":3600}}`)
	}))
	defer mock.Close()

	c := NewTikTokConnector("client-key", "secretXYZ", "https://api.example.com")
	c.TokenEndpoint = mock.URL

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "auth-code-1")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "AT-1" || tokens.RefreshToken != "RT-1" {
		t.Errorf("tokens: %+v", tokens)
	}
	if len(tokens.Scopes) == 0 || !containsString(tokens.Scopes, "video.publish") {
		t.Errorf("scopes: %v", tokens.Scopes)
	}
}

func TestTikTokFetchProfile(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer AT-1" {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}
		if !strings.Contains(r.URL.RawQuery, "fields=open_id%2Cdisplay_name%2Cavatar_url") {
			t.Errorf("query: %q", r.URL.RawQuery)
		}
		_, _ = io.WriteString(w, `{"data":{"user":{"open_id":"open-123","display_name":"TailTales","avatar_url":"https://example.com/avatar.jpg"}},"error":{"code":"ok"}}`)
	}))
	defer mock.Close()

	c := NewTikTokConnector("c", "s", "https://api.example.com")
	c.UserInfoEndpoint = mock.URL

	p, err := c.FetchProfile(context.Background(), "AT-1")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.ExternalAccountID != "open-123" || p.Username != "TailTales" || p.AvatarURL == "" {
		t.Errorf("profile: %+v", p)
	}
}

func TestTikTokRefresh_KeepsRefreshTokenWhenTikTokDoesNotRotate(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type: %q", r.FormValue("grant_type"))
		}
		_, _ = io.WriteString(w, `{"access_token":"new-at","expires_in":3600}`)
	}))
	defer mock.Close()

	c := NewTikTokConnector("c", "s", "https://api.example.com")
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

func TestNewTikTokConnector_NilOnMissingCreds(t *testing.T) {
	if c := NewTikTokConnector("", "secret", "https://api.example.com"); c != nil {
		t.Error("expected nil when client key is empty")
	}
	if c := NewTikTokConnector("client", "", "https://api.example.com"); c != nil {
		t.Error("expected nil when client secret is empty")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
