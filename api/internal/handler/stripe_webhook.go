package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
	"github.com/xiaoboyu/unipost-api/internal/trials"
)

var errStripeWebhookNotApplicable = errors.New("stripe webhook is not applicable to this environment")

type StripeWebhookHandler struct {
	queries            *db.Queries
	stripe             *billing.Manager
	bus                events.EventBus
	appBaseURL         string
	loopsSyncer        loopsLifecycleSyncer
	holdReconciler     paidquota.HoldReconciler
	paidQuotaEvaluator paidQuotaEvaluationService
	trialWebhooks      stripeTrialWebhookService
}

type stripeTrialWebhookService interface {
	RetrieveSubscription(context.Context, string, string) (trials.SubscriptionSnapshot, error)
	ReconcileSubscription(context.Context, trials.WebhookSubscriptionRequest) (trials.WebhookReconcileResult, error)
	ReconcileCheckoutExpired(context.Context, trials.WebhookCheckoutRequest) (trials.WebhookReconcileResult, error)
	ReconcileSchedule(context.Context, trials.WebhookScheduleRequest) (trials.WebhookReconcileResult, error)
	ReconcileOrdinaryCheckout(context.Context, string, string, time.Time) (trials.WebhookReconcileResult, error)
	IsTerminalGrant(context.Context, string, string) (bool, error)
}

func (h *StripeWebhookHandler) SetTrialWebhookService(service stripeTrialWebhookService) *StripeWebhookHandler {
	h.trialWebhooks = service
	return h
}

func (h *StripeWebhookHandler) SetHoldReconciler(reconciler paidquota.HoldReconciler) *StripeWebhookHandler {
	h.holdReconciler = reconciler
	return h
}

func (h *StripeWebhookHandler) SetPaidQuotaEvaluator(evaluator paidQuotaEvaluationService) *StripeWebhookHandler {
	h.paidQuotaEvaluator = evaluator
	return h
}

func (h *StripeWebhookHandler) evaluatePaidQuotaHorizon(ctx context.Context, workspaceID string) {
	if h == nil || h.paidQuotaEvaluator == nil {
		return
	}
	now := time.Now().UTC()
	end := now.AddDate(0, 0, 90)
	for cursor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC); !cursor.After(end); cursor = cursor.AddDate(0, 1, 0) {
		period := cursor.Format("2006-01")
		if err := h.paidQuotaEvaluator.Evaluate(ctx, workspaceID, period); err != nil {
			slog.Warn("stripe webhook: paid quota evaluation failed", "workspace_id", workspaceID, "period", period, "error", err)
		}
		if resolver, ok := h.paidQuotaEvaluator.(interface {
			ResolveFollowUpsBelowLimit(context.Context, string, string) error
		}); ok {
			if err := resolver.ResolveFollowUpsBelowLimit(ctx, workspaceID, period); err != nil {
				slog.Warn("stripe webhook: paid quota follow-up resolution failed", "workspace_id", workspaceID, "period", period, "error", err)
			}
		}
	}
}

func (h *StripeWebhookHandler) reconcileQuotaHoldsForPlanChange(
	ctx context.Context,
	workspaceID string,
	currentPlan db.Plan,
	nextPlan db.Plan,
	effectiveAt time.Time,
) error {
	if h == nil || h.holdReconciler == nil || currentPlan.ID == nextPlan.ID {
		return nil
	}
	if isPlanCapacityDowngrade(currentPlan, nextPlan) {
		return h.holdReconciler.ReconcileWorkspaceForPlan(
			ctx,
			workspaceID,
			nextPlan.ID,
			int(nextPlan.PostLimit),
			"plan_downgrade",
			effectiveAt,
		)
	}
	return h.holdReconciler.ReconcileWorkspaceForPlan(
		ctx,
		workspaceID,
		nextPlan.ID,
		int(nextPlan.PostLimit),
		"plan_upgrade",
		time.Time{},
	)
}

func (h *StripeWebhookHandler) applyPlanChangeMutation(
	ctx context.Context,
	workspaceID string,
	currentPlan db.Plan,
	nextPlan db.Plan,
	effectiveAt time.Time,
	mutation paidquota.PlanChangeMutation,
) error {
	if currentPlan.ID == nextPlan.ID || h == nil || h.holdReconciler == nil {
		if mutation != nil {
			return mutation(h.queries)
		}
		return nil
	}
	reason := "plan_upgrade"
	downgradeEffectiveAt := time.Time{}
	if isPlanCapacityDowngrade(currentPlan, nextPlan) {
		reason = "plan_downgrade"
		downgradeEffectiveAt = effectiveAt
	}
	return h.holdReconciler.ApplyPlanChange(
		ctx,
		workspaceID,
		nextPlan.ID,
		int(nextPlan.PostLimit),
		reason,
		downgradeEffectiveAt,
		mutation,
	)
}

