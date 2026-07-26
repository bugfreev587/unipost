package worker

import (
	"context"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type fakeRestrictionCampaignEmailStore struct {
	work     []PublishingRestrictionEmailWork
	sent     []string
	failed   []string
	skipped  []string
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

type captureRestrictionCampaignSender struct{ emails []loops.TransactionalEmail }

func (s *captureRestrictionCampaignSender) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	s.emails = append(s.emails, email)
	return nil
}

func TestPublishingRestrictionEmailWorkerUsesExactCopyAndStableAuditIdentity(t *testing.T) {
	copy := publishingrestrictions.CampaignCopyFor(publishingrestrictions.RestrictionNotice)
	store := &fakeRestrictionCampaignEmailStore{eligible: true, work: []PublishingRestrictionEmailWork{{
		RecipientID: "recipient_1", CampaignID: "campaign_1", CycleID: "cycle_1",
		CampaignType: publishingrestrictions.RestrictionNotice, CanonicalUserID: "user_1",
		RecipientEmail: "owner@example.com", FirstName: "Alex", IdempotencyKey: "cycle_1:restriction_notice:user_1",
		SubjectSnapshot: copy.Subject, BodySnapshot: copy.Body,
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
	if email.DataVariables["first_name"] != "Alex" || email.DataVariables["body"] != copy.Body {
		t.Fatalf("variables=%+v", email.DataVariables)
	}
	if email.Audit.EventKey != "email.publishing_restriction.restriction_notice.v1" || email.Audit.TriggerReferenceID != "recipient_1" || email.Audit.DeliveryClass != "service_alert" {
		t.Fatalf("audit=%+v", email.Audit)
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
