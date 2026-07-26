package worker

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

type PublishingRestrictionEmailWork struct {
	RecipientID             string
	CampaignID              string
	CycleID                 string
	CampaignType            publishingrestrictions.CampaignType
	Platform                string
	CanonicalUserID         string
	RecipientEmail          string
	NormalizedEmail         string
	FirstName               string
	RepresentedWorkspaceIDs []string
	IdempotencyKey          string
	SubjectSnapshot         string
	BodySnapshot            string
}

type PublishingRestrictionEmailStore interface {
	ClaimPublishingRestrictionEmailRecipients(context.Context, int) ([]PublishingRestrictionEmailWork, error)
	PublishingRestrictionEmailRecipientEligible(context.Context, PublishingRestrictionEmailWork) (bool, error)
	MarkPublishingRestrictionEmailRecipientSent(context.Context, string) error
	MarkPublishingRestrictionEmailRecipientFailed(context.Context, string, string) error
	MarkPublishingRestrictionEmailRecipientSkipped(context.Context, string, string) error
	RefreshPublishingRestrictionEmailCampaign(context.Context, string) error
}

type transactionalEmailSender interface {
	SendTransactional(context.Context, loops.TransactionalEmail) error
}

type PublishingRestrictionEmailWorker struct {
	store                 PublishingRestrictionEmailStore
	sender                transactionalEmailSender
	restrictionTemplateID string
	recoveryTemplateID    string
	batchSize             int
}

func NewPublishingRestrictionEmailWorker(
	store PublishingRestrictionEmailStore,
	sender transactionalEmailSender,
	restrictionTemplateID string,
	recoveryTemplateID string,
) *PublishingRestrictionEmailWorker {
	return &PublishingRestrictionEmailWorker{
		store: store, sender: sender,
		restrictionTemplateID: strings.TrimSpace(restrictionTemplateID),
		recoveryTemplateID:    strings.TrimSpace(recoveryTemplateID),
		batchSize:             50,
	}
}

func (w *PublishingRestrictionEmailWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.ProcessBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("publishing restriction email worker failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *PublishingRestrictionEmailWorker) ProcessBatch(ctx context.Context) error {
	if w == nil || w.store == nil {
		return errors.New("publishing restriction email worker store is not configured")
	}
	work, err := w.store.ClaimPublishingRestrictionEmailRecipients(ctx, w.batchSize)
	if err != nil {
		return err
	}
	for _, recipient := range work {
		eligible, eligibilityErr := w.store.PublishingRestrictionEmailRecipientEligible(ctx, recipient)
		if eligibilityErr != nil {
			_ = w.store.MarkPublishingRestrictionEmailRecipientFailed(ctx, recipient.RecipientID, eligibilityErr.Error())
			_ = w.store.RefreshPublishingRestrictionEmailCampaign(ctx, recipient.CampaignID)
			continue
		}
		if !eligible {
			_ = w.store.MarkPublishingRestrictionEmailRecipientSkipped(ctx, recipient.RecipientID, "recipient no longer eligible")
			_ = w.store.RefreshPublishingRestrictionEmailCampaign(ctx, recipient.CampaignID)
			continue
		}
		templateID := w.restrictionTemplateID
		eventKey := "email.publishing_restriction.restriction_notice.v1"
		if recipient.CampaignType == publishingrestrictions.RecoveryNotice {
			templateID = w.recoveryTemplateID
			eventKey = "email.publishing_restriction.recovery_notice.v1"
		}
		if w.sender == nil || templateID == "" {
			_ = w.store.MarkPublishingRestrictionEmailRecipientFailed(ctx, recipient.RecipientID, "audited email sender or transactional template is not configured")
			_ = w.store.RefreshPublishingRestrictionEmailCampaign(ctx, recipient.CampaignID)
			continue
		}
		firstName := strings.TrimSpace(recipient.FirstName)
		if firstName == "" {
			firstName = "there"
		}
		workspaceID := ""
		if len(recipient.RepresentedWorkspaceIDs) > 0 {
			workspaceID = recipient.RepresentedWorkspaceIDs[0]
		}
		err := w.sender.SendTransactional(ctx, loops.TransactionalEmail{
			TransactionalID: templateID,
			Email:           recipient.RecipientEmail,
			UserID:          recipient.CanonicalUserID,
			IdempotencyKey:  recipient.IdempotencyKey,
			DataVariables: map[string]any{
				"first_name": firstName,
				"subject":    recipient.SubjectSnapshot,
				"body":       recipient.BodySnapshot,
			},
			Audit: loops.EmailAudit{
				EventKey: eventKey, WorkspaceID: workspaceID, Provider: "loops",
				DeliveryClass: "service_alert", TriggerSource: "admin_confirmed_campaign",
				TriggerReferenceID: recipient.RecipientID, Subject: recipient.SubjectSnapshot,
			},
		})
		if err != nil {
			_ = w.store.MarkPublishingRestrictionEmailRecipientFailed(ctx, recipient.RecipientID, err.Error())
		} else {
			_ = w.store.MarkPublishingRestrictionEmailRecipientSent(ctx, recipient.RecipientID)
		}
		_ = w.store.RefreshPublishingRestrictionEmailCampaign(ctx, recipient.CampaignID)
	}
	return nil
}