func stripeEventEffectiveAt(event stripe.Event) time.Time {
	if event.Created > 0 {
		return time.Unix(event.Created, 0).UTC()
	}
	return time.Now().UTC()
}

func isPlanCapacityDowngrade(currentPlan, nextPlan db.Plan) bool {
	currentLimit := int(currentPlan.PostLimit)
	nextLimit := int(nextPlan.PostLimit)
	if nextLimit < 0 {
		return false
	}
	if currentLimit < 0 {
		return true
	}
	return nextLimit < currentLimit
}

func NewStripeWebhookHandler(queries *db.Queries, stripeMgr *billing.Manager, bus events.EventBus, appBaseURL string) *StripeWebhookHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	if appBaseURL == "" {
		appBaseURL = "https://app.unipost.dev"
	}
	return &StripeWebhookHandler{
		queries:    queries,
		stripe:     stripeMgr,
		bus:        bus,
		appBaseURL: strings.TrimRight(appBaseURL, "/"),
	}
}

// HandleStripe handles POST /webhooks/stripe
//
// Verifies the incoming signature against BOTH live and sandbox secrets via
// billing.Manager so the same endpoint can serve both modes. The matching
// mode is logged but not otherwise needed downstream because all webhook
// events carry their own metadata (project_id, etc.) — we don't need to
// make any further Stripe API calls in this handler.
func (h *StripeWebhookHandler) HandleStripe(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Failed to read body")
		return
	}

	event, mode, err := h.stripe.VerifyWebhook(body, r.Header.Get("Stripe-Signature"))
	if err != nil {
		slog.Error("stripe webhook: signature verification failed", "error", err)
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid signature")
		return
	}

	slog.Info("stripe webhook received", "type", event.Type, "mode", mode.Name)

	switch event.Type {
	case "checkout.session.completed":
		if err := h.handleCheckoutCompleted(r, event, mode); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: checkout completion failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply checkout")
			return
		}
	case "checkout.session.expired":
		if err := h.handleCheckoutExpired(r, event, mode); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: checkout expiry failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply checkout expiry")
			return
		}
	case "customer.subscription.created", "customer.subscription.updated":
		if err := h.handleSubscriptionUpdated(r, event, mode); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: subscription update failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply subscription update")
			return
		}
	case "customer.subscription.deleted":
		if err := h.handleSubscriptionDeleted(r, event, mode); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: subscription cancellation failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply subscription cancellation")
			return
		}
	case "invoice.payment_failed":
		h.handlePaymentFailed(r, event)
	case "invoice.payment_succeeded":
		h.handlePaymentSucceeded(r, event)
	case "subscription_schedule.created", "subscription_schedule.updated", "subscription_schedule.completed", "subscription_schedule.canceled", "subscription_schedule.released", "subscription_schedule.aborted":
		if err := h.handleSubscriptionSchedule(r, event, mode); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: subscription schedule reconciliation failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply subscription schedule")
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(r *http.Request, event stripe.Event, mode *billing.Mode) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.Error("stripe webhook: failed to parse checkout session", "error", err)
		return err
	}

	workspaceID := session.Metadata["workspace_id"]
	planID := session.Metadata["plan_id"]
	if workspaceID == "" || planID == "" {
		slog.Error("stripe webhook: missing metadata", "workspace_id", workspaceID, "plan_id", planID)
		return fmt.Errorf("checkout session missing workspace_id or plan_id metadata")
	}
	if err := h.validateCheckoutTarget(r.Context(), event, session); err != nil {
		return err
	}

	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	if mode == nil || mode.Name == "" || h.trialWebhooks == nil || subscriptionID == "" {
		return fmt.Errorf("Stripe subscription retrieval is unavailable")
	}
	snapshot, err := h.trialWebhooks.RetrieveSubscription(r.Context(), mode.Name, subscriptionID)
	if err != nil {
		return fmt.Errorf("retrieve Checkout subscription: %w", err)
	}
	if snapshot.ID != subscriptionID || snapshot.StripeMode != mode.Name {
		return fmt.Errorf("retrieved Checkout subscription does not match Session")
	}
	advancedReplay, err := h.validateCheckoutSubscriptionBinding(r.Context(), session, snapshot)
	if err != nil {
		return err
	}
	if advancedReplay {
		return nil
	}
	projected, err := h.projectStripeSubscription(r, event, snapshot)
	if err != nil {
		return err
	}
	if projected.Skipped {
		return nil
	}
	if !projected.Managed && !projected.Skipped {
		if _, err := h.trialWebhooks.ReconcileOrdinaryCheckout(r.Context(), workspaceID, planID, stripeEventEffectiveAt(event)); err != nil && !errors.Is(err, trials.ErrGrantNotFound) {
			return fmt.Errorf("reconcile pending trial after ordinary Checkout: %w", err)
		}
	}

	h.syncLoopsPlanChanged(
		r.Context(),
		workspaceID,
		projected.PreviousPlanID,
		planID,
		fmt.Sprintf("plan_changed:stripe.checkout.completed:%s:%s:%s", session.ID, normalizePlanID(projected.PreviousPlanID), normalizePlanID(planID)),
	)
	h.evaluatePaidQuotaHorizon(r.Context(), workspaceID)
	return nil
}

