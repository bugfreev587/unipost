package loops

import (
	"context"
	"errors"
	"testing"
)

func TestAuditedClientStrictTransactionalSendRequiresAuditBeforeNetwork(t *testing.T) {
	client := &fakeLifecycleClient{}
	audit := &fakeEmailAuditStore{createErr: errors.New("audit database unavailable")}
	sender := NewAuditedClient(client, audit)

	_, err := sender.SendTransactionalWithAttempt(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_restriction",
		Email:           "owner@example.com",
		IdempotencyKey:  "cycle_1:restriction_notice:user_1",
		Audit: EmailAudit{
			EventKey:              "email.publishing_restriction.restriction_notice.v1",
			AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
		},
	}, nil)
	if err == nil {
		t.Fatal("strict audited send should fail closed when the audit row cannot be created")
	}
	if client.transactionals != 0 {
		t.Fatalf("network sends = %d, want 0 without a durable audit row", client.transactionals)
	}
}

func TestAuditedClientStrictTransactionalSendLinksAttemptBeforeNetwork(t *testing.T) {
	client := &fakeLifecycleClient{}
	audit := &fakeEmailAuditStore{}
	sender := NewAuditedClient(client, audit)
	linked := false

	record, err := sender.SendTransactionalWithAttempt(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_restriction",
		Email:           "owner@example.com",
		IdempotencyKey:  "cycle_1:restriction_notice:user_1",
		Audit: EmailAudit{
			EventKey:              "email.publishing_restriction.restriction_notice.v1",
			AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
		},
	}, func(_ context.Context, attemptID string) error {
		if client.transactionals != 0 {
			t.Fatal("attempt linkage must happen before the network send")
		}
		if attemptID != "audit_123" {
			t.Fatalf("attempt id = %q, want audit_123", attemptID)
		}
		linked = true
		return nil
	})
	if err != nil {
		t.Fatalf("strict audited send: %v", err)
	}
	if record.ID != "audit_123" || !linked || client.transactionals != 1 {
		t.Fatalf("record=%+v linked=%v sends=%d", record, linked, client.transactionals)
	}
	if audit.lastAttempt.IdempotencyKey != "cycle_1:restriction_notice:user_1:g1:a1" {
		t.Fatalf("audit attempt key = %q", audit.lastAttempt.IdempotencyKey)
	}
	if client.lastTransactional.IdempotencyKey != "cycle_1:restriction_notice:user_1" {
		t.Fatalf("provider idempotency key = %q", client.lastTransactional.IdempotencyKey)
	}
}

func TestAuditedClientStrictTransactionalSendStopsWhenRecipientLinkFails(t *testing.T) {
	client := &fakeLifecycleClient{}
	audit := &fakeEmailAuditStore{}
	sender := NewAuditedClient(client, audit)

	_, err := sender.SendTransactionalWithAttempt(context.Background(), TransactionalEmail{
		TransactionalID: "tmpl_restriction",
		Email:           "owner@example.com",
		IdempotencyKey:  "cycle_1:restriction_notice:user_1",
		Audit: EmailAudit{
			EventKey:              "email.publishing_restriction.restriction_notice.v1",
			AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
		},
	}, func(context.Context, string) error {
		return errors.New("recipient claim no longer active")
	})
	if err == nil {
		t.Fatal("strict audited send should stop when the local recipient claim cannot be linked")
	}
	if client.transactionals != 0 {
		t.Fatalf("network sends = %d, want 0 after linkage failure", client.transactionals)
	}
	if audit.markedFailed != 1 {
		t.Fatalf("failed audit updates = %d, want 1", audit.markedFailed)
	}
}
