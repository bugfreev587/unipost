package connect

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type facebookRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f facebookRoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newFacebookExchangeTestConnector(t *testing.T, pages []map[string]any) (*FacebookConnector, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/access_token":
			if r.URL.Query().Get("grant_type") == "fb_exchange_token" {
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "long-user-token"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "short-user-token"})
		case "/me/accounts":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": pages})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	c := NewFacebookConnector("fb-client", "client-secret", "https://api.example.com")
	c.TokenEndpoint = srv.URL + "/oauth/access_token"
	c.PagesEndpoint = srv.URL + "/me/accounts"
	return c, srv.Close
}

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

func TestFacebookConnectorExchangeClassifiesPageAvailability(t *testing.T) {
	tests := []struct {
		name            string
		pages           []map[string]any
		wantCode        FacebookConnectFailureCode
		wantPageCount   int
		wantPublishable int
	}{
		{
			name:            "no accessible pages",
			pages:           []map[string]any{},
			wantCode:        FacebookPageNotAvailable,
			wantPageCount:   0,
			wantPublishable: 0,
		},
		{
			name: "pages lack publishing tasks",
			pages: []map[string]any{{
				"id": "page_1", "name": "Read only", "access_token": "page-token",
				"tasks": []string{"ADVERTISE"},
			}},
			wantCode:        FacebookPagePermissionRequired,
			wantPageCount:   1,
			wantPublishable: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connector, closeServer := newFacebookExchangeTestConnector(t, tt.pages)
			defer closeServer()

			_, err := connector.ExchangeCode(context.Background(), SessionView{}, "code")
			var failure *FacebookConnectFailure
			if !errors.As(err, &failure) {
				t.Fatalf("error = %v, want *FacebookConnectFailure", err)
			}
			if failure.Code != tt.wantCode || failure.PageCount != tt.wantPageCount || failure.PublishablePageCount != tt.wantPublishable {
				t.Fatalf("failure = %+v", failure)
			}
		})
	}
}

func TestFacebookConnectorExchangeAcceptsPublishingTasks(t *testing.T) {
	for _, task := range []string{"CREATE_CONTENT", "MANAGE", "MODERATE"} {
		t.Run(task, func(t *testing.T) {
			connector, closeServer := newFacebookExchangeTestConnector(t, []map[string]any{{
				"id": "page_1", "name": "Publishable", "access_token": "page-token",
				"tasks": []string{task},
			}})
			defer closeServer()

			tokens, err := connector.ExchangeCode(context.Background(), SessionView{}, "code")
			if err != nil {
				t.Fatalf("ExchangeCode: %v", err)
			}
			if tokens.AccessToken != "page-token" {
				t.Fatalf("access token = %q", tokens.AccessToken)
			}
		})
	}
}

func TestFacebookConnectorExchangeMissingPageToken(t *testing.T) {
	connector, closeServer := newFacebookExchangeTestConnector(t, []map[string]any{{
		"id": "page_1", "name": "Publishable", "tasks": []string{"CREATE_CONTENT"},
	}})
	defer closeServer()

	_, err := connector.ExchangeCode(context.Background(), SessionView{}, "code")
	var failure *FacebookConnectFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want *FacebookConnectFailure", err)
	}
	if failure.Code != FacebookAuthorizationFailed || failure.PageCount != 1 || failure.PublishablePageCount != 1 {
		t.Fatalf("failure = %+v", failure)
	}
}

func TestFacebookConnectorExchangeDoesNotLeakProviderBody(t *testing.T) {
	for _, failStage := range []string{"short_token", "long_token", "pages"} {
		t.Run(failStage, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				stage := "pages"
				if r.URL.Path == "/oauth/access_token" {
					stage = "short_token"
					if r.URL.Query().Get("grant_type") == "fb_exchange_token" {
						stage = "long_token"
					}
				}
				if stage == failStage {
					w.WriteHeader(http.StatusBadRequest)
					_, _ = w.Write([]byte(`{"error":{"message":"raw-provider-secret","code":190,"error_subcode":463}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"access_token": stage + "-token"})
			}))
			defer srv.Close()

			connector := NewFacebookConnector("fb-client", "client-secret", "https://api.example.com")
			connector.TokenEndpoint = srv.URL + "/oauth/access_token"
			connector.PagesEndpoint = srv.URL + "/me/accounts"
			_, err := connector.ExchangeCode(context.Background(), SessionView{}, "oauth-code-secret")
			assertBoundedFacebookFailure(t, err)
		})
	}
}

func TestFacebookConnectorFetchProfileDoesNotLeakProviderBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"raw-provider-secret","code":190,"error_subcode":463}}`))
	}))
	defer srv.Close()

	connector := NewFacebookConnector("fb-client", "client-secret", "https://api.example.com")
	connector.ProfileEndpoint = srv.URL + "/me"
	_, err := connector.FetchProfile(context.Background(), "user-token-secret")
	assertBoundedFacebookFailure(t, err)
}

func TestFacebookConnectorExchangeDoesNotLeakTransportErrorURL(t *testing.T) {
	connector := NewFacebookConnector("fb-client", "client-secret", "https://api.example.com")
	connector.httpClient = &http.Client{Transport: facebookRoundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, &url.Error{
			Op:  "Get",
			URL: "https://graph.facebook.test/oauth/access_token?client_secret=client-secret&code=oauth-code-secret",
			Err: errors.New("transport failed"),
		}
	})}

	_, err := connector.ExchangeCode(context.Background(), SessionView{}, "oauth-code-secret")
	assertBoundedFacebookFailure(t, err)
}

func assertBoundedFacebookFailure(t *testing.T, err error) {
	t.Helper()
	var failure *FacebookConnectFailure
	if !errors.As(err, &failure) {
		t.Fatalf("error = %v, want typed Facebook failure", err)
	}
	for _, forbidden := range []string{"raw-provider-secret", "oauth-code-secret", "user-token-secret", "client-secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("error leaked %q: %v", forbidden, err)
		}
	}
	if failure.RemoteStatusCode != http.StatusBadRequest && !strings.HasSuffix(failure.Stage, "_transport") {
		t.Fatalf("remote status = %d, stage = %q", failure.RemoteStatusCode, failure.Stage)
	}
	if failure.RemoteStatusCode == http.StatusBadRequest && (failure.MetaCode != 190 || failure.MetaSubcode != 463) {
		t.Fatalf("Meta codes = (%d, %d), want (190, 463)", failure.MetaCode, failure.MetaSubcode)
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
