package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
)

type loopsLifecycleSyncer interface {
	SendLifecycleEvent(context.Context, loops.LifecycleEvent) error
}

func (h *StripeWebhookHandler) SetLoopsSyncer(syncer loopsLifecycleSyncer) *StripeWebhookHandler {
	h.loopsSyncer = syncer
	return h
}

func (h *MeHandler) SetLoopsSyncer(syncer loopsLifecycleSyncer) *MeHandler {
	h.loopsSyncer = syncer
	return h
}

func (h *SocialPostHandler) SetLoopsSyncer(syncer loopsLifecycleSyncer) *SocialPostHandler {
	h.loopsSyncer = syncer
	return h
}

func (h *SocialPostHandler) SetAppBaseURL(appBaseURL string) *SocialPostHandler {
	h.appBaseURL = normalizeAppBaseURL(appBaseURL)
	return h
}

func (h *StripeWebhookHandler) syncLoopsPlanChanged(ctx context.Context, workspaceID, oldPlanID, newPlanID, idempotencyKey string) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil {
		return
	}
	oldPlanID = normalizePlanID(oldPlanID)
	newPlanID = normalizePlanID(newPlanID)
	if oldPlanID == newPlanID {
		return
	}

	workspace, err := h.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to load workspace for plan_changed", "workspace_id", workspaceID, "error", err)
		return
	}
	owner, err := h.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		slog.Warn("loops: failed to load workspace owner for plan_changed", "workspace_id", workspaceID, "user_id", workspace.UserID, "error", err)
		return
	}

	event := buildLoopsPlanChangedEvent(owner, workspace, oldPlanID, newPlanID, h.planChangeType(ctx, oldPlanID, newPlanID), idempotencyKey, h.appBaseURL)
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send plan_changed", "workspace_id", workspaceID, "user_id", owner.ID, "error", err)
	}
}

func (h *StripeWebhookHandler) syncLoopsBillingPaymentFailed(ctx context.Context, sub db.Subscription, invoice stripe.Invoice, stripeEventID string) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil {
		return
	}
	workspace, owner, ok := h.billingEmailRecipient(ctx, sub.WorkspaceID, "payment_failed")
	if !ok {
		return
	}
	event := buildLoopsBillingPaymentFailedEvent(owner, workspace, sub, invoice, stripeEventID, h.appBaseURL)
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send billing_payment_failed", "workspace_id", sub.WorkspaceID, "user_id", owner.ID, "invoice_id", invoice.ID, "error", err)
	}
}

func (h *StripeWebhookHandler) syncLoopsBillingPaymentRecovered(ctx context.Context, sub db.Subscription, invoice stripe.Invoice) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil {
		return
	}
	workspace, owner, ok := h.billingEmailRecipient(ctx, sub.WorkspaceID, "payment_recovered")
	if !ok {
		return
	}
	event := buildLoopsBillingPaymentRecoveredEvent(owner, workspace, sub, invoice, h.appBaseURL)
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send billing_payment_recovered", "workspace_id", sub.WorkspaceID, "user_id", owner.ID, "invoice_id", invoice.ID, "error", err)
	}
}

func (h *StripeWebhookHandler) syncLoopsBillingSubscriptionCanceled(ctx context.Context, localSub db.Subscription, stripeSub stripe.Subscription) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil {
		return
	}
	workspace, owner, ok := h.billingEmailRecipient(ctx, localSub.WorkspaceID, "subscription_canceled")
	if !ok {
		return
	}
	event := buildLoopsBillingSubscriptionCanceledEvent(owner, workspace, localSub, stripeSub, h.appBaseURL)
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send billing_subscription_canceled", "workspace_id", localSub.WorkspaceID, "user_id", owner.ID, "subscription_id", stripeSub.ID, "error", err)
	}
}

