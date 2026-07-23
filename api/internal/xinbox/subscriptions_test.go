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

func TestXClientListActivitySubscriptionsUsesBoundedProviderContract(t *testing.T) {
	makePage := func(page int) []ActivitySubscription {
		result := make([]ActivitySubscription, 100)
		for i := range result {
			result[i] = ActivitySubscription{
				ID:        fmt.Sprintf("%d", 100000+page*100+i),
				EventType: "dm.received",
				Filter:    ActivityFilter{UserID: "2244994945"},
				Tag:       fmt.Sprintf("tag-%d-%d", page, i),
				WebhookID: "1001",
			}
		}
		return result
	}

	t.Run("ten pages of one hundred reach the self serve limit", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("max_results"); got != "100" {
				t.Fatalf("max_results = %q, want 100", got)
			}
			if calls == 0 {
				if got := r.URL.Query().Get("pagination_token"); got != "" {
					t.Fatalf("first pagination token = %q", got)
				}
			} else if got, want := r.URL.Query().Get("pagination_token"), fmt.Sprintf("token-%d", calls); got != want {
				t.Fatalf("pagination token = %q, want %q", got, want)
			}
			response := map[string]any{"data": makePage(calls)}
			calls++
			if calls < 10 {
				response["meta"] = map[string]any{"next_token": fmt.Sprintf("token-%d", calls)}
			}
			_ = json.NewEncoder(w).Encode(response)
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		subscriptions, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err != nil {
			t.Fatal(err)
		}
		if len(subscriptions) != 1000 || calls != 10 {
			t.Fatalf("subscriptions=%d calls=%d, want 1000 and 10", len(subscriptions), calls)
		}
	})

	t.Run("tenth page next token fails closed", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if got := r.URL.Query().Get("max_results"); got != "100" {
				t.Fatalf("max_results = %q, want 100", got)
			}
			calls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": makePage(calls),
				"meta": map[string]any{"next_token": fmt.Sprintf("token-%d", calls)},
			})
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err == nil || !strings.Contains(err.Error(), "bound") {
			t.Fatalf("error = %v, want bounded discovery failure", err)
		}
		if calls != 10 {
			t.Fatalf("calls = %d, want exactly 10", calls)
		}
	})

	t.Run("empty page token chain cannot exceed ten pages", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []ActivitySubscription{},
				"meta": map[string]any{"next_token": fmt.Sprintf("empty-token-%d", calls)},
			})
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err == nil || !strings.Contains(err.Error(), "bound") {
			t.Fatalf("error = %v, want bounded discovery failure", err)
		}
		if calls != 10 {
			t.Fatalf("calls = %d, want exactly 10", calls)
		}
	})

	t.Run("repeated next token fails closed", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []ActivitySubscription{},
				"meta": map[string]any{"next_token": "repeated-token"},
			})
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err == nil || !strings.Contains(err.Error(), "repeated next_token") {
			t.Fatalf("error = %v, want repeated token failure", err)
		}
		if calls != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})
}

