package platform

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestThreadsGetUserIDNon2XXIncludesStatusBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{
			name:   "invalid token",
			status: http.StatusUnauthorized,
			body:   `{"error":{"message":"Invalid OAuth access token"}}`,
			want:   `threads get user id failed (401): {"error":{"message":"Invalid OAuth access token"}}`,
		},
		{
			name:   "missing permission",
			status: http.StatusForbidden,
			body:   `{"error":{"message":"Missing required permission threads_basic"}}`,
			want:   `threads get user id failed (403): {"error":{"message":"Missing required permission threads_basic"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := newThreadsAdapterForUserIDTest(tt.status, tt.body)

			_, err := adapter.getUserID(context.Background(), "threads-token")
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Fatalf("error = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestThreadsGetUserIDDecodeError(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":`)

	_, err := adapter.getUserID(context.Background(), "threads-token")
	if err == nil {
		t.Fatal("expected decode error")
	}
	if got := err.Error(); !strings.Contains(got, "threads get user id decode") {
		t.Fatalf("error = %q, want decode context", got)
	}
}

func TestThreadsGetUserIDEmptyID(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":""}`)

	_, err := adapter.getUserID(context.Background(), "threads-token")
	if err == nil {
		t.Fatal("expected empty id error")
	}
	if got := err.Error(); !strings.Contains(got, `threads get user id: empty id`) {
		t.Fatalf("error = %q, want empty-id context", got)
	}
}

func TestThreadsGetUserIDSuccess(t *testing.T) {
	adapter := newThreadsAdapterForUserIDTest(http.StatusOK, `{"id":"th_123"}`)

	got, err := adapter.getUserID(context.Background(), "threads-token")
	if err != nil {
		t.Fatalf("getUserID: %v", err)
	}
	if got != "th_123" {
		t.Fatalf("id = %q, want th_123", got)
	}
}

func TestThreadsExchangeCodeFailsWhenLongLivedTokenSwapFails(t *testing.T) {
	adapter := &ThreadsAdapter{client: &http.Client{Transport: routeResponseTransport{
		routes: map[string]routeResponse{
			"POST graph.threads.net/oauth/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"short-token","token_type":"bearer","user_id":"th_123"}`,
			},
			"GET graph.threads.net/access_token": {
				status: http.StatusBadRequest,
				body:   `{"error":{"message":"Cannot exchange token","type":"OAuthException"}}`,
			},
		},
	}}}

	_, err := adapter.ExchangeCode(context.Background(), OAuthConfig{
		ClientID:     "threads-client",
		ClientSecret: "threads-secret",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/threads",
	}, "oauth-code")
	if err == nil {
		t.Fatal("expected long-lived token exchange failure")
	}
	if got := err.Error(); !strings.Contains(got, "threads long-lived token exchange failed") {
		t.Fatalf("error = %q, want long-lived exchange context", got)
	}
}

func TestThreadsExchangeCodeRejectsShortLivedLongLivedToken(t *testing.T) {
	adapter := &ThreadsAdapter{client: &http.Client{Transport: routeResponseTransport{
		routes: map[string]routeResponse{
			"POST graph.threads.net/oauth/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"short-token","token_type":"bearer","user_id":"th_123"}`,
			},
			"GET graph.threads.net/access_token": {
				status: http.StatusOK,
				body:   `{"access_token":"still-short-token","expires_in":3600}`,
			},
		},
	}}}

	_, err := adapter.ExchangeCode(context.Background(), OAuthConfig{
		ClientID:     "threads-client",
		ClientSecret: "threads-secret",
		TokenURL:     "https://graph.threads.net/oauth/access_token",
		RedirectURL:  "https://api.example.com/v1/oauth/callback/threads",
	}, "oauth-code")
	if err == nil {
		t.Fatal("expected short-lived token rejection")
	}
	if got := err.Error(); !strings.Contains(got, "short-lived Threads token") {
		t.Fatalf("error = %q, want short-lived token context", got)
	}
}

func newThreadsAdapterForUserIDTest(status int, body string) *ThreadsAdapter {
	return &ThreadsAdapter{client: &http.Client{Transport: staticResponseTransport{
		status: status,
		body:   body,
	}}}
}

type staticResponseTransport struct {
	status int
	body   string
}

func (t staticResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type routeResponse struct {
	status int
	body   string
}

type routeResponseTransport struct {
	routes map[string]routeResponse
}

func (t routeResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	key := req.Method + " " + req.URL.Host + req.URL.Path
	if req.Method == http.MethodPost {
		_ = req.ParseForm()
		req.URL.RawQuery = url.Values(req.Form).Encode()
	}
	resp, ok := t.routes[key]
	if !ok {
		resp = routeResponse{status: http.StatusNotFound, body: `{"error":"missing test route"}`}
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}
