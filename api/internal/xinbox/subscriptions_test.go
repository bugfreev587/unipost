package xinbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
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
			_, _ = w.Write([]byte(`{"data":{"valid":true}}`))
		}
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	webhook, err := client.EnsureWebhook(context.Background(), "app-token", "https://dev-api.unipost.dev/v1/webhooks/twitter")
	if err != nil {
		t.Fatal(err)
	}
	if !webhook.Valid || calls != 2 {
		t.Fatalf("webhook=%+v calls=%d", webhook, calls)
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
		if got := r.Header.Get("Authorization"); got != "Bearer user-oauth-token" {
			t.Fatalf("Authorization = %q", got)
		}
		if r.Method != http.MethodDelete || r.URL.Path != "/2/activity/subscriptions/subscription-missing" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(http.StatusGone)
	}))
	defer server.Close()

	client := NewClient(ClientConfig{BaseURL: server.URL, HTTPClient: server.Client()})
	if err := client.DeleteActivitySubscription(context.Background(), "user-oauth-token", "subscription-missing"); err != nil {
		t.Fatal(err)
	}
}