func (h *StripeWebhookHandler) billingEmailRecipient(ctx context.Context, workspaceID, eventName string) (db.Workspace, db.User, bool) {
	workspace, err := h.queries.GetWorkspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("loops: failed to load workspace for billing email", "workspace_id", workspaceID, "event", eventName, "error", err)
		return db.Workspace{}, db.User{}, false
	}
	owner, err := h.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		slog.Warn("loops: failed to load workspace owner for billing email", "workspace_id", workspaceID, "user_id", workspace.UserID, "event", eventName, "error", err)
		return db.Workspace{}, db.User{}, false
	}
	if strings.TrimSpace(owner.Email) == "" {
		return db.Workspace{}, db.User{}, false
	}
	return workspace, owner, true
}

func (h *MeHandler) prepareLoopsAccountCanceled(ctx context.Context, userID string, canceledAt time.Time) (loops.LifecycleEvent, bool) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil {
		return loops.LifecycleEvent{}, false
	}
	user, err := h.queries.GetUser(ctx, userID)
	if err != nil {
		slog.Warn("loops: failed to load user for account cancel", "user_id", userID, "error", err)
		return loops.LifecycleEvent{}, false
	}

	workspace := db.Workspace{}
	if ws, ok := h.primaryWorkspaceForUser(ctx, userID); ok {
		workspace = ws
	}

	return buildLoopsAccountCanceledEvent(user, workspace, canceledAt), true
}

func (h *MeHandler) sendLoopsAccountCanceled(ctx context.Context, event loops.LifecycleEvent) {
	if h == nil || h.loopsSyncer == nil {
		return
	}
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send user_account_canceled", "user_id", event.UserID, "error", err)
	}
}

func (h *SocialPostHandler) syncLoopsPostFailed(ctx context.Context, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, failure db.CreatePostFailureParams, anotherAttempt bool) {
	if h == nil || h.loopsSyncer == nil || h.queries == nil || anotherAttempt {
		return
	}

	workspace, err := h.queries.GetWorkspace(ctx, post.WorkspaceID)
	if err != nil {
		slog.Warn("loops: failed to load workspace for post_failed", "workspace_id", post.WorkspaceID, "post_id", post.ID, "error", err)
		return
	}
	owner, err := h.queries.GetUser(ctx, workspace.UserID)
	if err != nil {
		slog.Warn("loops: failed to load workspace owner for post_failed", "workspace_id", post.WorkspaceID, "post_id", post.ID, "error", err)
		return
	}

	event := buildLoopsPostFailedEvent(owner, workspace, post, res, job, failure, h.appBaseURL)
	if err := h.loopsSyncer.SendLifecycleEvent(ctx, event); err != nil {
		slog.Warn("loops: failed to send post_failed", "workspace_id", post.WorkspaceID, "post_id", post.ID, "user_id", owner.ID, "error", err)
	}
}

func buildLoopsPlanChangedEvent(owner db.User, workspace db.Workspace, oldPlanID, newPlanID, changeType, idempotencyKey, appBaseURL string) loops.LifecycleEvent {
	oldPlanID = normalizePlanID(oldPlanID)
	newPlanID = normalizePlanID(newPlanID)
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           userName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		PlanID:         newPlanID,
		EventName:      "plan_changed",
		IdempotencyKey: idempotencyKey,
		Properties: map[string]any{
			"old_plan_id": oldPlanID,
			"new_plan_id": newPlanID,
			"change_type": changeType,
			"billing_url": normalizeAppBaseURL(appBaseURL) + "/settings/billing",
		},
	}
}

