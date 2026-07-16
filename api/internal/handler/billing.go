package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

type BillingHandler struct {
	queries     *db.Queries
	quota       *quota.Checker
	stripe      *billing.Manager
	xCredits    xCreditsSnapshotService
	xInboundCap xCreditsInboundCapService
}

type xCreditsSnapshotService interface {
	Snapshot(context.Context, string, time.Time) (xcredits.Snapshot, error)
}

type xCreditsInboundCapService interface {
	UpdateInboundCap(context.Context, xcredits.UpdateInboundCapRequest) (xcredits.InboundCapSetting, error)
}

func NewBillingHandler(queries *db.Queries, quotaChecker *quota.Checker, stripeMgr *billing.Manager) *BillingHandler {
	return &BillingHandler{queries: queries, quota: quotaChecker, stripe: stripeMgr}
}

func (h *BillingHandler) SetXCreditsService(service xCreditsSnapshotService) *BillingHandler {
	h.xCredits = service
	if inboundCap, ok := service.(xCreditsInboundCapService); ok {
		h.xInboundCap = inboundCap
	}
	return h
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

type xCreditsAllowanceResponse struct {
	Mode                string    `json:"mode"`
	PlanID              string    `json:"plan_id"`
	MonthlyAllowance    *int64    `json:"monthly_allowance"`
	MonthlyUsed         int64     `json:"monthly_used"`
	MonthlyRemaining    *int64    `json:"monthly_remaining"`
	BillingPeriodStart  time.Time `json:"billing_period_start"`
	BillingPeriodEnd    time.Time `json:"billing_period_end"`
	CatalogVersion      string    `json:"catalog_version"`
	InboundDailyUsage   int64     `json:"inbound_daily_usage"`
	InboundDailyLimit   *int64    `json:"inbound_daily_limit"`
	InboundAccepted     int64     `json:"inbound_events_accepted"`
	InboundSuppressed   int64     `json:"inbound_events_suppressed"`
	InboundDailyResetAt time.Time `json:"inbound_daily_reset_at"`
	InboundDailyPercent float64   `json:"inbound_daily_percent"`
	PausePaidSources    bool      `json:"pause_paid_sources"`
	InboundPauseReason  string    `json:"inbound_pause_reason,omitempty"`
	ConnectionModeNote  string    `json:"connection_mode_note"`
}

// GetXCredits handles GET /v1/billing/x-credits.
func (h *BillingHandler) GetXCredits(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.xCredits == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "X Credits service is not configured")
		return
	}

	snapshot, err := h.xCredits.Snapshot(r.Context(), workspaceID, time.Now().UTC())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load X Credits allowance")
		return
	}

	writeSuccess(w, xCreditsAllowanceResponse{
		Mode:                "monthly_allowance",
		PlanID:              snapshot.PlanID,
		MonthlyAllowance:    snapshot.MonthlyAllowance,
		MonthlyUsed:         snapshot.MonthlyUsed,
		MonthlyRemaining:    snapshot.MonthlyRemaining,
		BillingPeriodStart:  snapshot.PeriodStart,
		BillingPeriodEnd:    snapshot.PeriodEnd,
		CatalogVersion:      snapshot.CatalogVersion,
		InboundDailyUsage:   snapshot.InboundDailyUsed,
		InboundDailyLimit:   snapshot.InboundDailyLimit,
		InboundAccepted:     snapshot.InboundAccepted,
		InboundSuppressed:   snapshot.InboundSuppressed,
		InboundDailyResetAt: snapshot.InboundResetAt,
		InboundDailyPercent: snapshot.InboundPercent,
		PausePaidSources:    snapshot.PausePaidSources,
		InboundPauseReason:  snapshot.InboundPauseReason,
		ConnectionModeNote:  "Activity through the UniPost-managed X app consumes this allowance. Workspace X app activity does not consume UniPost X Credits.",
	})
}

// UpdateXInboundCap handles PATCH /v1/billing/x-credits/inbound-cap.
func (h *BillingHandler) UpdateXInboundCap(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h.xInboundCap == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "X inbound cap service is not configured")
		return
	}

	var body struct {
		InboundDailyLimit    *int64 `json:"inbound_daily_limit"`
		AcknowledgedExposure bool   `json:"acknowledged_exposure"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "Invalid request body")
		return
	}
	if body.InboundDailyLimit == nil {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "inbound_daily_limit is required")
		return
	}
	if *body.InboundDailyLimit < 0 {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "inbound_daily_limit cannot be negative")
		return
	}

	updatedBy := auth.GetUserID(r.Context())
	if updatedBy == "" {
		updatedBy = "api_key"
	}
	setting, err := h.xInboundCap.UpdateInboundCap(r.Context(), xcredits.UpdateInboundCapRequest{
		WorkspaceID:          workspaceID,
		InboundDailyLimit:    *body.InboundDailyLimit,
		UpdatedBy:            updatedBy,
		AcknowledgedExposure: body.AcknowledgedExposure,
		Now:                  time.Now().UTC(),
	})
	if err != nil {
		switch {
		case errors.Is(err, xcredits.ErrInboundCapExceedsMonthlyRemaining),
			errors.Is(err, xcredits.ErrInboundExposureAcknowledgementRequired):
			writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update X inbound daily cap")
		}
		return
	}
	writeSuccess(w, setting)
}

type planResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	PriceCents   *int32 `json:"price_cents"`
	PostLimit    int32  `json:"post_limit"`
	PricingModel string `json:"pricing_model"`
}

func planResponseFromDB(plan db.Plan) planResponse {
	if plan.ID == "enterprise" {
		return planResponse{
			ID:           plan.ID,
			Name:         plan.Name,
			PostLimit:    plan.PostLimit,
			PricingModel: "custom",
		}
	}
	priceCents := plan.PriceCents
	return planResponse{
		ID:           plan.ID,
		Name:         plan.Name,
		PriceCents:   &priceCents,
		PostLimit:    plan.PostLimit,
		PricingModel: "fixed",
	}
}

// ListPlans handles GET /v1/plans
func (h *BillingHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.queries.ListPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list plans")
		return
	}

	result := make([]planResponse, len(plans))
	for i, p := range plans {
		result[i] = planResponseFromDB(p)
	}

	writeSuccess(w, result)
}
