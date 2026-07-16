package xinbox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

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

func TestXClientEnsureDMSubscriptionUsesPrivateUserOAuthContract(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if got := r.Header.Get("Authorization"); got != "Bearer user-oauth-token" {
			t.Fatalf("Authorization = %q, want connected user token", got)
		}
		switch calls {
		case 1:
			if r.Method != http.MethodGet || r.URL.Path != "/2/activity/subscriptions" {
				t.Fatalf("list request = %s %s", r.Method, r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case 2:
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
		"user-oauth-token",
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
		"user-oauth-token",
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
		"user-oauth-token",
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
		"user-oauth-token",
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
		"user-oauth-token",
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
		"user-oauth-token",
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

func TestXClientDeleteActivitySubscriptionRequiresConfirmedDeletedTrue(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "confirmed", body: `{"data":{"deleted":true},"meta":{"total_subscriptions":0}}`},
		{name: "deleted false", body: `{"data":{"deleted":false}}`, wantErr: true},
		{
			name:    "partial 200 error",
			body:    `{"data":{"deleted":true},"errors":[{"title":"Invalid Request","type":"https://api.x.com/2/problems/invalid-request","detail":"partial failure","status":400}]}`,
			wantErr: true,
		},
		{
			name: "explicit already missing",
			body: `{"errors":[{"resource_id":"subscription-1","title":"Not Found Error","type":"https://api.x.com/2/problems/resource-not-found","detail":"subscription missing","status":404}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("Authorization"); got != "Bearer app-token" {
					t.Fatalf("Authorization = %q, want app bearer", got)
				}
				_, _ = w.Write([]byte(tt.body))
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