func (h *StripeWebhookHandler) validateCheckoutSubscriptionBinding(ctx context.Context, session stripe.CheckoutSession, snapshot trials.SubscriptionSnapshot) (bool, error) {
	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	if customerID == "" || customerID != snapshot.CustomerID {
		return false, fmt.Errorf("Checkout Session customer does not match Subscription")
	}
	workspaceID := strings.TrimSpace(session.Metadata["workspace_id"])
	planID := strings.TrimSpace(session.Metadata["plan_id"])
	environment := strings.TrimSpace(session.Metadata[stripeCheckoutEnvironmentMetadataKey])
	if workspaceID == "" || planID == "" || environment == "" || environment != runtimeenv.Current() {
		return false, fmt.Errorf("Checkout Session billing metadata is incomplete")
	}
	if snapshot.Metadata["workspace_id"] != workspaceID || snapshot.Metadata[stripeCheckoutEnvironmentMetadataKey] != environment {
		return false, fmt.Errorf("Checkout Session metadata does not match Subscription")
	}
	if snapshot.Metadata["trial_grant_id"] != session.Metadata["trial_grant_id"] || snapshot.Metadata["trial_kind"] != session.Metadata["trial_kind"] {
		return false, fmt.Errorf("Checkout Session trial metadata does not match Subscription")
	}
	plan, err := h.queries.GetPlanByStripePriceID(ctx, pgtype.Text{String: snapshot.PriceID, Valid: snapshot.PriceID != ""})
	if err != nil || snapshot.Metadata["plan_id"] != plan.ID {
		return false, fmt.Errorf("Checkout Session plan does not match Subscription price")
	}
	if plan.ID == planID {
		return false, nil
	}
	grantID := strings.TrimSpace(session.Metadata["trial_grant_id"])
	if grantID == "" || h.trialWebhooks == nil || snapshot.CurrentPeriodStartAt == nil || snapshot.CurrentPeriodEndAt == nil {
		return false, fmt.Errorf("Checkout Session plan does not match Subscription price")
	}
	localSub, err := h.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil || !localSub.StripeSubscriptionID.Valid || localSub.StripeSubscriptionID.String != snapshot.ID || !localSub.StripeCustomerID.Valid || localSub.StripeCustomerID.String != snapshot.CustomerID || localSub.PlanID != plan.ID || localSub.Status != snapshot.Status || !localSub.CurrentPeriodStart.Valid || !localSub.CurrentPeriodEnd.Valid || !localSub.CurrentPeriodStart.Time.Equal(snapshot.CurrentPeriodStartAt.UTC()) || !localSub.CurrentPeriodEnd.Time.Equal(snapshot.CurrentPeriodEndAt.UTC()) {
		return false, fmt.Errorf("Checkout Session plan does not match the current local Subscription")
	}
	terminal, err := h.trialWebhooks.IsTerminalGrant(ctx, grantID, workspaceID)
	if err != nil || !terminal {
		return false, fmt.Errorf("Checkout Session references a non-terminal trial grant")
	}
	return true, nil
}

func (h *StripeWebhookHandler) validateCheckoutTarget(ctx context.Context, event stripe.Event, session stripe.CheckoutSession) error {
	eventEnvironment := strings.ToLower(strings.TrimSpace(session.Metadata[stripeCheckoutEnvironmentMetadataKey]))
	localEnvironment := runtimeenv.Current()
	if eventEnvironment != "" && eventEnvironment != localEnvironment {
		slog.Info(
			"stripe webhook: checkout ignored",
			"event_id", event.ID,
			"workspace_id", session.Metadata["workspace_id"],
			"reason", "environment_mismatch",
			"event_environment", eventEnvironment,
			"local_environment", localEnvironment,
		)
		return errStripeWebhookNotApplicable
	}
	if session.Mode != stripe.CheckoutSessionModeSubscription ||
		session.Status != stripe.CheckoutSessionStatusComplete ||
		(session.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid &&
			session.PaymentStatus != stripe.CheckoutSessionPaymentStatusNoPaymentRequired) {
		slog.Info(
			"stripe webhook: checkout ignored",
			"event_id", event.ID,
			"workspace_id", session.Metadata["workspace_id"],
			"reason", "checkout_not_paid",
			"mode", session.Mode,
			"status", session.Status,
			"payment_status", session.PaymentStatus,
			"local_environment", localEnvironment,
		)
		return errStripeWebhookNotApplicable
	}
	if _, err := h.queries.GetWorkspace(ctx, session.Metadata["workspace_id"]); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info(
				"stripe webhook: checkout ignored",
				"event_id", event.ID,
				"workspace_id", session.Metadata["workspace_id"],
				"reason", "workspace_not_local",
				"event_environment", eventEnvironment,
				"local_environment", localEnvironment,
			)
			return errStripeWebhookNotApplicable
		}
		return fmt.Errorf("load checkout workspace: %w", err)
	}
	return nil
}

