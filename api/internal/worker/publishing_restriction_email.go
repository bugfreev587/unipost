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
	CampaignCreatedAt       time.Time
	Platform                string
	CanonicalUserID         string
	RecipientEmail          string
	NormalizedEmail         string
	FirstName               string
	RepresentedWorkspaceIDs []string
	RepresentedOwnerUserIDs []string
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
		if recipient.AttemptGeneration > 1 && !recipient.CampaignCreatedAt.IsZero() && time.Since(recipient.CampaignCreatedAt) >= 24*time.Hour {
			if persistErr := w.finalizeRecipientAndCampaign(ctx, recipient.CampaignID, "expire publishing restriction email retry outside provider idempotency window", func(finalizeCtx context.Context) error {
				return w.store.MarkPublishingRestrictionEmailRecipientTerminalFailed(finalizeCtx, recipient.RecipientID, "manual retry refused: provider idempotency window expired")
			}); persistErr != nil {
				return persistErr
			}
			continue
		}
		eligible, eligibilityErr := w.store.PublishingRestrictionEmailRecipientEligible(ctx, recipient)
		if eligibilityErr != nil {
			if persistErr := w.finalizeRecipientAndCampaign(ctx, recipient.CampaignID, "persist publishing restriction email eligibility failure", func(finalizeCtx context.Context) error {
				return w.store.MarkPublishingRestrictionEmailRecipientFailed(finalizeCtx, recipient.RecipientID, eligibilityErr.Error())
			}); persistErr != nil {
				return persistErr
			}
			continue
		}
		if !eligible {
			if persistErr := w.finalizeRecipientAndCampaign(ctx, recipient.CampaignID, "persist ineligible publishing restriction email recipient", func(finalizeCtx context.Context) error {
				return w.store.MarkPublishingRestrictionEmailRecipientSkipped(finalizeCtx, recipient.RecipientID, "recipient no longer eligible")
			}); persistErr != nil {
				return persistErr
			}
			continue
		}
		templateID := w.restrictionTemplateID
		eventKey := "email.publishing_restriction.restriction_notice.v1"
		if recipient.CampaignType == publishingrestrictions.RecoveryNotice {
			templateID = w.recoveryTemplateID
			eventKey = "email.publishing_restriction.recovery_notice.v1"
		}
		if w.sender == nil || templateID == "" {
			if persistErr := w.finalizeRecipientAndCampaign(ctx, recipient.CampaignID, "persist unconfigured publishing restriction email failure", func(finalizeCtx context.Context) error {
				return w.store.MarkPublishingRestrictionEmailRecipientFailed(finalizeCtx, recipient.RecipientID, "audited email sender or transactional template is not configured")
			}); persistErr != nil {
				return persistErr
			}
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
			if loops.IsEmailAuditFinalizationError(err) {
				return fmt.Errorf("publishing restriction email audit finalization: %w", err)
			}
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
				if persistErr := w.finalizeRecipientAndCampaign(ctx, recipient.CampaignID, "persist definitive publishing restriction email failure", func(finalizeCtx context.Context) error {
					return w.store.MarkPublishingRestrictionEmailRecipientFailed(finalizeCtx, recipient.RecipientID, err.Error())
				}); persistErr != nil {
					return persistErr
				}
				continue
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
	}
	return nil
}

func (w *PublishingRestrictionEmailWorker) finalizeRecipientAndCampaign(
	ctx context.Context,
	campaignID string,
	transitionLabel string,
	transition func(context.Context) error,
) error {
	finalizeCtx, cancelFinalize := context.WithTimeout(
		context.WithoutCancel(ctx),
		publishingRestrictionEmailUnknownOutcomeCleanupTimeout,
	)
	transitionErr := transition(finalizeCtx)
	if transitionErr != nil {
		transitionErr = fmt.Errorf("%s: %w", transitionLabel, transitionErr)
	}
	refreshErr := w.store.RefreshPublishingRestrictionEmailCampaign(finalizeCtx, campaignID)
	if refreshErr != nil {
		refreshErr = fmt.Errorf("refresh publishing restriction email campaign: %w", refreshErr)
	}
	cancelFinalize()
	return errors.Join(transitionErr, refreshErr)
}

func (w *PublishingRestrictionEmailWorker) configured() bool {
	return w != nil && w.readiness.Ready() && w.store != nil && w.sender != nil &&
		w.restrictionTemplateID != "" && w.recoveryTemplateID != ""
}

func renderPublishingRestrictionEmailBody(body, firstName string) string {
	return strings.ReplaceAll(body, "{{first_name}}", firstName)
}
