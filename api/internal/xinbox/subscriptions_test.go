package xinbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

type trackingReadCloser struct {
	reader    io.Reader
	bytesRead int
	closed    bool
}

func (b *trackingReadCloser) Read(p []byte) (int, error) {
	n, err := b.reader.Read(p)
	b.bytesRead += n
	return n, err
}

func (b *trackingReadCloser) Close() error {
	b.closed = true
	return nil
}

func TestProviderHTTPErrorIsStatusAwareAndSecretSafe(t *testing.T) {
	const (
		bearerToken    = "super-secret-bearer"
		typeSecret     = "bearer-sentinel-must-stay-private"
		titleSecret    = "query-sentinel-must-stay-private"
		detailSecret   = "dm-body-sentinel-must-stay-private"
		bodySecret     = "raw response body must stay private"
		secondErrorRaw = "ignored-second-error"
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+bearerToken {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/2/activity/subscriptions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprintf(
			w,
			`{"errors":[{"type":%q,"title":%q,"status":403,"detail":%q},{"code":%q,"title":"Ignored","status":429}],"debug":%q}`,
			typeSecret,
			titleSecret,
			detailSecret,
			secondErrorRaw,
			bodySecret,
		)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := client.ListActivitySubscriptions(context.Background(), bearerToken)
	if err == nil {
		t.Fatal("expected provider HTTP error")
	}
	var providerErr *ProviderHTTPError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want *ProviderHTTPError: %v", err, err)
	}
	if providerErr.Method != http.MethodGet || providerErr.Path != "/2/activity/subscriptions" ||
		providerErr.StatusCode != http.StatusForbidden {
		t.Fatalf("provider error = %+v", providerErr)
	}
	if !strings.HasPrefix(providerErr.Code, "provider_code_") || len(providerErr.Code) != len("provider_code_")+12 {
		t.Fatalf("provider error code = %q, want safe non-empty classification", providerErr.Code)
	}
	if providerErr.Title != "provider_error" {
		t.Fatalf("provider error title = %q, want safe non-empty classification", providerErr.Title)
	}
	if !IsProviderHTTPStatus(fmt.Errorf("wrapped: %w", err), http.StatusForbidden) {
		t.Fatal("wrapped provider error did not match HTTP 403")
	}
	message := err.Error()
	for _, want := range []string{http.MethodGet, "/2/activity/subscriptions", "403", providerErr.Code, providerErr.Title} {
		if !strings.Contains(message, want) {
			t.Errorf("error %q does not contain %q", message, want)
		}
	}
	for _, forbidden := range []string{
		"Authorization",
		"Bearer",
		bearerToken,
		typeSecret,
		titleSecret,
		detailSecret,
		bodySecret,
		"max_results=",
		secondErrorRaw,
	} {
		if strings.Contains(message, forbidden) || strings.Contains(providerErr.Code, forbidden) ||
			strings.Contains(providerErr.Title, forbidden) {
			t.Errorf("provider error %+v / %q leaked %q", providerErr, message, forbidden)
		}
	}
}

func TestProviderHTTPErrorRawCodeIsClassifiedWithoutLeaking(t *testing.T) {
	const codeSecret = "provider-code-must-stay-private"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"code":%q}`, codeSecret)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	err := client.doJSON(context.Background(), http.MethodGet, "/2/webhooks", "app-token", nil, nil)
	var providerErr *ProviderHTTPError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want *ProviderHTTPError: %v", err, err)
	}
	if !strings.HasPrefix(providerErr.Code, "provider_code_") || strings.Contains(providerErr.Code, codeSecret) {
		t.Fatalf("provider error code = %q, want safe classification", providerErr.Code)
	}
	if strings.Contains(err.Error(), codeSecret) {
		t.Fatalf("error %q leaked raw provider code", err)
	}
}

func TestProviderTransportErrorIsSecretSafeAndPreservesCause(t *testing.T) {
	const (
		secretURL   = "https://api.x.test/2/webhooks?access_token=secret-query-token"
		bearerToken = "secret-bearer-token"
	)
	sentinel := errors.New("transport sentinel")
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "direct error contains URL",
			err:  fmt.Errorf("dial failed for %s: %w", secretURL, sentinel),
		},
		{
			name: "wrapped URL error",
			err: fmt.Errorf("outer transport failure: %w", &url.Error{
				Op:  "round trip",
				URL: secretURL,
				Err: sentinel,
			}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(ClientConfig{
				BaseURL: "https://api.x.test",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					return nil, tt.err
				})},
			})

			err := client.doJSON(
				context.Background(),
				http.MethodGet,
				"/2/webhooks?cursor=request-query-secret",
				bearerToken,
				nil,
				nil,
			)
			if !errors.Is(err, sentinel) {
				t.Fatalf("err = %v, want wrapped transport sentinel", err)
			}
			if unwrapped := errors.Unwrap(err); unwrapped != nil {
				t.Fatalf("errors.Unwrap(err) = %T, want nil", unwrapped)
			}
			var urlErr *url.Error
			if errors.As(err, &urlErr) {
				t.Fatalf("errors.As exposed raw URL error: %T", urlErr)
			}
			formatted := []string{
				err.Error(),
				fmt.Sprintf("%v", err),
				fmt.Sprintf("%+v", err),
				fmt.Sprintf("%#v", err),
				fmt.Sprintf("%q", err),
			}
			message := formatted[0]
			for _, want := range []string{http.MethodGet, "/2/webhooks", "request failed"} {
				if !strings.Contains(message, want) {
					t.Errorf("error %q does not contain %q", message, want)
				}
			}
			for _, rendered := range formatted {
				for _, forbidden := range []string{
					secretURL,
					"secret-query-token",
					"request-query-secret",
					bearerToken,
					tt.err.Error(),
					sentinel.Error(),
				} {
					if strings.Contains(rendered, forbidden) {
						t.Errorf("formatted error %q leaked %q", rendered, forbidden)
					}
				}
			}
		})
	}
}

func TestProviderHTTPErrorUsesSafeFallbackForUnclassifiedBodies(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		forbidden  string
		bodyStatus int
	}{
		{
			name:       "only detail",
			body:       `{"detail":"dm-body-secret-must-stay-private"}`,
			forbidden:  "dm-body-secret-must-stay-private",
			bodyStatus: http.StatusForbidden,
		},
		{name: "empty", bodyStatus: http.StatusForbidden},
		{
			name:       "invalid",
			body:       `not-json-secret-must-stay-private`,
			forbidden:  "not-json-secret-must-stay-private",
			bodyStatus: http.StatusForbidden,
		},
		{
			name:       "truncated",
			body:       `{"code":"truncated-secret-must-stay-private`,
			forbidden:  "truncated-secret-must-stay-private",
			bodyStatus: http.StatusForbidden,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.bodyStatus)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.doJSON(context.Background(), http.MethodGet, "/2/webhooks", "app-token", nil, nil)
			var providerErr *ProviderHTTPError
			if !errors.As(err, &providerErr) {
				t.Fatalf("error type = %T, want *ProviderHTTPError: %v", err, err)
			}
			if providerErr.Code != "provider_http_403" || providerErr.Title != "provider_error" {
				t.Fatalf("provider error = %+v, want fixed safe fallback", providerErr)
			}
			if tt.forbidden != "" && (strings.Contains(err.Error(), tt.forbidden) ||
				strings.Contains(providerErr.Code, tt.forbidden) || strings.Contains(providerErr.Title, tt.forbidden)) {
				t.Fatalf("provider error %+v / %q leaked %q", providerErr, err, tt.forbidden)
			}
		})
	}
}

