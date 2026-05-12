package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
)

type StripeWebhookHandler struct {
	queries *db.Queries
	stripe  *billing.Manager
	bus     events.EventBus
}

func NewStripeWebhookHandler(queries *db.Queries, stripeMgr *billing.Manager, bus events.EventBus) *StripeWebhookHandler {
	if bus == nil {
		bus = events.NoopBus{}
	}
	return &StripeWebhookHandler{queries: queries, stripe: stripeMgr, bus: bus}
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
		h.handleCheckoutCompleted(r, event)
	case "customer.subscription.updated":
		h.handleSubscriptionUpdated(r, event)
	case "customer.subscription.deleted":
		h.handleSubscriptionDeleted(r, event)
	case "invoice.payment_failed":
		h.handlePaymentFailed(r, event)
	case "invoice.payment_succeeded":
		h.handlePaymentSucceeded(r, event)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *StripeWebhookHandler) handleCheckoutCompleted(r *http.Request, event stripe.Event) {
	var session stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
		slog.Error("stripe webhook: failed to parse checkout session", "error", err)
		return
	}

	workspaceID := session.Metadata["workspace_id"]
	planID := session.Metadata["plan_id"]
	if workspaceID == "" || planID == "" {
		slog.Error("stripe webhook: missing metadata", "workspace_id", workspaceID, "plan_id", planID)
		return
	}

	customerID := ""
	if session.Customer != nil {
		customerID = session.Customer.ID
	}
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	// Use upsert to handle both new and existing subscription rows
	_, err := h.queries.CreateSubscription(r.Context(), db.CreateSubscriptionParams{
		WorkspaceID:            workspaceID,
		PlanID:               planID,
		StripeCustomerID:     pgtype.Text{String: customerID, Valid: customerID != ""},
		StripeSubscriptionID: pgtype.Text{String: subscriptionID, Valid: subscriptionID != ""},
		Status:               "active",
	})
	if err != nil {
		slog.Error("stripe webhook: failed to upsert subscription", "error", err, "workspace_id", workspaceID)
		return
	}

	slog.Info("stripe webhook: subscription created", "workspace_id", workspaceID, "plan_id", planID, "customer", customerID)
}

func (h *StripeWebhookHandler) handleSubscriptionUpdated(r *http.Request, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return
	}

	localSub, err := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})
	if err != nil {
		slog.Error("stripe webhook: subscription row not found", "subscription_id", sub.ID, "error", err)
		return
	}

	if isTrialCancellation(sub) {
		if err := h.queries.CancelSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true}); err != nil {
			slog.Error("stripe webhook: failed to stop trialing subscription immediately", "subscription_id", sub.ID, "error", err)
			return
		}
		slog.Info("stripe webhook: trialing subscription canceled immediately", "subscription_id", sub.ID, "workspace_id", localSub.WorkspaceID)
		return
	}

	planID := localSub.PlanID
	if resolvedPlanID, ok := h.resolvePlanIDFromStripeSubscription(r, &sub); ok {
		planID = resolvedPlanID
		if h.shouldKeepCurrentPlanForDowngrade(r, localSub, resolvedPlanID, &sub) {
			planID = localSub.PlanID
		}
	}

	status := string(sub.Status)
	if err := h.queries.UpdateSubscriptionStripe(r.Context(), db.UpdateSubscriptionStripeParams{
		WorkspaceID:          localSub.WorkspaceID,
		StripeCustomerID:     stripeCustomerID(sub.Customer),
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
		PlanID:               planID,
		Status:               status,
		CurrentPeriodStart:   stripeUnixToTimestamptz(subscriptionCurrentPeriodStart(&sub)),
		CurrentPeriodEnd:     stripeUnixToTimestamptz(subscriptionCurrentPeriodEnd(&sub)),
		CancelAtPeriodEnd:    pgtype.Bool{Bool: sub.CancelAtPeriodEnd, Valid: true},
	}); err != nil {
		slog.Error("stripe webhook: failed to sync subscription state", "subscription_id", sub.ID, "error", err)
		return
	}

	slog.Info(
		"stripe webhook: subscription updated",
		"subscription_id", sub.ID,
		"status", status,
		"plan_id", planID,
		"cancel_at_period_end", sub.CancelAtPeriodEnd,
	)
}

func (h *StripeWebhookHandler) handleSubscriptionDeleted(r *http.Request, event stripe.Event) {
	var sub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
		slog.Error("stripe webhook: failed to parse subscription", "error", err)
		return
	}

	h.queries.CancelSubscription(r.Context(), pgtype.Text{String: sub.ID, Valid: true})

	slog.Info("stripe webhook: subscription canceled", "subscription_id", sub.ID)
}

func (h *StripeWebhookHandler) handlePaymentFailed(r *http.Request, event stripe.Event) {
	var invoice stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &invoice); err != nil {
		return
	}

	subID := h.getSubIDFromInvoice(&invoice)
	if subID != "" {
		h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtype.Text{String: subID, Valid: true},
			Status:               "past_due",
		})
		slog.Warn("stripe webhook: payment failed", "subscription_id", subID)

		// Notify the workspace owner — critical event, per schema in
		// events/bus.go + worker/notification.go renderer.
		if sub, err := h.queries.GetSubscriptionByStripeSubscription(r.Context(), pgtype.Text{String: subID, Valid: true}); err == nil {
			h.bus.Publish(r.Context(), sub.WorkspaceID, events.EventBillingPaymentFailed, map[string]any{
				"subscription_id": subID,
				"plan_id":         sub.PlanID,
			})
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
		h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
			StripeSubscriptionID: pgtype.Text{String: subID, Valid: true},
			Status:               "active",
		})
		slog.Info("stripe webhook: payment succeeded", "subscription_id", subID)
	}
}

func (h *StripeWebhookHandler) getSubIDFromInvoice(invoice *stripe.Invoice) string {
	if invoice.Parent != nil && invoice.Parent.SubscriptionDetails != nil && invoice.Parent.SubscriptionDetails.Subscription != nil {
		return invoice.Parent.SubscriptionDetails.Subscription.ID
	}
	return ""
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
