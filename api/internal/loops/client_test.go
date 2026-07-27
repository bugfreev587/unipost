package loops

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestClientUpsertContactSendsLoopsUpdateRequest(t *testing.T) {
	var gotMethod string
	var gotPath string
	var gotAuth string
	var payload map[string]any

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(http.StatusOK, `{"success":true,"id":"contact_123"}`), nil
	})}

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	if err := client.UpsertContact(context.Background(), Contact{
		Email:     "alex@example.com",
		UserID:    "user_123",
		FirstName: "Alex",
		LastName:  "Smith",
		Source:    "unipost_dashboard",
		Properties: map[string]any{
			"plan_id":       "free",
			"workspace_id":  "ws_123",
			"workspaceName": "Alex Workspace",
		},
	}); err != nil {
		t.Fatalf("UpsertContact returned error: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s, want PUT", gotMethod)
	}
	if gotPath != "/api/v1/contacts/update" {
		t.Fatalf("path = %s, want /api/v1/contacts/update", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Authorization = %q", gotAuth)
	}
	assertPayloadValue(t, payload, "email", "alex@example.com")
	assertPayloadValue(t, payload, "userId", "user_123")
	assertPayloadValue(t, payload, "firstName", "Alex")
	assertPayloadValue(t, payload, "lastName", "Smith")
	assertPayloadValue(t, payload, "source", "unipost_dashboard")
	assertPayloadValue(t, payload, "plan_id", "free")
	assertPayloadValue(t, payload, "workspace_id", "ws_123")
	assertPayloadValue(t, payload, "workspaceName", "Alex Workspace")
	if _, ok := payload["subscribed"]; ok {
		t.Fatal("UpsertContact should not set subscribed and accidentally resubscribe contacts")
	}
}

func TestClientSendEventUsesIdempotencyKey(t *testing.T) {
	var gotPath string
	var gotIDKey string
	var payload map[string]any

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotIDKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(http.StatusOK, `{"success":true}`), nil
	})}

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	if err := client.SendEvent(context.Background(), Event{
		Email:          "alex@example.com",
		UserID:         "user_123",
		Name:           "user_signed_up",
		IdempotencyKey: "evt_123",
		Properties: map[string]any{
			"workspace_id": "ws_123",
		},
	}); err != nil {
		t.Fatalf("SendEvent returned error: %v", err)
	}

	if gotPath != "/api/v1/events/send" {
		t.Fatalf("path = %s, want /api/v1/events/send", gotPath)
	}
	if gotIDKey != "evt_123" {
		t.Fatalf("Idempotency-Key = %q", gotIDKey)
	}
	assertPayloadValue(t, payload, "email", "alex@example.com")
	assertPayloadValue(t, payload, "userId", "user_123")
	assertPayloadValue(t, payload, "eventName", "user_signed_up")
	props, ok := payload["eventProperties"].(map[string]any)
	if !ok {
		t.Fatalf("eventProperties = %#v, want object", payload["eventProperties"])
	}
	assertPayloadValue(t, props, "workspace_id", "ws_123")
}

func TestClientSendTransactionalEmail(t *testing.T) {
	var gotPath string
	var gotIDKey string
	var payload map[string]any

	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotPath = r.URL.Path
		gotIDKey = r.Header.Get("Idempotency-Key")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		return jsonResponse(http.StatusOK, `{"success":true}`), nil
	})}

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	if err := client.SendTransactional(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_123",
		Email:           "alex@example.com",
		UserID:          "user_123",
		IdempotencyKey:  "txn_123",
		DataVariables: map[string]any{
			"first_name": "Alex",
		},
	}); err != nil {
		t.Fatalf("SendTransactional returned error: %v", err)
	}

	if gotPath != "/api/v1/transactional" {
		t.Fatalf("path = %s, want /api/v1/transactional", gotPath)
	}
	if gotIDKey != "txn_123" {
		t.Fatalf("Idempotency-Key = %q, want txn_123", gotIDKey)
	}
	assertPayloadValue(t, payload, "transactionalId", "tmpl_123")
	assertPayloadValue(t, payload, "email", "alex@example.com")
	if _, ok := payload["userId"]; ok {
		t.Fatal("payload included userId, want transactional API body to omit it")
	}
	vars, ok := payload["dataVariables"].(map[string]any)
	if !ok {
		t.Fatalf("dataVariables = %#v, want object", payload["dataVariables"])
	}
	assertPayloadValue(t, vars, "first_name", "Alex")
}