func TestFilteredStreamProviderHTTPErrorIsStatusAwareAndSecretSafe(t *testing.T) {
	const (
		bearerToken = "stream-secret-bearer"
		detail      = "stream provider detail must stay private"
		bodySecret  = "stream raw body must stay private"
	)
	payload := fmt.Sprintf(
		`{"errors":[{"code":"stream-forbidden","title":"Forbidden","status":403,"detail":%q},{"code":"ignored-stream-error","title":"Ignored"}],"debug":%q}`,
		detail,
		bodySecret,
	)
	body := &trackingReadCloser{reader: strings.NewReader(payload)}
	client := NewClient(ClientConfig{
		BaseURL: "https://api.x.test",
		StreamHTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if got := request.Header.Get("Authorization"); got != "Bearer "+bearerToken {
				t.Fatalf("Authorization = %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusForbidden,
				Header:     make(http.Header),
				Body:       body,
			}, nil
		})},
	})

	err := client.ConsumeFilteredStream(context.Background(), bearerToken, func(StreamEvent) error {
		t.Fatal("stream handler must not run for provider HTTP failure")
		return nil
	})
	var providerErr *ProviderHTTPError
	if !errors.As(err, &providerErr) {
		t.Fatalf("error type = %T, want *ProviderHTTPError: %v", err, err)
	}
	if providerErr.Method != http.MethodGet || providerErr.Path != "/2/tweets/search/stream" ||
		providerErr.StatusCode != http.StatusForbidden ||
		!strings.HasPrefix(providerErr.Code, "provider_code_") || providerErr.Title != "provider_error" {
		t.Fatalf("provider error = %+v", providerErr)
	}
	if !IsProviderHTTPStatus(err, http.StatusForbidden) {
		t.Fatal("stream provider error did not match HTTP 403")
	}
	message := err.Error()
	for _, forbidden := range []string{
		"tweet.fields",
		"expansions=",
		bearerToken,
		detail,
		bodySecret,
		"ignored-stream-error",
	} {
		if strings.Contains(message, forbidden) {
			t.Errorf("error %q leaked %q", message, forbidden)
		}
	}
	if !body.closed {
		t.Fatal("stream provider error response body was not closed")
	}
}

