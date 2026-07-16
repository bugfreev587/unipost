package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type BillingHandler struct {
	queries *db.Queries
	quota   *quota.Checker
	stripe  *billing.Manager
}

func NewBillingHandler(queries *db.Queries, quotaChecker *quota.Checker, stripeMgr *billing.Manager) *BillingHandler {
	return &BillingHandler{queries: queries, quota: quotaChecker, stripe: stripeMgr}
}

type billingResponse struct {
	Plan                string    `json:"plan"`
	PlanName            string    `json:"plan_name"`
	Status              string    `json:"status"`
	Usage               int       `json:"usage"`
	CompletedUsage      int       `json:"completed_usage"`
	ScheduledUsage      int       `json:"scheduled_usage"`
	QuotaHoldUsage      int       `json:"quota_hold_usage"`
	EffectiveUsage      int       `json:"effective_usage"`
	Limit               int       `json:"limit"`
	Percentage          float64   `json:"percentage"`
	EffectivePercentage float64   `json:"effective_percentage"`
	Period              string    `json:"period"`
	Warning             string    `json:"warning,omitempty"`
	SchedulingAllowed   bool      `json:"scheduling_allowed"`
	ResetsAt            time.Time `json:"resets_at"`
	CancelAtEnd         bool      `json:"cancel_at_period_end"`
	TrialEligible       bool      `json:"trial_eligible"`
}

type usageResponse struct {
	Period              string    `json:"period"`
	PostCount           int       `json:"post_count"`
	ScheduledCount      int       `json:"scheduled_count"`
	QuotaHoldCount      int       `json:"quota_hold_count"`
	EffectiveUsage      int       `json:"effective_usage"`
	PostLimit           int       `json:"post_limit"`
	Plan                string    `json:"plan"`
	Percentage          float64   `json:"percentage"`
	EffectivePercentage float64   `json:"effective_percentage"`
	Warning             string    `json:"warning,omitempty"`
	SchedulingAllowed   bool      `json:"scheduling_allowed"`
	ResetsAt            time.Time `json:"resets_at"`
}

func usageResponseFromSnapshot(snapshot quota.MonthlySnapshot) usageResponse {
	completedPercentage := 0.0
	if snapshot.Limit > 0 {
		completedPercentage = float64(snapshot.Completed) / float64(snapshot.Limit) * 100
	}
	schedulingAllowed := true
	warning := ""
	if paidquota.AppliesToPlan(snapshot.PlanID) && snapshot.Limit > 0 {
		switch {
		case snapshot.QuotaHold > 0 || snapshot.EffectiveUsage() >= snapshot.Limit:
			schedulingAllowed = false
			warning = "scheduled_quota_reached"
		case snapshot.Reached(80):
			warning = "approaching_limit"
		}
	}

	resetsAt := time.Time{}
	if periodStart, err := time.Parse("2006-01", snapshot.Period); err == nil {
		resetsAt = periodStart.AddDate(0, 1, 0).UTC()
	}
	return usageResponse{
		Period:              snapshot.Period,
		PostCount:           snapshot.Completed,
		ScheduledCount:      snapshot.Scheduled,
		QuotaHoldCount:      snapshot.QuotaHold,
		EffectiveUsage:      snapshot.EffectiveUsage(),
		PostLimit:           snapshot.Limit,
		Plan:                snapshot.PlanID,
		Percentage:          completedPercentage,
		EffectivePercentage: snapshot.EffectivePercentage(),
		Warning:             warning,
		SchedulingAllowed:   schedulingAllowed,
		ResetsAt:            resetsAt,
	}
}

// GetBilling handles GET /v1/billing (dual auth, workspace-scoped)
func (h *BillingHandler) GetBilling(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	h.quota.EnsureSubscription(r.Context(), workspaceID)

	sub, _ := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
	plan, _ := h.queries.GetPlan(r.Context(), sub.PlanID)
	snapshot, err := h.quota.MonthlySnapshotForPeriod(r.Context(), workspaceID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load billing usage")
		return
	}
	usage := usageResponseFromSnapshot(snapshot)

	writeSuccess(w, billingResponse{
		Plan:                sub.PlanID,
		PlanName:            plan.Name,
		Status:              sub.Status,
		Usage:               usage.PostCount,
		CompletedUsage:      usage.PostCount,
		ScheduledUsage:      usage.ScheduledCount,
		QuotaHoldUsage:      usage.QuotaHoldCount,
		EffectiveUsage:      usage.EffectiveUsage,
		Limit:               usage.PostLimit,
		Percentage:          usage.Percentage,
		EffectivePercentage: usage.EffectivePercentage,
		Period:              usage.Period,
		Warning:             usage.Warning,
		SchedulingAllowed:   usage.SchedulingAllowed,
		ResetsAt:            usage.ResetsAt,
		CancelAtEnd:         sub.CancelAtPeriodEnd.Bool,
		TrialEligible:       false,
	})
}

