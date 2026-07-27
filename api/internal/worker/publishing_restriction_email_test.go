package worker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

func TestPublishingRestrictionEligibilityRechecksEveryRepresentedWorkspace(t *testing.T) {
	if strings.Contains(publishingRestrictionRecipientEligibilitySQL, "member.user_id=$4") {
		t.Fatalf("eligibility must not discard represented workspaces owned by another user sharing the email:\n%s", publishingRestrictionRecipientEligibilitySQL)
	}
	for _, want := range []string{"UNNEST($5::text[])", "account.workspace_id=represented.workspace_id", "LOWER(TRIM(owner_user.email))=$4"} {
		if !strings.Contains(publishingRestrictionRecipientEligibilitySQL, want) {
			t.Fatalf("eligibility query missing %q:\n%s", want, publishingRestrictionRecipientEligibilitySQL)
		}
	}
}

type fakeRestrictionCampaignEmailStore struct {
	work            []PublishingRestrictionEmailWork
	claimCalls      int
	sent            []string
	failed          []string
	terminalFailed  []string
	skipped         []string
	refreshed       []string
	linked          map[string]string
	eligible        bool
	terminalErr     error
	refreshErr      error
	terminalCtx     context.Context
	refreshCtx      context.Context
	terminalCtxErr  error
	refreshCtxErr   error
	terminalBounded bool
	refreshBounded  bool
}

func (f *fakeRestrictionCampaignEmailStore) ClaimPublishingRestrictionEmailRecipients(context.Context, int) ([]PublishingRestrictionEmailWork, error) {
	f.claimCalls++
	work := f.work
	f.work = nil
	return work, nil
}

func readyPublishingRestrictionCampaignDelivery() publishingrestrictions.CampaignDeliveryReadiness {
	return publishingrestrictions.CampaignDeliveryReadiness{
		PreviewSecret: true, AuditedSender: true, RestrictionTemplate: true, RecoveryTemplate: true,
	}
}

func TestPublishingRestrictionEmailWorkerRejectsUnreadyConfigBeforeClaim(t *testing.T) {
	tests := []struct {
		name      string
		readiness publishingrestrictions.CampaignDeliveryReadiness
	}{
		{name: "preview secret", readiness: publishingrestrictions.CampaignDeliveryReadiness{AuditedSender: true, RestrictionTemplate: true, RecoveryTemplate: true}},
		{name: "audited sender", readiness: publishingrestrictions.CampaignDeliveryReadiness{PreviewSecret: true, RestrictionTemplate: true, RecoveryTemplate: true}},
		{name: "restriction template", readiness: publishingrestrictions.CampaignDeliveryReadiness{PreviewSecret: true, AuditedSender: true, RecoveryTemplate: true}},
		{name: "recovery template", readiness: publishingrestrictions.CampaignDeliveryReadiness{PreviewSecret: true, AuditedSender: true, RestrictionTemplate: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeRestrictionCampaignEmailStore{}
			worker := NewPublishingRestrictionEmailWorker(store, &captureRestrictionCampaignSender{}, "restriction-template", "recovery-template", tt.readiness)
			err := worker.ProcessBatch(context.Background())
			if !errors.Is(err, ErrPublishingRestrictionEmailNotConfigured) {
				t.Fatalf("error = %v, want ErrPublishingRestrictionEmailNotConfigured", err)
			}
			if store.claimCalls != 0 {
				t.Fatalf("claim calls = %d, want 0", store.claimCalls)
			}
		})
	}
}

func TestPublishingRestrictionEmailWorkerStartLogsOneWarningAndReturnsWhenUnready(t *testing.T) {
	store := &fakeRestrictionCampaignEmailStore{}
	worker := NewPublishingRestrictionEmailWorker(
		store,
		&captureRestrictionCampaignSender{},
		"restriction-template",
		"recovery-template",
		publishingrestrictions.CampaignDeliveryReadiness{},
	)
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		worker.Start(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("unready worker did not exit immediately")
	}

	if store.claimCalls != 0 {
		t.Fatalf("claim calls = %d, want 0", store.claimCalls)
	}
	if count := strings.Count(logs.String(), "publishing restriction email worker is not configured"); count != 1 {
		t.Fatalf("warning count = %d, want 1; logs=%q", count, logs.String())
	}
}

