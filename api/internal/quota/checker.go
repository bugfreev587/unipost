package quota

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/db"
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

func NewChecker(queries *db.Queries) *Checker {
	return &Checker{queries: queries}
}

// Check returns the quota status for a workspace. Never blocks — soft limit only.
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
