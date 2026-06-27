package errortriage

import (
	"context"
	"errors"
	"testing"
)

func TestEmailSendServiceRetriesFailedRecipientWithStableIdempotencyKey(t *testing.T) {
	store := &fakeEmailStore{
		ctx: EmailSendContext{
			ItemID:            "item_1",
			RecipientID:       "recipient_1",
			RecipientScopeKey: "workspace:ws_1:user:user_1",
			RecipientUserID:   "user_1",
			CurrentEmail:      "fresh@example.com",
			Item: ItemState{
				Classification: ClassificationUserActionNeeded,
				ActionKind:     ActionKindEmail,
				WorkflowStatus: WorkflowStatusReady,
			},
			Recipient: RecipientState{Status: RecipientStatusSendFailed},
			Draft:     EmailDraft{Subject: "Reconnect Threads", Body: "Please reconnect Threads."},
			CTAURL:    "https://app.unipost.dev/projects/ws_1/accounts",
		},
	}
	sender := &fakeTransactionalSender{}
	service := NewEmailSendService(store, sender, "txn_error_triage")

	out, err := service.SendRecipient(context.Background(), "item_1", "recipient_1", "admin_1")
	if err != nil {
		t.Fatalf("SendRecipient returned error: %v", err)
	}

	if out.AttemptID != "attempt_1" {
		t.Fatalf("attempt id = %q, want attempt_1", out.AttemptID)
	}
	if sender.sent.IdempotencyKey != "error_triage:item_1:workspace:ws_1:user:user_1" {
		t.Fatalf("idempotency key = %q", sender.sent.IdempotencyKey)
	}
	if !store.markedSucceeded {
		t.Fatalf("expected success state to be marked")
	}
	if sender.sent.Email != "fresh@example.com" {
		t.Fatalf("email = %q, want current user email", sender.sent.Email)
	}
	if got := sender.sent.DataVariables["cta_url"]; got != "https://app.unipost.dev/projects/ws_1/accounts" {
		t.Fatalf("cta_url = %#v, want configured CTA URL", got)
	}
}

func TestEmailSendServiceMarksFailureRetryable(t *testing.T) {
	store := &fakeEmailStore{
		ctx: EmailSendContext{
			ItemID:            "item_1",
			RecipientID:       "recipient_1",
			RecipientScopeKey: "workspace:ws_1:user:user_1",
			RecipientUserID:   "user_1",
			CurrentEmail:      "fresh@example.com",
			Item: ItemState{
				Classification: ClassificationUserActionNeeded,
				ActionKind:     ActionKindEmail,
				WorkflowStatus: WorkflowStatusReady,
			},
			Recipient: RecipientState{Status: RecipientStatusPending},
			Draft:     EmailDraft{Subject: "Reconnect Threads", Body: "Please reconnect Threads."},
		},
	}
	sender := &fakeTransactionalSender{err: errors.New("provider down")}
	service := NewEmailSendService(store, sender, "txn_error_triage")

	_, err := service.SendRecipient(context.Background(), "item_1", "recipient_1", "admin_1")
	if err == nil {
		t.Fatalf("expected provider error")
	}
	if !store.markedFailed {
		t.Fatalf("expected failed state to be marked")
	}
	if store.failureMessage != "provider down" {
		t.Fatalf("failure message = %q", store.failureMessage)
	}
}

type fakeEmailStore struct {
	ctx             EmailSendContext
	markedSucceeded bool
	markedFailed    bool
	failureMessage  string
}

func (s *fakeEmailStore) LoadEmailSendContext(ctx context.Context, itemID, recipientID string) (EmailSendContext, error) {
	return s.ctx, nil
}

func (s *fakeEmailStore) CreateEmailSendAttempt(ctx context.Context, params CreateEmailSendAttemptParams) (EmailSendAttempt, error) {
	return EmailSendAttempt{ID: "attempt_1", AttemptNumber: 2}, nil
}

func (s *fakeEmailStore) MarkEmailSendSucceeded(ctx context.Context, attemptID, recipientID string) error {
	s.markedSucceeded = true
	return nil
}

func (s *fakeEmailStore) MarkEmailSendFailed(ctx context.Context, attemptID, recipientID, message string) error {
	s.markedFailed = true
	s.failureMessage = message
	return nil
}

type fakeTransactionalSender struct {
	sent TransactionalEmail
	err  error
}

func (s *fakeTransactionalSender) Enabled() bool { return true }

func (s *fakeTransactionalSender) SendTransactional(ctx context.Context, email TransactionalEmail) error {
	s.sent = email
	return s.err
}
