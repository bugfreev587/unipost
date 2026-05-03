package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