func buildLoopsBillingPaymentFailedEvent(owner db.User, workspace db.Workspace, sub db.Subscription, invoice stripe.Invoice, stripeEventID, appBaseURL string) loops.LifecycleEvent {
	nextPaymentAttempt := "not_scheduled"
	retryMessage := "Please update your billing details to keep UniPost active."
	if invoice.NextPaymentAttempt > 0 {
		nextPaymentAttempt = time.Unix(invoice.NextPaymentAttempt, 0).UTC().Format(time.RFC3339)
		retryMessage = "Stripe will retry this payment automatically."
	}
	planID := normalizePlanID(sub.PlanID)
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           userName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		PlanID:         planID,
		EventName:      "billing_payment_failed",
		IdempotencyKey: fmt.Sprintf("billing_payment_failed:%s:%s", firstNonEmptyString(invoice.ID, "unknown_invoice"), billingPaymentFailedAttemptKey(invoice, stripeEventID)),
		Properties: map[string]any{
			"workspace_name":       workspace.Name,
			"plan_id":              planID,
			"billing_url":          normalizeAppBaseURL(appBaseURL) + "/settings/billing",
			"retry_message":        retryMessage,
			"attempt_count":        invoice.AttemptCount,
			"next_payment_attempt": nextPaymentAttempt,
		},
	}
}

func buildLoopsBillingPaymentRecoveredEvent(owner db.User, workspace db.Workspace, sub db.Subscription, invoice stripe.Invoice, appBaseURL string) loops.LifecycleEvent {
	planID := normalizePlanID(sub.PlanID)
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           userName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		PlanID:         planID,
		EventName:      "billing_payment_recovered",
		IdempotencyKey: "billing_payment_recovered:" + firstNonEmptyString(invoice.ID, "unknown_invoice"),
		Properties: map[string]any{
			"workspace_name": workspace.Name,
			"plan_id":        planID,
			"billing_url":    normalizeAppBaseURL(appBaseURL) + "/settings/billing",
		},
	}
}

func buildLoopsBillingSubscriptionCanceledEvent(owner db.User, workspace db.Workspace, localSub db.Subscription, stripeSub stripe.Subscription, appBaseURL string) loops.LifecycleEvent {
	effectiveAt := subscriptionCanceledEffectiveAt(stripeSub).UTC().Format(time.RFC3339)
	planID := normalizePlanID(localSub.PlanID)
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           userName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		PlanID:         planID,
		EventName:      "billing_subscription_canceled",
		IdempotencyKey: fmt.Sprintf("billing_subscription_canceled:%s:%s", firstNonEmptyString(stripeSub.ID, "unknown_subscription"), effectiveAt),
		Properties: map[string]any{
			"workspace_name": workspace.Name,
			"plan_id":        planID,
			"effective_at":   effectiveAt,
			"billing_url":    normalizeAppBaseURL(appBaseURL) + "/settings/billing",
		},
	}
}

func billingPaymentFailedAttemptKey(invoice stripe.Invoice, stripeEventID string) string {
	if invoice.AttemptCount > 0 {
		return fmt.Sprint(invoice.AttemptCount)
	}
	if invoice.NextPaymentAttempt > 0 {
		return fmt.Sprintf("next_%d", invoice.NextPaymentAttempt)
	}
	if strings.TrimSpace(stripeEventID) != "" {
		return "event_" + strings.TrimSpace(stripeEventID)
	}
	return "attempt_unknown"
}

func subscriptionCanceledEffectiveAt(sub stripe.Subscription) time.Time {
	switch {
	case sub.EndedAt > 0:
		return time.Unix(sub.EndedAt, 0)
	case sub.CanceledAt > 0:
		return time.Unix(sub.CanceledAt, 0)
	default:
		return time.Now().UTC()
	}
}

func buildLoopsAccountCanceledEvent(user db.User, workspace db.Workspace, canceledAt time.Time) loops.LifecycleEvent {
	return loops.LifecycleEvent{
		UserID:         user.ID,
		Email:          user.Email,
		Name:           userName(user),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		EventName:      "user_account_canceled",
		IdempotencyKey: "user_account_canceled:" + user.ID,
		SkipContact:    true,
		Properties: map[string]any{
			"canceled_at": canceledAt.UTC().Format(time.RFC3339),
		},
	}
}