func (f *fakeRestrictionCampaignEmailStore) PublishingRestrictionEmailRecipientEligible(context.Context, PublishingRestrictionEmailWork) (bool, error) {
	return f.eligible, nil
}

func (f *fakeRestrictionCampaignEmailStore) LinkPublishingRestrictionEmailAttempt(_ context.Context, recipientID, attemptID string, attemptCount, attemptGeneration int) error {
	if attemptCount <= 0 || attemptGeneration <= 0 {
		return errors.New("invalid attempt identity")
	}
	if f.linked == nil {
		f.linked = map[string]string{}
	}
	f.linked[recipientID] = attemptID
	return nil
}

func (f *fakeRestrictionCampaignEmailStore) MarkPublishingRestrictionEmailRecipientSent(_ context.Context, recipientID string) error {
	f.sent = append(f.sent, recipientID)
	return nil
}

func (f *fakeRestrictionCampaignEmailStore) MarkPublishingRestrictionEmailRecipientFailed(_ context.Context, recipientID, _ string) error {
	f.failed = append(f.failed, recipientID)
	return nil
}

func (f *fakeRestrictionCampaignEmailStore) MarkPublishingRestrictionEmailRecipientTerminalFailed(ctx context.Context, recipientID, _ string) error {
	f.terminalFailed = append(f.terminalFailed, recipientID)
	f.terminalCtx = ctx
	f.terminalCtxErr = ctx.Err()
	_, f.terminalBounded = ctx.Deadline()
	return f.terminalErr
}

func (f *fakeRestrictionCampaignEmailStore) MarkPublishingRestrictionEmailRecipientSkipped(_ context.Context, recipientID, _ string) error {
	f.skipped = append(f.skipped, recipientID)
	return nil
}

func (f *fakeRestrictionCampaignEmailStore) RefreshPublishingRestrictionEmailCampaign(ctx context.Context, campaignID string) error {
	f.refreshed = append(f.refreshed, campaignID)
	f.refreshCtx = ctx
	f.refreshCtxErr = ctx.Err()
	_, f.refreshBounded = ctx.Deadline()
	return f.refreshErr
}

type captureRestrictionCampaignSender struct {
	emails     []loops.TransactionalEmail
	attemptIDs []string
	err        error
	afterSend  func()
}

func (s *captureRestrictionCampaignSender) SendTransactionalWithAttempt(
	ctx context.Context,
	email loops.TransactionalEmail,
	beforeSend func(context.Context, string) error,
) (loops.EmailSendAttemptRecord, error) {
	attemptID := "attempt_1"
	if err := beforeSend(ctx, attemptID); err != nil {
		return loops.EmailSendAttemptRecord{}, err
	}
	s.emails = append(s.emails, email)
	s.attemptIDs = append(s.attemptIDs, attemptID)
	if s.afterSend != nil {
		s.afterSend()
	}
	return loops.EmailSendAttemptRecord{ID: attemptID}, s.err
}

func TestPublishingRestrictionEmailWorkerUsesExactCopyAndStableAuditIdentity(t *testing.T) {
	copy := publishingrestrictions.CampaignCopyFor(publishingrestrictions.RestrictionNotice)
	store := &fakeRestrictionCampaignEmailStore{eligible: true, work: []PublishingRestrictionEmailWork{{
		RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
		CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
		RecipientEmail: "owner@example.com", FirstName: "Alex", IdempotencyKey: "cycle_1:restriction_notice:user_1",
		SubjectSnapshot: copy.Subject, BodySnapshot: copy.Body, AttemptCount: 1, AttemptGeneration: 1,
	}}}
	sender := &captureRestrictionCampaignSender{}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 1 || len(store.sent) != 1 {
		t.Fatalf("emails=%d sent=%v", len(sender.emails), store.sent)
	}
	email := sender.emails[0]
	if email.TransactionalID != "restriction-template" || email.IdempotencyKey != "cycle_1:restriction_notice:user_1" {
		t.Fatalf("email identity=%+v", email)
	}
	renderedBody, _ := email.DataVariables["body"].(string)
	if strings.Contains(renderedBody, "{{first_name}}") || !strings.HasPrefix(renderedBody, "Hi Alex,") {
		t.Fatalf("variables=%+v", email.DataVariables)
	}
	if email.DataVariables["subject"] != copy.Subject {
		t.Fatalf("subject variable=%+v", email.DataVariables)
	}
	if _, exists := email.DataVariables["first_name"]; exists {
		t.Fatalf("first_name must be rendered into immutable body before Loops: %+v", email.DataVariables)
	}
	if email.Audit.EventKey != "email.publishing_restriction.restriction_notice.v1" || email.Audit.TriggerReferenceID != "recipient_1" || email.Audit.DeliveryClass != "service_alert" {
		t.Fatalf("audit=%+v", email.Audit)
	}
	if email.Audit.AttemptIdempotencyKey != "cycle_1:restriction_notice:user_1:g1:a1" {
		t.Fatalf("audit attempt key=%q", email.Audit.AttemptIdempotencyKey)
	}
	if store.linked["recipient_1"] != "attempt_1" {
		t.Fatalf("recipient audit linkage=%v", store.linked)
	}
}