func TestProviderRequestErrorIsSecretSafe(t *testing.T) {
	const (
		secretQuery = "secret-query-value"
		bearerToken = "secret-bearer-value"
	)
	client := NewClient(ClientConfig{BaseURL: "https://api.x.test"})
	err := client.doJSON(
		context.Background(),
		http.MethodGet,
		"/2/webhooks?cursor="+secretQuery+"\n",
		bearerToken,
		nil,
		nil,
	)
	if err == nil {
		t.Fatal("expected request construction error")
	}
	message := err.Error()
	for _, want := range []string{http.MethodGet, "/2/webhooks"} {
		if !strings.Contains(message, want) {
			t.Errorf("error %q does not contain %q", message, want)
		}
	}
	for _, forbidden := range []string{secretQuery, bearerToken, "cursor="} {
		if strings.Contains(message, forbidden) {
			t.Errorf("error %q leaked %q", message, forbidden)
		}
	}
}

func TestProviderHTTPErrorBodyIsBoundedAndClosed(t *testing.T) {
	const responseLimit = 64
	payload := `{"errors":[{"title":"` + strings.Repeat("x", 1024)
	body := &trackingReadCloser{reader: strings.NewReader(payload)}
	client := NewClient(ClientConfig{
		BaseURL:              "https://api.x.test",
		MaxJSONResponseBytes: responseLimit,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       body,
			}, nil
		})},
	})

	err := client.doJSON(context.Background(), http.MethodGet, "/2/webhooks", "app-token", nil, nil)
	var providerErr *ProviderHTTPError
	if !errors.As(err, &providerErr) || providerErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("err = %v, want provider HTTP 500", err)
	}
	if body.bytesRead > responseLimit || body.bytesRead >= len(payload) {
		t.Fatalf("bytes read = %d, want bounded at %d of %d", body.bytesRead, responseLimit, len(payload))
	}
	if !body.closed {
		t.Fatal("provider error response body was not closed")
	}
}

