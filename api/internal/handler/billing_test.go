package handler

import (
	"reflect"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
)

func TestCheckoutMetadataIncludesRuntimeEnvironment(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, " staging ")

	got := stripeCheckoutMetadata("ws_staging", "basic", "sandbox")

	want := map[string]string{
		"workspace_id":        "ws_staging",
		"plan_id":             "basic",
		"mode":                "sandbox",
		"unipost_environment": "staging",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
}

func TestCheckoutSubscriptionDataReusesRoutingMetadata(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	metadata := stripeCheckoutMetadata("ws_staging", "basic", "sandbox")

	data := stripeCheckoutSubscriptionData(metadata)

	if !reflect.DeepEqual(data.Metadata, metadata) {
		t.Fatalf("subscription metadata = %#v, want %#v", data.Metadata, metadata)
	}
}

func TestUsageResponseFromMonthlySnapshot(t *testing.T) {
	snapshot := quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   2488,
		Scheduled:   12,
		QuotaHold:   2,
		Limit:       2500,
	}

	response := usageResponseFromSnapshot(snapshot)

	if response.PostCount != 2488 || response.ScheduledCount != 12 || response.QuotaHoldCount != 2 {
		t.Fatalf("usage counts = %#v", response)
	}
	if response.EffectiveUsage != 2500 || response.Percentage != 99.52 || response.EffectivePercentage != 100 {
		t.Fatalf("usage percentages = %#v", response)
	}
	if response.Warning != "scheduled_quota_reached" || response.SchedulingAllowed {
		t.Fatalf("scheduling state = %#v", response)
	}
	wantReset := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	if !response.ResetsAt.Equal(wantReset) {
		t.Fatalf("resets_at = %s, want %s", response.ResetsAt, wantReset)
	}
}

func TestUsageResponseKeepsPaidSchedulingOpenBelow100(t *testing.T) {
	response := usageResponseFromSnapshot(quota.MonthlySnapshot{
		PlanID:    "api",
		Period:    "2026-07",
		Completed: 790,
		Scheduled: 10,
		Limit:     1000,
	})
	if response.Warning != "approaching_limit" || !response.SchedulingAllowed {
		t.Fatalf("response = %#v", response)
	}
}

func TestUsageResponsePausesSchedulingWhileQuotaHoldsExist(t *testing.T) {
	response := usageResponseFromSnapshot(quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   70,
		Scheduled:   10,
		QuotaHold:   5,
		Limit:       100,
	})
	if response.SchedulingAllowed || response.Warning != "scheduled_quota_reached" {
		t.Fatalf("hold response = %#v, want scheduling paused", response)
	}
}

func TestUsageResponseDoesNotApplyPaidCircuitBreakerToExcludedPlans(t *testing.T) {
	for _, planID := range []string{"free", "team", "enterprise"} {
		response := usageResponseFromSnapshot(quota.MonthlySnapshot{
			PlanID:    planID,
			Period:    "2026-07",
			Completed: 200,
			Limit:     100,
		})
		if !response.SchedulingAllowed {
			t.Fatalf("%s scheduling should remain governed by its existing policy", planID)
		}
	}
}