func TestPublishingRestrictionEmailWorkerSkipsIneligibleRecipient(t *testing.T) {
	store := &fakeRestrictionCampaignEmailStore{eligible: false, work: []PublishingRestrictionEmailWork{{RecipientID: "recipient_1", CampaignID: "campaign_1"}}}
	sender := &captureRestrictionCampaignSender{}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 0 || len(store.skipped) != 1 {
		t.Fatalf("emails=%d skipped=%v", len(sender.emails), store.skipped)
	}
}

func TestPublishingRestrictionEmailWorkerTerminalizesUnknownSendOutcome(t *testing.T) {
	store := &fakeRestrictionCampaignEmailStore{eligible: true, work: []PublishingRestrictionEmailWork{{
		RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
		CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
		RecipientEmail: "owner@example.com", IdempotencyKey: "cycle_1:restriction_notice:user_1",
		SubjectSnapshot: "subject", BodySnapshot: "body", AttemptCount: 1, AttemptGeneration: 1,
	}}}
	sender := &captureRestrictionCampaignSender{err: fmt.Errorf(
		"audited provider send failed: %w",
		&loops.SendOutcomeUnknownError{Err: errors.New("response lost")},
	)}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 1 {
		t.Fatalf("sender attempts = %d, want exactly 1", len(sender.emails))
	}
	if store.linked["recipient_1"] != "attempt_1" {
		t.Fatalf("recipient audit linkage = %v, want attempt_1", store.linked)
	}
	if len(store.terminalFailed) != 1 || store.terminalFailed[0] != "recipient_1" {
		t.Fatalf("terminal failures = %v, want recipient_1", store.terminalFailed)
	}
	if len(store.failed) != 0 || len(store.sent) != 0 {
		t.Fatalf("bounded failures=%v sent=%v, want neither", store.failed, store.sent)
	}
	if len(store.refreshed) != 1 || store.refreshed[0] != "campaign_1" {
		t.Fatalf("campaign refreshes = %v, want campaign_1", store.refreshed)
	}
}

func TestPublishingRestrictionEmailWorkerBoundedRetriesExplicitProviderFailure(t *testing.T) {
	store := &fakeRestrictionCampaignEmailStore{eligible: true, work: []PublishingRestrictionEmailWork{{
		RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
		CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
		RecipientEmail: "owner@example.com", IdempotencyKey: "cycle_1:restriction_notice:user_1",
		SubjectSnapshot: "subject", BodySnapshot: "body", AttemptCount: 1, AttemptGeneration: 1,
	}}}
	sender := &captureRestrictionCampaignSender{err: errors.New("loops: 503: temporarily unavailable")}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 1 {
		t.Fatalf("sender attempts = %d, want exactly 1", len(sender.emails))
	}
	if len(store.failed) != 1 || store.failed[0] != "recipient_1" {
		t.Fatalf("bounded failures = %v, want recipient_1", store.failed)
	}
	if len(store.terminalFailed) != 0 || len(store.sent) != 0 {
		t.Fatalf("terminal failures=%v sent=%v, want neither", store.terminalFailed, store.sent)
	}
	if len(store.refreshed) != 1 || store.refreshed[0] != "campaign_1" {
		t.Fatalf("campaign refreshes = %v, want campaign_1", store.refreshed)
	}
}

