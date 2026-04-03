package handler

import (
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/billingportal/session"
	checkoutsession "github.com/stripe/stripe-go/v82/checkout/session"
	stripecustomer "github.com/stripe/stripe-go/v82/customer"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type BillingHandler struct {
	queries *db.Queries
	quota   *quota.Checker
}

func NewBillingHandler(queries *db.Queries, quotaChecker *quota.Checker) *BillingHandler {
	return &BillingHandler{queries: queries, quota: quotaChecker}
}

type billingResponse struct {
	Plan        string  `json:"plan"`
	PlanName    string  `json:"plan_name"`
	Status      string  `json:"status"`
	Usage       int     `json:"usage"`
	Limit       int     `json:"limit"`
	Percentage  float64 `json:"percentage"`
	Period      string  `json:"period"`
	Warning     string  `json:"warning,omitempty"`
	CancelAtEnd bool    `json:"cancel_at_period_end"`
}

// GetBilling handles GET /v1/billing (Clerk auth, project-scoped)
func (h *BillingHandler) GetBilling(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := auth.GetUserID(r.Context())

	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID: projectID, OwnerID: userID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
		return
	}

	h.quota.EnsureSubscription(r.Context(), projectID)

	sub, _ := h.queries.GetSubscriptionByProject(r.Context(), projectID)
	plan, _ := h.queries.GetPlan(r.Context(), sub.PlanID)
	status := h.quota.Check(r.Context(), projectID)

	writeSuccess(w, billingResponse{
		Plan:        sub.PlanID,
		PlanName:    plan.Name,
		Status:      sub.Status,
		Usage:       status.Usage,
		Limit:       status.Limit,
		Percentage:  status.Percentage,
		Period:      status.Period(),
		Warning:     status.Warning,
		CancelAtEnd: sub.CancelAtPeriodEnd.Bool,
	})
}

// CreateCheckout handles POST /v1/billing/checkout
func (h *BillingHandler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := auth.GetUserID(r.Context())

	project, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID: projectID, OwnerID: userID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
		return
	}

	var body struct {
		PlanID string `json:"plan_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PlanID == "" {
		writeError(w, http.StatusUnprocessableEntity, "VALIDATION_ERROR", "plan_id is required")
		return
	}

	plan, err := h.queries.GetPlan(r.Context(), body.PlanID)
	if err != nil || plan.StripePriceID.String == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid plan")
		return
	}

	// Get or create Stripe customer
	sub, _ := h.queries.GetSubscriptionByProject(r.Context(), projectID)
	customerID := ""
	if sub.StripeCustomerID.String != "" {
		customerID = sub.StripeCustomerID.String
	} else {
		user, _ := h.queries.GetUser(r.Context(), userID)
		params := &stripe.CustomerParams{
			Email: stripe.String(user.Email),
			Name:  stripe.String(project.Name),
			Params: stripe.Params{
				Metadata: map[string]string{
					"project_id": projectID,
					"user_id":    userID,
				},
			},
		}
		c, err := stripecustomer.New(params)
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
			{Price: stripe.String(plan.StripePriceID.String), Quantity: stripe.Int64(1)},
		},
		SuccessURL: stripe.String(appURL + "/projects/" + projectID + "/billing?status=success"),
		CancelURL:  stripe.String(appURL + "/projects/" + projectID + "/billing?status=canceled"),
		Params: stripe.Params{
			Metadata: map[string]string{
				"project_id": projectID,
				"plan_id":    body.PlanID,
			},
		},
	}

	s, err := checkoutsession.New(checkoutParams)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create checkout session")
		return
	}

	writeSuccess(w, map[string]string{"checkout_url": s.URL})
}

// CreatePortal handles POST /v1/billing/portal
func (h *BillingHandler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "projectID")
	userID := auth.GetUserID(r.Context())

	_, err := h.queries.GetProjectByIDAndOwner(r.Context(), db.GetProjectByIDAndOwnerParams{
		ID: projectID, OwnerID: userID,
	})
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Project not found")
		return
	}

	sub, err := h.queries.GetSubscriptionByProject(r.Context(), projectID)
	if err != nil || sub.StripeCustomerID.String == "" {
		writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "No active subscription")
		return
	}

	appURL := os.Getenv("NEXT_PUBLIC_APP_URL")
	if appURL == "" {
		appURL = "https://app.unipost.dev"
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sub.StripeCustomerID.String),
		ReturnURL: stripe.String(appURL + "/projects/" + projectID + "/billing"),
	}

	s, err := session.New(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create portal session")
		return
	}

	writeSuccess(w, map[string]string{"portal_url": s.URL})
}

// GetUsage handles GET /v1/usage (API key auth)
func (h *BillingHandler) GetUsage(w http.ResponseWriter, r *http.Request) {
	projectID := auth.GetProjectID(r.Context())
	if projectID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing project context")
		return
	}

	h.quota.EnsureSubscription(r.Context(), projectID)
	status := h.quota.Check(r.Context(), projectID)

	sub, _ := h.queries.GetSubscriptionByProject(r.Context(), projectID)
	planID := "free"
	if sub.PlanID != "" {
		planID = sub.PlanID
	}

	writeSuccess(w, map[string]any{
		"period":     status.Period(),
		"post_count": status.Usage,
		"post_limit": status.Limit,
		"plan":       planID,
		"percentage": status.Percentage,
		"warning":    status.Warning,
	})
}

// ListPlans handles GET /v1/plans
func (h *BillingHandler) ListPlans(w http.ResponseWriter, r *http.Request) {
	plans, err := h.queries.ListPlans(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list plans")
		return
	}

	type planResponse struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		PriceCents int32 `json:"price_cents"`
		PostLimit  int32 `json:"post_limit"`
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
