// api_limits.go is the public-facing read of the rate-limit /
// queue-admission thresholds for the caller's workspace. The
// dashboard's "API Limits" settings page reads this so customers
// can see what their plan's runtime safety caps actually are
// without having to read source code or guess from 429 responses.
//
// This handler is read-only and cheap — one indexed plan lookup
// + one indexed COUNT against post_delivery_jobs (covered by
// migration 054's partial index). Safe to call on every dashboard
// page render.
//
// The shape mirrors what the backend actually enforces in
// internal/ratelimit/plans.go; if those numbers change, this
// response changes with them automatically since both read from
// the same LimitsForPlan() function.

package handler

import (
	"net/http"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/ratelimit"
)

type ApiLimitsHandler struct {
	queries *db.Queries
	quota   *quota.Checker
}

func NewApiLimitsHandler(queries *db.Queries, q *quota.Checker) *ApiLimitsHandler {
	return &ApiLimitsHandler{queries: queries, quota: q}
}

// apiLimitsResponse is the wire shape returned by GET /v1/limits.
// Field naming matches the human-readable concepts on the dashboard
// page (request rate / enqueue throughput / queue depth) so the
// frontend doesn't need a translation layer.
//
// QueueDepthCurrent is a snapshot, not real-time — the dashboard
// page polls this endpoint to refresh. Don't expose it as a
// streaming subscription; the cost-vs-utility is wrong for a page
// users open occasionally.
type apiLimitsResponse struct {
	PlanID string `json:"plan_id"`

	// Request limiter
	RequestRatePerMin int `json:"request_rate_per_min"`
	RequestBurst      int `json:"request_burst"`

	// Enqueue limiter
	EnqueuePostsPerMin  int `json:"enqueue_posts_per_min"`
	EnqueuePostsPer5Min int `json:"enqueue_posts_per_5min"`

	// Queue depth limiter
	QueueDepthCap       int `json:"queue_depth_cap"`
	ManagedUserDepthCap int `json:"managed_user_depth_cap"`

	// Live snapshot of the workspace's active delivery jobs
	// (states: pending / running / retrying). Compared against
	// QueueDepthCap to show "47 / 1000".
	QueueDepthCurrent int `json:"queue_depth_current"`
}

// Get handles GET /v1/limits. Auth comes from DualAuthMiddleware
// (API key or Clerk session). Workspace context is stamped into
// the request — handler reads it the same way every other
// workspace-scoped endpoint does.
func (h *ApiLimitsHandler) Get(w http.ResponseWriter, r *http.Request) {
	workspaceID := auth.GetWorkspaceID(r.Context())
	if workspaceID == "" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing workspace context")
		return
	}

	planID := h.quota.PlanIDFor(r.Context(), workspaceID)
	limits := ratelimit.LimitsForPlan(planID)

	// Best-effort current depth — a transient DB hiccup should not
	// take the whole settings page down. Zero is a safe fallback;
	// the page renders the rest of the limits and the user can
	// retry.
	depth, _ := h.queries.CountActiveDeliveryJobsByWorkspace(r.Context(), workspaceID)

	// Convert the float64 rate-per-second into the integer
	// per-minute number a customer naturally thinks in. Round down
	// — overstating the limit could make a customer think they
	// have more headroom than they do.
	requestPerMin := int(limits.RequestRatePerSec * 60)

	writeSuccess(w, apiLimitsResponse{
		PlanID:              planID,
		RequestRatePerMin:   requestPerMin,
		RequestBurst:        limits.RequestBurstTokens,
		EnqueuePostsPerMin:  limits.EnqueuePostsPerMin,
		EnqueuePostsPer5Min: limits.EnqueuePostsPer5Min,
		QueueDepthCap:       limits.WorkspaceQueueDepthCap,
		ManagedUserDepthCap: limits.ManagedUserQueueDepthCap,
		QueueDepthCurrent:   int(depth),
	})
}
