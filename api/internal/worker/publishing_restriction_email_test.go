package worker

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

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
	work     []PublishingRestrictionEmailWork
	sent     []string
	failed   []string
	skipped  []string
	linked   map[string]string
	eligible bool
}

func (f *fakeRestrictionCampaignEmailStore) ClaimPublishingRestrictionEmailRecipients(context.Context, int) ([]PublishingRestrictionEmailWork, error) {
	work := f.work
	f.work = nil
	return work, nil
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

func (f *fakeRestrictionCampaignEmailStore) MarkPublishingRestrictionEmailRecipientSkipped(_ context.Context, recipientID, _ string) error {
	f.skipped = append(f.skipped, recipientID)
	return nil
}

func (f *fakeRestrictionCampaignEmailStore) RefreshPublishingRestrictionEmailCampaign(context.Context, string) error {
	return nil
}

type captureRestrictionCampaignSender struct {
	emails     []loops.TransactionalEmail
	attemptIDs []string
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
	return loops.EmailSendAttemptRecord{ID: attemptID}, nil
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
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template")

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
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template")

	if err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 0 || len(store.skipped) != 1 {
		t.Fatalf("emails=%d skipped=%v", len(sender.emails), store.skipped)
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
