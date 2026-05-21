package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestPinterestEndpointsDefaultToProduction(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	t.Setenv("PINTEREST_API_BASE_URL", "")
	t.Setenv("PINTEREST_TOKEN_URL", "")
	t.Setenv("PINTEREST_AUTH_URL", "")

	if got := pinterestAPIBaseURL(); got != pinterestAPIBase {
		t.Fatalf("api base = %q, want %q", got, pinterestAPIBase)
	}
	if got := pinterestTokenURL(); got != pinterestTokenEndpoint {
		t.Fatalf("token url = %q, want %q", got, pinterestTokenEndpoint)
	}
	if got := pinterestAuthURL(); got != pinterestOAuthEndpoint {
		t.Fatalf("auth url = %q, want %q", got, pinterestOAuthEndpoint)
	}
}

func TestPinterestEndpointsUseSandboxShortcut(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	t.Setenv("PINTEREST_API_BASE_URL", "")
	t.Setenv("PINTEREST_TOKEN_URL", "")
	t.Setenv("PINTEREST_AUTH_URL", "")

	if got := pinterestAPIBaseURL(); got != pinterestSandboxAPIBase {
		t.Fatalf("api base = %q, want %q", got, pinterestSandboxAPIBase)
	}
	if got := pinterestTokenURL(); got != pinterestSandboxAPIBase+"/oauth/token" {
		t.Fatalf("token url = %q, want %q", got, pinterestSandboxAPIBase+"/oauth/token")
	}
	if got := pinterestAuthURL(); got != pinterestOAuthEndpoint {
		t.Fatalf("auth url = %q, want %q", got, pinterestOAuthEndpoint)
	}
}

func TestPinterestEndpointsHonorExplicitOverrides(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	t.Setenv("PINTEREST_API_BASE_URL", "https://example.test/v5/")
	t.Setenv("PINTEREST_TOKEN_URL", "https://example.test/oauth/token")
	t.Setenv("PINTEREST_AUTH_URL", "https://example.test/oauth/")

	if got := pinterestAPIBaseURL(); got != "https://example.test/v5" {
		t.Fatalf("api base = %q, want trimmed override", got)
	}
	if got := pinterestTokenURL(); got != "https://example.test/oauth/token" {
		t.Fatalf("token url = %q, want explicit override", got)
	}
	if got := pinterestAuthURL(); got != "https://example.test/oauth/" {
		t.Fatalf("auth url = %q, want explicit override", got)
	}
}

func TestPinterestGetAuthURLUsesClientID(t *testing.T) {
	adapter := NewPinterestAdapter()
	got := adapter.GetAuthURL(OAuthConfig{
		ClientID:    "pin-client",
		AuthURL:     pinterestOAuthEndpoint,
		RedirectURL: "https://api.example.com/v1/oauth/callback/pinterest",
		Scopes:      pinterestScopes,
	}, "state-123")
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}
	q := parsed.Query()
	if q.Get("client_id") != "pin-client" {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("consumer_id") != "" {
		t.Fatalf("consumer_id should not be sent, got %q", q.Get("consumer_id"))
	}
	if q.Get("state") != "state-123" || q.Get("response_type") != "code" {
		t.Fatalf("unexpected auth params: %s", got)
	}
}

func TestPinterestCreateBoardUsesBoardsEndpoint(t *testing.T) {
	var gotMethod, gotAuth string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/v5/boards" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		body, _ := io.ReadAll(r.Body)
		gotBody = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"board-123","name":"Sandbox test board"}`))
	}))
	defer srv.Close()

	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")
	adapter := &PinterestAdapter{client: srv.Client()}

	board, err := adapter.CreateBoard(context.Background(), "token-123", "Sandbox test board")
	if err != nil {
		t.Fatalf("CreateBoard failed: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotAuth != "Bearer token-123" {
		t.Fatalf("auth = %q, want bearer token", gotAuth)
	}

	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if payload["name"] != "Sandbox test board" {
		t.Fatalf("payload name = %q", payload["name"])
	}
	if board.ID != "board-123" || board.Name != "Sandbox test board" {
		t.Fatalf("unexpected board: %#v", board)
	}
}

func TestPinterestGetAnalyticsParsesSummaryAndLifetimeMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/pins/1107111520928571145/analytics" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		if got := r.URL.Query().Get("metric_types"); got == "" {
			http.Error(w, "missing metric_types", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"ALL": {
				"summary_metrics": {
					"IMPRESSION": 120,
					"OUTBOUND_CLICK": 4,
					"SAVE": 7
				},
				"lifetime_metrics": {
					"TOTAL_COMMENTS": 3,
					"TOTAL_REACTIONS": 9
				}
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("PINTEREST_USE_SANDBOX", "")
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")
	adapter := &PinterestAdapter{client: srv.Client()}

	metrics, err := adapter.GetAnalytics(context.Background(), "token-123", "1107111520928571145")
	if err != nil {
		t.Fatalf("GetAnalytics failed: %v", err)
	}
	if metrics.Impressions != 120 || metrics.Clicks != 4 || metrics.Saves != 7 {
		t.Fatalf("unexpected summary metrics: %#v", metrics)
	}
	if metrics.Comments != 3 || metrics.Likes != 9 {
		t.Fatalf("unexpected lifetime metrics: %#v", metrics)
	}
}

func TestPinterestPostStagesEphemeralImageURL(t *testing.T) {
	var gotMediaURL string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v5/pins" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		var payload struct {
			MediaSource struct {
				URL string `json:"url"`
			} `json:"media_source"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		gotMediaURL = payload.MediaSource.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"pin-123","url":"https://www.pinterest.com/pin/pin-123/"}`))
	}))
	defer srv.Close()

	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")

	staged := "https://public.example/media/staged.jpg"
	adapter := &PinterestAdapter{
		client: srv.Client(),
		stageFromURL: func(_ context.Context, sourceURL string) (string, error) {
			if !looksEphemeralFetchURL(sourceURL) {
				t.Fatalf("expected ephemeral source URL, got %q", sourceURL)
			}
			return staged, nil
		},
	}

	_, err := adapter.Post(context.Background(), "token-123", "caption", []MediaItem{{
		URL:  "https://example.r2.cloudflarestorage.com/media/a.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc",
		Kind: MediaKindImage,
	}}, map[string]any{
		"board_id": "board-123",
		"title":    "Hello Pinterest",
	})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if gotMediaURL != staged {
		t.Fatalf("media_source.url = %q, want %q", gotMediaURL, staged)
	}
}

func TestLooksEphemeralFetchURL(t *testing.T) {
	if !looksEphemeralFetchURL("https://example.com/a.jpg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc") {
		t.Fatal("expected aws-signed URL to be detected as ephemeral")
	}
	if looksEphemeralFetchURL("https://cdn.example.com/a.jpg") {
		t.Fatal("expected plain public URL to not be treated as ephemeral")
	}
}
