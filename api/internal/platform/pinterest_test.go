package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestPinterestEnvironmentMatchesActiveAPI(t *testing.T) {
	t.Setenv("PINTEREST_USE_SANDBOX", "")
	if got := PinterestEnvironment(); got != "production" {
		t.Fatalf("environment = %q, want production", got)
	}
	t.Setenv("PINTEREST_USE_SANDBOX", "true")
	if got := PinterestEnvironment(); got != "sandbox" {
		t.Fatalf("environment = %q, want sandbox", got)
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

func TestPinterestExchangeCodeStoresReturnedScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token":  "pin-access",
				"refresh_token": "pin-refresh",
				"expires_in":    3600,
				"scope":         "boards:read pins:read pins:write user_accounts:read",
			})
		case "/user_account":
			if got := r.Header.Get("Authorization"); got != "Bearer pin-access" {
				t.Fatalf("Authorization = %q", got)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":           "pin-user-123",
				"username":     "bugfreev587",
				"account_type": "BUSINESS",
			})
		default:
			http.Error(w, "unexpected path", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	t.Setenv("PINTEREST_API_BASE_URL", srv.URL)
	adapter := &PinterestAdapter{client: srv.Client()}
	result, err := adapter.ExchangeCode(context.Background(), OAuthConfig{
		ClientID:     "pin-client",
		ClientSecret: "pin-secret",
		TokenURL:     srv.URL + "/oauth/token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/pinterest",
	}, "code-123")
	if err != nil {
		t.Fatalf("ExchangeCode failed: %v", err)
	}
	want := []string{"boards:read", "pins:read", "pins:write", "user_accounts:read"}
	if len(result.Scopes) != len(want) {
		t.Fatalf("scopes = %#v, want %#v", result.Scopes, want)
	}
	for i := range want {
		if result.Scopes[i] != want[i] {
			t.Fatalf("scopes = %#v, want %#v", result.Scopes, want)
		}
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
		"board_id": "1151725373397121465",
		"title":    "Hello Pinterest",
	})
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	if gotMediaURL != staged {
		t.Fatalf("media_source.url = %q, want %q", gotMediaURL, staged)
	}
}

