package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
	"github.com/xiaoboyu/unipost-api/internal/trials"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

const stripeCheckoutEnvironmentMetadataKey = "unipost_environment"

func stripeCheckoutMetadata(workspaceID, planID, mode string) map[string]string {
	return map[string]string{
		"workspace_id":                       workspaceID,
		"plan_id":                            planID,
		"mode":                               mode,
		stripeCheckoutEnvironmentMetadataKey: runtimeenv.Current(),
	}
}

func stripeCheckoutSubscriptionData(metadata map[string]string) *stripe.CheckoutSessionSubscriptionDataParams {
	return &stripe.CheckoutSessionSubscriptionDataParams{Metadata: metadata}
}

func stripeCustomerParams(workspaceID, userID, workspaceName, email, mode string) *stripe.CustomerParams {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(workspaceName),
		Params: stripe.Params{Metadata: map[string]string{
			"workspace_id": workspaceID,
			"user_id":      userID,
			"mode":         mode,
		}},
	}
	params.SetIdempotencyKey("billing:" + mode + ":workspace:" + workspaceID + ":customer")
	return params
}

func stripeCheckoutURL(session *stripe.CheckoutSession) (string, error) {
	if session == nil || session.ID == "" || session.URL == "" {
		return "", errors.New("Stripe returned an incomplete Checkout Session")
	}
	return session.URL, nil
}

type stripeCustomerService interface {
	Get(string, *stripe.CustomerParams) (*stripe.Customer, error)
	New(*stripe.CustomerParams) (*stripe.Customer, error)
}

func resolveTrialStripeCustomer(ctx context.Context, customers stripeCustomerService, candidateID, workspaceID, userID, workspaceName, email, mode string) (string, error) {
	if customers == nil {
		return "", errors.New("Stripe customer service is unavailable")
	}
	if candidateID != "" {
		params := &stripe.CustomerParams{}
		params.Context = ctx
		customer, err := customers.Get(candidateID, params)
		if err == nil {
			workspaceMetadata := ""
			if customer != nil {
				workspaceMetadata = customer.Metadata["workspace_id"]
			}
			if customer != nil && customer.ID == candidateID && !customer.Deleted && (workspaceMetadata == "" || workspaceMetadata == workspaceID) {
				return customer.ID, nil
			}
		} else if !isStripeResourceMissing(err) {
			return "", fmt.Errorf("validate Stripe customer %s: %w", candidateID, err)
		}
	}

	params := stripeCustomerParams(workspaceID, userID, workspaceName, email, mode)
	params.Context = ctx
	customer, err := customers.New(params)
	if err != nil {
		return "", fmt.Errorf("create Stripe customer: %w", err)
	}
	if customer == nil || customer.ID == "" || customer.Deleted {
		return "", errors.New("Stripe returned an invalid customer")
	}
	return customer.ID, nil
}

func isStripeResourceMissing(err error) bool {
	var stripeErr *stripe.Error
	return errors.As(err, &stripeErr) && stripeErr.Code == stripe.ErrorCodeResourceMissing
}

type BillingHandler struct {
	queries      *db.Queries
	quota        *quota.Checker
	stripe       *billing.Manager
	xCredits     xCreditsSnapshotService
	xInboundCap  xCreditsInboundCapService
	featureFlags interface {
		ForWorkspace(context.Context, string, string) (bool, error)
	}
	trialCheckout billingTrialCheckoutService
	trialRead     billingTrialReadService
}

type billingTrialCheckoutService interface {
	PrepareCheckout(context.Context, trials.CheckoutRequest) (trials.CheckoutResult, error)
	ChangePlan(context.Context, trials.ChangePlanRequest) (trials.Grant, error)
	CancelRenewal(context.Context, trials.CancelRequest) (trials.Grant, error)
	CreateTrialSafePortal(context.Context, trials.PortalRequest) (string, bool, error)
}

