package platform

import (
	"context"
	"io"
	"net/http"
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