func (h *StripeWebhookHandler) handleSubscriptionUpdated(r *http.Request, event stripe.Event, mode *billing.Mode) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return err
	}
	if err := h.validateSubscriptionTarget(r.Context(), event, sub); err != nil {
		return err
	}
	if mode == nil || mode.Name == "" {
		return fmt.Errorf("verified Stripe mode is unavailable")
	}
	snapshot := subscriptionWebhookSnapshot(mode.Name, &sub)
	projected, err := h.projectStripeSubscription(r, event, snapshot)
	if err != nil {
		return err
	}
	if projected.PreviousPlanID != projected.PlanID {
		h.syncLoopsPlanChanged(r.Context(), projected.WorkspaceID, projected.PreviousPlanID, projected.PlanID, fmt.Sprintf("plan_changed:stripe.subscription:%s:%s:%s:%d", snapshot.ID, normalizePlanID(projected.PreviousPlanID), normalizePlanID(projected.PlanID), snapshotTimeUnix(snapshot.CurrentPeriodStartAt)))
		h.evaluatePaidQuotaHorizon(r.Context(), projected.WorkspaceID)
	}
	return nil
}

type subscriptionProjectionResult struct {
	Managed        bool
	Skipped        bool
	WorkspaceID    string
	PreviousPlanID string
	PlanID         string
}

