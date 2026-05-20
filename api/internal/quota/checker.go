package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/featureflags"
)

type QuotaStatus struct {
	Allowed    bool    `json:"allowed"`
	Warning    string  `json:"warning,omitempty"`
	Message    string  `json:"message,omitempty"`
	Usage      int     `json:"usage"`
	Limit      int     `json:"limit"`
	Percentage float64 `json:"percentage"`
}

func (s QuotaStatus) Period() string {
	return currentPeriod()
}

type Checker struct {
	queries *db.Queries
}

type FreePlanHardBlockGate struct {
	Status  QuotaStatus
	planID  string
	enabled bool
}

func NewChecker(queries *db.Queries) *Checker {
	return &Checker{queries: queries}
}

// Check returns the quota status for a workspace. Paid plans use soft
// overage semantics; Free can be hard-blocked by
// FreePlanHardBlockStatus at publish-admission call sites.
func (c *Checker) Check(ctx context.Context, workspaceID string) QuotaStatus {
	sub, err := c.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil {
		// No subscription = free plan defaults
		return QuotaStatus{Allowed: true, Usage: 0, Limit: 100}
	}

	plan, err := c.queries.GetPlan(ctx, sub.PlanID)
	if err != nil {
		return QuotaStatus{Allowed: true, Usage: 0, Limit: 100}
	}

	if plan.PostLimit < 0 {
		return QuotaStatus{Allowed: true, Limit: -1} // unlimited
	}

	period := currentPeriod()
	usage, err := c.queries.GetUsage(ctx, db.GetUsageParams{
		WorkspaceID: workspaceID,
		Period:      period,
	})

	postCount := 0
	if err == nil {
		postCount = int(usage.PostCount)
	}

	pct := float64(postCount) / float64(plan.PostLimit) * 100

	status := QuotaStatus{
		Allowed:    true, // Never block
		Usage:      postCount,
		Limit:      int(plan.PostLimit),
		Percentage: pct,
	}

	switch {
	case pct >= 100:
		status.Warning = "over_limit"
		status.Message = "You've exceeded your monthly limit. Upgrade to avoid interruption."
	case pct >= 80:
		status.Warning = "approaching_limit"
		status.Message = fmt.Sprintf("You've used %.0f%% of your monthly posts.", pct)
	}

	return status
}

// FreePlanHardBlockStatus reports whether accepting additional publish
// units would push a Free workspace beyond its monthly post quota.
//
// The behavior is feature-flagged because it changes user-visible write
// semantics. Paid plans deliberately keep the existing soft-overage model.
func (c *Checker) FreePlanHardBlockStatus(ctx context.Context, workspaceID string, additionalPosts int) (QuotaStatus, bool) {
	gate := c.FreePlanHardBlockGate(ctx, workspaceID)
	return gate.Status, gate.Blocked(additionalPosts)
}

// FreePlanHardBlockGate snapshots the quota state for one request so
// batch callers can project accepted posts without re-reading quota for
// every item.
func (c *Checker) FreePlanHardBlockGate(ctx context.Context, workspaceID string) FreePlanHardBlockGate {
	status := c.Check(ctx, workspaceID)
	gate := FreePlanHardBlockGate{Status: status}
	if status.Limit < 0 {
		return gate
	}
	if !featureflags.Enabled(ctx, featureflags.FreePlanHardPostQuota, featureflags.Target{
		WorkspaceID: workspaceID,
	}) {
		return gate
	}
	gate.enabled = true
	gate.planID = c.PlanIDFor(ctx, workspaceID)
	return gate
}

func (g FreePlanHardBlockGate) Blocked(additionalPosts int) bool {
	if !g.enabled {
		return false
	}
	return shouldHardBlockFreePlanQuota(g.planID, g.Status, additionalPosts)
}

func shouldHardBlockFreePlanQuota(planID string, status QuotaStatus, additionalPosts int) bool {
	if planID != "free" || additionalPosts <= 0 || status.Limit < 0 {
		return false
	}
	return status.Usage+additionalPosts > status.Limit
}

// PlanIDFor returns the plan ID associated with a workspace, or
// "free" when no subscription row exists. Used by the rate-limit
// admission layer to look up the workspace's runtime safety
// thresholds (see internal/ratelimit/plans.go).
func (c *Checker) PlanIDFor(ctx context.Context, workspaceID string) string {
	sub, err := c.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil || sub.PlanID == "" {
		return "free"
	}
	return sub.PlanID
}

// PlanAllowsPlatform reports whether the workspace's plan permits
// publishing to the given platform. Today this only encodes one rule —
// the free plan disallows X / Twitter (migration 057) — but the helper
// is shaped so future plan-gated platforms slot in without churning
// every call site.
//
// Fail-open: any DB or lookup failure returns true. We would rather
// occasionally let a plan-gated publish through than break publishing
// for paying customers when subscriptions / plans tables are
// transiently unreadable. The publish path's other safety nets
// (validator, adapter rejection, billing usage cap) still apply.
//
// Unknown plan IDs and the empty string fall through to the
// "free" plan's row in the plans table; if that row is also
// unreadable, the helper returns true.
func (c *Checker) PlanAllowsPlatform(ctx context.Context, workspaceID, platform string) bool {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil {
		return true
	}
	switch platform {
	case "twitter":
		return plan.AllowTwitter
	default:
		return true
	}
}

