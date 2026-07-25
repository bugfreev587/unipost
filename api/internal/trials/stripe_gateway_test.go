package trials

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"
)

func TestBuildTrialCheckoutParams(t *testing.T) {
	req := CreateTrialCheckoutRequest{
		StripeMode:   "sandbox",
		WorkspaceID:  "ws_123",
		PlanID:       "growth",
		TrialGrantID: "grant_123",
		TrialKind:    KindFreeToPaid,
		Environment:  "development",
		CustomerID:   "cus_123",
		PriceID:      "price_growth",
		DurationDays: 45,
		SuccessURL:   "https://app.example/success",
		CancelURL:    "https://app.example/cancel",
	}

	params, err := buildTrialCheckoutParams(req)
	if err != nil {
		t.Fatalf("buildTrialCheckoutParams() error = %v", err)
	}
	if got := stripe.StringValue(params.Mode); got != "subscription" {
		t.Fatalf("Mode = %q, want subscription", got)
	}
	if got := stripe.StringValue(params.PaymentMethodCollection); got != "always" {
		t.Fatalf("PaymentMethodCollection = %q, want always", got)
	}
	if got := stripe.StringValue(params.SuccessURL); got != req.SuccessURL {
		t.Fatalf("SuccessURL = %q", got)
	}
	if got := stripe.StringValue(params.CancelURL); got != req.CancelURL {
		t.Fatalf("CancelURL = %q", got)
	}
	if got := stripe.StringValue(params.Customer); got != req.CustomerID {
		t.Fatalf("Customer = %q", got)
	}
	if len(params.LineItems) != 1 || stripe.StringValue(params.LineItems[0].Price) != req.PriceID || stripe.Int64Value(params.LineItems[0].Quantity) != 1 {
		t.Fatalf("LineItems = %#v, want exactly one unit of %q", params.LineItems, req.PriceID)
	}
	if params.SubscriptionData == nil || stripe.Int64Value(params.SubscriptionData.TrialPeriodDays) != 45 {
		t.Fatalf("SubscriptionData trial = %#v, want 45 days", params.SubscriptionData)
	}
	assertTrialMetadata(t, params.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	assertTrialMetadata(t, params.SubscriptionData.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	if got := stripe.StringValue(params.IdempotencyKey); got != "trial:grant_123:checkout" {
		t.Fatalf("IdempotencyKey = %q", got)
	}
}

func TestBuildTrialCheckoutParamsRejectsDurationOutsideStripeBounds(t *testing.T) {
	for _, days := range []int32{0, 731} {
		_, err := buildTrialCheckoutParams(CreateTrialCheckoutRequest{DurationDays: days})
		if err == nil {
			t.Fatalf("buildTrialCheckoutParams(DurationDays=%d) error = nil", days)
		}
	}
}

func TestBuildPaidTrialScheduleParamsUsesStableSubOperationKeysAndMetadata(t *testing.T) {
	currentStart := time.Unix(1_800_000_000, 0).UTC()
	trialStart := currentStart.Add(15 * 24 * time.Hour)
	trialEnd := trialStart.Add(45 * 24 * time.Hour)
	req := CreatePaidTrialScheduleRequest{
		StripeMode:     "live",
		WorkspaceID:    "ws_paid",
		PlanID:         "team",
		TrialGrantID:   "grant_paid",
		TrialKind:      KindPaidSamePlan,
		Environment:    "production",
		SubscriptionID: "sub_paid",
		PriceID:        "price_team",
		DurationDays:   45,
		CurrentPhase: SchedulePhase{
			PriceID:            "price_team",
			StartAt:            currentStart,
			EndAt:              trialStart,
			BillingCycleAnchor: "automatic",
			Metadata:           map[string]string{"retained": "yes"},
		},
		TrialStartAt: trialStart,
		TrialEndAt:   trialEnd,
	}

	createParams, updateParams, err := buildPaidTrialScheduleParams(req)
	if err != nil {
		t.Fatalf("buildPaidTrialScheduleParams() error = %v", err)
	}
	if got := stripe.StringValue(createParams.FromSubscription); got != req.SubscriptionID {
		t.Fatalf("FromSubscription = %q", got)
	}
	if got := stripe.StringValue(createParams.IdempotencyKey); got != "trial:grant_paid:schedule:create" {
		t.Fatalf("create IdempotencyKey = %q", got)
	}
	if got := stripe.StringValue(updateParams.IdempotencyKey); got != "trial:grant_paid:schedule:update" {
		t.Fatalf("update IdempotencyKey = %q", got)
	}
	if got := stripe.StringValue(updateParams.EndBehavior); got != "release" {
		t.Fatalf("EndBehavior = %q, want release", got)
	}
	if got := stripe.StringValue(updateParams.ProrationBehavior); got != "none" {
		t.Fatalf("ProrationBehavior = %q, want none", got)
	}
	assertTrialMetadata(t, updateParams.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	if len(updateParams.Phases) != 3 {
		t.Fatalf("len(Phases) = %d, want 3", len(updateParams.Phases))
	}
	for i, phase := range updateParams.Phases {
		assertTrialMetadata(t, phase.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
		if len(phase.Items) != 1 || stripe.StringValue(phase.Items[0].Price) != req.PriceID {
			t.Fatalf("phase %d Items = %#v", i, phase.Items)
		}
	}
	if updateParams.Phases[0].Metadata["retained"] != "yes" {
		t.Fatalf("current phase metadata was not retained: %#v", updateParams.Phases[0].Metadata)
	}
	if !stripe.BoolValue(updateParams.Phases[1].Trial) || updateParams.Phases[1].TrialEnd != nil {
		t.Fatalf("trial phase = %#v", updateParams.Phases[1])
	}
	paidPhase := updateParams.Phases[2]
	if stripe.BoolValue(paidPhase.Trial) || paidPhase.TrialEnd != nil {
		t.Fatalf("resumed paid phase unexpectedly has a trial: %#v", paidPhase)
	}
	if got := stripe.StringValue(paidPhase.BillingCycleAnchor); got != "phase_start" {
		t.Fatalf("resumed paid BillingCycleAnchor = %q", got)
	}
}

func TestBuildChangeFreeTrialPlanParamsReplacesExistingItemAndEndsTrialNow(t *testing.T) {
	req := ChangeFreeTrialPlanRequest{
		StripeMode:         "live",
		WorkspaceID:        "ws_free",
		PlanID:             "basic",
		TrialGrantID:       "grant_free",
		TrialKind:          KindFreeToPaid,
		Environment:        "production",
		SubscriptionID:     "sub_free",
		SubscriptionItemID: "si_existing",
		PriceID:            "price_basic",
	}
	params, err := buildChangeFreeTrialPlanParams(req)
	if err != nil {
		t.Fatalf("buildChangeFreeTrialPlanParams() error = %v", err)
	}
	if len(params.Items) != 1 {
		t.Fatalf("len(Items) = %d, want one replacement", len(params.Items))
	}
	if got := stripe.StringValue(params.Items[0].ID); got != req.SubscriptionItemID {
		t.Fatalf("item ID = %q", got)
	}
	if got := stripe.StringValue(params.Items[0].Price); got != req.PriceID {
		t.Fatalf("item Price = %q", got)
	}
	if !stripe.BoolValue(params.TrialEndNow) || !stripe.BoolValue(params.BillingCycleAnchorNow) {
		t.Fatalf("trial/billing anchors not set to now: %#v", params)
	}
	if got := stripe.StringValue(params.ProrationBehavior); got != "none" {
		t.Fatalf("ProrationBehavior = %q", got)
	}
	if got := stripe.StringValue(params.PaymentBehavior); got != "error_if_incomplete" {
		t.Fatalf("PaymentBehavior = %q", got)
	}
	if got := stripe.StringValue(params.IdempotencyKey); got != "trial:grant_free:change_plan" {
		t.Fatalf("IdempotencyKey = %q", got)
	}
	assertTrialMetadata(t, params.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
}

func TestBuildChangeScheduledTrialPlanParamsIsOneScheduleUpdateWithNoRelease(t *testing.T) {
	req := ChangeScheduledTrialPlanRequest{
		StripeMode:   "sandbox",
		WorkspaceID:  "ws_paid",
		PlanID:       "api",
		TrialGrantID: "grant_paid",
		TrialKind:    KindPaidSamePlan,
		Environment:  "development",
		ScheduleID:   "sub_sched_123",
		PriceID:      "price_api",
	}
	params, err := buildChangeScheduledTrialPlanParams(req)
	if err != nil {
		t.Fatalf("buildChangeScheduledTrialPlanParams() error = %v", err)
	}
	if len(params.Phases) != 1 {
		t.Fatalf("len(Phases) = %d, want one paid target phase", len(params.Phases))
	}
	phase := params.Phases[0]
	if !stripe.BoolValue(phase.StartDateNow) {
		t.Fatal("paid target phase must start now")
	}
	if got := stripe.StringValue(phase.BillingCycleAnchor); got != "phase_start" {
		t.Fatalf("BillingCycleAnchor = %q", got)
	}
	if stripe.BoolValue(phase.Trial) || phase.TrialEnd != nil || stripe.BoolValue(phase.TrialEndNow) {
		t.Fatalf("paid target phase has trial fields: %#v", phase)
	}
	if got := stripe.StringValue(phase.ProrationBehavior); got != "none" {
		t.Fatalf("phase ProrationBehavior = %q", got)
	}
	if got := stripe.StringValue(params.ProrationBehavior); got != "none" {
		t.Fatalf("request ProrationBehavior = %q", got)
	}
	if got := stripe.StringValue(params.EndBehavior); got != "release" {
		t.Fatalf("EndBehavior = %q", got)
	}
	if got := stripe.StringValue(params.IdempotencyKey); got != "trial:grant_paid:change_plan" {
		t.Fatalf("IdempotencyKey = %q", got)
	}
	if _, ok := reflect.TypeOf((*StripeGateway)(nil)).Elem().MethodByName("Release"); ok {
		t.Fatal("StripeGateway must not expose a Release operation")
	}
}

func TestChangeScheduledTrialPlanCallsUpdateExactlyOnceAndNeverRelease(t *testing.T) {
	client := &recordingSchedulePlanChanger{}
	req := ChangeScheduledTrialPlanRequest{
		StripeMode:   "sandbox",
		WorkspaceID:  "ws_paid",
		PlanID:       "api",
		TrialGrantID: "grant_paid",
		TrialKind:    KindPaidSamePlan,
		Environment:  "development",
		ScheduleID:   "sub_sched_123",
		PriceID:      "price_api",
	}

	if _, err := changeScheduledTrialPlanNow(t.Context(), client, req); err != nil {
		t.Fatalf("changeScheduledTrialPlanNow() error = %v", err)
	}
	if client.updateCalls != 1 {
		t.Fatalf("Update calls = %d, want exactly 1", client.updateCalls)
	}
	if client.releaseCalls != 0 {
		t.Fatalf("Release calls = %d, want 0", client.releaseCalls)
	}
}

func TestBuildCancelParams(t *testing.T) {
	freeReq := CancelFreeTrialRequest{
		StripeMode:     "live",
		WorkspaceID:    "ws_free",
		PlanID:         "growth",
		TrialGrantID:   "grant_free",
		TrialKind:      KindFreeToPaid,
		Environment:    "production",
		SubscriptionID: "sub_free",
	}
	freeParams, err := buildCancelFreeTrialParams(freeReq)
	if err != nil {
		t.Fatalf("buildCancelFreeTrialParams() error = %v", err)
	}
	if !stripe.BoolValue(freeParams.CancelAtPeriodEnd) {
		t.Fatal("free cancellation must set cancel_at_period_end=true")
	}
	if got := stripe.StringValue(freeParams.IdempotencyKey); got != "trial:grant_free:cancel_renewal" {
		t.Fatalf("free IdempotencyKey = %q", got)
	}

	start := time.Unix(1_800_000_000, 0).UTC()
	paidReq := CancelPaidScheduleRequest{
		StripeMode:   "live",
		WorkspaceID:  "ws_paid",
		PlanID:       "growth",
		TrialGrantID: "grant_paid",
		TrialKind:    KindPaidSamePlan,
		Environment:  "production",
		ScheduleID:   "sub_sched_paid",
		RetainedPhases: []SchedulePhase{
			{PriceID: "price_growth", StartAt: start, EndAt: start.Add(24 * time.Hour)},
			{PriceID: "price_growth", StartAt: start.Add(24 * time.Hour), EndAt: start.Add(31 * 24 * time.Hour), TrialEndAt: start.Add(31 * 24 * time.Hour)},
		},
	}
	paidParams, err := buildCancelPaidScheduleParams(paidReq)
	if err != nil {
		t.Fatalf("buildCancelPaidScheduleParams() error = %v", err)
	}
	if got := stripe.StringValue(paidParams.EndBehavior); got != "cancel" {
		t.Fatalf("EndBehavior = %q", got)
	}
	if len(paidParams.Phases) != len(paidReq.RetainedPhases) {
		t.Fatalf("len(Phases) = %d, want %d retained phases", len(paidParams.Phases), len(paidReq.RetainedPhases))
	}
	if !stripe.BoolValue(paidParams.Phases[1].Trial) || paidParams.Phases[1].TrialEnd != nil {
		t.Fatalf("retained full trial phase = %#v, want trial=true without an invalid trial_end equal to end_date", paidParams.Phases[1])
	}
	if got := stripe.StringValue(paidParams.IdempotencyKey); got != "trial:grant_paid:cancel_renewal" {
		t.Fatalf("paid IdempotencyKey = %q", got)
	}
}

func TestMutatingParamBuildersRejectEmptyGrantID(t *testing.T) {
	now := time.Unix(1_800_000_000, 0).UTC()
	checks := []struct {
		name  string
		build func() error
	}{
		{name: "checkout", build: func() error {
			_, err := buildTrialCheckoutParams(CreateTrialCheckoutRequest{DurationDays: 1})
			return err
		}},
		{name: "schedule", build: func() error {
			_, _, err := buildPaidTrialScheduleParams(CreatePaidTrialScheduleRequest{
				SubscriptionID: "sub_123", PriceID: "price_123", DurationDays: 1,
				TrialStartAt: now, TrialEndAt: now.Add(24 * time.Hour),
			})
			return err
		}},
		{name: "free plan change", build: func() error {
			_, err := buildChangeFreeTrialPlanParams(ChangeFreeTrialPlanRequest{
				SubscriptionID: "sub_123", SubscriptionItemID: "si_123", PriceID: "price_123",
			})
			return err
		}},
		{name: "scheduled plan change", build: func() error {
			_, err := buildChangeScheduledTrialPlanParams(ChangeScheduledTrialPlanRequest{ScheduleID: "sub_sched_123", PriceID: "price_123"})
			return err
		}},
		{name: "free cancellation", build: func() error {
			_, err := buildCancelFreeTrialParams(CancelFreeTrialRequest{SubscriptionID: "sub_123"})
			return err
		}},
		{name: "schedule cancellation", build: func() error {
			_, err := buildCancelPaidScheduleParams(CancelPaidScheduleRequest{
				ScheduleID: "sub_sched_123", RetainedPhases: []SchedulePhase{{PriceID: "price_123"}},
			})
			return err
		}},
	}

	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.build(); err == nil {
				t.Fatal("empty TrialGrantID error = nil")
			}
		})
	}
}

func TestBuildPortalParamsOnlyUsesRequiredTrialConfiguration(t *testing.T) {
	req := CreatePortalRequest{
		StripeMode:                    "live",
		CustomerID:                    "cus_123",
		ReturnURL:                     "https://app.example/billing",
		RequireTrialSafeConfiguration: true,
	}
	params, err := buildPortalParams(req, "bpc_trial")
	if err != nil {
		t.Fatalf("buildPortalParams() error = %v", err)
	}
	if got := stripe.StringValue(params.Configuration); got != "bpc_trial" {
		t.Fatalf("Configuration = %q", got)
	}
	if got := stripe.StringValue(params.Customer); got != req.CustomerID {
		t.Fatalf("Customer = %q", got)
	}
	if got := stripe.StringValue(params.ReturnURL); got != req.ReturnURL {
		t.Fatalf("ReturnURL = %q", got)
	}

	withoutTrialConfig, err := buildPortalParams(CreatePortalRequest{CustomerID: "cus_123"}, "bpc_trial")
	if err != nil {
		t.Fatalf("non-trial buildPortalParams() error = %v", err)
	}
	if withoutTrialConfig.Configuration != nil {
		t.Fatalf("non-trial Configuration = %q, want omitted", stripe.StringValue(withoutTrialConfig.Configuration))
	}

	if _, err := buildPortalParams(req, ""); err == nil {
		t.Fatal("required trial-safe configuration missing: error = nil")
	}
}

func TestGatewayRejectsUnavailableModeWithoutFallingBackLive(t *testing.T) {
	gateway := NewStripeGateway(nil)
	if _, err := gateway.RetrieveCheckout(t.Context(), "sandbox", "cs_123"); err == nil {
		t.Fatal("RetrieveCheckout with unavailable mode error = nil")
	}
}

func TestSnapshotsExposeStripeStatusPeriodsCancellationAndMetadata(t *testing.T) {
	metadata := map[string]string{"trial_grant_id": "grant_123"}
	subscription := &stripe.Subscription{
		ID:                "sub_123",
		Status:            stripe.SubscriptionStatusTrialing,
		Customer:          &stripe.Customer{ID: "cus_123"},
		Schedule:          &stripe.SubscriptionSchedule{ID: "sub_sched_123"},
		TrialStart:        1_800_000_000,
		TrialEnd:          1_800_086_400,
		CancelAt:          1_800_086_400,
		CancelAtPeriodEnd: true,
		CanceledAt:        1_800_000_100,
		EndedAt:           1_800_086_500,
		Metadata:          metadata,
		Items: &stripe.SubscriptionItemList{Data: []*stripe.SubscriptionItem{{
			ID:                 "si_123",
			Price:              &stripe.Price{ID: "price_123"},
			CurrentPeriodStart: 1_800_000_000,
			CurrentPeriodEnd:   1_800_086_400,
		}}},
	}
	subscriptionResult := subscriptionSnapshot("sandbox", subscription)
	if subscriptionResult.Status != "trialing" || subscriptionResult.ItemID != "si_123" || subscriptionResult.PriceID != "price_123" {
		t.Fatalf("subscription identity/status = %#v", subscriptionResult)
	}
	if subscriptionResult.TrialStartAt == nil || subscriptionResult.TrialEndAt == nil || subscriptionResult.CurrentPeriodStartAt == nil || subscriptionResult.CurrentPeriodEndAt == nil {
		t.Fatalf("subscription trial/period dates missing: %#v", subscriptionResult)
	}
	if subscriptionResult.CancelAt == nil || !subscriptionResult.CancelAtPeriodEnd || subscriptionResult.CanceledAt == nil || subscriptionResult.EndedAt == nil {
		t.Fatalf("subscription cancel/end fields missing: %#v", subscriptionResult)
	}
	if subscriptionResult.Metadata["trial_grant_id"] != "grant_123" {
		t.Fatalf("subscription metadata = %#v", subscriptionResult.Metadata)
	}

	schedule := &stripe.SubscriptionSchedule{
		ID:          "sub_sched_123",
		Status:      stripe.SubscriptionScheduleStatusActive,
		EndBehavior: stripe.SubscriptionScheduleEndBehaviorCancel,
		CanceledAt:  1_800_000_200,
		CompletedAt: 1_800_000_300,
		ReleasedAt:  1_800_000_400,
		Metadata:    metadata,
		Phases: []*stripe.SubscriptionSchedulePhase{{
			StartDate:          1_800_000_000,
			EndDate:            1_800_086_400,
			TrialEnd:           1_800_086_400,
			BillingCycleAnchor: stripe.SubscriptionSchedulePhaseBillingCycleAnchorPhaseStart,
			Metadata:           metadata,
			Items:              []*stripe.SubscriptionSchedulePhaseItem{{Price: &stripe.Price{ID: "price_123"}}},
		}},
	}
	scheduleResult := scheduleSnapshot("sandbox", schedule)
	if scheduleResult.Status != "active" || scheduleResult.EndBehavior != "cancel" || len(scheduleResult.Phases) != 1 {
		t.Fatalf("schedule status/phases = %#v", scheduleResult)
	}
	if scheduleResult.CanceledAt == nil || scheduleResult.CompletedAt == nil || scheduleResult.ReleasedAt == nil {
		t.Fatalf("schedule terminal dates missing: %#v", scheduleResult)
	}
	phase := scheduleResult.Phases[0]
	if phase.PriceID != "price_123" || phase.StartAt.IsZero() || phase.EndAt.IsZero() || phase.TrialEndAt.IsZero() || phase.BillingCycleAnchor != "phase_start" {
		t.Fatalf("normalized schedule phase = %#v", phase)
	}
}

func assertTrialMetadata(t *testing.T, got map[string]string, workspaceID, planID, grantID string, kind Kind, environment string) {
	t.Helper()
	want := map[string]string{
		"workspace_id":        workspaceID,
		"plan_id":             planID,
		"trial_grant_id":      grantID,
		"trial_kind":          string(kind),
		"unipost_environment": environment,
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("metadata[%q] = %q, want %q; metadata=%#v", key, got[key], value, got)
		}
	}
}

type recordingSchedulePlanChanger struct {
	updateCalls  int
	releaseCalls int
}

func (c *recordingSchedulePlanChanger) Update(_ string, _ *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	c.updateCalls++
	return &stripe.SubscriptionSchedule{ID: "sub_sched_123"}, nil
}

// Release intentionally exists on the fake so the behavioral assertion proves
// the plan-change helper does not choose it even when it is available.
func (c *recordingSchedulePlanChanger) Release(_ context.Context, _ string) (*stripe.SubscriptionSchedule, error) {
	c.releaseCalls++
	return &stripe.SubscriptionSchedule{ID: "released"}, nil
}
