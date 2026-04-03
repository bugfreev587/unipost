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

// Check returns the quota status for a project. Never blocks — soft limit only.
func (c *Checker) Check(ctx context.Context, projectID string) QuotaStatus {
	sub, err := c.queries.GetSubscriptionByProject(ctx, projectID)
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
		ProjectID: projectID,
		Period:    period,
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

// Increment adds to the usage count for the current period.
func (c *Checker) Increment(ctx context.Context, projectID string, count int) {
	c.queries.IncrementUsage(ctx, db.IncrementUsageParams{
		ProjectID: projectID,
		Period:    currentPeriod(),
		PostCount: int32(count),
	})
}

// EnsureSubscription creates a free subscription if one doesn't exist.
func (c *Checker) EnsureSubscription(ctx context.Context, projectID string) {
	c.queries.CreateSubscription(ctx, db.CreateSubscriptionParams{
		ProjectID: projectID,
		PlanID:    "free",
		Status:    "active",
	})
}

func currentPeriod() string {
	return time.Now().Format("2006-01")
}