func TestPinterestPostStagesKnownTemporaryFileHosts(t *testing.T) {
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
				t.Fatalf("expected temporary file host URL to be staged, got %q", sourceURL)
			}
			return staged, nil
		},
	}

	_, err := adapter.Post(context.Background(), "token-123", "caption", []MediaItem{{
		URL:  "https://tmpfiles.org/wAwy63vn7vOa/20260605_071425_cover.jpg",
		Kind: MediaKindImage,
	}}, map[string]any{
		"board_id": "1151725373397121465",
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

func TestPinterestProviderFailureNormalizesKnownResponses(t *testing.T) {
	tests := []struct {
		name          string
		operation     string
		status        int
		body          string
		stage         string
		wantMessage   string
		wantErrorCode string
		wantCode      string
		wantReason    string
		wantTransient bool
		wantRetriable bool
	}{
		{
			name: "invalid token", operation: "user account", status: 401,
			body: `{"code":2,"message":"Authentication failed."}`, stage: "destination_preflight",
			wantMessage:   "Pinterest rejected this connection. Reconnect the account, then try again.",
			wantErrorCode: "auth_token_invalid", wantCode: "2", wantReason: "token_invalid",
		},
		{
			name: "board inaccessible", operation: "board preflight", status: 403,
			body: `{"code":29,"message":"You are not permitted to access that resource."}`, stage: "destination_preflight",
			wantMessage:   "The selected Pinterest board is unavailable for this connected account.",
			wantErrorCode: "target_not_found", wantCode: "29", wantReason: "board_not_accessible",
		},
		{
			name: "board missing", operation: "board preflight", status: 404,
			body: `{"code":40,"message":"Board not found."}`, stage: "destination_preflight",
			wantMessage:   "The selected Pinterest board is unavailable for this connected account.",
			wantErrorCode: "target_not_found", wantCode: "40", wantReason: "board_not_found",
		},
		{
			name: "rate limit", operation: "board preflight", status: 429,
			body: `{"code":8,"message":"Rate limit."}`, stage: "destination_preflight",
			wantMessage:   "Pinterest is rate limiting this request. Try again later.",
			wantErrorCode: "rate_limit", wantCode: "8", wantReason: "rate_limited",
			wantTransient: true, wantRetriable: true,
		},
		{
			name: "provider temporary", operation: "board preflight", status: 503,
			body: `{"code":1,"message":"database host db.internal unavailable; token=secret"}`, stage: "destination_preflight",
			wantMessage:   "Pinterest is temporarily unavailable. Try again later.",
			wantErrorCode: "temporary_platform_error", wantCode: "1", wantReason: "provider_temporary_failure",
			wantTransient: true, wantRetriable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pinterestProviderFailure(tt.operation, tt.status, []byte(tt.body), tt.stage)
			if err.Error() != tt.wantMessage {
				t.Fatalf("message = %q, want %q", err.Error(), tt.wantMessage)
			}
			if strings.Contains(err.Error(), "db.internal") || strings.Contains(err.Error(), "secret") {
				t.Fatalf("public error leaked provider body: %q", err)
			}

			providerCarrier, ok := err.(interface{ ProviderErrorFields() map[string]any })
			if !ok {
				t.Fatalf("error does not carry provider fields: %T", err)
			}
			provider := providerCarrier.ProviderErrorFields()
			if provider["provider"] != "pinterest" || provider["http_status"] != tt.status || provider["code"] != tt.wantCode || provider["reason"] != tt.wantReason || provider["is_transient"] != tt.wantTransient {
				t.Fatalf("provider fields = %#v", provider)
			}

			failureCarrier, ok := err.(interface{ FailureContractFields() map[string]any })
			if !ok {
				t.Fatalf("error does not carry failure fields: %T", err)
			}
			failure := failureCarrier.FailureContractFields()
			if failure["error_code"] != tt.wantErrorCode || failure["failure_stage"] != tt.stage || failure["is_retriable"] != tt.wantRetriable {
				t.Fatalf("failure fields = %#v", failure)
			}
		})
	}
}

func TestPinterestTransportFailureIsTemporaryAndSanitized(t *testing.T) {
	err := pinterestTransportFailure("board preflight", context.DeadlineExceeded, "destination_preflight")
	if err.Error() != "Pinterest is temporarily unavailable. Try again later." {
		t.Fatalf("message = %q", err.Error())
	}
	if strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("public error leaked transport detail: %q", err)
	}
	provider := err.(interface{ ProviderErrorFields() map[string]any }).ProviderErrorFields()
	if provider["provider"] != "pinterest" || provider["reason"] != "provider_temporary_failure" || provider["is_transient"] != true {
		t.Fatalf("provider fields = %#v", provider)
	}
	failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
	if failure["error_code"] != "temporary_platform_error" || failure["failure_stage"] != "destination_preflight" || failure["is_retriable"] != true {
		t.Fatalf("failure fields = %#v", failure)
	}
}

func TestProviderOnlyErrorDoesNotClaimFailureContract(t *testing.T) {
	err := newProviderError("provider temporary failure", map[string]any{
		"provider": "example",
	})
	if _, ok := err.(interface{ FailureContractFields() map[string]any }); ok {
		t.Fatalf("legacy provider-only error unexpectedly implements failure contract: %T", err)
	}
}

func TestPinterestPostRejectsStoredEnvironmentMismatchBeforeProviderCall(t *testing.T) {
	providerCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		providerCalls++
		http.Error(w, "must not be called", http.StatusInternalServerError)
	}))
	defer srv.Close()

	t.Setenv("PINTEREST_USE_SANDBOX", "")
	t.Setenv("PINTEREST_API_BASE_URL", srv.URL+"/v5")
	adapter := &PinterestAdapter{client: srv.Client()}
	ctx := WithDispatchMetadata(context.Background(), DispatchMetadata{
		SocialAccountID: "sa_pin_1",
		Environment:     "sandbox",
	})

	_, err := adapter.Post(ctx, "token", "caption", []MediaItem{{URL: "https://cdn.example.com/image.jpg", Kind: MediaKindImage}}, map[string]any{
		"board_id": "1131529543818288706",
	})
	if err == nil {
		t.Fatal("expected environment mismatch")
	}
	if providerCalls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerCalls)
	}
	failure := err.(interface{ FailureContractFields() map[string]any }).FailureContractFields()
	if failure["error_code"] != "target_not_found" || failure["failure_stage"] != "destination_preflight" || failure["is_retriable"] != false {
		t.Fatalf("failure fields = %#v", failure)
	}
	provider := err.(interface{ ProviderErrorFields() map[string]any }).ProviderErrorFields()
	if provider["reason"] != "board_environment_mismatch" {
		t.Fatalf("provider fields = %#v", provider)
	}
}
