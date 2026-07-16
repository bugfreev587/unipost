package xcredits

import "testing"

func TestPlanAllowance(t *testing.T) {
	tests := map[string]int64{
		"free":   0,
		"api":    1500,
		"basic":  4000,
		"growth": 12000,
		"team":   30000,
	}
	for plan, want := range tests {
		got, ok := PlanAllowance(plan)
		if !ok || got != want {
			t.Fatalf("PlanAllowance(%q) = %d, %v; want %d, true", plan, got, ok, want)
		}
	}
	if _, ok := PlanAllowance("enterprise"); ok {
		t.Fatal("enterprise allowance must remain contract-defined")
	}
}

func TestInboundDailyLimit(t *testing.T) {
	tests := map[string]int64{
		"free":   0,
		"api":    0,
		"basic":  400,
		"growth": 1200,
		"team":   3000,
	}
	for plan, want := range tests {
		got, ok := InboundDailyLimit(plan)
		if !ok || got != want {
			t.Fatalf("InboundDailyLimit(%q) = %d, %v; want %d, true", plan, got, ok, want)
		}
	}
	if _, ok := InboundDailyLimit("enterprise"); ok {
		t.Fatal("enterprise inbound limit must remain contract-defined")
	}
}

func TestOperationWeights(t *testing.T) {
	tests := map[string]int64{
		"post.create":           15,
		"post.create_url":       200,
		"post.reply_summoned":   10,
		"post.read":             5,
		"user.read":             10,
		"dm.read":               10,
		"dm.send":               15,
		"post.mention.received": 5,
		"dm.received":           10,
	}
	for operation, want := range tests {
		if got := OperationWeight(operation); got != want {
			t.Fatalf("OperationWeight(%q) = %d, want %d", operation, got, want)
		}
	}
}

func TestOperationCapacityUsesFloorRounding(t *testing.T) {
	basic, ok := PlanCapacity("basic")
	if !ok {
		t.Fatal("basic capacity missing")
	}
	if basic.NormalPosts != 266 || basic.URLPosts != 20 || basic.CommentInteractions != 200 || basic.DMInteractions != 160 {
		t.Fatalf("basic capacity = %+v", basic)
	}
}