func TestIsProviderHTTPStatusHandlesTypedNil(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("IsProviderHTTPStatus panicked for typed nil: %v", recovered)
		}
	}()
	var providerErr *ProviderHTTPError
	if IsProviderHTTPStatus(providerErr, http.StatusNotFound) {
		t.Fatal("typed nil must not match a provider HTTP status")
	}
}

func TestXClientEnsureWebhookDiscoversConfiguredURL(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodGet || r.URL.Path != "/2/webhooks" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"webhook-1","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "webhook-1" || calls != 1 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookRevalidatesConfiguredInvalidWebhook(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-1","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 2:
			if r.Method != http.MethodPut || r.URL.Path != "/2/webhooks/webhook-1" {
				t.Fatalf("revalidate request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":{"attempted":true}}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-1","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 4:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-1","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		WebhookValidationPolls:   3,
		WebhookValidationBackoff: time.Millisecond,
		Sleep:                    func(context.Context, time.Duration) error { return nil },
	})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if !webhook.Valid || calls != 4 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookRetainsInvalidStateWhenCRCWasNotAttempted(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-1","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":{"attempted":false}}`))
		default:
			t.Fatalf("unexpected poll after attempted=false")
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err == nil || !strings.Contains(err.Error(), "was not attempted") {
		t.Fatalf("err = %v, want attempted=false error", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestXClientEnsureWebhookCreatesConfiguredURLWhenMissing(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			if r.Method != http.MethodPost || r.URL.Path != "/2/webhooks" {
				t.Fatalf("request = %s %s", r.Method, r.URL.Path)
			}
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body["url"] != "https://dev-api.unipost.dev/v1/webhooks/twitter" {
				t.Fatalf("url = %q", body["url"])
			}
			_, _ = w.Write([]byte(`{"id":"webhook-new","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "webhook-new" || calls != 2 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookAcceptsWrappedCreateResponseWithoutRetry(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":{"id":"webhook-wrapped","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}}`))
		default:
			t.Fatalf("unexpected duplicate create request %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "webhook-wrapped" || calls != 2 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookPollsUntilCreatedWebhookIsExactAndValid(t *testing.T) {
	const configuredURL = "https://dev-api.unipost.dev/v1/webhooks/twitter"
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"id":"webhook-new","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-new","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 4:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-new","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
		default:
			t.Fatalf("unexpected request %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		WebhookValidationPolls:   3,
		WebhookValidationBackoff: time.Millisecond,
		Sleep:                    func(context.Context, time.Duration) error { return nil },
	})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", configuredURL)
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "webhook-new" || webhook.URL != configuredURL || !webhook.Valid || calls != 4 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookRejectsCreatedWebhookWithWrongURLImmediately(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":{"id":"webhook-wrapped","url":"https://wrong.example/webhook","valid":true}}`))
		default:
			t.Fatalf("unexpected poll after wrong create URL")
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	_, err := client.EnsureWebhook(
		context.Background(),
		"app-token",
		"https://dev-api.unipost.dev/v1/webhooks/twitter",
	)
	if err == nil || !strings.Contains(err.Error(), "configured URL") {
		t.Fatalf("err = %v, want mismatched configured URL error", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want no validation polls", calls)
	}
}

func TestXClientEnsureWebhookTimesOutWhenCreatedWebhookIsNeverConfirmed(t *testing.T) {
	const configuredURL = "https://dev-api.unipost.dev/v1/webhooks/twitter"
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":{"id":"webhook-wrapped","valid":false}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[{"id":"webhook-wrapped","url":"https://wrong.example/webhook","valid":true}]}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{
		BaseURL:                  server.URL,
		HTTPClient:               server.Client(),
		WebhookValidationPolls:   2,
		WebhookValidationBackoff: time.Millisecond,
		Sleep:                    func(context.Context, time.Duration) error { return nil },
	})
	_, err := client.EnsureWebhook(context.Background(), "app-token", configuredURL)
	if err == nil || !strings.Contains(err.Error(), "did not become valid") {
		t.Fatalf("err = %v, want validation timeout", err)
	}
	if calls != 4 {
		t.Fatalf("calls = %d, want list, create, and two polls", calls)
	}
}

func TestXClientEnsureDMSubscriptionUsesAppBearerForListAndCreate(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
				t.Fatalf("list Authorization = %q, want app bearer", got)
			}
			if r.Method != http.MethodGet || r.URL.Path != "/2/activity/subscriptions" {
				t.Fatalf("list request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
				t.Fatalf("create Authorization = %q, want app bearer", got)
			}
			if r.Method != http.MethodPost || r.URL.Path != "/2/activity/subscriptions" {
				t.Fatalf("create request = %s %s", r.Method, r.URL.Path)
			}
			var body ActivitySubscription
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			want := ActivitySubscription{
				EventType: "dm.received",
				Filter:    ActivityFilter{UserID: "2244994945"},
				Tag:       "unipost:x:dm:account-123",
				WebhookID: "webhook-1",
			}
			if !reflect.DeepEqual(body, want) {
				t.Fatalf("body = %#v, want %#v", body, want)
			}
			_, _ = w.Write([]byte(`{"data":{"subscription":{"subscription_id":"subscription-1","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}}}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-1" || calls != 2 {
		t.Fatalf("subscription=%+v calls=%d", subscription, calls)
	}
}

func TestXClientEnsureDMSubscriptionReplacesStaleStableTag(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"subscription-old","event_type":"dm.received","filter":{"user_id":"old-user"},"tag":"unipost:x:dm:account-123","webhook_id":"old-webhook"}]}`))
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
				t.Fatalf("delete Authorization = %q, want app bearer", got)
			}
			if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/subscription-old" {
				t.Fatalf("delete request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":{"deleted":true}}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":{"subscription":{"subscription_id":"subscription-new","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}}}`))
		default:
			t.Fatalf("unexpected call %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-new" || calls != 3 {
		t.Fatalf("subscription=%+v calls=%d", subscription, calls)
	}
}

func TestXClientEnsureDMSubscriptionFindsStableTagOnSecondPage(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/2/activity/subscriptions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("max_results") != "1000" {
			t.Fatalf("max_results = %q", r.URL.Query().Get("max_results"))
		}
		switch calls {
		case 1:
			if got := r.URL.Query().Get("pagination_token"); got != "" {
				t.Fatalf("first pagination token = %q", got)
			}
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"other","event_type":"dm.received","filter":{"user_id":"another-user"},"tag":"another-tag","webhook_id":"webhook-1"}],"meta":{"next_token":"NEXTTOKEN1234567","result_count":1}}`))
		case 2:
			if got := r.URL.Query().Get("pagination_token"); got != "NEXTTOKEN1234567" {
				t.Fatalf("second pagination token = %q", got)
			}
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"subscription-page-2","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}],"meta":{"result_count":1}}`))
		default:
			t.Fatalf("unexpected request %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-page-2" || calls != 2 {
		t.Fatalf("subscription=%+v calls=%d", subscription, calls)
	}
}

