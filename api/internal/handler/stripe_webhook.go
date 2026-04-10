package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

type StripeWebhookHandler struct {
	queries *db.Queries
	stripe  *billing.Manager
}

func NewStripeWebhookHandler(queries *db.Queries, stripeMgr *billing.Manager) *StripeWebhookHandler {
	return &StripeWebhookHandler{queries: queries, stripe: stripeMgr}
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

	status := string(sub.Status)
	h.queries.UpdateSubscriptionStatus(r.Context(), db.UpdateSubscriptionStatusParams{
		StripeSubscriptionID: pgtype.Text{String: sub.ID, Valid: true},
		Status:               status,
	})

	slog.Info("stripe webhook: subscription updated", "subscription_id", sub.ID, "status", status)
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