func TestPublishingRestrictionEmailWorkerReturnsUnknownOutcomeCleanupFailures(t *testing.T) {
	terminalErr := errors.New("terminal transition failed")
	refreshErr := errors.New("campaign refresh failed")
	tests := []struct {
		name        string
		terminalErr error
		refreshErr  error
		wantErrors  []error
	}{
		{name: "terminal transition", terminalErr: terminalErr, wantErrors: []error{terminalErr}},
		{name: "campaign refresh", refreshErr: refreshErr, wantErrors: []error{refreshErr}},
		{name: "both", terminalErr: terminalErr, refreshErr: refreshErr, wantErrors: []error{terminalErr, refreshErr}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeRestrictionCampaignEmailStore{
				eligible: true, terminalErr: tt.terminalErr, refreshErr: tt.refreshErr,
				work: []PublishingRestrictionEmailWork{{
					RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
					CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
					RecipientEmail: "owner@example.com", IdempotencyKey: "cycle_1:restriction_notice:user_1",
					SubjectSnapshot: "subject", BodySnapshot: "body", AttemptCount: 1, AttemptGeneration: 1,
				}},
			}
			sender := &captureRestrictionCampaignSender{err: &loops.SendOutcomeUnknownError{Err: errors.New("response lost")}}
			worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

			err := worker.ProcessBatch(context.Background())
			if err == nil {
				t.Fatal("ProcessBatch returned nil, want unknown-outcome cleanup failure")
			}
			for _, wantErr := range tt.wantErrors {
				if !errors.Is(err, wantErr) {
					t.Fatalf("ProcessBatch error = %v, want errors.Is(_, %v)", err, wantErr)
				}
			}
			if len(sender.emails) != 1 || len(store.terminalFailed) != 1 || len(store.refreshed) != 1 {
				t.Fatalf("sends=%d terminal=%v refreshes=%v, want one of each", len(sender.emails), store.terminalFailed, store.refreshed)
			}
		})
	}
}

func TestPublishingRestrictionEmailWorkerPersistsUnknownOutcomeAfterParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeRestrictionCampaignEmailStore{eligible: true, work: []PublishingRestrictionEmailWork{{
		RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
		CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
		RecipientEmail: "owner@example.com", IdempotencyKey: "cycle_1:restriction_notice:user_1",
		SubjectSnapshot: "subject", BodySnapshot: "body", AttemptCount: 1, AttemptGeneration: 1,
	}}}
	sender := &captureRestrictionCampaignSender{
		err:       &loops.SendOutcomeUnknownError{Err: errors.New("response lost")},
		afterSend: cancel,
	}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", readyPublishingRestrictionCampaignDelivery())

	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("parent context error = %v, want canceled", ctx.Err())
	}
	if store.terminalCtxErr != nil || store.refreshCtxErr != nil {
		t.Fatalf("cleanup context errors: terminal=%v refresh=%v, want live contexts", store.terminalCtxErr, store.refreshCtxErr)
	}
	if !store.terminalBounded || !store.refreshBounded {
		t.Fatalf("cleanup context deadlines: terminal=%v refresh=%v, want both bounded", store.terminalBounded, store.refreshBounded)
	}
	if !errors.Is(store.terminalCtx.Err(), context.Canceled) || !errors.Is(store.refreshCtx.Err(), context.Canceled) {
		t.Fatalf("cleanup contexts were not canceled after persistence: terminal=%v refresh=%v", store.terminalCtx.Err(), store.refreshCtx.Err())
	}
	if len(sender.emails) != 1 || len(store.terminalFailed) != 1 || len(store.refreshed) != 1 {
		t.Fatalf("sends=%d terminal=%v refreshes=%v, want one of each", len(sender.emails), store.terminalFailed, store.refreshed)
	}
}

func TestPublishingRestrictionEmailClaimNeverRecyclesStaleSendingAutomatically(t *testing.T) {
	source, err := os.ReadFile("publishing_restriction_email_postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if strings.Contains(text, "OR (recipient.status='sending'") {
		t.Fatal("stale sending recipients must not be reclaimed for a second network send")
	}
	for _, want := range []string{
		"publishingRestrictionEmailMaxAttempts",
		"retryable = FALSE",
		"email_send_attempt_id",
		"LinkPublishingRestrictionEmailAttempt",
		"attempt_generation",
		"email_send_attempt_id=NULL",
		"RETURNING recipient.campaign_id",
		"publishingRestrictionEmailCampaignRefreshSQL",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("durable send gate missing %q", want)
		}
	}
}
