package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/integrationlogs"
)

type recordingOutcomeWriter struct {
	events []integrationlogs.Event
	err    error
	before func()
}

func (w *recordingOutcomeWriter) WriteSync(_ context.Context, event integrationlogs.Event) error {
	if w.before != nil {
		w.before()
	}
	w.events = append(w.events, event)
	return w.err
}

func hostedConnectTestAttempt() hostedConnectAttempt {
	return hostedConnectAttempt{
		WorkspaceID:    "ws_1",
		ProfileID:      "pr_1",
		Platform:       "facebook",
		SessionID:      "cs_1",
		ExternalUserID: "managed_1",
		RequestID:      "req_1",
	}
}

func TestHostedConnectOutcomeResponseWriterWritesOnceBeforeResponse(t *testing.T) {
	base := httptest.NewRecorder()
	responseStarted := false
	writer := &recordingOutcomeWriter{before: func() {
		if responseStarted {
			t.Fatal("outcome persisted after response started")
		}
	}}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectTestAttempt())
	outcome.Fail(
		"facebook_page_not_available",
		"Hosted Connect failed: no manageable Facebook Page was found.",
		map[string]any{"page_count": 0},
	)
	w := wrapHostedConnectResponse(responseStartWriter{ResponseWriter: base, started: &responseStarted}, outcome)

	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte("body"))

	if len(writer.events) != 1 {
		t.Fatalf("events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != integrationlogs.ActionAccountConnectCallbackFailed || event.ErrorCode != "facebook_page_not_available" {
		t.Fatalf("event = %+v", event)
	}
	metadata, ok := event.Metadata.(map[string]any)
	if !ok {
		t.Fatalf("metadata type = %T", event.Metadata)
	}
	for key, want := range map[string]any{
		"connect_session_id": "cs_1",
		"external_user_id":   "managed_1",
		"callback_status":    "error",
		"connection_type":    "managed",
		"page_count":         0,
	} {
		if got := metadata[key]; got != want {
			t.Fatalf("metadata[%q] = %#v, want %#v", key, got, want)
		}
	}
	if event.ResponsePayload != nil {
		t.Fatalf("response payload must be empty: %#v", event.ResponsePayload)
	}
}

func TestHostedConnectOutcomeSuccess(t *testing.T) {
	writer := &recordingOutcomeWriter{}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectTestAttempt())
	outcome.Success("sa_1", "Page Name")
	w := wrapHostedConnectResponse(httptest.NewRecorder(), outcome)
	_, _ = w.Write([]byte("ok"))

	if len(writer.events) != 1 {
		t.Fatalf("events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != integrationlogs.ActionAccountConnectCallbackOK || event.Status != integrationlogs.StatusSuccess || event.SocialAccountID != "sa_1" || event.ErrorCode != "" {
		t.Fatalf("event = %+v", event)
	}
	metadata := event.Metadata.(map[string]any)
	if metadata["callback_status"] != "success" || metadata["account_name"] != "Page Name" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestHostedConnectOutcomeCancellation(t *testing.T) {
	writer := &recordingOutcomeWriter{}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectTestAttempt())
	outcome.Cancel("access_denied")
	wrapHostedConnectResponse(httptest.NewRecorder(), outcome).WriteHeader(http.StatusOK)

	if len(writer.events) != 1 {
		t.Fatalf("events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.Action != integrationlogs.ActionAccountConnectCallbackCancelled || event.Status != integrationlogs.StatusWarning || event.ErrorCode != "access_denied" {
		t.Fatalf("event = %+v", event)
	}
	if event.Metadata.(map[string]any)["callback_status"] != "cancelled" {
		t.Fatalf("metadata = %#v", event.Metadata)
	}
}

func TestHostedConnectOutcomeDefaultsToBoundedFailure(t *testing.T) {
	writer := &recordingOutcomeWriter{}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectTestAttempt())
	wrapHostedConnectResponse(httptest.NewRecorder(), outcome).WriteHeader(http.StatusInternalServerError)

	if len(writer.events) != 1 {
		t.Fatalf("events = %d, want 1", len(writer.events))
	}
	event := writer.events[0]
	if event.ErrorCode != "connect_callback_failed" || event.Message != "Hosted Connect failed." {
		t.Fatalf("event = %+v", event)
	}
}

func TestHostedConnectOutcomeWriterFailureDoesNotChangeResponse(t *testing.T) {
	writer := &recordingOutcomeWriter{err: errors.New("database unavailable")}
	outcome := newHostedConnectOutcome(context.Background(), writer, hostedConnectTestAttempt())
	outcome.Fail("safe_failure", "Hosted Connect failed safely.", nil)
	base := httptest.NewRecorder()
	w := wrapHostedConnectResponse(base, outcome)

	w.WriteHeader(http.StatusTeapot)
	_, _ = w.Write([]byte("expected body"))

	if base.Code != http.StatusTeapot || base.Body.String() != "expected body" {
		t.Fatalf("response = (%d, %q)", base.Code, base.Body.String())
	}
	if len(writer.events) != 1 {
		t.Fatalf("events = %d, want one attempted write", len(writer.events))
	}
}

type responseStartWriter struct {
	http.ResponseWriter
	started *bool
}

func (w responseStartWriter) WriteHeader(code int) {
	*w.started = true
	w.ResponseWriter.WriteHeader(code)
}

func (w responseStartWriter) Write(body []byte) (int, error) {
	*w.started = true
	return w.ResponseWriter.Write(body)
}
