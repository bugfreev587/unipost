package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestRevokeYouTubeOAuthTokenPostsTokenToGoogle(t *testing.T) {
	var gotRequest *http.Request
	var gotBody string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotRequest = r
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
			Request:    r,
		}, nil
	})}

	previous := googleOAuthRevokeEndpoint
	googleOAuthRevokeEndpoint = "https://oauth2.googleapis.test/revoke"
	t.Cleanup(func() { googleOAuthRevokeEndpoint = previous })

	if err := revokeYouTubeOAuthToken(context.Background(), client, "refresh-token-123"); err != nil {
		t.Fatalf("revokeYouTubeOAuthToken returned error: %v", err)
	}

	if gotRequest == nil {
		t.Fatal("no request captured")
	}
	if gotRequest.Method != http.MethodPost {
		t.Fatalf("method = %s, want POST", gotRequest.Method)
	}
	if gotRequest.URL.String() != googleOAuthRevokeEndpoint {
		t.Fatalf("url = %s, want revoke endpoint", gotRequest.URL.String())
	}
	if gotBody != "token=refresh-token-123" {
		t.Fatalf("body = %q, want token form body", gotBody)
	}
}
