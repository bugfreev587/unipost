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
		if err := h.handleCheckoutCompleted(r, event); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: checkout completion failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply checkout")
			return
		}
	case "customer.subscription.updated":
		if err := h.handleSubscriptionUpdated(r, event); err != nil {
			if errors.Is(err, errStripeWebhookNotApplicable) {
				break
			}
			slog.Error("stripe webhook: subscription update failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply subscription update")
			return
		}
	case "customer.subscription.deleted":
		if err := h.handleSubscriptionDeleted(r, event); err != nil {
			slog.Error("stripe webhook: subscription cancellation failed", "error", err)
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to apply subscription cancellation")
			return
		}
	case "invoice.payment_failed":
		h.handlePaymentFailed(r, event)
	case "invoice.payment_succeeded":
		h.handlePaymentSucceeded(r, event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(r *http.Request, event stripe.Event) error {
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

	var previousSub *db.Subscription
	if sub, err := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID); err == nil {
		previousSub = &sub
	}

	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	oldPlanID := "free"
	if previousSub != nil {
		oldPlanID = previousSub.PlanID
	}
	currentPlan, err := h.queries.GetPlan(r.Context(), oldPlanID)
	if err != nil {
		return fmt.Errorf("load checkout current plan for quota reconciliation: %w", err)
	}
	nextPlan, err := h.queries.GetPlan(r.Context(), planID)
	if err != nil {
		return fmt.Errorf("load checkout target plan for quota reconciliation: %w", err)
	}

	if err := h.applyPlanChangeMutation(
		r.Context(),
		workspaceID,
		currentPlan,
		nextPlan,
		stripeEventEffectiveAt(event),
		func(queries *db.Queries) error {
			_, err := queries.CreateSubscription(r.Context(), db.CreateSubscriptionParams{
				WorkspaceID:          workspaceID,
				PlanID:               planID,
				StripeCustomerID:     pgtype.Text{String: customerID, Valid: customerID != ""},
				StripeSubscriptionID: pgtype.Text{String: subscriptionID, Valid: subscriptionID != ""},
				Status:               "active",
			})
			return err
		},
	); err != nil {
		slog.Error("stripe webhook: failed to upsert subscription", "error", err, "workspace_id", workspaceID)
		return err
	}

	slog.Info("stripe webhook: subscription created", "workspace_id", workspaceID, "plan_id", planID, "customer", customerID)

	h.syncLoopsPlanChanged(
		r.Context(),
		workspaceID,
		oldPlanID,
		planID,
		fmt.Sprintf("plan_changed:stripe.checkout.completed:%s:%s:%s", session.ID, normalizePlanID(oldPlanID), normalizePlanID(planID)),
	)
	h.evaluatePaidQuotaHorizon(r.Context(), workspaceID)
	return nil
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

func (h *StripeWebhookHandler) handleSubscriptionUpdated(r *http.Request, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return err
	}
	if err := h.validateSubscriptionTarget(r.Context(), event, sub); err != nil {
		return err
	}

	localSub, err := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) && strings.TrimSpace(sub.Metadata[stripeCheckoutEnvironmentMetadataKey]) == "" {
			slog.Info(
				"stripe webhook: subscription update ignored",
				"event_id", event.ID,
				"subscription_id", sub.ID,
				"reason", "legacy_subscription_not_local",
				"local_environment", runtimeenv.Current(),
			)
			return errStripeWebhookNotApplicable
		}
		slog.Error("stripe webhook: subscription row not found", "subscription_id", sub.ID, "error", err)
		return err
	}

	if isTrialCancellation(sub) {
		currentPlan, err := h.queries.GetPlan(r.Context(), localSub.PlanID)
		if err != nil {
			return fmt.Errorf("load trial plan for quota reconciliation: %w", err)
		}
		freePlan, err := h.queries.GetPlan(r.Context(), "free")
		if err != nil {
			return fmt.Errorf("load free plan for trial cancellation reconciliation: %w", err)
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
			slog.Error("stripe webhook: failed to stop trialing subscription immediately", "subscription_id", sub.ID, "error", err)
			return err
		}
		h.evaluatePaidQuotaHorizon(r.Context(), localSub.WorkspaceID)
		slog.Info("stripe webhook: trialing subscription canceled immediately", "subscription_id", sub.ID, "workspace_id", localSub.WorkspaceID)
		return nil
	}

	planID := localSub.PlanID
	if resolvedPlanID, ok := h.resolvePlanIDFromStripeSubscription(r, &sub); ok {
		planID = resolvedPlanID
		if h.shouldKeepCurrentPlanForDowngrade(r, localSub, resolvedPlanID, &sub) {
			planID = localSub.PlanID
		}
	}

	status := string(sub.Status)
	planChanged := planID != localSub.PlanID
	updateParams := db.UpdateSubscriptionStripeParams{
		WorkspaceID:          localSub.WorkspaceID,
		StripeCustomerID:     stripeCustomerID(sub.Customer),
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
		PlanID:               planID,
		Status:               status,
		CurrentPeriodStart:   stripeUnixToTimestamptz(subscriptionCurrentPeriodStart(&sub)),
		CurrentPeriodEnd:     stripeUnixToTimestamptz(subscriptionCurrentPeriodEnd(&sub)),
		CancelAtPeriodEnd:    pgtype.Bool{Bool: sub.CancelAtPeriodEnd, Valid: true},
	}
	if planChanged {
		currentPlan, err := h.queries.GetPlan(r.Context(), localSub.PlanID)
		if err != nil {
			return fmt.Errorf("load current plan for quota reconciliation: %w", err)
		}
		nextPlan, err := h.queries.GetPlan(r.Context(), planID)
		if err != nil {
			return fmt.Errorf("load next plan for quota reconciliation: %w", err)
		}
		if err := h.applyPlanChangeMutation(
			r.Context(),
			localSub.WorkspaceID,
			currentPlan,
			nextPlan,
			stripeEventEffectiveAt(event),
			func(queries *db.Queries) error {
				return queries.UpdateSubscriptionStripe(r.Context(), updateParams)
			},
		); err != nil {
			return fmt.Errorf("reconcile quota holds: %w", err)
		}
	} else if err := h.queries.UpdateSubscriptionStripe(r.Context(), updateParams); err != nil {
		slog.Error("stripe webhook: failed to sync subscription state", "subscription_id", sub.ID, "error", err)
		return err
	}

	slog.Info(
		"stripe webhook: subscription updated",
		"subscription_id", sub.ID,
		"status", status,
		"plan_id", planID,
		"cancel_at_period_end", sub.CancelAtPeriodEnd,
	)

	h.syncLoopsPlanChanged(
		r.Context(),
		localSub.WorkspaceID,
		localSub.PlanID,
		planID,
		fmt.Sprintf("plan_changed:stripe.subscription.updated:%s:%s:%s:%d", sub.ID, normalizePlanID(localSub.PlanID), normalizePlanID(planID), subscriptionCurrentPeriodStart(&sub)),
	)

	if planChanged {
		h.evaluatePaidQuotaHorizon(r.Context(), localSub.WorkspaceID)
	}
	return nil
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

func (h *StripeWebhookHandler) handleSubscriptionDeleted(r *http.Request, event stripe.Event) error {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return err
	}

	localSub, localSubErr := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})
	if localSubErr == nil {
		h.syncLoopsBillingSubscriptionCanceled(r.Context(), localSub, sub)
	} else {
		slog.Warn("stripe webhook: subscription row not found for cancellation email", "subscription_id", sub.ID, "error", localSubErr)
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
		h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtype.Text{String: subID, Valid: true},
			Status:               "active",
		})
		slog.Info("stripe webhook: payment succeeded", "subscription_id", subID)
		if subErr == nil && shouldSendBillingPaymentRecovered(localSub.Status) {
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