func (h *StripeWebhookHandler) projectStripeSubscription(r *http.Request, event stripe.Event, snapshot trials.SubscriptionSnapshot) (subscriptionProjectionResult, error) {
	if snapshot.ID == "" || snapshot.CustomerID == "" || snapshot.PriceID == "" {
		return subscriptionProjectionResult{}, fmt.Errorf("Stripe subscription projection is incomplete")
	}
	plan, err := h.queries.GetPlanByStripePriceID(r.Context(), pgtype.Text{String: snapshot.PriceID, Valid: true})
	if err != nil {
		return subscriptionProjectionResult{}, fmt.Errorf("resolve Stripe subscription price: %w", err)
	}

	localSub, err := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: snapshot.ID, Valid: true})
	if errors.Is(err, pgx.ErrNoRows) {
		workspaceID := strings.TrimSpace(snapshot.Metadata["workspace_id"])
		if workspaceID == "" || strings.TrimSpace(snapshot.Metadata[stripeCheckoutEnvironmentMetadataKey]) != runtimeenv.Current() {
			return subscriptionProjectionResult{}, errStripeWebhookNotApplicable
		}
		localSub, err = h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
		if err == nil && ((localSub.StripeCustomerID.Valid && localSub.StripeCustomerID.String != snapshot.CustomerID) || (localSub.StripeSubscriptionID.Valid && localSub.StripeSubscriptionID.String != snapshot.ID)) {
			return subscriptionProjectionResult{}, fmt.Errorf("Stripe subscription metadata conflicts with the workspace billing identity")
		}
	}
	if err != nil {
		return subscriptionProjectionResult{}, fmt.Errorf("load local subscription projection: %w", err)
	}
	if subscriptionPeriodRegresses(localSub, snapshot) {
		slog.Info("stripe webhook: stale subscription period ignored", "subscription_id", snapshot.ID, "workspace_id", localSub.WorkspaceID, "snapshot_period_start", snapshot.CurrentPeriodStartAt, "local_period_start", localSub.CurrentPeriodStart.Time)
		return subscriptionProjectionResult{Skipped: true, WorkspaceID: localSub.WorkspaceID, PreviousPlanID: localSub.PlanID, PlanID: localSub.PlanID}, nil
	}

	trialResult := trials.WebhookReconcileResult{}
	if h.trialWebhooks != nil {
		trialResult, err = h.trialWebhooks.ReconcileSubscription(r.Context(), trials.WebhookSubscriptionRequest{Snapshot: snapshot, PlanID: plan.ID, OccurredAt: stripeEventEffectiveAt(event)})
		if errors.Is(err, trials.ErrWebhookNotApplicable) {
			return subscriptionProjectionResult{}, errStripeWebhookNotApplicable
		}
		if err != nil {
			return subscriptionProjectionResult{}, fmt.Errorf("reconcile managed trial subscription: %w", err)
		}
		if trialResult.DoNotProject {
			return subscriptionProjectionResult{Managed: true, WorkspaceID: localSub.WorkspaceID, PreviousPlanID: localSub.PlanID, PlanID: localSub.PlanID}, nil
		}
	}

	if snapshot.Status == string(stripe.SubscriptionStatusTrialing) && snapshot.CancelAtPeriodEnd && !trialResult.Managed {
		if err := h.cancelLegacyTrialImmediately(r.Context(), event, localSub, snapshot.ID); err != nil {
			return subscriptionProjectionResult{}, err
		}
		return subscriptionProjectionResult{WorkspaceID: localSub.WorkspaceID, PreviousPlanID: localSub.PlanID, PlanID: "free"}, nil
	}

	planID := plan.ID
	if h.shouldKeepCurrentPlanForDowngradeSnapshot(r.Context(), localSub, plan, snapshot.CurrentPeriodStartAt) {
		planID = localSub.PlanID
	}
	cancelAtPeriodEnd := snapshot.CancelAtPeriodEnd
	if trialResult.PreserveCancelAtPeriodEnd {
		cancelAtPeriodEnd = true
	}
	update := db.UpdateSubscriptionStripeParams{
		WorkspaceID: localSub.WorkspaceID, StripeCustomerID: pgtype.Text{String: snapshot.CustomerID, Valid: true},
		StripeSubscriptionID: pgtype.Text{String: snapshot.ID, Valid: true}, PlanID: planID, Status: snapshot.Status,
		CurrentPeriodStart: webhookTimestamptz(snapshot.CurrentPeriodStartAt), CurrentPeriodEnd: webhookTimestamptz(snapshot.CurrentPeriodEndAt),
		CancelAtPeriodEnd: pgtype.Bool{Bool: cancelAtPeriodEnd, Valid: true},
	}
	currentPlan, err := h.queries.GetPlan(r.Context(), localSub.PlanID)
	if err != nil {
		return subscriptionProjectionResult{}, fmt.Errorf("load current subscription plan: %w", err)
	}
	nextPlan, err := h.queries.GetPlan(r.Context(), planID)
	if err != nil {
		return subscriptionProjectionResult{}, fmt.Errorf("load projected subscription plan: %w", err)
	}
	if err := h.applyPlanChangeMutation(r.Context(), localSub.WorkspaceID, currentPlan, nextPlan, stripeEventEffectiveAt(event), func(queries *db.Queries) error {
		return queries.UpdateSubscriptionStripe(r.Context(), update)
	}); err != nil {
		return subscriptionProjectionResult{}, fmt.Errorf("persist Stripe subscription projection: %w", err)
	}
	return subscriptionProjectionResult{Managed: trialResult.Managed, WorkspaceID: localSub.WorkspaceID, PreviousPlanID: localSub.PlanID, PlanID: planID}, nil
}

func subscriptionPeriodRegresses(localSub db.Subscription, snapshot trials.SubscriptionSnapshot) bool {
	return localSub.StripeSubscriptionID.Valid &&
		localSub.StripeSubscriptionID.String == snapshot.ID &&
		localSub.CurrentPeriodStart.Valid &&
		snapshot.CurrentPeriodStartAt != nil &&
		snapshot.CurrentPeriodStartAt.Before(localSub.CurrentPeriodStart.Time)
}

func (h *StripeWebhookHandler) cancelLegacyTrialImmediately(ctx context.Context, event stripe.Event, localSub db.Subscription, subscriptionID string) error {
	currentPlan, err := h.queries.GetPlan(ctx, localSub.PlanID)
	if err != nil {
		return err
	}
	freePlan, err := h.queries.GetPlan(ctx, "free")
	if err != nil {
		return err
	}
	return h.applyPlanChangeMutation(ctx, localSub.WorkspaceID, currentPlan, freePlan, stripeEventEffectiveAt(event), func(queries *db.Queries) error {
		return queries.CancelSubscription(ctx, pgtype.Text{String: subscriptionID, Valid: true})
	})
}

