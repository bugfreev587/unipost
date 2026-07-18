package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/cors"
)

func TestAPICORSAllowsIdempotentXInboxReplyAndExposesOperationID(t *testing.T) {
	handler := cors.Handler(apiCORSOptions())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-UniPost-Operation-Id", "operation_123")
		w.WriteHeader(http.StatusAccepted)
	}))

	preflight := httptest.NewRequest(http.MethodOptions, "/v1/inbox/item_123/reply", nil)
	preflight.Header.Set("Origin", "https://dev-app.unipost.dev")
	preflight.Header.Set("Access-Control-Request-Method", http.MethodPost)
	preflight.Header.Set("Access-Control-Request-Headers", "Idempotency-Key, Content-Type")
	preflightResponse := httptest.NewRecorder()
	handler.ServeHTTP(preflightResponse, preflight)
	if preflightResponse.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", preflightResponse.Code)
	}
	if got := preflightResponse.Header().Get("Access-Control-Allow-Origin"); got != "https://dev-app.unipost.dev" {
		t.Fatalf("Access-Control-Allow-Origin = %q", got)
	}
	if got := strings.ToLower(preflightResponse.Header().Get("Access-Control-Allow-Headers")); !strings.Contains(got, "idempotency-key") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want Idempotency-Key", got)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/inbox/item_123/reply", strings.NewReader(`{"text":"hello"}`))
	request.Header.Set("Origin", "https://dev-app.unipost.dev")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "reply_key_123")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("response status = %d, want 202", response.Code)
	}
	if got := strings.ToLower(response.Header().Get("Access-Control-Expose-Headers")); !strings.Contains(got, "x-unipost-operation-id") {
		t.Fatalf("Access-Control-Expose-Headers = %q, want X-UniPost-Operation-Id", got)
	}
}

func TestAPICORSAllowsConfiguredVercelPreviewOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://*.vercel.app")

	handler := cors.Handler(apiCORSOptions())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	origin := "https://unipost-dev-git-dev-example-xiaobo-yus-projects.vercel.app"
	preflight := httptest.NewRequest(http.MethodOptions, "/health", nil)
	preflight.Header.Set("Origin", origin)
	preflight.Header.Set("Access-Control-Request-Method", http.MethodGet)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, preflight)

	if response.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200", response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
}