type billingTrialReadService interface {
	CurrentTrialProjection(context.Context, string) (*trials.TrialProjection, error)
	ListTrialHistory(context.Context, string) ([]trials.HistoryProjection, error)
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

func (h *BillingHandler) SetFeatureFlags(flags interface {
	ForWorkspace(context.Context, string, string) (bool, error)
}) *BillingHandler {
	h.featureFlags = flags
	return h
}

func (h *BillingHandler) SetTrialService(service billingTrialCheckoutService) *BillingHandler {
	h.trialCheckout = service
	if reader, ok := service.(billingTrialReadService); ok {
		h.trialRead = reader
	}
	return h
}

func (h *BillingHandler) requireXCreditsAvailable(w http.ResponseWriter, r *http.Request, workspaceID string) bool {
	if h.featureFlags == nil {
		return true
	}
	enabled, err := h.featureFlags.ForWorkspace(r.Context(), workspaceID, featureflags.XCreditsBillingV1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to evaluate X Credits availability")
		return false
	}
	if !enabled {
		writeError(w, http.StatusForbidden, "FEATURE_NOT_AVAILABLE", "X Credits billing is not available yet")
		return false
	}
	return true
}

type billingResponse struct {
	Plan                string                  `json:"plan"`
	PlanName            string                  `json:"plan_name"`
	Status              string                  `json:"status"`
	Usage               int                     `json:"usage"`
	CompletedUsage      int                     `json:"completed_usage"`
	ScheduledUsage      int                     `json:"scheduled_usage"`
	QuotaHoldUsage      int                     `json:"quota_hold_usage"`
	EffectiveUsage      int                     `json:"effective_usage"`
	Limit               int                     `json:"limit"`
	Percentage          float64                 `json:"percentage"`
	EffectivePercentage float64                 `json:"effective_percentage"`
	Period              string                  `json:"period"`
	Warning             string                  `json:"warning,omitempty"`
	SchedulingAllowed   bool                    `json:"scheduling_allowed"`
	ResetsAt            time.Time               `json:"resets_at"`
	CancelAtEnd         bool                    `json:"cancel_at_period_end"`
	TrialEligible       bool                    `json:"trial_eligible"`
	Trial               *trials.TrialProjection `json:"trial,omitempty"`
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
	trial, err := h.getCurrentTrialProjection(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trial billing")
		return
	}

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
		Trial:               trial,
	})
}

func (h *BillingHandler) getCurrentTrialProjection(ctx context.Context, workspaceID string) (*trials.TrialProjection, error) {
	if h == nil || h.trialRead == nil {
		return nil, nil
	}
	return h.trialRead.CurrentTrialProjection(ctx, workspaceID)
}

// ListTrialHistory handles GET /v1/billing/trials for every workspace role.
func (h *BillingHandler) ListTrialHistory(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h == nil || h.trialRead == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Trial history is unavailable")
		return
	}
	history, err := h.trialRead.ListTrialHistory(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load trial history")
		return
	}
	if history == nil {
		history = []trials.HistoryProjection{}
	}
	writeSuccess(w, history)
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
	appURL := os.Getenv("NEXT_PUBLIC_APP_URL")
	if appURL == "" {
		appURL = "https://app.unipost.dev"
	}
	trialReq := trials.CheckoutRequest{
		WorkspaceID: workspaceID, PlanID: body.PlanID, StripeMode: mode.Name,
		PriceID:    priceID,
		SuccessURL: appURL + "/settings/billing?status=success",
		CancelURL:  appURL + "/settings/billing?status=canceled",
	}
	handled, trialNeedsCustomer := h.tryTrialCheckout(w, r, trialReq)
	if handled {
		return
	}

	// Get or create Stripe customer in the chosen mode. NB: customer IDs
	// from live and sandbox don't overlap, so even if a workspace changes
	// modes (user added/removed from SUPER_ADMINS), the previous mode's
	// customer ID will be invalid in the new mode and we'll mint a new one.
	sub, _ := h.queries.GetSubscriptionByWorkspace(r.Context(), workspaceID)
	customerID := ""
	if trialNeedsCustomer {
		user, _ := h.queries.GetUser(r.Context(), userID)
		customerID, err = resolveTrialStripeCustomer(
			r.Context(), mode.Client.Customers, sub.StripeCustomerID.String,
			workspaceID, userID, workspace.Name, user.Email, mode.Name,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to validate customer for trial checkout")
			return
		}
	} else if sub.StripeCustomerID.String != "" {
		customerID = sub.StripeCustomerID.String
	} else {
		user, _ := h.queries.GetUser(r.Context(), userID)
		params := stripeCustomerParams(workspaceID, userID, workspace.Name, user.Email, mode.Name)
		c, err := mode.Client.Customers.New(params)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create customer")
			return
		}
		if c == nil || c.ID == "" {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create customer")
			return
		}
		customerID = c.ID
	}
	if trialNeedsCustomer {
		trialReq.CustomerID = customerID
		handled, needsCustomer := h.tryTrialCheckout(w, r, trialReq)
		if handled {
			return
		}
		if needsCustomer {
			writeError(w, http.StatusConflict, "CHECKOUT_IN_PROGRESS", "Trial checkout is already being processed")
			return
		}
		// The offer may have been revoked between the initial probe and
		// customer creation. In that case this becomes an ordinary Checkout.
	}

	checkoutMetadata := stripeCheckoutMetadata(workspaceID, body.PlanID, mode.Name)
	checkoutParams := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{Price: stripe.String(priceID), Quantity: stripe.Int64(1)},
		},
		SuccessURL:       stripe.String(trialReq.SuccessURL),
		CancelURL:        stripe.String(trialReq.CancelURL),
		SubscriptionData: stripeCheckoutSubscriptionData(checkoutMetadata),
		Params: stripe.Params{
			Metadata: checkoutMetadata,
		},
	}

	s, err := mode.Client.CheckoutSessions.New(checkoutParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create checkout session")
		return
	}
	checkoutURL, err := stripeCheckoutURL(s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create checkout session")
		return
	}
	writeSuccess(w, map[string]string{"checkout_url": checkoutURL})
}

