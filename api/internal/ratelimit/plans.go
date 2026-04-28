package ratelimit

import "time"

// PlanLimits is the static per-plan threshold map. Keys match
// plans.id values from migration 012 (free, p10, p25, p50, p75,
// p150, p300, p500, p1000). Values are intentionally conservative
// runtime safety limits — they do NOT track monthly post quota. See
// §7 of the PRD for the rationale.
//
// Unknown plan IDs and the empty string fall back to FreeLimits,
// which is the safest default for a workspace with no subscription
// row yet.
type PlanLimits struct {
	// Request limiter (token bucket).
	RequestRatePerSec  float64
	RequestBurstTokens int

	// Enqueue limiter (sliding window). The 1-min and 5-min windows
	// are checked together; either breach denies admission.
	EnqueuePostsPerMin    int
	EnqueuePostsPer5Min   int

	// Queue depth limiter.
	WorkspaceQueueDepthCap   int
	ManagedUserQueueDepthCap int
}

// Window returns the sliding-window durations the enqueue limiter
// checks. Kept here so all rate-limit-relevant timing decisions live
// next to the threshold values they pair with.
func (PlanLimits) Window1m() time.Duration  { return 1 * time.Minute }
func (PlanLimits) Window5m() time.Duration  { return 5 * time.Minute }

// FreeLimits is also used as the fallback for unknown / missing
// plan IDs.
var FreeLimits = PlanLimits{
	RequestRatePerSec:        0.5, // 30 / minute
	RequestBurstTokens:       10,
	EnqueuePostsPerMin:       50,
	EnqueuePostsPer5Min:      100,
	WorkspaceQueueDepthCap:   200,
	ManagedUserQueueDepthCap: 25,
}

var p10Limits = PlanLimits{
	RequestRatePerSec:        1.0, // 60 / minute
	RequestBurstTokens:       20,
	EnqueuePostsPerMin:       200,
	EnqueuePostsPer5Min:      500,
	WorkspaceQueueDepthCap:   1000,
	ManagedUserQueueDepthCap: 50,
}

var p25Limits = PlanLimits{
	RequestRatePerSec:        2.0, // 120 / minute
	RequestBurstTokens:       40,
	EnqueuePostsPerMin:       500,
	EnqueuePostsPer5Min:      1500,
	WorkspaceQueueDepthCap:   3000,
	ManagedUserQueueDepthCap: 100,
}

var p50Limits = PlanLimits{
	RequestRatePerSec:        4.0, // 240 / minute
	RequestBurstTokens:       60,
	EnqueuePostsPerMin:       1000,
	EnqueuePostsPer5Min:      3000,
	WorkspaceQueueDepthCap:   10000,
	ManagedUserQueueDepthCap: 250,
}

// p75 shares the p50 envelope. p150+ shares one tier per the PRD §7.
var p75Limits = p50Limits

var p150PlusLimits = PlanLimits{
	RequestRatePerSec:        8.0, // 480 / minute
	RequestBurstTokens:       120,
	EnqueuePostsPerMin:       3000,
	EnqueuePostsPer5Min:      10000,
	WorkspaceQueueDepthCap:   50000,
	ManagedUserQueueDepthCap: 1000,
}

// LimitsForPlan returns the runtime safety limits for a plans.id
// value. Anything unknown (including the empty string for workspaces
// without a subscription row) falls back to FreeLimits.
func LimitsForPlan(planID string) PlanLimits {
	switch planID {
	case "free":
		return FreeLimits
	case "p10":
		return p10Limits
	case "p25":
		return p25Limits
	case "p50":
		return p50Limits
	case "p75":
		return p75Limits
	case "p150", "p300", "p500", "p1000":
		return p150PlusLimits
	default:
		return FreeLimits
	}
}