func (h *StripeWebhookHandler) validateSubscriptionTarget(ctx context.Context, event stripe.Event, sub stripe.Subscription) error {
	eventEnvironment := strings.ToLower(strings.TrimSpace(sub.Metadata[stripeCheckoutEnvironmentMetadataKey]))
	localEnvironment := runtimeenv.Current()
	if eventEnvironment != "" && eventEnvironment != localEnvironment {
		slog.Info(
			"stripe webhook: subscription update ignored",
			"event_id", event.ID,
			"subscription_id", sub.ID,
			"workspace_id", sub.Metadata["workspace_id"],
			"reason", "environment_mismatch",
			"event_environment", eventEnvironment,
			"local_environment", localEnvironment,
		)
		return errStripeWebhookNotApplicable
	}
	workspaceID := strings.TrimSpace(sub.Metadata["workspace_id"])
	if eventEnvironment == "" || workspaceID == "" {
		return nil
	}
	if _, err := h.queries.GetWorkspace(ctx, workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			slog.Info(
				"stripe webhook: subscription update ignored",
				"event_id", event.ID,
				"subscription_id", sub.ID,
				"workspace_id", workspaceID,
				"reason", "workspace_not_local",
				"event_environment", eventEnvironment,
				"local_environment", localEnvironment,
			)
			return errStripeWebhookNotApplicable
		}
		return fmt.Errorf("load subscription workspace: %w", err)
	}
	return nil
}

func (h *StripeWebhookHandler) handleSubscriptionDeleted(r *http.Request, event stripe.Event, mode *billing.Mode) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return err
	}

	if err := h.validateSubscriptionTarget(r.Context(), event, sub); err != nil {
		return err
	}
	if mode == nil || mode.Name == "" {
		return fmt.Errorf("verified Stripe mode is unavailable")
	}
	localSub, localSubErr := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})
	if localSubErr == nil {
		h.syncLoopsBillingSubscriptionCanceled(r.Context(), localSub, sub)
	} else {
		slog.Warn("stripe webhook: subscription row not found for cancellation email", "subscription_id", sub.ID, "error", localSubErr)
	}
	if _, err := h.projectStripeSubscription(r, event, subscriptionWebhookSnapshot(mode.Name, &sub)); err != nil && !errors.Is(err, errStripeWebhookNotApplicable) {
		return err
	}
	if localSubErr == nil {
		currentPlan, err := h.queries.GetPlan(r.Context(), localSub.PlanID)
		if err != nil {
			return fmt.Errorf("load canceled plan for quota reconciliation: %w", err)
		}
		freePlan, err := h.queries.GetPlan(r.Context(), "free")
		if err != nil {
			return fmt.Errorf("load free plan for quota reconciliation: %w", err)
		}
		if err := h.applyPlanChangeMutation(
			r.Context(),
			localSub.WorkspaceID,
			currentPlan,
			freePlan,
			stripeEventEffectiveAt(event),
			func(queries *db.Queries) error {
				return queries.CancelSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})
			},
		); err != nil {
			return fmt.Errorf("reconcile canceled subscription quota holds: %w", err)
		}
	} else if err := h.queries.CancelSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true}); err != nil {
		return err
	}
	if localSubErr == nil {
		h.evaluatePaidQuotaHorizon(r.Context(), localSub.WorkspaceID)
	}

	slog.Info("stripe webhook: subscription canceled", "subscription_id", sub.ID)
	return nil
}

func (h *StripeWebhookHandler) handlePaymentFailed(r *http.Request, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return
	}

	subID := h.getSubIDFromInvoice(&invoice)
	if subID != "" {
		localSub, subErr := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: subID, Valid: true})
		h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtype.Text{String: subID, Valid: true},
			Status:               "past_due",
		})
		slog.Warn("stripe webhook: payment failed", "subscription_id", subID)

		if subErr == nil {
			h.syncLoopsBillingPaymentFailed(r.Context(), localSub, invoice, event.ID)
		} else {
			slog.Warn("stripe webhook: subscription row not found for payment_failed email", "subscription_id", subID, "invoice_id", invoice.ID, "error", subErr)
		}
	}
}

func (h *StripeWebhookHandler) handlePaymentSucceeded(r *http.Request, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return
	}

	subID := h.getSubIDFromInvoice(&invoice)
	if subID != "" {
		localSub, subErr := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: subID, Valid: true})
		if subErr == nil && shouldSendBillingPaymentRecovered(localSub.Status) {
			h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
				StripeSubscriptionID: pgtype.Text{String: subID, Valid: true},
				Status:               "active",
			})
			slog.Info("stripe webhook: payment recovered", "subscription_id", subID)
			h.syncLoopsBillingPaymentRecovered(r.Context(), localSub, invoice)
		}
	}
}

func (h *StripeWebhookHandler) getSubIDFromInvoice(invoice *stripe.Invoice) string {
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		return invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	return ""
}

func shouldSendBillingPaymentRecovered(previousStatus string) bool {
	return strings.TrimSpace(previousStatus) == "past_due"
}

func isTrialCancellation(sub stripe.Subscription) bool {
	return sub.Status == stripe.SubscriptionStatusTrialing && sub.CancelAtPeriodEnd
}