func TestClientReturnsProviderErrors(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusBadRequest, `{"success":false,"message":"invalid contact"}`), nil
	})}

	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	if err := client.UpsertContact(context.Background(), Contact{Email: "bad@example.com"}); err == nil {
		t.Fatal("expected provider error")
	}
}

func TestClientSendTransactionalClassifiesLostResponseAsUnknownOutcome(t *testing.T) {
	requestRecorded := false
	cause := errors.New("response lost")
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestRecorded = true
		return nil, cause
	})}
	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	err := client.SendTransactional(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_123",
		Email:           "alex@example.com",
		IdempotencyKey:  "txn_123",
	})
	if !requestRecorded {
		t.Fatal("transport did not record the provider request")
	}
	if !IsSendOutcomeUnknown(err) {
		t.Fatalf("IsSendOutcomeUnknown(%v) = false, want true", err)
	}
	if !errors.Is(err, cause) {
		t.Fatalf("unknown outcome error does not preserve its cause: %v", err)
	}
}

func TestAuditedClientPreservesUnknownSendOutcomeClassification(t *testing.T) {
	client := &fakeLifecycleClient{transactionalErr: &SendOutcomeUnknownError{Err: errors.New("response lost")}}
	audit := &fakeEmailAuditStore{}
	sender := NewAuditedClient(client, audit)

	_, err := sender.SendTransactionalWithAttempt(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_123",
		Email:           "alex@example.com",
		IdempotencyKey:  "txn_123",
		Audit: EmailAudit{
			EventKey:              "email.publishing_restriction.restriction_notice.v1",
			AttemptIdempotencyKey: "txn_123:g1:a1",
		},
	}, nil)
	if !IsSendOutcomeUnknown(err) {
		t.Fatalf("audited send lost unknown-outcome classification: %v", err)
	}
	if audit.markedFailed != 1 || !strings.Contains(audit.failedReason, "provider request outcome unknown") {
		t.Fatalf("audit failed updates=%d reason=%q, want clear unknown-outcome failure", audit.markedFailed, audit.failedReason)
	}
}

func TestClientSendTransactionalDoesNotClassifyExplicitProviderFailureAsUnknownOutcome(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusServiceUnavailable, `{"message":"temporarily unavailable"}`), nil
	})}
	client := NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	})

	err := client.SendTransactional(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_123",
		Email:           "alex@example.com",
	})
	if err == nil {
		t.Fatal("expected explicit provider failure")
	}
	if IsSendOutcomeUnknown(err) {
		t.Fatalf("explicit provider failure classified as unknown outcome: %v", err)
	}
}

func TestClientSendTransactionalDoesNotClassifyLocalPreDoErrorsAsUnknownOutcome(t *testing.T) {
	tests := []struct {
		name   string
		client *Client
		email  TransactionalEmail
	}{
		{name: "nil client", client: nil},
		{name: "missing API key", client: NewClient(Config{BaseURL: "https://loops.test/api"})},
		{
			name:   "marshal payload",
			client: NewClient(Config{APIKey: "test-key", BaseURL: "https://loops.test/api"}),
			email:  TransactionalEmail{DataVariables: map[string]any{"invalid": func() {}}},
		},
		{
			name:   "construct request",
			client: NewClient(Config{APIKey: "test-key", BaseURL: ":"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.client.SendTransactional(context.Background(), tt.email)
			if err == nil {
				t.Fatal("expected local pre-Do error")
			}
			if IsSendOutcomeUnknown(err) {
				t.Fatalf("local pre-Do error classified as unknown outcome: %v", err)
			}
		})
	}
}

func assertPayloadValue(t *testing.T, payload map[string]any, key string, want any) {
	t.Helper()
	if got := payload[key]; got != want {
		t.Fatalf("payload[%s] = %#v, want %#v", key, got, want)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
