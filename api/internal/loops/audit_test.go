package loops

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

type contextAwareEmailAuditStore struct {
	status           string
	attempt          EmailSendAttempt
	failedReason     string
	sentContext      context.Context
	failedContext    context.Context
	sentBounded      bool
	failedBounded    bool
	sentContextErr   error
	failedContextErr error
	failedErr        error
}

func (s *contextAwareEmailAuditStore) CreateEmailSendAttempt(ctx context.Context, attempt EmailSendAttempt) (EmailSendAttemptRecord, error) {
	if err := ctx.Err(); err != nil {
		return EmailSendAttemptRecord{}, err
	}
	s.status = "pending"
	s.attempt = attempt
	return EmailSendAttemptRecord{ID: "audit_detached"}, nil
}

func (s *contextAwareEmailAuditStore) CreateSkippedEmailSendAttempt(context.Context, EmailSendAttempt, string) (EmailSendAttemptRecord, error) {
	return EmailSendAttemptRecord{}, errors.New("unexpected skipped audit")
}

func (s *contextAwareEmailAuditStore) MarkEmailSendAttemptSent(ctx context.Context, _ string) error {
	s.sentContext = ctx
	s.sentContextErr = ctx.Err()
	_, s.sentBounded = ctx.Deadline()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.status = "sent"
	return nil
}

func (s *contextAwareEmailAuditStore) MarkEmailSendAttemptFailed(ctx context.Context, _ string, reason string) error {
	s.failedContext = ctx
	s.failedContextErr = ctx.Err()
	_, s.failedBounded = ctx.Deadline()
	if err := ctx.Err(); err != nil {
		return err
	}
	s.failedReason = reason
	if s.failedErr != nil {
		return s.failedErr
	}
	s.status = "failed"
	return nil
}

func TestAuditedClientDoesNotClassifyAuditFinalizationFailureAsProviderUnknown(t *testing.T) {
	auditErr := errors.New("audit database unavailable")
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("response lost")
	})}
	store := &contextAwareEmailAuditStore{failedErr: auditErr}
	sender := NewAuditedClient(NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	}), store)

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
		t.Fatal("send returned nil, want audit finalization failure")
	}
	if IsSendOutcomeUnknown(err) {
		t.Fatalf("audit finalization failure classified as provider unknown: %v", err)
	}
	if !errors.Is(err, auditErr) {
		t.Fatalf("send error = %v, want audit failure cause", err)
	}
	var finalizationErr *EmailAuditFinalizationError
	if !errors.As(err, &finalizationErr) || !IsSendOutcomeUnknown(finalizationErr.ProviderError) {
		t.Fatalf("send error = %v, want typed finalization error retaining provider outcome", err)
	}
	if store.status != "pending" {
		t.Fatalf("in-memory audit status = %q, want pending after failed transition", store.status)
	}
}

func TestAuditedClientFinalizesUnknownOutcomeAfterInflightRequestCancellation(t *testing.T) {
	requestStarted := make(chan string, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestStarted <- request.Header.Get("Idempotency-Key")
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	store := &contextAwareEmailAuditStore{}
	sender := NewAuditedClient(NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	}), store)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := sender.SendTransactionalWithAttempt(ctx, TransactionalEmail{
			TransactionalID: "tmpl_restriction",
			Email:           "owner@example.com",
			IdempotencyKey:  "cycle_1:restriction_notice:user_1",
			Audit: EmailAudit{
				EventKey:              "email.publishing_restriction.restriction_notice.v1",
				AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
			},
		}, nil)
		errCh <- err
	}()

	if providerKey := <-requestStarted; providerKey != "cycle_1:restriction_notice:user_1" {
		t.Fatalf("provider idempotency key = %q", providerKey)
	}
	cancel()
	err := <-errCh
	if !IsSendOutcomeUnknown(err) {
		t.Fatalf("send error = %v, want unknown outcome", err)
	}
	if store.status != "failed" {
		t.Fatalf("audit status = %q, want failed after request cancellation", store.status)
	}
	if store.failedContextErr != nil || !store.failedBounded {
		t.Fatalf("failed audit context error=%v bounded=%v, want live bounded context", store.failedContextErr, store.failedBounded)
	}
	if !errors.Is(store.failedContext.Err(), context.Canceled) {
		t.Fatalf("failed audit context was not released after finalization: %v", store.failedContext.Err())
	}
	if store.attempt.IdempotencyKey != "cycle_1:restriction_notice:user_1:g1:a1" {
		t.Fatalf("audit attempt key = %q", store.attempt.IdempotencyKey)
	}
	if !strings.Contains(store.failedReason, "provider request outcome unknown") || !strings.Contains(store.failedReason, "context canceled") {
		t.Fatalf("audit failure evidence = %q", store.failedReason)
	}
}

