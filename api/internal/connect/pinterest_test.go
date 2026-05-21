package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPinterestConnectorAuthorizeURL(t *testing.T) {
	c := NewPinterestConnector("pin-client", "pin-secret", "https://api.example.com")
	u, err := c.AuthorizeURL(SessionView{OAuthState: "state_123"})
	if err != nil {
		t.Fatalf("AuthorizeURL: %v", err)
	}
	if !strings.HasPrefix(u, "https://www.pinterest.com/oauth/?") {
		t.Fatalf("authorize url = %q", u)
	}
	for _, part := range []string{
		"consumer_id=pin-client",
		"redirect_uri=https%3A%2F%2Fapi.example.com%2Fv1%2Fconnect%2Fcallback%2Fpinterest",
		"refreshable=true",
		"state=state_123",
		"pins%3Awrite",
	} {
		if !strings.Contains(u, part) {
			t.Fatalf("authorize url missing %q: %s", part, u)
		}
	}
}

func TestPinterestConnectorExchangeFetchAndRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}
			switch r.Form.Get("grant_type") {
			case "authorization_code":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token":  "pin-access",
					"refresh_token": "pin-refresh",
					"expires_in":    3600,
					"scope":         "pins:read,pins:write",
				})
			case "refresh_token":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"access_token":  "pin-access-2",
					"refresh_token": "pin-refresh-2",
					"expires_in":    7200,
					"scope":         "pins:read,pins:write",
				})
			default:
				t.Fatalf("grant_type = %q", r.Form.Get("grant_type"))
			}
		case "/user_account":
			if got := r.Header.Get("Authorization"); got != "Bearer pin-access" {
				t.Fatalf("Authorization = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":            "pin_user_123",
				"username":      "tailtales",
				"account_type":  "BUSINESS",
				"profile_image": "https://example.com/pin.jpg",
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewPinterestConnector("pin-client", "pin-secret", "https://api.example.com")
	c.TokenEndpoint = srv.URL + "/oauth/token"
	c.ProfileEndpoint = srv.URL + "/user_account"

	tokens, err := c.ExchangeCode(context.Background(), SessionView{}, "code_123")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tokens.AccessToken != "pin-access" || tokens.RefreshToken != "pin-refresh" {
		t.Fatalf("tokens = %+v", tokens)
	}
	if len(tokens.Scopes) != 2 || tokens.Scopes[1] != "pins:write" {
		t.Fatalf("scopes = %#v", tokens.Scopes)
	}

	profile, err := c.FetchProfile(context.Background(), tokens.AccessToken)
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if profile.ExternalAccountID != "pin_user_123" || profile.Username != "tailtales" {
		t.Fatalf("profile = %+v", profile)
	}

	refreshed, err := c.Refresh(context.Background(), tokens.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if refreshed.AccessToken != "pin-access-2" || refreshed.RefreshToken != "pin-refresh-2" {
		t.Fatalf("refreshed = %+v", refreshed)
	}
}

func TestNewPinterestConnectorNilOnMissingCreds(t *testing.T) {
	if c := NewPinterestConnector("", "secret", "https://api.example.com"); c != nil {
		t.Fatal("missing client id should return nil")
	}
	if c := NewPinterestConnector("client", "", "https://api.example.com"); c != nil {
		t.Fatal("missing client secret should return nil")
	}
}
