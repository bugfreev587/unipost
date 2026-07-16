package instagramwebhooks

import (
	"context"
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

func TestSubscriberSubscribeIncludesMetaFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"error":{"message":"missing permission"}}`)
	}))
	defer server.Close()

	subscriber := NewSubscriber(server.Client(), server.URL)
	err := subscriber.Subscribe(context.Background(), "ig_123", "token_123")
	if err == nil {
		t.Fatal("expected subscription error")
	}
	for _, want := range []string{"403", "missing permission"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err.Error(), want)
		}
	}
}