func (h *StripeWebhookHandler) handleCheckoutExpired(r *http.Request, event stripe.Event, mode *billing.Mode) error {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		return err
	}
	if mode == nil || mode.Name == "" || h.trialWebhooks == nil {
		return fmt.Errorf("trial webhook reconciliation is unavailable")
	}
	_, err := h.trialWebhooks.ReconcileCheckoutExpired(r.Context(), trials.WebhookCheckoutRequest{StripeMode: mode.Name, SessionID: session.ID, Metadata: session.Metadata, OccurredAt: stripeEventEffectiveAt(event)})
	if errors.Is(err, trials.ErrWebhookNotApplicable) {
		return errStripeWebhookNotApplicable
	}
	return err
}

func (h *StripeWebhookHandler) handleSubscriptionSchedule(r *http.Request, event stripe.Event, mode *billing.Mode) error {
	var schedule stripe.SubscriptionSchedule
	if err := json.Unmarshal(event.Data.Raw, &schedule); err != nil {
		return err
	}
	if mode == nil || mode.Name == "" || h.trialWebhooks == nil {
		return fmt.Errorf("trial webhook reconciliation is unavailable")
	}
	snapshot := scheduleWebhookSnapshot(mode.Name, &schedule)
	_, err := h.trialWebhooks.ReconcileSchedule(r.Context(), trials.WebhookScheduleRequest{EventType: string(event.Type), Snapshot: snapshot, OccurredAt: stripeEventEffectiveAt(event)})
	if errors.Is(err, trials.ErrWebhookNotApplicable) {
		return errStripeWebhookNotApplicable
	}
	return err
}

func subscriptionWebhookSnapshot(mode string, sub *stripe.Subscription) trials.SubscriptionSnapshot {
	if sub == nil {
		return trials.SubscriptionSnapshot{StripeMode: mode}
	}
	snapshot := trials.SubscriptionSnapshot{
		StripeMode: mode, ID: sub.ID, Status: string(sub.Status), CustomerID: stripeObjectIDForWebhook(sub.Customer),
		ScheduleID: stripeObjectIDForWebhook(sub.Schedule), TrialStartAt: webhookUnixTime(sub.TrialStart), TrialEndAt: webhookUnixTime(sub.TrialEnd),
		CancelAt: webhookUnixTime(sub.CancelAt), CancelAtPeriodEnd: sub.CancelAtPeriodEnd, CanceledAt: webhookUnixTime(sub.CanceledAt), EndedAt: webhookUnixTime(sub.EndedAt), Metadata: cloneWebhookMetadata(sub.Metadata),
	}
	if sub.Items != nil && len(sub.Items.Data) > 0 && sub.Items.Data[0] != nil {
		item := sub.Items.Data[0]
		snapshot.ItemID = item.ID
		snapshot.PriceID = stripeObjectIDForWebhook(item.Price)
		snapshot.CurrentPeriodStartAt = webhookUnixTime(item.CurrentPeriodStart)
		snapshot.CurrentPeriodEndAt = webhookUnixTime(item.CurrentPeriodEnd)
	}
	return snapshot
}

func scheduleWebhookSnapshot(mode string, schedule *stripe.SubscriptionSchedule) trials.ScheduleSnapshot {
	if schedule == nil {
		return trials.ScheduleSnapshot{StripeMode: mode}
	}
	snapshot := trials.ScheduleSnapshot{StripeMode: mode, ID: schedule.ID, Status: string(schedule.Status), EndBehavior: string(schedule.EndBehavior), CustomerID: stripeObjectIDForWebhook(schedule.Customer), SubscriptionID: stripeObjectIDForWebhook(schedule.Subscription), ReleasedSubscriptionID: stripeObjectIDForWebhook(schedule.ReleasedSubscription), CanceledAt: webhookUnixTime(schedule.CanceledAt), CompletedAt: webhookUnixTime(schedule.CompletedAt), ReleasedAt: webhookUnixTime(schedule.ReleasedAt), Metadata: cloneWebhookMetadata(schedule.Metadata)}
	if schedule.CurrentPhase != nil {
		snapshot.CurrentPhaseStartAt = webhookUnixTime(schedule.CurrentPhase.StartDate)
		snapshot.CurrentPhaseEndAt = webhookUnixTime(schedule.CurrentPhase.EndDate)
	}
	for _, phase := range schedule.Phases {
		if phase == nil {
			continue
		}
		item := trials.SchedulePhase{StartAt: webhookUnixTimeValue(phase.StartDate), EndAt: webhookUnixTimeValue(phase.EndDate), TrialEndAt: webhookUnixTimeValue(phase.TrialEnd), BillingCycleAnchor: string(phase.BillingCycleAnchor), Metadata: cloneWebhookMetadata(phase.Metadata)}
		if len(phase.Items) > 0 && phase.Items[0] != nil {
			item.PriceID = stripeObjectIDForWebhook(phase.Items[0].Price)
		}
		snapshot.Phases = append(snapshot.Phases, item)
	}
	return snapshot
}