func TestAuditedClientKeepsExplicitProviderFailureAfterInflightRequestCancellation(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	releaseResponse := make(chan struct{})
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requestStarted <- struct{}{}
		<-releaseResponse
		return jsonResponse(http.StatusServiceUnavailable, `{"message":"temporarily unavailable"}`), nil
	})}
	store := &contextAwareEmailAuditStore{}
	sender := NewAuditedClient(NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	}), store)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := sender.SendTransactionalWithAttempt(ctx, TransactionalEmail{
			TransactionalID: "tmpl_restriction",
			Email:           "owner@example.com",
			IdempotencyKey:  "cycle_1:restriction_notice:user_1",
			Audit: EmailAudit{
				EventKey:              "email.publishing_restriction.restriction_notice.v1",
				AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
			},
		}, nil)
		errCh <- err
	}()

	<-requestStarted
	cancel()
	close(releaseResponse)
	err := <-errCh
	if err == nil || IsSendOutcomeUnknown(err) {
		t.Fatalf("send error = %v, want explicit provider failure", err)
	}
	if store.status != "failed" {
		t.Fatalf("audit status = %q, want failed", store.status)
	}
	if !strings.Contains(store.failedReason, "503") || !strings.Contains(store.failedReason, "temporarily unavailable") {
		t.Fatalf("audit failure evidence = %q", store.failedReason)
	}
	if strings.Contains(store.failedReason, "outcome unknown") {
		t.Fatalf("explicit provider failure mislabeled unknown: %q", store.failedReason)
	}
}

func TestAuditedClientFinalizesSuccessfulProviderResponseAfterInflightRequestCancellation(t *testing.T) {
	requestStarted := make(chan struct{}, 1)
	releaseResponse := make(chan struct{})
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		requestStarted <- struct{}{}
		<-releaseResponse
		return jsonResponse(http.StatusOK, `{"success":true}`), nil
	})}
	store := &contextAwareEmailAuditStore{}
	sender := NewAuditedClient(NewClient(Config{
		APIKey:  "test-key",
		BaseURL: "https://loops.test/api",
		Client:  httpClient,
	}), store)
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		_, err := sender.SendTransactionalWithAttempt(ctx, TransactionalEmail{
			TransactionalID: "tmpl_restriction",
			Email:           "owner@example.com",
			IdempotencyKey:  "cycle_1:restriction_notice:user_1",
			Audit: EmailAudit{
				EventKey:              "email.publishing_restriction.restriction_notice.v1",
				AttemptIdempotencyKey: "cycle_1:restriction_notice:user_1:g1:a1",
			},
		}, nil)
		errCh <- err
	}()

	<-requestStarted
	cancel()
	close(releaseResponse)
	if err := <-errCh; err != nil {
		t.Fatalf("send: %v", err)
	}
	if store.status != "sent" {
		t.Fatalf("audit status = %q, want sent", store.status)
	}
	if store.sentContextErr != nil || !store.sentBounded {
		t.Fatalf("sent audit context error=%v bounded=%v, want live bounded context", store.sentContextErr, store.sentBounded)
	}
	if !errors.Is(store.sentContext.Err(), context.Canceled) {
		t.Fatalf("sent audit context was not released after finalization: %v", store.sentContext.Err())
	}
}

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
