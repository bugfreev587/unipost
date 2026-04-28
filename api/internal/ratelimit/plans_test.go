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
		{"p10", 20, 200, 1000},
		{"p25", 40, 500, 3000},
		{"p50", 60, 1000, 10000},
		{"p75", 60, 1000, 10000},
		{"p150", 120, 3000, 50000},
		{"p300", 120, 3000, 50000},
		{"p500", 120, 3000, 50000},
		{"p1000", 120, 3000, 50000},
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

func TestLimitsForPlan_UnknownFallsBackToFree(t *testing.T) {
	for _, id := range []string{"", "garbage", "enterprise", "p999"} {
		got := LimitsForPlan(id)
		want := FreeLimits
		if got != want {
			t.Errorf("LimitsForPlan(%q) = %+v, want FreeLimits %+v", id, got, want)
		}
	}
}