func stripeObjectIDForWebhook(value any) string {
	switch typed := value.(type) {
	case *stripe.Customer:
		if typed != nil {
			return typed.ID
		}
	case *stripe.Subscription:
		if typed != nil {
			return typed.ID
		}
	case *stripe.SubscriptionSchedule:
		if typed != nil {
			return typed.ID
		}
	case *stripe.Price:
		if typed != nil {
			return typed.ID
		}
	}
	return ""
}

func webhookUnixTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	result := time.Unix(value, 0).UTC()
	return &result
}

func webhookUnixTimeValue(value int64) time.Time {
	if result := webhookUnixTime(value); result != nil {
		return *result
	}
	return time.Time{}
}

func cloneWebhookMetadata(metadata map[string]string) map[string]string {
	result := make(map[string]string, len(metadata))
	for key, value := range metadata {
		result[key] = value
	}
	return result
}

func webhookTimestamptz(value *time.Time) pgtype.Timestamptz {
	if value == nil || value.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: value.UTC(), Valid: true}
}

func snapshotTimeUnix(value *time.Time) int64 {
	if value == nil {
		return 0
	}
	return value.Unix()
}

func (h *StripeWebhookHandler) shouldKeepCurrentPlanForDowngradeSnapshot(ctx context.Context, current db.Subscription, nextPlan db.Plan, nextPeriodStart *time.Time) bool {
	if nextPeriodStart == nil || current.PlanID == "" || current.PlanID == "free" || nextPlan.ID == current.PlanID {
		return false
	}
	currentPlan, err := h.queries.GetPlan(ctx, current.PlanID)
	if err != nil || nextPlan.PriceCents >= currentPlan.PriceCents {
		return false
	}
	return shouldKeepCurrentPlanForScheduledDowngrade(currentPlan.PriceCents, nextPlan.PriceCents, current, nextPeriodStart.UTC())
}

func stripeUnixToTimestamptz(ts int64) pgtype.Timestamptz {
	if ts <= 0 {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: time.Unix(ts, 0).UTC(), Valid: true}
}

func stripeCustomerID(customer *stripe.Customer) pgtype.Text {
	if customer == nil || customer.ID == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: customer.ID, Valid: true}
}

func (h *StripeWebhookHandler) resolvePlanIDFromStripeSubscription(r *http.Request, sub *stripe.Subscription) (string, bool) {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil || sub.Items.Data[0].Price == nil {
		return "", false
	}
	priceID := sub.Items.Data[0].Price.ID
	if priceID == "" {
		return "", false
	}
	plan, err := h.queries.GetPlanByStripePriceID(r.Context(), pgtype.Text{String: priceID, Valid: true})
	if err != nil {
		slog.Warn("stripe webhook: unknown stripe price id on subscription update", "subscription_id", sub.ID, "price_id", priceID, "error", err)
		return "", false
	}
	return plan.ID, true
}

func (h *StripeWebhookHandler) shouldKeepCurrentPlanForDowngrade(r *http.Request, current db.Subscription, nextPlanID string, sub *stripe.Subscription) bool {
	if sub == nil || current.PlanID == "" || current.PlanID == "free" || nextPlanID == "" || nextPlanID == current.PlanID {
		return false
	}
	currentPlan, err := h.queries.GetPlan(r.Context(), current.PlanID)
	if err != nil {
		return false
	}
	nextPlan, err := h.queries.GetPlan(r.Context(), nextPlanID)
	if err != nil {
		return false
	}
	if nextPlan.PriceCents >= currentPlan.PriceCents {
		return false
	}
	return shouldKeepCurrentPlanForScheduledDowngrade(
		currentPlan.PriceCents,
		nextPlan.PriceCents,
		current,
		time.Unix(subscriptionCurrentPeriodStart(sub), 0).UTC(),
	)
}

func shouldKeepCurrentPlanForScheduledDowngrade(currentPriceCents, nextPriceCents int32, current db.Subscription, nextPeriodStart time.Time) bool {
	if current.PlanID == "" || current.PlanID == "free" {
		return false
	}
	if nextPriceCents >= currentPriceCents {
		return false
	}
	if !current.CurrentPeriodEnd.Valid || nextPeriodStart.IsZero() {
		return false
	}
	return nextPeriodStart.Before(current.CurrentPeriodEnd.Time)
}

func subscriptionCurrentPeriodStart(sub *stripe.Subscription) int64 {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil {
		return 0
	}
	return sub.Items.Data[0].CurrentPeriodStart
}

func subscriptionCurrentPeriodEnd(sub *stripe.Subscription) int64 {
	if sub == nil || sub.Items == nil || len(sub.Items.Data) == 0 || sub.Items.Data[0] == nil {
		return 0
	}
	return sub.Items.Data[0].CurrentPeriodEnd
}
