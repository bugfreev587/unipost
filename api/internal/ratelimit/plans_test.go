package ratelimit

import "testing"

func TestLimitsForPlan_KnownTiers(t *testing.T) {
	cases := []struct {
		planID       string
		wantBurst    int
		wantPerMin   int
		wantDepthCap int
	}{
		{"free", 10, 50, 200},
		{"api", 20, 200, 1000},
		{"basic", 40, 500, 3000},
		{"growth", 60, 1000, 10000},
		{"team", 120, 3000, 50000},
		{"enterprise", 120, 3000, 50000},
	}
	for _, tc := range cases {
		t.Run(tc.planID, func(t *testing.T) {
			lim := LimitsForPlan(tc.planID)
			if lim.RequestBurstTokens != tc.wantBurst {
				t.Errorf("plan %s: burst = %d, want %d", tc.planID, lim.RequestBurstTokens, tc.wantBurst)
			}
			if lim.EnqueuePostsPerMin != tc.wantPerMin {
				t.Errorf("plan %s: enqueue/min = %d, want %d", tc.planID, lim.EnqueuePostsPerMin, tc.wantPerMin)
			}
			if lim.WorkspaceQueueDepthCap != tc.wantDepthCap {
				t.Errorf("plan %s: depth cap = %d, want %d", tc.planID, lim.WorkspaceQueueDepthCap, tc.wantDepthCap)
			}
		})
	}
}

// TestLimitsForPlan_UnknownFallsBackToFree — anything not explicitly
// listed (legacy p10..p1000 IDs, garbage strings, empty) falls through
// to FreeLimits. This is the safe-by-default behavior the May 2026
// pricing migration relies on: even if a stale plan_id sneaks through
// during deploy ordering, the affected workspace gets the most
// conservative envelope rather than failing open.
func TestLimitsForPlan_UnknownFallsBackToFree(t *testing.T) {
	for _, id := range []string{"", "garbage", "p10", "p25", "p50", "p75", "p150", "p300", "p500", "p1000", "p999"} {
		got := LimitsForPlan(id)
		want := FreeLimits
		if got != want {
			t.Errorf("LimitsForPlan(%q) = %+v, want FreeLimits %+v", id, got, want)
		}
	}
}
