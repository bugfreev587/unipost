package worker

import (
	"context"
	"errors"
	"fmt"
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
	AttemptCount            int
	AttemptGeneration       int
}

type PublishingRestrictionEmailStore interface {
	ClaimPublishingRestrictionEmailRecipients(context.Context, int) ([]PublishingRestrictionEmailWork, error)
	PublishingRestrictionEmailRecipientEligible(context.Context, PublishingRestrictionEmailWork) (bool, error)
	LinkPublishingRestrictionEmailAttempt(context.Context, string, string, int, int) error
	MarkPublishingRestrictionEmailRecipientSent(context.Context, string) error
	MarkPublishingRestrictionEmailRecipientFailed(context.Context, string, string) error
	MarkPublishingRestrictionEmailRecipientTerminalFailed(context.Context, string, string) error
	MarkPublishingRestrictionEmailRecipientSkipped(context.Context, string, string) error
	RefreshPublishingRestrictionEmailCampaign(context.Context, string) error
}

type transactionalEmailSender interface {
	SendTransactionalWithAttempt(
		context.Context,
		loops.TransactionalEmail,
		func(context.Context, string) error,
	) (loops.EmailSendAttemptRecord, error)
}

type PublishingRestrictionEmailWorker struct {
	store                 PublishingRestrictionEmailStore
	sender                transactionalEmailSender
	restrictionTemplateID string
	recoveryTemplateID    string
	readiness             publishingrestrictions.CampaignDeliveryReadiness
	batchSize             int
}

const publishingRestrictionEmailUnknownOutcomeCleanupTimeout = 5 * time.Second

var ErrPublishingRestrictionEmailNotConfigured = errors.New("publishing restriction email worker is not configured")

func NewPublishingRestrictionEmailWorker(
	store PublishingRestrictionEmailStore,
	sender transactionalEmailSender,
	restrictionTemplateID string,
	recoveryTemplateID string,
	readiness publishingrestrictions.CampaignDeliveryReadiness,
) *PublishingRestrictionEmailWorker {
	return &PublishingRestrictionEmailWorker{
		store: store, sender: sender,
		restrictionTemplateID: strings.TrimSpace(restrictionTemplateID),
		recoveryTemplateID:    strings.TrimSpace(recoveryTemplateID),
		readiness:             readiness,
		batchSize:             50,
	}
}

func (w *PublishingRestrictionEmailWorker) Start(ctx context.Context) {
	if !w.configured() {
		slog.Warn("publishing restriction email worker is not configured")
		return
	}
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
	if !w.configured() {
		return ErrPublishingRestrictionEmailNotConfigured
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
		renderedBody := renderPublishingRestrictionEmailBody(recipient.BodySnapshot, firstName)
		_, err := w.sender.SendTransactionalWithAttempt(ctx, loops.TransactionalEmail{
			TransactionalID: templateID,
			Email:           recipient.RecipientEmail,
			UserID:          recipient.CanonicalUserID,
			IdempotencyKey:  recipient.IdempotencyKey,
			DataVariables: map[string]any{
				"subject": recipient.SubjectSnapshot,
				"body":    renderedBody,
			},
			Audit: loops.EmailAudit{
				EventKey: eventKey, WorkspaceID: workspaceID, Provider: "loops",
				DeliveryClass: "service_alert", TriggerSource: "admin_confirmed_campaign",
				TriggerReferenceID: recipient.RecipientID, Subject: recipient.SubjectSnapshot,
				AttemptIdempotencyKey: fmt.Sprintf(
					"%s:g%d:a%d",
					recipient.IdempotencyKey,
					recipient.AttemptGeneration,
					recipient.AttemptCount,
				),
			},
		}, func(ctx context.Context, attemptID string) error {
			return w.store.LinkPublishingRestrictionEmailAttempt(
				ctx,
				recipient.RecipientID,
				attemptID,
				recipient.AttemptCount,
				recipient.AttemptGeneration,
			)
		})
		if err != nil {
			if loops.IsSendOutcomeUnknown(err) {
				cleanupCtx, cancelCleanup := context.WithTimeout(
					context.WithoutCancel(ctx),
					publishingRestrictionEmailUnknownOutcomeCleanupTimeout,
				)
				terminalErr := w.store.MarkPublishingRestrictionEmailRecipientTerminalFailed(
					cleanupCtx,
					recipient.RecipientID,
					"manual review required: "+err.Error(),
				)
				if terminalErr != nil {
					terminalErr = fmt.Errorf("terminalize unknown publishing restriction email outcome: %w", terminalErr)
				}
				refreshErr := w.store.RefreshPublishingRestrictionEmailCampaign(cleanupCtx, recipient.CampaignID)
				if refreshErr != nil {
					refreshErr = fmt.Errorf("refresh publishing restriction email campaign after unknown outcome: %w", refreshErr)
				}
				cancelCleanup()
				if cleanupErr := errors.Join(terminalErr, refreshErr); cleanupErr != nil {
					return cleanupErr
				}
				continue
			} else {
				_ = w.store.MarkPublishingRestrictionEmailRecipientFailed(ctx, recipient.RecipientID, err.Error())
			}
		} else {
			finalizeCtx, cancelFinalize := context.WithTimeout(
				context.WithoutCancel(ctx),
				publishingRestrictionEmailUnknownOutcomeCleanupTimeout,
			)
			sentErr := w.store.MarkPublishingRestrictionEmailRecipientSent(finalizeCtx, recipient.RecipientID)
			cancelFinalize()
			if sentErr != nil {
				return fmt.Errorf("mark publishing restriction email recipient sent: %w", sentErr)
			}
			continue
		}
		_ = w.store.RefreshPublishingRestrictionEmailCampaign(ctx, recipient.CampaignID)
	}
	return nil
}

func (w *PublishingRestrictionEmailWorker) configured() bool {
	return w != nil && w.readiness.Ready() && w.store != nil && w.sender != nil &&
		w.restrictionTemplateID != "" && w.recoveryTemplateID != ""
}

func renderPublishingRestrictionEmailBody(body, firstName string) string {
	return strings.ReplaceAll(body, "{{first_name}}", firstName)
}