func TestXClientEnsureDMSubscriptionFollowsFullFirstPageToItem1001(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet || r.URL.Path != "/2/activity/subscriptions" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		switch calls {
		case 1:
			page := make([]ActivitySubscription, 1000)
			for i := range page {
				page[i] = ActivitySubscription{
					ID:        fmt.Sprintf("other-%d", i),
					EventType: "dm.received",
					Filter:    ActivityFilter{UserID: "another-user"},
					Tag:       fmt.Sprintf("another-tag-%d", i),
					WebhookID: "webhook-1",
				}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": page,
				"meta": map[string]any{
					"next_token":   "NEXTTOKEN1234567",
					"result_count": 1000,
				},
			})
		case 2:
			if got := r.URL.Query().Get("pagination_token"); got != "NEXTTOKEN1234567" {
				t.Fatalf("second pagination token = %q", got)
			}
			if got := r.URL.Query().Get("max_results"); got != "500" {
				t.Fatalf("second max_results = %q, want remaining self-serve capacity 500", got)
			}
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"subscription-1001","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}],"meta":{"result_count":1}}`))
		default:
			t.Fatalf("unexpected request %d", calls)
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-1001" || calls != 2 {
		t.Fatalf("subscription=%+v calls=%d", subscription, calls)
	}
}