// CreateCheckout handles POST /v1/billing/checkout
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	userID := auth.GetUserID(r.Context())
	workspace, err := h.queries.GetWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Workspace not found")
		return
	}

	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlanID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "plan_id is required")
		return
	}

	// Pick the Stripe mode (live or sandbox) based on whether the workspace
	// owner is a superadmin. The price ID for the requested plan must
	// exist *in that mode* — if a sandbox user requests a plan whose
	// sandbox price ID isn't configured we reject with a clear message
	// instead of falling back to live and accidentally charging real money.
	mode := h.stripe.For(r.Context(), userID)
	if mode == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Stripe is not configured")
		return
	}
	priceID := mode.PriceID(body.PlanID)
	if priceID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan or plan not configured for this mode")
		return
	}

	// Verify the plan exists in our DB (used for display name + post limit
	// elsewhere). We don't read plan.StripePriceID anymore — that's mode-
	// specific now and lives in the billing.Manager price map.
	if _, err := h.queries.GetPlan(r.Context(), body.PlanID); err != nil {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan")
		return
	}

	// Get or create Stripe customer in the chosen mode. NB: customer IDs
	// from live and sandbox don't overlap, so even if a workspace changes
	// modes (user added/removed from SUPER_ADMINS), the previous mode's
	// customer ID will be invalid in the new mode and we'll mint a new one.
	sub, _ := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
	customerID := ""
	if sub.StripeCustomerID.String != "" {
		customerID = sub.StripeCustomerID.String
	} else {
		user, _ := h.queries.GetUser(r.Context(), userID)
		params := &stripe.CustomerParams{
			Email: stripe.String(user.Email),
			Name:  stripe.String(workspace.Name),
			Params: stripe.Params{
				Metadata: map[string]string{
					"workspace_id": workspaceID,
					"user_id":      userID,
					"mode":         mode.Name,
				},
			},
		}
		c, err := mode.Client.Customers.New(params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create customer")
			return
		}
		customerID = c.ID
	}

	appURL := os.Getenv("NEXT_PUBLIC_APP_URL")
	if appURL == "" {
		appURL = "https://app.unipost.dev"
	}

	checkoutParams := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL: stripe.String(appURL + "/settings/billing?status=success"),
		CancelURL:  stripe.String(appURL + "/settings/billing?status=canceled"),
		Params: stripe.Params{
			Metadata: map[string]string{
				"workspace_id": workspaceID,
				"plan_id":      body.PlanID,
				// Stamp the mode on the session so the webhook handler can
				// double-check which mode it came from when both signing
				// secrets are valid (e.g. test events sent during setup).
				"mode": mode.Name,
			},
		},
	}

	s, err := mode.Client.CheckoutSessions.New(checkoutParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create checkout session")
		return
	}

	writeSuccess(w, map[string]string{"checkout_url": s.URL})
}

// CreatePortal handles POST /v1/billing/portal
func (h *BillingHandler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	userID := auth.GetUserID(r.Context())

	sub, err := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
	if err != nil || sub.StripeCustomerID.String == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No active subscription")
		return
	}

	mode := h.stripe.For(r.Context(), userID)
	if mode == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Stripe is not configured")
		return
	}

	appURL := os.Getenv("NEXT_PUBLIC_APP_URL")
	if appURL == "" {
		appURL = "https://app.unipost.dev"
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID.String),
		ReturnURL: stripe.String(appURL + "/settings/billing"),
	}

	s, err := mode.Client.BillingPortalSessions.New(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create portal session")
		return
	}

	writeSuccess(w, map[string]string{"portal_url": s.URL})
}

// GetUsage handles GET /v1/usage (API key auth)
func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	h.quota.EnsureSubscription(r.Context(), workspaceID)
	snapshot, err := h.quota.MonthlySnapshotForPeriod(r.Context(), workspaceID, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load usage")
		return
	}
	writeSuccess(w, usageResponseFromSnapshot(snapshot))
}

// ListPlans handles GET /v1/plans
func (h *BillingHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.queries.ListPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list plans")
		return
	}

	type planResponse struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		PriceCents int32  `json:"price_cents"`
		PostLimit  int32  `json:"post_limit"`
	}

	result := make([]planResponse, len(plans))
	for i, p := range plans {
		result[i] = planResponse{
			ID:         p.ID,
			Name:       p.Name,
			PriceCents: p.PriceCents,
			PostLimit:  p.PostLimit,
		}
	}

	writeSuccess(w, result)
}
