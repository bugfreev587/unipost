package instagramwebhooks

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSubscriberSubscribe(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotForm url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request: %v", err)
		}
		gotForm, err = url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse form: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"success":true}`)
	}))
	defer server.Close()

	subscriber := NewSubscriber(server.Client(), server.URL)
	if err := subscriber.Subscribe(context.Background(), "ig_123", "token_123"); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotPath != "/ig_123/subscribed_apps" {
		t.Fatalf("path = %q", gotPath)
	}
	if gotForm.Get("access_token") != "token_123" {
		t.Fatalf("access_token = %q", gotForm.Get("access_token"))
	}
	fields := strings.Split(gotForm.Get("subscribed_fields"), ",")
	wantFields := []string{"messages", "messaging_postbacks", "comments"}
	if strings.Join(fields, ",") != strings.Join(wantFields, ",") {
		t.Fatalf("subscribed_fields = %v, want %v", fields, wantFields)
	}
}

func TestInstagramWebhookSubscriptionHTTPFailureDoesNotLeakToken(t *testing.T) {
	const accessToken = "secret_subscriber_token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"access_token=secret_subscriber_token"}}`)
	}))
	defer server.Close()

	subscriber := NewSubscriber(server.Client(), server.URL)
	err := subscriber.Subscribe(context.Background(), "ig_123", accessToken)
	if err == nil {
		t.Fatal("expected subscription error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Fatalf("error = %q, want HTTP status", err)
	}
	assertSubscriberErrorDoesNotLeakToken(t, err, accessToken)
}

func TestInstagramWebhookSubscriptionTransportFailureDoesNotLeakToken(t *testing.T) {
	const accessToken = "secret_subscriber_token"
	subscriber := NewSubscriber(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("transport echoed access_token=%s", accessToken)
	})}, "https://graph.instagram.test/v21.0")

	err := subscriber.Subscribe(context.Background(), "ig_123", accessToken)
	if err == nil {
		t.Fatal("expected subscription error")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Fatalf("error = %q, want request failure context", err)
	}
	assertSubscriberErrorDoesNotLeakToken(t, err, accessToken)
}

func TestInstagramWebhookSubscriptionRejectedResponseDoesNotLeakToken(t *testing.T) {
	const accessToken = "secret_subscriber_token"
	subscriber := NewSubscriber(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"success":false,"debug":"access_token=secret_subscriber_token"}`)),
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}, "https://graph.instagram.test/v21.0")

	err := subscriber.Subscribe(context.Background(), "ig_123", accessToken)
	if err == nil {
		t.Fatal("expected rejected subscription error")
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("error = %q, want rejection context", err)
	}
	assertSubscriberErrorDoesNotLeakToken(t, err, accessToken)
}

func assertSubscriberErrorDoesNotLeakToken(t *testing.T, err error, accessToken string) {
	t.Helper()
	if strings.Contains(err.Error(), accessToken) || strings.Contains(strings.ToLower(err.Error()), "access_token=") {
		t.Fatalf("error leaked access token: %q", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