func TestXClientEnsureDMSubscriptionAcceptsDirectDataResponse(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":{"subscription_id":"subscription-direct","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-direct" {
		t.Fatalf("subscription = %+v", subscription)
	}
}

func TestXClientEnsureDMSubscriptionAcceptsArrayDataResponse(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"subscription-array","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"webhook-1"}]}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.EnsureDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"webhook-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "subscription-array" {
		t.Fatalf("subscription = %+v", subscription)
	}
}

func TestXClientDeleteActivitySubscriptionIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/subscription-missing" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	if err := client.DeleteActivitySubscription(context.Background(), "app-token", "subscription-missing"); err != nil {
		t.Fatal(err)
	}
}

func TestXClientDeleteActivitySubscriptionRequiresOfficialConfirmation(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "confirmed JSON", status: http.StatusOK, body: `{"data":{"deleted":true},"meta":{"total_subscriptions":0}}`},
		{name: "empty 200", status: http.StatusOK, wantErr: true},
		{name: "accepted", status: http.StatusAccepted, body: `{"data":{"deleted":true}}`, wantErr: true},
		{name: "no content", status: http.StatusNoContent, wantErr: true},
		{name: "deleted false", status: http.StatusOK, body: `{"data":{"deleted":false}}`, wantErr: true},
		{
			name:    "partial 200 error body",
			status:  http.StatusOK,
			body:    `{"data":{"deleted":true},"errors":[{"title":"Invalid Request","type":"https://api.x.com/2/problems/invalid-request","detail":"partial failure","status":400}]}`,
			wantErr: true,
		},
		{
			name:    "explicit already missing body",
			status:  http.StatusOK,
			body:    `{"errors":[{"resource_id":"subscription-1","title":"Not Found Error","type":"https://api.x.com/2/problems/resource-not-found","detail":"subscription missing","status":404}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
					t.Fatalf("Authorization = %q, want app bearer", got)
				}
				w.WriteHeader(tt.status)
				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteActivitySubscription(context.Background(), "app-token", "subscription-1")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXClientDeleteActivitySubscriptionResponseIsBoundedAndClosed(t *testing.T) {
	const responseLimit = 64
	payload := `{"data":{"deleted":true},"padding":"` + strings.Repeat("x", 1024)
	body := &trackingReadCloser{reader: strings.NewReader(payload)}
	client := NewClient(ClientConfig{
		BaseURL:              "https://api.x.test",
		MaxJSONResponseBytes: responseLimit,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       body,
			}, nil
		})},
	})

	err := client.DeleteActivitySubscription(context.Background(), "app-token", "subscription-1")
	if err == nil || !strings.Contains(err.Error(), "response exceeded") {
		t.Fatalf("err = %v, want bounded response error", err)
	}
	if body.bytesRead > responseLimit+1 || body.bytesRead >= len(payload) {
		t.Fatalf("bytes read = %d, want bounded near %d of %d", body.bytesRead, responseLimit, len(payload))
	}
	if !body.closed {
		t.Fatal("delete response body was not closed")
	}
}

