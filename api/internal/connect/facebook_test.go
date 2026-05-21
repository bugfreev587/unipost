package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFacebookConnectorAuthorizeURL(t *testing.T) {
	c := NewFacebookConnector("fb-client", "fb-secret", "https://api.example.com")
	u, err := c.AuthorizeURL(SessionView{OAuthState: "state_123"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	if !strings.HasPrefix(u, "https://www.facebook.com/v22.0/dialog/oauth?") {
		t.Fatalf("authorize url = %q", u)
	}
	for _, part := range []string{
		"client_id=fb-client",
		"redirect_uri=https%3A%2F%2Fapi.example.com%2Fv1%2Fconnect%2Fcallback%2Ffacebook",
		"state=state_123",
		"pages_manage_posts",
	} {
		if !strings.Contains(u, part) {
			t.Fatalf("authorize url missing %q: %s", part, u)
		}
	}
}

func TestFacebookConnectorExchangeSelectsFirstPublishablePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/access_token":
			if r.URL.Query().Get("grant_type") == "fb_exchange_token" {
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "long-user-token"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "short-user-token"})
		case "/me/accounts":
			if got := r.URL.Query().Get("access_token"); got != "long-user-token" {
				t.Fatalf("access_token = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{
					{
						"id":           "page_1",
						"name":         "No Publish",
						"access_token": "page-token-1",
						"tasks":        []string{"ADVERTISE"},
					},
					{
						"id":           "page_2",
						"name":         "Publish Page",
						"access_token": "page-token-2",
						"tasks":        []string{"CREATE_CONTENT"},
					},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewFacebookConnector("fb-client", "fb-secret", "https://api.example.com")
	c.TokenEndpoint = srv.URL + "/oauth/access_token"
	c.PagesEndpoint = srv.URL + "/me/accounts"

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "code_123")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "page-token-2" {
		t.Fatalf("access token = %q", tokens.AccessToken)
	}
	if !tokens.ExpiresAt.IsZero() {
		t.Fatalf("facebook page tokens should not store expiry, got %v", tokens.ExpiresAt)
	}
	if tokens.RefreshToken != "" {
		t.Fatalf("refresh token = %q", tokens.RefreshToken)
	}
}

func TestFacebookConnectorFetchProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("access_token"); got != "page-token" {
			t.Fatalf("access_token = %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":   "page_123",
			"name": "TailTales Page",
			"picture": map[string]any{
				"data": map[string]any{"url": "https://example.com/page.jpg"},
			},
		})
	}))
	defer srv.Close()

	c := NewFacebookConnector("fb-client", "fb-secret", "https://api.example.com")
	c.ProfileEndpoint = srv.URL + "/me"

	profile, err := c.FetchProfile(context.Background(), "page-token")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if profile.ExternalAccountID != "page_123" || profile.Username != "TailTales Page" {
		t.Fatalf("profile = %+v", profile)
	}
	if profile.AvatarURL != "https://example.com/page.jpg" {
		t.Fatalf("avatar = %q", profile.AvatarURL)
	}
}

func TestNewFacebookConnectorNilOnMissingCreds(t *testing.T) {
	if c := NewFacebookConnector("", "secret", "https://api.example.com"); c != nil {
		t.Fatal("missing client id should return nil")
	}
	if c := NewFacebookConnector("client", "", "https://api.example.com"); c != nil {
		t.Fatal("missing client secret should return nil")
	}
}