// PlanAllowsInbox reports whether the workspace's plan unlocks the
// Inbox surface (DMs + comments). Migration 059 sets this true on
// Basic and up; Free + API are gated. Same fail-open rule as
// PlanAllowsPlatform.
func (c *Checker) PlanAllowsInbox(ctx context.Context, workspaceID string) bool {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil {
		return true
	}
	return plan.AllowInbox
}

// PlanAllowsAnalytics reports whether the workspace's plan unlocks
// the Analytics endpoints. Migration 059 sets this true on API and
// up; Free is gated. The "API tier is read-only" framing on the
// pricing page is a dashboard-side label — the API endpoints are all
// read-only anyway, so the server-side gate is a single boolean.
func (c *Checker) PlanAllowsAnalytics(ctx context.Context, workspaceID string) bool {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil {
		return true
	}
	return plan.AllowAnalytics
}

// PlanAllowsWhiteLabel reports whether the workspace's plan unlocks
// white-label / native-mode platform credentials. Migration 013
// added the plans.white_label column; the May 2026 ladder (058)
// sets this true on Growth and up. Free / API / Basic are gated.
func (c *Checker) PlanAllowsWhiteLabel(ctx context.Context, workspaceID string) bool {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil {
		return true
	}
	return plan.WhiteLabel
}

// PlanAllowsHostedConnectBranding reports whether the workspace may
// customize the hosted Connect surface (logo, display name, primary
// color). Basic and up are allowed; Free and API stay on UniPost's
// default branding. Fail-open on lookup errors, matching the rest of
// the package.
func (c *Checker) PlanAllowsHostedConnectBranding(ctx context.Context, workspaceID string) bool {
	planID := c.PlanIDFor(ctx, workspaceID)
	switch planID {
	case "basic", "growth", "team":
		return true
	default:
		return false
	}
}

// WhiteLabelPlatformLimit returns how many BYO platform credential rows
// the workspace may actively configure. -1 means unlimited.
//
// Product packaging:
//   - Free / API: 0
//   - Basic:      1
//   - Growth+:    unlimited
//
// Existing rows are not retroactively pruned on downgrade; handlers use
// this helper to block new additions when the plan is at capacity.
func (c *Checker) WhiteLabelPlatformLimit(ctx context.Context, workspaceID string) int {
	switch c.PlanIDFor(ctx, workspaceID) {
	case "basic":
		return 1
	case "growth", "team":
		return -1
	default:
		return 0
	}
}

// PlanAllowsHidePoweredBy reports whether the workspace may remove the
// "Powered by UniPost" attribution on hosted Connect pages. Growth and
// up are allowed.
func (c *Checker) PlanAllowsHidePoweredBy(ctx context.Context, workspaceID string) bool {
	switch c.PlanIDFor(ctx, workspaceID) {
	case "growth", "team":
		return true
	default:
		return false
	}
}

// MaxProfilesForPlan returns (limit, true) when the workspace's plan
// caps the number of profiles, or (0, false) when the plan permits
// unlimited profiles (Team / Enterprise) or when the plan row can't
// be loaded (fail-open).
//
// Used by the profile-create handler to reject a CREATE that would
// push the workspace over its cap. Existing profiles are NEVER
// retroactively pruned — a downgrade just blocks new creation.
func (c *Checker) MaxProfilesForPlan(ctx context.Context, workspaceID string) (int, bool) {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil || !plan.MaxProfiles.Valid {
		return 0, false
	}
	return int(plan.MaxProfiles.Int32), true
}

// MaxMembersForPlan returns (limit, true) when the workspace's plan
// caps the number of team members. Same semantics as MaxProfilesForPlan
// — Team / Enterprise return (0, false) for "unlimited", DB read
// failures fail open. Used by the invite handler.
func (c *Checker) MaxMembersForPlan(ctx context.Context, workspaceID string) (int, bool) {
	planID := c.PlanIDFor(ctx, workspaceID)
	plan, err := c.queries.GetPlan(ctx, planID)
	if err != nil || !plan.MaxMembers.Valid {
		return 0, false
	}
	return int(plan.MaxMembers.Int32), true
}

// Increment adds to the usage count for the current period.
func (c *Checker) Increment(ctx context.Context, workspaceID string, count int) {
	c.queries.IncrementUsage(ctx, db.IncrementUsageParams{
		WorkspaceID: workspaceID,
		Period:      currentPeriod(),
		PostCount:   int32(count),
	})
}

// EnsureSubscription creates a free subscription if one doesn't exist.
// Uses DO NOTHING to avoid overwriting an existing paid subscription.
func (c *Checker) EnsureSubscription(ctx context.Context, workspaceID string) {
	c.queries.EnsureSubscription(ctx, workspaceID)
}

func currentPeriod() string {
	return time.Now().Format("2006-01")
}
