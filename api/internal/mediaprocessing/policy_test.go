package mediaprocessing

import "testing"

func TestLimitsForPlan(t *testing.T) {
	tests := []struct {
		plan          string
		active, daily int
	}{
		{plan: "free", active: 1, daily: 10},
		{plan: "api", active: 2, daily: 50},
		{plan: "basic", active: 2, daily: 100},
		{plan: "growth", active: 4, daily: 300},
		{plan: "team", active: 6, daily: 1000},
		{plan: "enterprise", active: 6, daily: 1000},
		{plan: "unknown", active: 1, daily: 10},
	}
	for _, tt := range tests {
		t.Run(tt.plan, func(t *testing.T) {
			got := LimitsForPlan(tt.plan, nil)
			if got.ActiveJobs != tt.active || got.GIFConversions24H != tt.daily {
				t.Fatalf("limits = %#v", got)
			}
		})
	}
}

func TestLimitsForPlanAllowsValidatedEnterpriseContractOverride(t *testing.T) {
	override := Limits{ActiveJobs: 12, GIFConversions24H: 5000}
	if got := LimitsForPlan("enterprise", &override); got != override {
		t.Fatalf("limits = %#v", got)
	}
	if got := LimitsForPlan("team", &override); got == override {
		t.Fatal("enterprise override leaked to team")
	}
	invalid := Limits{ActiveJobs: 0, GIFConversions24H: 5000}
	if got := LimitsForPlan("enterprise", &invalid); got != (Limits{ActiveJobs: 6, GIFConversions24H: 1000}) {
		t.Fatalf("invalid override was accepted: %#v", got)
	}
}