func (h *BillingHandler) tryTrialCheckout(w http.ResponseWriter, r *http.Request, req trials.CheckoutRequest) (bool, bool) {
	if h == nil || h.trialCheckout == nil {
		return false, false
	}
	checkout, err := h.trialCheckout.PrepareCheckout(r.Context(), req)
	if errors.Is(err, trials.ErrTrialCheckoutNotApplicable) {
		return false, false
	}
	if errors.Is(err, trials.ErrCheckoutCustomerRequired) {
		return false, true
	}
	if errors.Is(err, trials.ErrTrialPlanChangeRequired) {
		writeError(w, http.StatusConflict, "TRIAL_PLAN_CHANGE_REQUIRED", "Use the billing plan-change action while a managed trial is scheduled or active")
		return true, false
	}
	if errors.Is(err, trials.ErrCheckoutCompletionPending) || errors.Is(err, trials.ErrCheckoutStateConflict) {
		writeError(w, http.StatusConflict, "CHECKOUT_IN_PROGRESS", "Trial checkout is already being processed")
		return true, false
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to prepare trial checkout")
		return true, false
	}
	writeSuccess(w, map[string]string{"checkout_url": checkout.URL})
	return true, false
}

// ChangePlan handles POST /v1/billing/change-plan. The route is owner-only;
// the service dispatches to Subscription or Schedule according to grant kind.
func (h *BillingHandler) ChangePlan(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	if h == nil || h.trialCheckout == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Trial billing is not configured")
		return
	}
	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlanID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "plan_id is required")
		return
	}
	grant, err := h.trialCheckout.ChangePlan(r.Context(), trials.ChangePlanRequest{WorkspaceID: workspaceID, TargetPlanID: body.PlanID})
	if err != nil {
		writeTrialMutationError(w, err)
		return
	}
	writeSuccess(w, trialMutationResponse(grant, body.PlanID, true, false))
}

// CancelTrialRenewal handles POST /v1/billing/trials/{trialID}/cancel-renewal.
func (h *BillingHandler) CancelTrialRenewal(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}
	grantID := chi.URLParam(r, "trialID")
	if grantID == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "trialID is required")
		return
	}
	if h == nil || h.trialCheckout == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Trial billing is not configured")
		return
	}
	grant, err := h.trialCheckout.CancelRenewal(r.Context(), trials.CancelRequest{WorkspaceID: workspaceID, GrantID: grantID})
	if err != nil {
		writeTrialMutationError(w, err)
		return
	}
	writeSuccess(w, trialMutationResponse(grant, grant.PlanID, false, true))
}

