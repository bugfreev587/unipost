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

	// PerPlatformDailyCap exposes the static (account, platform) →
	// daily publish ceiling so the dashboard can render a "your
	// safety caps" panel without hard-coding the numbers in the
	// frontend. Mirrors quota.PerPlatformDailyCap exactly.
	PerPlatformDailyCap map[string]int `json:"per_platform_daily_cap"`

	// Plan-feature gate flags (migrations 013 + 057 + 059). The
	// dashboard uses these to render upgrade gates / grey out tiles
	// instead of letting the user click into a feature only to hit
	// a 402.
	PlanAllowsTwitter               bool    `json:"plan_allows_twitter"`
	PlanAllowsInbox                 bool    `json:"plan_allows_inbox"`
	PlanAllowsAnalytics             bool    `json:"plan_allows_analytics"`
	PlanAllowsWhiteLabel            bool    `json:"plan_allows_white_label"`
	PlanAllowsHostedConnectBranding bool    `json:"plan_allows_hosted_connect_branding"`
	PlanAllowsHidePoweredBy         bool    `json:"plan_allows_hide_powered_by"`
	WhiteLabelPlatformLimit         int     `json:"white_label_platform_limit"`
	CustomPlatformSlot              *string `json:"custom_platform_slot"`

	// MaxProfiles is the per-plan profile cap (NULL/unlimited = -1).
	// CurrentProfiles is the live count for this workspace. The
	// dashboard renders "5 of 5 profiles used — upgrade for more"
	// from this pair.
	MaxProfiles     int `json:"max_profiles"`
	CurrentProfiles int `json:"current_profiles"`

	// MaxMembers / CurrentMembers — same shape as profiles, gates
	// the Members invite form. -1 = unlimited (Team / Enterprise).
	MaxMembers     int `json:"max_members"`
	CurrentMembers int `json:"current_members"`

	// Plan-packaging caps used by Free-plan admission checks. Paid
	// plans return -1 for the max fields in Phase 2 because they are
	// intentionally not hard-capped on these dimensions yet.
	MaxAPIKeys             int `json:"max_api_keys"`
	CurrentAPIKeys         int `json:"current_api_keys"`
	MaxWebhooks            int `json:"max_webhooks"`
	CurrentWebhooks        int `json:"current_webhooks"`
	MaxManagedAccounts     int `json:"max_managed_accounts"`
	CurrentManagedAccounts int `json:"current_managed_accounts"`
	MaxManagedUsers        int `json:"max_managed_users"`
	CurrentManagedUsers    int `json:"current_managed_users"`
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

	maxProfiles := -1
	if cap, hasCap := h.quota.MaxProfilesForPlan(r.Context(), workspaceID); hasCap {
		maxProfiles = cap
	}
	currentProfiles, _ := h.queries.CountProfilesByWorkspace(r.Context(), workspaceID)

	maxMembers := -1
	if cap, hasCap := h.quota.MaxMembersForPlan(r.Context(), workspaceID); hasCap {
		maxMembers = cap
	}
	currentMembers, _ := h.queries.CountActiveMembersByWorkspace(r.Context(), workspaceID)

	maxAPIKeys := -1
	if cap, hasCap := h.quota.MaxAPIKeysForPlan(r.Context(), workspaceID); hasCap {
		maxAPIKeys = cap
	}
	currentAPIKeys, _ := h.queries.CountActiveAPIKeysByWorkspace(r.Context(), workspaceID)

	maxWebhooks := -1
	if cap, hasCap := h.quota.MaxWebhooksForPlan(r.Context(), workspaceID); hasCap {
		maxWebhooks = cap
	}
	currentWebhooks, _ := h.queries.CountActiveWebhooksByWorkspace(r.Context(), workspaceID)

	maxManagedAccounts := -1
	if cap, hasCap := h.quota.MaxManagedAccountsForPlan(r.Context(), workspaceID); hasCap {
		maxManagedAccounts = cap
	}
	currentManagedAccounts, _ := h.queries.CountActiveManagedAccountsByWorkspace(r.Context(), workspaceID)

	maxManagedUsers := -1
	if cap, hasCap := h.quota.MaxManagedUsersForPlan(r.Context(), workspaceID); hasCap {
		maxManagedUsers = cap
	}
	currentManagedUsers, _ := h.queries.CountManagedUsersByWorkspace(r.Context(), workspaceID)

	var customPlatformSlot *string
	if ws, err := h.queries.GetWorkspace(r.Context(), workspaceID); err == nil && ws.CustomPlatformSlot.Valid {
		v := ws.CustomPlatformSlot.String
		customPlatformSlot = &v
	}

	writeSuccess(w, apiLimitsResponse{
		PlanID:                          planID,
		RequestRatePerMin:               requestPerMin,
		RequestBurst:                    limits.RequestBurstTokens,
		EnqueuePostsPerMin:              limits.EnqueuePostsPerMin,
		EnqueuePostsPer5Min:             limits.EnqueuePostsPer5Min,
		QueueDepthCap:                   limits.WorkspaceQueueDepthCap,
		ManagedUserDepthCap:             limits.ManagedUserQueueDepthCap,
		QueueDepthCurrent:               int(depth),
		PerPlatformDailyCap:             copyDailyCapMap(),
		PlanAllowsTwitter:               h.quota.PlanAllowsPlatform(r.Context(), workspaceID, "twitter"),
		PlanAllowsInbox:                 h.quota.PlanAllowsInbox(r.Context(), workspaceID),
		PlanAllowsAnalytics:             h.quota.PlanAllowsAnalytics(r.Context(), workspaceID),
		PlanAllowsWhiteLabel:            h.quota.PlanAllowsWhiteLabel(r.Context(), workspaceID),
		PlanAllowsHostedConnectBranding: h.quota.PlanAllowsHostedConnectBranding(r.Context(), workspaceID),
		PlanAllowsHidePoweredBy:         h.quota.PlanAllowsHidePoweredBy(r.Context(), workspaceID),
		WhiteLabelPlatformLimit:         h.quota.WhiteLabelPlatformLimit(r.Context(), workspaceID),
		CustomPlatformSlot:              customPlatformSlot,
		MaxProfiles:                     maxProfiles,
		CurrentProfiles:                 int(currentProfiles),
		MaxMembers:                      maxMembers,
		CurrentMembers:                  int(currentMembers),
		MaxAPIKeys:                      maxAPIKeys,
		CurrentAPIKeys:                  int(currentAPIKeys),
		MaxWebhooks:                     maxWebhooks,
		CurrentWebhooks:                 int(currentWebhooks),
		MaxManagedAccounts:              maxManagedAccounts,
		CurrentManagedAccounts:          int(currentManagedAccounts),
		MaxManagedUsers:                 maxManagedUsers,
		CurrentManagedUsers:             int(currentManagedUsers),
	})
}

// copyDailyCapMap returns a defensive copy of quota.PerPlatformDailyCap
// so a JSON encoder mutating callers can't reach the package-level map.
// Cheap — the map has fewer than a dozen entries.
func copyDailyCapMap() map[string]int {
	out := make(map[string]int, len(quota.PerPlatformDailyCap))
	for k, v := range quota.PerPlatformDailyCap {
		out[k] = v
	}
	return out
}
