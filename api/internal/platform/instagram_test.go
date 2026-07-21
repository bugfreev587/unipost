package platform

import (
	"context"
	"fmt"
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

func TestInstagramExchangeCodePersistsWebhookAccountID(t *testing.T) {
	transport := &instagramSequenceTransport{responses: []instagramHTTPResponse{
		{status: http.StatusOK, body: `{"access_token":"SHORT-AT","user_id":12345}`},
		{status: http.StatusOK, body: `{"access_token":"LONG-AT","token_type":"bearer","expires_in":5184000}`},
		{status: http.StatusOK, body: `{"id":"app-scoped-99","user_id":"professional-42","username":"shipper","profile_picture_url":"https://example.com/p.jpg"}`},
	}}
	adapter := &InstagramAdapter{client: &http.Client{Transport: transport}}
	config := adapter.DefaultOAuthConfig("https://api.example.com")
	config.ClientID = "ig-client"
	config.ClientSecret = "ig-secret"
	config.TokenURL = "https://api.instagram.test/oauth/access_token"

	result, err := adapter.ExchangeCode(context.Background(), config, "auth-code")
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if len(transport.requests) != 3 {
		t.Fatalf("request count = %d, want 3", len(transport.requests))
	}
	if got, want := transport.requests[2].URL.Query().Get("fields"), "id,user_id,username,profile_picture_url"; got != want {
		t.Fatalf("profile fields = %q, want %q", got, want)
	}
	if result.ExternalAccountID != "app-scoped-99" {
		t.Fatalf("external account id = %q", result.ExternalAccountID)
	}
	if got := result.Metadata["ig_user_id"]; got != "app-scoped-99" {
		t.Fatalf("ig_user_id metadata = %#v", got)
	}
	if got := result.Metadata["instagram_webhook_user_id"]; got != "professional-42" {
		t.Fatalf("instagram_webhook_user_id metadata = %#v", got)
	}
}

func TestInstagramExchangeCodeMissingWebhookUserIDFails(t *testing.T) {
	transport := &instagramSequenceTransport{responses: []instagramHTTPResponse{
		{status: http.StatusOK, body: `{"access_token":"SHORT-AT","user_id":12345}`},
		{status: http.StatusOK, body: `{"access_token":"LONG-AT","token_type":"bearer","expires_in":5184000}`},
		{status: http.StatusOK, body: `{"id":"app-scoped-99","username":"shipper"}`},
	}}
	adapter := &InstagramAdapter{client: &http.Client{Transport: transport}}
	config := adapter.DefaultOAuthConfig("https://api.example.com")
	config.ClientID = "ig-client"
	config.ClientSecret = "ig-secret"
	config.TokenURL = "https://api.instagram.test/oauth/access_token"

	_, err := adapter.ExchangeCode(context.Background(), config, "auth-code")
	if err == nil {
		t.Fatal("expected missing user_id error")
	}
	if !strings.Contains(err.Error(), "user_id") {
		t.Fatalf("error = %q, want missing user_id diagnostic", err)
	}
}

func TestInstagramGetProfileErrorsDoNotLeakAccessToken(t *testing.T) {
	const accessToken = "secret_ig_profile_token"
	for _, tc := range []struct {
		name     string
		response instagramHTTPResponse
		want     string
	}{
		{
			name:     "transport error",
			response: instagramHTTPResponse{err: fmt.Errorf("transport echoed access_token=%s", accessToken)},
			want:     "instagram profile request failed",
		},
		{
			name:     "non-200 response",
			response: instagramHTTPResponse{status: http.StatusUnauthorized, body: `{"error":"access_token=secret_ig_profile_token"}`},
			want:     "instagram profile request failed (401)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
				responses: []instagramHTTPResponse{tc.response},
			}}}

			_, err := adapter.getProfile(context.Background(), accessToken)
			if err == nil {
				t.Fatal("expected profile error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want %q", err, tc.want)
			}
			if strings.Contains(err.Error(), accessToken) || strings.Contains(strings.ToLower(err.Error()), "access_token=") {
				t.Fatalf("error leaked access token: %q", err)
			}
		})
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

func TestInstagramGetIGUserIDErrorIncludesMetaBodyWithoutToken(t *testing.T) {
	const token = "secret_ig_token"
	adapter := &InstagramAdapter{client: &http.Client{Transport: &instagramSequenceTransport{
		responses: []instagramHTTPResponse{{
			status: http.StatusBadRequest,
			body:   `{"error":{"message":"The user must log in again.","type":"OAuthException","code":190,"error_subcode":460}}`,
		}},
	}}}

	_, err := adapter.getIGUserID(context.Background(), token)
	if err == nil {
		t.Fatal("expected get user id error")
	}
	got := err.Error()
	for _, want := range []string{
		"instagram get user id failed (400)",
		`"code":190`,
		`"error_subcode":460`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("error = %q, want to contain %q", got, want)
		}
	}
	if strings.Contains(got, token) || strings.Contains(strings.ToLower(got), "access_token=") {
		t.Fatalf("error leaked request token details: %q", got)
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
	err    error
}

type instagramSequenceTransport struct {
	responses []instagramHTTPResponse
	requests  []*http.Request
}

func (t *instagramSequenceTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.requests = append(t.requests, req)
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
	if resp.err != nil {
		return nil, resp.err
	}
	return &http.Response{
		StatusCode: resp.status,
		Body:       io.NopCloser(strings.NewReader(resp.body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}