type trialMutationTrialProjection struct {
	ID                        string        `json:"id"`
	Kind                      trials.Kind   `json:"kind"`
	PlanID                    string        `json:"plan_id"`
	DurationDays              int32         `json:"duration_days"`
	Status                    trials.Status `json:"status"`
	GrantedAt                 *time.Time    `json:"granted_at,omitempty"`
	ScheduledStartAt          *time.Time    `json:"scheduled_start_at,omitempty"`
	StartedAt                 *time.Time    `json:"started_at,omitempty"`
	EndsAt                    *time.Time    `json:"ends_at,omitempty"`
	ActivatedAt               *time.Time    `json:"activated_at,omitempty"`
	CanceledAt                *time.Time    `json:"canceled_at,omitempty"`
	ChangingPlanForfeitsTrial bool          `json:"changing_plan_forfeits_trial"`
}

type trialMutationBillingProjection struct {
	CurrentPlan       string     `json:"current_plan"`
	TargetPlan        string     `json:"target_plan"`
	Status            string     `json:"status"`
	ChangePending     bool       `json:"change_pending"`
	CancelAtPeriodEnd bool       `json:"cancel_at_period_end"`
	PeriodStartAt     *time.Time `json:"period_start_at,omitempty"`
	PeriodEndAt       *time.Time `json:"period_end_at,omitempty"`
}

type trialMutationProjection struct {
	Trial   trialMutationTrialProjection   `json:"trial"`
	Billing trialMutationBillingProjection `json:"billing"`
}

func trialMutationResponse(grant trials.Grant, targetPlan string, changePending, cancelAtPeriodEnd bool) trialMutationProjection {
	periodStart := grant.StartedAt
	if grant.Status == trials.StatusScheduled {
		periodStart = grant.ScheduledStartAt
	}
	return trialMutationProjection{
		Trial: trialMutationTrialProjection{
			ID: grant.ID, Kind: grant.Kind, PlanID: grant.PlanID, DurationDays: grant.DurationDays, Status: grant.Status,
			GrantedAt: nonZeroTrialTime(grant.GrantedAt), ScheduledStartAt: grant.ScheduledStartAt, StartedAt: grant.StartedAt,
			EndsAt: grant.EndsAt, ActivatedAt: grant.ActivatedAt, CanceledAt: grant.CanceledAt,
			ChangingPlanForfeitsTrial: grant.Status == trials.StatusScheduled || grant.Status == trials.StatusActive,
		},
		Billing: trialMutationBillingProjection{
			CurrentPlan: grant.PlanID, TargetPlan: targetPlan, ChangePending: changePending, CancelAtPeriodEnd: cancelAtPeriodEnd,
			Status: func() string {
				if changePending {
					return "change_pending"
				}
				return string(grant.Status)
			}(),
			PeriodStartAt: periodStart, PeriodEndAt: grant.EndsAt,
		},
	}
}

func nonZeroTrialTime(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	value = value.UTC()
	return &value
}

func writeTrialMutationError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, trials.ErrGrantNotFound):
		writeError(w, http.StatusNotFound, "TRIAL_NOT_FOUND", "Trial grant not found")
	case errors.Is(err, trials.ErrInvalidPlan), errors.Is(err, trials.ErrInvalidDuration):
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid trial billing request")
	case errors.Is(err, trials.ErrTrialMutationNotApplicable), errors.Is(err, trials.ErrBillingModeUnavailable), errors.Is(err, trials.ErrConcurrentTransition):
		writeError(w, http.StatusConflict, "TRIAL_CONFLICT", "Trial billing state changed; refresh and try again")
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update trial billing")
	}
}

func (h *BillingHandler) tryTrialSafePortal(w http.ResponseWriter, r *http.Request, req trials.PortalRequest) bool {
	if h == nil || h.trialCheckout == nil {
		return false
	}
	url, handled, err := h.trialCheckout.CreateTrialSafePortal(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create trial-safe portal session")
		return true
	}
	if !handled {
		return false
	}
	writeSuccess(w, map[string]string{"portal_url": url})
	return true
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
	returnURL := appURL + "/settings/billing"
	if h.tryTrialSafePortal(w, r, trials.PortalRequest{WorkspaceID: workspaceID, StripeMode: mode.Name, CustomerID: sub.StripeCustomerID.String, ReturnURL: returnURL}) {
		return
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID.String),
		ReturnURL: stripe.String(returnURL),
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
	if !h.requireXCreditsAvailable(w, r, workspaceID) {
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
	if !h.requireXCreditsAvailable(w, r, workspaceID) {
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