func TestXClientListActivitySubscriptionsUsesSingleAggregateDeadline(t *testing.T) {
	var firstDeadline time.Time
	calls := 0
	client := NewClient(ClientConfig{
		BaseURL:               "https://api.x.test",
		ControlRequestTimeout: time.Minute,
		HTTPClient: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			calls++
			deadline, ok := r.Context().Deadline()
			if !ok {
				t.Fatal("request context has no deadline")
			}
			if calls == 1 {
				firstDeadline = deadline
			} else if !deadline.Equal(firstDeadline) {
				t.Fatalf("page %d deadline = %s, want aggregate deadline %s", calls, deadline, firstDeadline)
			}
			body := `{"data":[]}`
			if calls == 1 {
				body = `{"data":[],"meta":{"next_token":"token-1"}}`
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	})

	if _, err := client.ListActivitySubscriptions(context.Background(), "app-token"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestXClientListActivitySubscriptionsRejectsPartialProviderErrorsAndDuplicateIDs(t *testing.T) {
	t.Run("HTTP 200 provider errors fail safely", func(t *testing.T) {
		const secret = "provider-detail-must-not-leak"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = fmt.Fprintf(w, `{"data":[{"subscription_id":"2001"}],"errors":[{"title":%q,"detail":%q,"code":%q}]}`, secret, secret, secret)
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err == nil || err.Error() != "X activity subscription discovery returned provider errors" {
			t.Fatalf("error = %v, want fixed safe provider error", err)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("error leaked provider detail: %v", err)
		}
	})

	t.Run("duplicate subscription ID fails closed", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":[{"subscription_id":"2001","event_type":"dm.received","filter":{"user_id":"1"},"tag":"one","webhook_id":"1001"},{"subscription_id":"2001","event_type":"dm.received","filter":{"user_id":"2"},"tag":"two","webhook_id":"1002"}]}`))
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		_, err := client.ListActivitySubscriptions(context.Background(), "app-token")
		if err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("error = %v, want duplicate ID failure", err)
		}
	})
}

func TestXClientListActivitySubscriptionsFailsClosedOnProviderOverflow(t *testing.T) {
	makeSubscriptions := func(start, count int) []ActivitySubscription {
		subscriptions := make([]ActivitySubscription, count)
		for i := range subscriptions {
			subscriptions[i] = ActivitySubscription{
				ID:        fmt.Sprintf("%d", start+i),
				EventType: "dm.received",
				Filter:    ActivityFilter{UserID: "2244994945"},
				Tag:       fmt.Sprintf("tag-%d", start+i),
				WebhookID: "1001",
			}
		}
		return subscriptions
	}

	t.Run("single page exceeds remaining capacity", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"data": makeSubscriptions(1000, 1501)})
		}))
		defer server.Close()

		client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
		if _, err := client.ListActivitySubscriptions(context.Background(), "app-token"); err == nil || !strings.Contains(err.Error(), "capacity") {
			t.Fatalf("ListActivitySubscriptions() error = %v, want fail-closed capacity error", err)
		}
	})

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
		_, _ = w.Write([]byte(`{"data":[{"id":"1001","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "1001" || calls != 1 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
	}
}

func TestXClientEnsureWebhookRevalidatesConfiguredInvalidWebhook(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch calls {
		case 1:
			_, _ = w.Write([]byte(`{"data":[{"id":"1001","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 2:
			if r.Method != http.MethodPut || r.URL.Path != "/2/webhooks/1001" {
				t.Fatalf("revalidate request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":{"attempted":true}}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"1001","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 4:
			_, _ = w.Write([]byte(`{"data":[{"id":"1001","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
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
			_, _ = w.Write([]byte(`{"data":[{"id":"1001","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
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
			_, _ = w.Write([]byte(`{"id":"1002","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if webhook.ID != "1002" || calls != 2 {
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
			_, _ = w.Write([]byte(`{"data":{"id":"1003","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}}`))
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
	if webhook.ID != "1003" || calls != 2 {
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
			_, _ = w.Write([]byte(`{"id":"1002","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}`))
		case 3:
			_, _ = w.Write([]byte(`{"data":[{"id":"1002","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`))
		case 4:
			_, _ = w.Write([]byte(`{"data":[{"id":"1002","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`))
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
	if webhook.ID != "1002" || webhook.URL != configuredURL || !webhook.Valid || calls != 4 {
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
			_, _ = w.Write([]byte(`{"data":{"id":"1003","url":"https://wrong.example/webhook","valid":true}}`))
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
			_, _ = w.Write([]byte(`{"data":{"id":"1003","valid":false}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[{"id":"1003","url":"https://wrong.example/webhook","valid":true}]}`))
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

func TestXClientEnsureWebhookRejectsMalformedProviderIDs(t *testing.T) {
	const configuredURL = "https://dev-api.unipost.dev/v1/webhooks/twitter"
	tests := []struct {
		name      string
		responses []string
		wantCalls int
	}{
		{
			name:      "reused valid webhook",
			responses: []string{`{"data":[{"id":"invalid-webhook-id","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}]}`},
			wantCalls: 1,
		},
		{
			name:      "revalidated webhook",
			responses: []string{`{"data":[{"id":"invalid-webhook-id","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":false}]}`},
			wantCalls: 1,
		},
		{
			name: "created webhook",
			responses: []string{
				`{"data":[]}`,
				`{"id":"invalid-webhook-id","url":"https://dev-api.unipost.dev/v1/webhooks/twitter","valid":true}`,
			},
			wantCalls: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls >= len(tt.responses) {
					t.Fatalf("unexpected provider request %d", calls+1)
				}
				_, _ = w.Write([]byte(tt.responses[calls]))
				calls++
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			if _, err := client.EnsureWebhook(context.Background(), "app-token", configuredURL); err == nil {
				t.Fatal("expected malformed provider webhook ID error")
			}
			if calls != tt.wantCalls {
				t.Fatalf("provider calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestXClientDoesNotExposeSubscriptionReconciliationMethod(t *testing.T) {
	methodName := "Ensure" + "DM" + "Subscription"
	if _, exists := reflect.TypeOf((*Client)(nil)).MethodByName(methodName); exists {
		t.Fatalf("Client unexpectedly exposes %s; reconciliation belongs to the worker", methodName)
	}
}

func TestXClientCreateDMSubscriptionIsPureCreate(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q, want app bearer", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/2/activity/subscriptions" {
			t.Fatalf("request = %s %s, want pure create POST", r.Method, r.URL.Path)
		}
		var body ActivitySubscription
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		want := ActivitySubscription{
			EventType: "dm.received",
			Filter:    ActivityFilter{UserID: "2244994945"},
			Tag:       DMSubscriptionTag("account-123"),
			WebhookID: "1001",
		}
		if !reflect.DeepEqual(body, want) {
			t.Fatalf("body = %#v, want %#v", body, want)
		}
		_, _ = w.Write([]byte(`{"data":{"subscription_id":"2001","event_type":"dm.received","filter":{"user_id":"2244994945"},"tag":"unipost:x:dm:account-123","webhook_id":"1001"}}`))
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	subscription, err := client.CreateDMSubscription(
		context.Background(),
		"app-token",
		"account-123",
		"2244994945",
		"1001",
	)
	if err != nil {
		t.Fatal(err)
	}
	if subscription.ID != "2001" || calls != 1 {
		t.Fatalf("subscription=%+v calls=%d, want one POST", subscription, calls)
	}
}

func TestXClientDeleteActivitySubscriptionIsIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/2008" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	if err := client.DeleteActivitySubscription(context.Background(), "app-token", "2008"); err != nil {
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
		{name: "malformed 200", status: http.StatusOK, body: `{`, wantErr: true},
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
			body:    `{"errors":[{"resource_id":"2001","title":"Not Found Error","type":"https://api.x.com/2/problems/resource-not-found","detail":"subscription missing","status":404}]}`,
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
			err := client.DeleteActivitySubscription(context.Background(), "app-token", "2001")
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

	err := client.DeleteActivitySubscription(context.Background(), "app-token", "2001")
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
				if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/2001" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":[{"code":"cleanup-denied","title":"Cleanup denied","status":403,"detail":"private provider detail"}]}`))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteActivitySubscription(context.Background(), "app-token", "2001")
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
			validStatus: http.StatusOK,
			validBody:   `{"data":{"deleted":true}}`,
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
	invalidIDs := []string{
		"",
		" ",
		"12345678901234567890",
		"provider",
		"123-456",
		"123/456",
		"123?456",
		"123#456",
	}
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
			if err := deleteCase.delete(client, "1234567890123456789"); err != nil {
				t.Fatalf("valid provider ID: %v", err)
			}
			if calls != 1 {
				t.Fatalf("provider calls = %d, want 1", calls)
			}
		})
	}
}

func TestXClientDeleteWebhookRequiresOfficialConfirmation(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr bool
	}{
		{name: "confirmed JSON", status: http.StatusOK, body: `{"data":{"deleted":true}}`},
		{name: "empty 200", status: http.StatusOK, wantErr: true},
		{name: "malformed 200", status: http.StatusOK, body: `{`, wantErr: true},
		{name: "accepted", status: http.StatusAccepted, body: `{"data":{"deleted":true}}`, wantErr: true},
		{name: "no content", status: http.StatusNoContent, wantErr: true},
		{name: "deleted false", status: http.StatusOK, body: `{"data":{"deleted":false}}`, wantErr: true},
		{
			name:    "partial 200 error body",
			status:  http.StatusOK,
			body:    `{"data":{"deleted":true},"errors":[{"title":"Invalid Request","detail":"private provider detail"}]}`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/2/webhooks/1001" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteWebhook(context.Background(), "app-token", "1001")
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestXClientDeleteWebhookResponseIsBoundedAndClosed(t *testing.T) {
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

	err := client.DeleteWebhook(context.Background(), "app-token", "1001")
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

func TestDeleteWebhookIdempotentProviderStatuses(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusGone, http.StatusForbidden} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete || r.URL.Path != "/2/webhooks/1001" {
					t.Fatalf("request = %s %s", r.Method, r.URL.Path)
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"errors":[{"code":"cleanup-denied","title":"Cleanup denied","status":403,"detail":"private provider detail"}]}`))
			}))
			defer server.Close()

			client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
			err := client.DeleteWebhook(context.Background(), "app-token", "1001")
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
		{name: "port only", baseURL: "https://:443/webhook", routeKey: "client"},
		{name: "explicit port", baseURL: "https://dev-api.unipost.dev:443/webhook", routeKey: "client"},
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
	t.Run("provider URL length boundary", func(t *testing.T) {
		got, err := AppWebhookURL("https://example.com", strings.Repeat("a", 180))
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 200 {
			t.Fatalf("URL length = %d, want 200", len(got))
		}
		if got, err := AppWebhookURL("https://example.com", strings.Repeat("a", 181)); err == nil {
			t.Fatalf("AppWebhookURL() length = %d, want over-limit rejection", len(got))
		}
	})
}
