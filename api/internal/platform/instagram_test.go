package platform

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestInstagramAuthURLUsesBusinessLoginContract(t *testing.T) {
	adapter := NewInstagramAdapter()
	config := adapter.DefaultOAuthConfig("https://api.unipost.dev")
	config.ClientID = "ig-client"

	got := adapter.GetAuthURL(config, "state-xyz")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse auth url: %v", err)
	}

	if u.Scheme != "https" || u.Host != "www.instagram.com" || u.Path != "/oauth/authorize" {
		t.Fatalf("auth endpoint = %s, want https://www.instagram.com/oauth/authorize", u.String())
	}

	q := u.Query()
	if q.Get("client_id") != "ig-client" || q.Get("response_type") != "code" || q.Get("state") != "state-xyz" {
		t.Fatalf("missing required params: %v", q)
	}
	if q.Get("redirect_uri") != "https://api.unipost.dev/v1/oauth/callback/instagram" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("enable_fb_login") != "0" {
		t.Fatalf("enable_fb_login = %q, want 0", q.Get("enable_fb_login"))
	}

	scope := q.Get("scope")
	if strings.Contains(scope, " ") {
		t.Fatalf("scope must be comma-separated for Instagram Business Login, got %q", scope)
	}
	want := strings.Join(config.Scopes, ",")
	if scope != want {
		t.Fatalf("scope = %q, want %q", scope, want)
	}
}

func TestInstagramWaitForContainerErrorIncludesDiagnostics(t *testing.T) {
	withInstagramPollConfig(t, 3, 0)
	adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
		responses: []instagramHTTPResponse{{
			status: http.StatusOK,
			body:   `{"status_code":"ERROR"}`,
		}},
	}}}

	err := adapter.waitForContainer(context.Background(), "ig-token", "container_123")
	if err == nil {
		t.Fatal("expected container error")
	}
	got := err.Error()
	for _, want := range []string{
		"instagram container processing failed",
		"container_id=container_123",
		"status_code=ERROR",
		"poll_count=1",
		"elapsed_ms=",
		`response_body={"status_code":"ERROR"}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
}

func TestInstagramWaitForContainerTimeoutIncludesLastObservedStatus(t *testing.T) {
	withInstagramPollConfig(t, 2, 0)
	adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
		responses: []instagramHTTPResponse{
			{status: http.StatusOK, body: `{"status_code":"IN_PROGRESS"}`},
			{status: http.StatusOK, body: `{"status_code":"IN_PROGRESS"}`},
		},
	}}}

	err := adapter.waitForContainer(context.Background(), "ig-token", "container_456")
	if err == nil {
		t.Fatal("expected timeout")
	}
	got := err.Error()
	for _, want := range []string{
		"instagram container processing timed out",
		"container_id=container_456",
		"status_code=IN_PROGRESS",
		"poll_count=2",
		"last_http_status=200",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
}

func TestInstagramWaitForContainerHTTPFailureIsDiagnosable(t *testing.T) {
	withInstagramPollConfig(t, 1, 0)
	adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
		responses: []instagramHTTPResponse{{
			status: http.StatusInternalServerError,
			body:   `{"error":{"message":"upstream unavailable"}}`,
		}},
	}}}

	err := adapter.waitForContainer(context.Background(), "ig-token", "container_789")
	if err == nil {
		t.Fatal("expected timeout with HTTP failure diagnostics")
	}
	got := err.Error()
	for _, want := range []string{
		"instagram container processing timed out",
		"container_id=container_789",
		"last_http_status=500",
		`response_body={"error":{"message":"upstream unavailable"}}`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
}

func TestInstagramWaitForContainerDecodeErrorIsDiagnosable(t *testing.T) {
	withInstagramPollConfig(t, 1, 0)
	adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
		responses: []instagramHTTPResponse{{
			status: http.StatusOK,
			body:   `{"status_code":`,
		}},
	}}}

	err := adapter.waitForContainer(context.Background(), "ig-token", "container_bad_json")
	if err == nil {
		t.Fatal("expected decode error")
	}
	got := err.Error()
	for _, want := range []string{
		"instagram container poll decode failed",
		"container_id=container_bad_json",
		"poll_count=1",
		`response_body={"status_code":`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
}

func withInstagramPollConfig(t *testing.T, attempts int, interval time.Duration) {
	t.Helper()
	oldAttempts := instagramContainerPollAttempts
	oldInterval := instagramContainerPollInterval
	instagramContainerPollAttempts = attempts
	instagramContainerPollInterval = interval
	t.Cleanup(func() {
		instagramContainerPollAttempts = oldAttempts
		instagramContainerPollInterval = oldInterval
	})
}

type instagramHTTPResponse struct {
	status int
	body   string
}

type instagramSequenceTransport struct {
	responses []instagramHTTPResponse
}

func (t *instagramSequenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if len(t.responses) == 0 {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status_code":"IN_PROGRESS"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	}
	resp := t.responses[0]
	if len(t.responses) > 1 {
		t.responses = t.responses[1:]
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}