func buildLoopsPostFailedEvent(owner db.User, workspace db.Workspace, post db.SocialPost, res db.SocialPostResult, job db.PostDeliveryJob, failure db.CreatePostFailureParams, appBaseURL string) loops.LifecycleEvent {
	platform := postfailures.FirstNonEmpty(failure.Platform, job.Platform, "unknown")
	properties := map[string]any{
		"post_id":           post.ID,
		"result_id":         res.ID,
		"social_account_id": res.SocialAccountID,
		"platform":          platform,
		"error_code":        failure.ErrorCode,
		"failure_stage":     failure.FailureStage,
		"retriable":         failure.IsRetriable,
		"attempts":          job.Attempts,
		"max_attempts":      job.MaxAttempts,
		"dashboard_url":     postFailureDashboardURL(appBaseURL, post),
	}
	if v := textString(failure.PlatformErrorCode); v != "" {
		properties["platform_error_code"] = v
	}
	if v := textString(failure.RawError); v != "" {
		properties["raw_error_preview"] = truncateString(v, 300)
	}
	return loops.LifecycleEvent{
		UserID:         owner.ID,
		Email:          owner.Email,
		Name:           userName(owner),
		WorkspaceID:    workspace.ID,
		WorkspaceName:  workspace.Name,
		EventName:      "post_failed",
		IdempotencyKey: fmt.Sprintf("post_failed:%s:%d", job.ID, job.Attempts),
		Properties:     properties,
	}
}

func (h *StripeWebhookHandler) planChangeType(ctx context.Context, oldPlanID, newPlanID string) string {
	oldPrice, oldOK := h.planPrice(ctx, oldPlanID)
	newPrice, newOK := h.planPrice(ctx, newPlanID)
	if !oldOK || !newOK {
		return "unknown"
	}
	switch {
	case newPrice > oldPrice:
		return "upgrade"
	case newPrice < oldPrice:
		return "downgrade"
	default:
		return "plan_update"
	}
}

func (h *StripeWebhookHandler) planPrice(ctx context.Context, planID string) (int32, bool) {
	planID = normalizePlanID(planID)
	if planID == "free" {
		return 0, true
	}
	if h == nil || h.queries == nil {
		return 0, false
	}
	plan, err := h.queries.GetPlan(ctx, planID)
	if err != nil {
		slog.Warn("loops: failed to load plan price for plan_changed", "plan_id", planID, "error", err)
		return 0, false
	}
	return plan.PriceCents, true
}

func (h *MeHandler) primaryWorkspaceForUser(ctx context.Context, userID string) (db.Workspace, bool) {
	if h == nil || h.queries == nil {
		return db.Workspace{}, false
	}
	if mem, err := h.queries.GetActiveMembership(ctx, userID); err == nil {
		if ws, wsErr := h.queries.GetWorkspace(ctx, mem.WorkspaceID); wsErr == nil {
			return ws, true
		}
	}
	workspaces, err := h.queries.ListWorkspacesByUser(ctx, userID)
	if err != nil || len(workspaces) == 0 {
		return db.Workspace{}, false
	}
	return workspaces[0], true
}

func postFailureDashboardURL(appBaseURL string, post db.SocialPost) string {
	base := normalizeAppBaseURL(appBaseURL)
	if len(post.ProfileIds) > 0 && strings.TrimSpace(post.ProfileIds[0]) != "" {
		return fmt.Sprintf("%s/projects/%s/logs?post_id=%s", base, url.PathEscape(post.ProfileIds[0]), url.QueryEscape(post.ID))
	}
	return fmt.Sprintf("%s/logs?post_id=%s", base, url.QueryEscape(post.ID))
}

func normalizeAppBaseURL(appBaseURL string) string {
	appBaseURL = strings.TrimRight(strings.TrimSpace(appBaseURL), "/")
	if appBaseURL == "" {
		return "https://app.unipost.dev"
	}
	return appBaseURL
}

func normalizePlanID(planID string) string {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return "free"
	}
	return planID
}

func userName(user db.User) string {
	if user.Name.Valid {
		return user.Name.String
	}
	return ""
}

func textString(value pgtype.Text) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}