func TestDeleteActivitySubscriptionIdempotentProviderStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/subscription-1" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":[{"code":"cleanup-denied","title":"Cleanup denied","status":403,"detail":"private provider detail"}]}`))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteActivitySubscription(context.Background(), "app-token", "subscription-1")
			if status == http.StatusForbidden {
				if err == nil || !IsProviderHTTPStatus(err, http.StatusForbidden) {
					t.Fatalf("err = %v, want provider HTTP 403", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want idempotent success", err)
			}
		})
	}
}

func TestDeleteProviderResourceIDValidation(t *testing.T) {
	deletes := []struct {
		name        string
		delete      func(*Client, string) error
		validStatus int
		validBody   string
	}{
		{
			name:        "webhook",
			validStatus: http.StatusNoContent,
			delete: func(client *Client, resourceID string) error {
				return client.DeleteWebhook(context.Background(), "app-token", resourceID)
			},
		},
		{
			name:        "activity subscription",
			validStatus: http.StatusOK,
			validBody:   `{"data":{"deleted":true}}`,
			delete: func(client *Client, resourceID string) error {
				return client.DeleteActivitySubscription(context.Background(), "app-token", resourceID)
			},
		},
	}
	invalidIDs := []string{"", " ", "\t", ".", "..", "provider/id", "provider?id", "provider#id"}
	for _, deleteCase := range deletes {
		t.Run(deleteCase.name, func(t *testing.T) {
			for _, resourceID := range invalidIDs {
				t.Run(fmt.Sprintf("invalid_%q", resourceID), func(t *testing.T) {
					calls := 0
					client := NewClient(ClientConfig{
						BaseURL: "https://api.x.test",
						HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
							calls++
							return &http.Response{
								StatusCode: http.StatusNoContent,
								Header:     make(http.Header),
								Body:       io.NopCloser(strings.NewReader("")),
							}, nil
						})},
					})
					if err := deleteCase.delete(client, resourceID); err == nil {
						t.Fatal("expected invalid provider resource ID error")
					}
					if calls != 0 {
						t.Fatalf("provider calls = %d, want 0", calls)
					}
				})
			}

			calls := 0
			client := NewClient(ClientConfig{
				BaseURL: "https://api.x.test",
				HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{
						StatusCode: deleteCase.validStatus,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(deleteCase.validBody)),
					}, nil
				})},
			})
			if err := deleteCase.delete(client, "provider-123_ABC"); err != nil {
				t.Fatalf("valid provider ID: %v", err)
			}
			if calls != 1 {
				t.Fatalf("provider calls = %d, want 1", calls)
			}
		})
	}
}

func TestDeleteWebhookIdempotentProviderStatuses(t *testing.T) {
	for _, status := range []int{
		http.StatusOK,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusNotFound,
		http.StatusGone,
		http.StatusForbidden,
	} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/2/webhooks/webhook-1" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":[{"code":"cleanup-denied","title":"Cleanup denied","status":403,"detail":"private provider detail"}]}`))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteWebhook(context.Background(), "app-token", "webhook-1")
			if status == http.StatusForbidden {
				if err == nil || !IsProviderHTTPStatus(err, http.StatusForbidden) {
					t.Fatalf("err = %v, want provider HTTP 403", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want success", err)
			}
		})
	}
}

func TestAppWebhookURLDerivesAppSpecificHTTPSRoute(t *testing.T) {
	got, err := AppWebhookURL(
		"https://dev-api.unipost.dev/v1/webhooks/twitter/",
		"workspace-client-id",
	)
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://dev-api.unipost.dev/v1/webhooks/twitter/workspace-client-id"; got != want {
		t.Fatalf("URL = %q, want %q", got, want)
	}
	invalid := []struct {
		name     string
		baseURL  string
		routeKey string
	}{
		{name: "non HTTPS", baseURL: "http://localhost/webhook", routeKey: "client"},
		{name: "missing route key", baseURL: "https://dev-api.unipost.dev/v1/webhooks/twitter", routeKey: ""},
		{name: "userinfo", baseURL: "https://user:password@dev-api.unipost.dev/webhook", routeKey: "client"},
		{name: "query", baseURL: "https://dev-api.unipost.dev/webhook?token=secret", routeKey: "client"},
		{name: "force query", baseURL: "https://dev-api.unipost.dev/webhook?", routeKey: "client"},
		{name: "fragment", baseURL: "https://dev-api.unipost.dev/webhook#secret", routeKey: "client"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := AppWebhookURL(tt.baseURL, tt.routeKey); err == nil {
				t.Fatalf("AppWebhookURL() = %q, want rejection", got)
			}
		})
	}
}
