package ratelimit

import "time"

// PlanLimits is the static per-plan threshold map. Keys match
// plans.id values from migration 058 (free, api, basic, growth, team,
// enterprise). Values are intentionally conservative runtime safety
// limits — they do NOT track monthly post quota. See §7 of the
// rate-limit PRD for the rationale.
//
// The May 2026 pricing redesign (migration 058) renamed the tiers
// from per-volume (p10..p1000) to product-tier (api/basic/growth/
// team). Rate-limit envelopes carry over conservatively:
//
//   free → FreeLimits           (unchanged)
//   api  → apiLimits            (single-developer API consumer)
//   basic → basicLimits          (dashboard + light API)
//   growth → growthLimits        (embedded SaaS, heavier load)
//   team → teamLimits            (multi-operator + agency scale)
//   enterprise → teamLimits      (custom contracts get manual tuning)
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

// apiLimits — single-developer API consumer. Carry-over from the
// legacy p10 envelope; comparable raw publish ceiling (1k posts/mo).
var apiLimits = PlanLimits{
	RequestRatePerSec:        1.0, // 60 / minute
	RequestBurstTokens:       20,
	EnqueuePostsPerMin:       200,
	EnqueuePostsPer5Min:      500,
	WorkspaceQueueDepthCap:   1000,
	ManagedUserQueueDepthCap: 50,
}

// basicLimits — dashboard + light API. Sized halfway between the
// legacy p25 and p50 envelopes; 2.5k posts/mo with manual posting.
var basicLimits = PlanLimits{
	RequestRatePerSec:        2.0, // 120 / minute
	RequestBurstTokens:       40,
	EnqueuePostsPerMin:       500,
	EnqueuePostsPer5Min:      1500,
	WorkspaceQueueDepthCap:   3000,
	ManagedUserQueueDepthCap: 100,
}

// growthLimits — embedded SaaS / white-label customer. Carry-over
// from the legacy p50 envelope; 7.5k posts/mo with heavier API load
// expected from customer-facing integrations.
var growthLimits = PlanLimits{
	RequestRatePerSec:        4.0, // 240 / minute
	RequestBurstTokens:       60,
	EnqueuePostsPerMin:       1000,
	EnqueuePostsPer5Min:      3000,
	WorkspaceQueueDepthCap:   10000,
	ManagedUserQueueDepthCap: 250,
}

// teamLimits — multi-operator team / agency scale. Carry-over from
// the legacy p150+ envelope; 25k posts/mo with concurrent-operator
// burst patterns expected.
var teamLimits = PlanLimits{
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
	case "api":
		return apiLimits
	case "basic":
		return basicLimits
	case "growth":
		return growthLimits
	case "team", "enterprise":
		return teamLimits
	default:
		return FreeLimits
	}
}
