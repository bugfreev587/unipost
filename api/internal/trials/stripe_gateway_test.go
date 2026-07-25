package trials

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/billing"
)

func TestBuildTrialCheckoutParams(t *testing.T) {
	req := CreateTrialCheckoutRequest{
		StripeMode:      "sandbox",
		WorkspaceID:     "ws_123",
		PlanID:          "growth",
		TrialGrantID:    "grant_123",
		TrialKind:       KindFreeToPaid,
		Environment:     "development",
		CustomerID:      "cus_123",
		PriceID:         "price_growth",
		DurationDays:    45,
		CheckoutAttempt: 1,
		SuccessURL:      "https://app.example/success",
		CancelURL:       "https://app.example/cancel",
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
	if got := stripe.StringValue(params.IdempotencyKey); got != "trial:grant_123:checkout:1" {
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

func TestBuildTrialCheckoutParamsFailsClosedOnInvalidRequest(t *testing.T) {
	valid := CreateTrialCheckoutRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "growth", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", CustomerID: "cus_123",
		PriceID: "price_123", DurationDays: 30,
		CheckoutAttempt: 1,
		SuccessURL:      "https://app.example/success", CancelURL: "https://app.example/cancel",
	}
	tests := []struct {
		name   string
		mutate func(*CreateTrialCheckoutRequest)
	}{
		{name: "mode", mutate: func(req *CreateTrialCheckoutRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *CreateTrialCheckoutRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *CreateTrialCheckoutRequest) { req.PlanID = "enterprise" }},
		{name: "grant", mutate: func(req *CreateTrialCheckoutRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *CreateTrialCheckoutRequest) { req.TrialKind = KindPaidSamePlan }},
		{name: "environment", mutate: func(req *CreateTrialCheckoutRequest) { req.Environment = "" }},
		{name: "customer", mutate: func(req *CreateTrialCheckoutRequest) { req.CustomerID = "" }},
		{name: "price", mutate: func(req *CreateTrialCheckoutRequest) { req.PriceID = "" }},
		{name: "attempt", mutate: func(req *CreateTrialCheckoutRequest) { req.CheckoutAttempt = 0 }},
		{name: "success relative", mutate: func(req *CreateTrialCheckoutRequest) { req.SuccessURL = "/success" }},
		{name: "success insecure", mutate: func(req *CreateTrialCheckoutRequest) { req.SuccessURL = "http://app.example/success" }},
		{name: "cancel malformed", mutate: func(req *CreateTrialCheckoutRequest) { req.CancelURL = "://bad" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := valid
			test.mutate(&req)
			if _, err := buildTrialCheckoutParams(req); err == nil {
				t.Fatalf("buildTrialCheckoutParams(%s) error = nil; request=%#v", test.name, req)
			}
		})
	}

	for _, localURL := range []string{"http://localhost:3000/success", "http://localhost/cancel"} {
		req := valid
		req.SuccessURL = localURL
		if _, err := buildTrialCheckoutParams(req); err != nil {
			t.Fatalf("localhost URL %q rejected: %v", localURL, err)
		}
	}
}

func TestBuildTrialCheckoutParamsUsesAttemptGenerationForReplacement(t *testing.T) {
	firstReq := validCheckoutRequest()
	firstReq.CheckoutAttempt = 1
	first, err := buildTrialCheckoutParams(firstReq)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := buildTrialCheckoutParams(firstReq)
	if err != nil {
		t.Fatal(err)
	}
	replacementReq := firstReq
	replacementReq.CheckoutAttempt = 2
	replacement, err := buildTrialCheckoutParams(replacementReq)
	if err != nil {
		t.Fatal(err)
	}
	firstKey := stripe.StringValue(first.IdempotencyKey)
	if retryKey := stripe.StringValue(retry.IdempotencyKey); retryKey != firstKey {
		t.Fatalf("retry key=%q, want %q", retryKey, firstKey)
	}
	if replacementKey := stripe.StringValue(replacement.IdempotencyKey); replacementKey == firstKey || replacementKey != "trial:grant_123:checkout:2" {
		t.Fatalf("replacement key=%q, first=%q", replacementKey, firstKey)
	}
}

func TestBuildExpireCheckoutParamsUsesStableRetryKey(t *testing.T) {
	req := ExpireCheckoutRequest{
		StripeMode:        "sandbox",
		TrialGrantID:      "grant_123",
		CheckoutSessionID: "cs_123",
	}

	first, err := buildExpireCheckoutParams(req)
	if err != nil {
		t.Fatalf("first buildExpireCheckoutParams() error = %v", err)
	}
	second, err := buildExpireCheckoutParams(req)
	if err != nil {
		t.Fatalf("retry buildExpireCheckoutParams() error = %v", err)
	}
	const want = "trial:grant_123:expire_checkout"
	if got := stripe.StringValue(first.IdempotencyKey); got != want {
		t.Fatalf("first IdempotencyKey = %q, want %q", got, want)
	}
	if got := stripe.StringValue(second.IdempotencyKey); got != want {
		t.Fatalf("retry IdempotencyKey = %q, want %q", got, want)
	}
}

func TestBuildExpireCheckoutParamsRejectsIncompleteIdentity(t *testing.T) {
	valid := ExpireCheckoutRequest{StripeMode: "live", TrialGrantID: "grant_123", CheckoutSessionID: "cs_123"}
	for _, mutate := range []func(*ExpireCheckoutRequest){
		func(req *ExpireCheckoutRequest) { req.StripeMode = "" },
		func(req *ExpireCheckoutRequest) { req.TrialGrantID = "" },
		func(req *ExpireCheckoutRequest) { req.CheckoutSessionID = "" },
		func(req *ExpireCheckoutRequest) { req.CheckoutSessionID = "   " },
	} {
		req := valid
		mutate(&req)
		if _, err := buildExpireCheckoutParams(req); err == nil {
			t.Fatalf("buildExpireCheckoutParams(%#v) error = nil", req)
		}
	}
}

func TestExpireCheckoutTimeoutRetryReusesSameIdempotencyKey(t *testing.T) {
	client := &fakeStripeTrialClient{
		checkoutResult: &stripe.CheckoutSession{ID: "cs_123", Status: stripe.CheckoutSessionStatusExpired},
		expireErrors:   []error{context.DeadlineExceeded, nil},
	}
	gateway := newFakeStripeGateway(client)
	req := ExpireCheckoutRequest{StripeMode: "live", TrialGrantID: "grant_123", CheckoutSessionID: "cs_123"}

	if _, err := gateway.ExpireCheckout(t.Context(), req); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("first ExpireCheckout() error = %v, want deadline exceeded", err)
	}
	if _, err := gateway.ExpireCheckout(t.Context(), req); err != nil {
		t.Fatalf("retry ExpireCheckout() error = %v", err)
	}
	if len(client.expireParams) != 2 {
		t.Fatalf("expire params count = %d, want 2", len(client.expireParams))
	}
	first := stripe.StringValue(client.expireParams[0].IdempotencyKey)
	second := stripe.StringValue(client.expireParams[1].IdempotencyKey)
	if first != "trial:grant_123:expire_checkout" || second != first {
		t.Fatalf("retry idempotency keys = %q, %q", first, second)
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
	assertTrialMetadata(t, createParams.Metadata, req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
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

func TestBuildPaidTrialScheduleParamsRejectsInvalidDataBeforeCreate(t *testing.T) {
	valid := validPaidScheduleRequest()
	nonUTC := time.FixedZone("UTC+1", 60*60)

	tests := []struct {
		name   string
		mutate func(*CreatePaidTrialScheduleRequest)
	}{
		{name: "unknown mode", mutate: func(req *CreatePaidTrialScheduleRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *CreatePaidTrialScheduleRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *CreatePaidTrialScheduleRequest) { req.PlanID = "enterprise" }},
		{name: "grant", mutate: func(req *CreatePaidTrialScheduleRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *CreatePaidTrialScheduleRequest) { req.TrialKind = Kind("other") }},
		{name: "environment", mutate: func(req *CreatePaidTrialScheduleRequest) { req.Environment = "" }},
		{name: "subscription", mutate: func(req *CreatePaidTrialScheduleRequest) { req.SubscriptionID = "" }},
		{name: "price", mutate: func(req *CreatePaidTrialScheduleRequest) { req.PriceID = "" }},
		{name: "duration low", mutate: func(req *CreatePaidTrialScheduleRequest) { req.DurationDays = 0 }},
		{name: "duration high", mutate: func(req *CreatePaidTrialScheduleRequest) { req.DurationDays = 731 }},
		{name: "current price", mutate: func(req *CreatePaidTrialScheduleRequest) { req.CurrentPhase.PriceID = "" }},
		{name: "current price mismatch", mutate: func(req *CreatePaidTrialScheduleRequest) { req.CurrentPhase.PriceID = "price_other" }},
		{name: "current already trialing", mutate: func(req *CreatePaidTrialScheduleRequest) { req.CurrentPhase.TrialEndAt = req.CurrentPhase.EndAt }},
		{name: "current start", mutate: func(req *CreatePaidTrialScheduleRequest) { req.CurrentPhase.StartAt = time.Time{} }},
		{name: "current start non UTC", mutate: func(req *CreatePaidTrialScheduleRequest) {
			req.CurrentPhase.StartAt = req.CurrentPhase.StartAt.In(nonUTC)
		}},
		{name: "current end before start", mutate: func(req *CreatePaidTrialScheduleRequest) { req.CurrentPhase.EndAt = req.CurrentPhase.StartAt }},
		{name: "current end not trial start", mutate: func(req *CreatePaidTrialScheduleRequest) {
			req.CurrentPhase.EndAt = req.CurrentPhase.EndAt.Add(time.Second)
		}},
		{name: "trial start non UTC", mutate: func(req *CreatePaidTrialScheduleRequest) { req.TrialStartAt = req.TrialStartAt.In(nonUTC) }},
		{name: "trial end non UTC", mutate: func(req *CreatePaidTrialScheduleRequest) { req.TrialEndAt = req.TrialEndAt.In(nonUTC) }},
		{name: "trial end duration mismatch", mutate: func(req *CreatePaidTrialScheduleRequest) { req.TrialEndAt = req.TrialEndAt.Add(24 * time.Hour) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := valid
			req.CurrentPhase.Metadata = cloneMetadata(valid.CurrentPhase.Metadata)
			test.mutate(&req)
			if _, _, err := buildPaidTrialScheduleParams(req); err == nil {
				t.Fatalf("buildPaidTrialScheduleParams(%s) error = nil; request=%#v", test.name, req)
			}
		})
	}
}

func validPaidScheduleRequest() CreatePaidTrialScheduleRequest {
	currentStart := time.Unix(1_800_000_000, 0).UTC()
	trialStart := currentStart.AddDate(0, 0, 15)
	return CreatePaidTrialScheduleRequest{
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
			PriceID:  "price_team",
			StartAt:  currentStart,
			EndAt:    trialStart,
			Metadata: map[string]string{"retained": "yes"},
		},
		TrialStartAt: trialStart,
		TrialEndAt:   trialStart.AddDate(0, 0, 45),
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

func TestMutationBuildersFailClosedOnInvalidIdentity(t *testing.T) {
	freeChange := ChangeFreeTrialPlanRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "basic", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", SubscriptionID: "sub_123",
		SubscriptionItemID: "si_123", PriceID: "price_123",
	}
	for _, test := range []struct {
		name   string
		mutate func(*ChangeFreeTrialPlanRequest)
	}{
		{name: "mode", mutate: func(req *ChangeFreeTrialPlanRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *ChangeFreeTrialPlanRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *ChangeFreeTrialPlanRequest) { req.PlanID = "enterprise" }},
		{name: "grant", mutate: func(req *ChangeFreeTrialPlanRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *ChangeFreeTrialPlanRequest) { req.TrialKind = KindPaidSamePlan }},
		{name: "environment", mutate: func(req *ChangeFreeTrialPlanRequest) { req.Environment = "" }},
		{name: "subscription", mutate: func(req *ChangeFreeTrialPlanRequest) { req.SubscriptionID = "" }},
		{name: "item", mutate: func(req *ChangeFreeTrialPlanRequest) { req.SubscriptionItemID = "" }},
		{name: "price", mutate: func(req *ChangeFreeTrialPlanRequest) { req.PriceID = "" }},
	} {
		t.Run("free change "+test.name, func(t *testing.T) {
			req := freeChange
			test.mutate(&req)
			if _, err := buildChangeFreeTrialPlanParams(req); err == nil {
				t.Fatalf("invalid free plan change accepted: %#v", req)
			}
		})
	}

	paidChange := ChangeScheduledTrialPlanRequest{
		StripeMode: "sandbox", WorkspaceID: "ws_123", PlanID: "team", TrialGrantID: "grant_123",
		TrialKind: KindPaidSamePlan, Environment: "development", ScheduleID: "sub_sched_123", PriceID: "price_123",
	}
	for _, test := range []struct {
		name   string
		mutate func(*ChangeScheduledTrialPlanRequest)
	}{
		{name: "mode", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.PlanID = "enterprise" }},
		{name: "grant", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.TrialKind = KindFreeToPaid }},
		{name: "environment", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.Environment = "" }},
		{name: "schedule", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.ScheduleID = "" }},
		{name: "price", mutate: func(req *ChangeScheduledTrialPlanRequest) { req.PriceID = "" }},
	} {
		t.Run("scheduled change "+test.name, func(t *testing.T) {
			req := paidChange
			test.mutate(&req)
			if _, err := buildChangeScheduledTrialPlanParams(req); err == nil {
				t.Fatalf("invalid scheduled plan change accepted: %#v", req)
			}
		})
	}

	freeCancel := CancelFreeTrialRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "api", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", SubscriptionID: "sub_123",
	}
	for _, test := range []struct {
		name   string
		mutate func(*CancelFreeTrialRequest)
	}{
		{name: "mode", mutate: func(req *CancelFreeTrialRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *CancelFreeTrialRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *CancelFreeTrialRequest) { req.PlanID = "free" }},
		{name: "grant", mutate: func(req *CancelFreeTrialRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *CancelFreeTrialRequest) { req.TrialKind = KindPaidSamePlan }},
		{name: "environment", mutate: func(req *CancelFreeTrialRequest) { req.Environment = "" }},
		{name: "subscription", mutate: func(req *CancelFreeTrialRequest) { req.SubscriptionID = "" }},
	} {
		t.Run("free cancel "+test.name, func(t *testing.T) {
			req := freeCancel
			test.mutate(&req)
			if _, err := buildCancelFreeTrialParams(req); err == nil {
				t.Fatalf("invalid free cancellation accepted: %#v", req)
			}
		})
	}
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

func TestBuildCancelPaidScheduleParamsRejectsInvalidRetainedPhases(t *testing.T) {
	valid := validPaidCancelRequest()
	nonUTC := time.FixedZone("UTC+1", 60*60)
	tests := []struct {
		name   string
		mutate func(*CancelPaidScheduleRequest)
	}{
		{name: "mode", mutate: func(req *CancelPaidScheduleRequest) { req.StripeMode = "other" }},
		{name: "workspace", mutate: func(req *CancelPaidScheduleRequest) { req.WorkspaceID = "" }},
		{name: "plan", mutate: func(req *CancelPaidScheduleRequest) { req.PlanID = "free" }},
		{name: "grant", mutate: func(req *CancelPaidScheduleRequest) { req.TrialGrantID = "" }},
		{name: "kind", mutate: func(req *CancelPaidScheduleRequest) { req.TrialKind = KindFreeToPaid }},
		{name: "environment", mutate: func(req *CancelPaidScheduleRequest) { req.Environment = "" }},
		{name: "schedule", mutate: func(req *CancelPaidScheduleRequest) { req.ScheduleID = "" }},
		{name: "no phases", mutate: func(req *CancelPaidScheduleRequest) { req.RetainedPhases = nil }},
		{name: "price", mutate: func(req *CancelPaidScheduleRequest) { req.RetainedPhases[0].PriceID = "" }},
		{name: "start", mutate: func(req *CancelPaidScheduleRequest) { req.RetainedPhases[0].StartAt = time.Time{} }},
		{name: "non UTC", mutate: func(req *CancelPaidScheduleRequest) {
			req.RetainedPhases[0].StartAt = req.RetainedPhases[0].StartAt.In(nonUTC)
		}},
		{name: "end before start", mutate: func(req *CancelPaidScheduleRequest) { req.RetainedPhases[0].EndAt = req.RetainedPhases[0].StartAt }},
		{name: "gap", mutate: func(req *CancelPaidScheduleRequest) {
			req.RetainedPhases[1].StartAt = req.RetainedPhases[1].StartAt.Add(time.Second)
		}},
		{name: "trial end outside phase", mutate: func(req *CancelPaidScheduleRequest) {
			req.RetainedPhases[1].TrialEndAt = req.RetainedPhases[1].EndAt.Add(time.Second)
		}},
		{name: "anchor", mutate: func(req *CancelPaidScheduleRequest) { req.RetainedPhases[0].BillingCycleAnchor = "other" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := valid
			req.RetainedPhases = append([]SchedulePhase(nil), valid.RetainedPhases...)
			test.mutate(&req)
			if _, err := buildCancelPaidScheduleParams(req); err == nil {
				t.Fatalf("invalid retained phases accepted: %#v", req)
			}
		})
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

	withoutTrialConfig, err := buildPortalParams(CreatePortalRequest{
		StripeMode: "live", CustomerID: "cus_123", ReturnURL: "https://app.example/billing",
	}, "bpc_trial")
	if err != nil {
		t.Fatalf("non-trial buildPortalParams() error = %v", err)
	}
	if withoutTrialConfig.Configuration != nil {
		t.Fatalf("non-trial Configuration = %q, want omitted", stripe.StringValue(withoutTrialConfig.Configuration))
	}

	if _, err := buildPortalParams(req, ""); err == nil {
		t.Fatal("required trial-safe configuration missing: error = nil")
	}
	if _, err := buildPortalParams(req, "   "); err == nil {
		t.Fatal("whitespace-only trial-safe configuration: error = nil")
	}
}

func TestBuildPortalParamsFailsClosedOnInvalidRequest(t *testing.T) {
	valid := CreatePortalRequest{StripeMode: "live", CustomerID: "cus_123", ReturnURL: "https://app.example/billing"}
	for _, test := range []struct {
		name   string
		mutate func(*CreatePortalRequest)
	}{
		{name: "mode", mutate: func(req *CreatePortalRequest) { req.StripeMode = "other" }},
		{name: "customer", mutate: func(req *CreatePortalRequest) { req.CustomerID = "" }},
		{name: "return relative", mutate: func(req *CreatePortalRequest) { req.ReturnURL = "/billing" }},
		{name: "return insecure", mutate: func(req *CreatePortalRequest) { req.ReturnURL = "http://app.example/billing" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			req := valid
			test.mutate(&req)
			if _, err := buildPortalParams(req, ""); err == nil {
				t.Fatalf("invalid portal request accepted: %#v", req)
			}
		})
	}
}

func TestGatewayRejectsInvalidRequestsBeforeAnyStripeCall(t *testing.T) {
	tests := []struct {
		name string
		call func(*stripeGateway) error
	}{
		{name: "create paid schedule", call: func(g *stripeGateway) error {
			req := validPaidScheduleRequest()
			req.DurationDays = 0
			_, err := g.CreatePaidTrialSchedule(t.Context(), req)
			return err
		}},
		{name: "create paid schedule price mismatch", call: func(g *stripeGateway) error {
			req := validPaidScheduleRequest()
			req.CurrentPhase.PriceID = "price_other"
			_, err := g.CreatePaidTrialSchedule(t.Context(), req)
			return err
		}},
		{name: "create paid schedule current trial", call: func(g *stripeGateway) error {
			req := validPaidScheduleRequest()
			req.CurrentPhase.TrialEndAt = req.CurrentPhase.EndAt
			_, err := g.CreatePaidTrialSchedule(t.Context(), req)
			return err
		}},
		{name: "create checkout", call: func(g *stripeGateway) error {
			req := validCheckoutRequest()
			req.PriceID = ""
			_, err := g.CreateTrialCheckout(t.Context(), req)
			return err
		}},
		{name: "retrieve checkout", call: func(g *stripeGateway) error {
			_, err := g.RetrieveCheckout(t.Context(), "live", "")
			return err
		}},
		{name: "expire checkout", call: func(g *stripeGateway) error {
			_, err := g.ExpireCheckout(t.Context(), ExpireCheckoutRequest{StripeMode: "live", TrialGrantID: "grant_123"})
			return err
		}},
		{name: "retrieve subscription", call: func(g *stripeGateway) error {
			_, err := g.RetrieveSubscription(t.Context(), "live", "")
			return err
		}},
		{name: "change free plan", call: func(g *stripeGateway) error {
			req := validFreeChangeRequest()
			req.SubscriptionItemID = ""
			_, err := g.ChangeFreeTrialPlanNow(t.Context(), req)
			return err
		}},
		{name: "change scheduled plan", call: func(g *stripeGateway) error {
			req := validScheduledChangeRequest()
			req.PriceID = ""
			_, err := g.ChangeScheduledTrialPlanNow(t.Context(), req)
			return err
		}},
		{name: "cancel free", call: func(g *stripeGateway) error {
			req := validFreeCancelRequest()
			req.SubscriptionID = ""
			_, err := g.CancelFreeTrialAtEnd(t.Context(), req)
			return err
		}},
		{name: "cancel paid", call: func(g *stripeGateway) error {
			req := validPaidCancelRequest()
			req.RetainedPhases[1].StartAt = req.RetainedPhases[1].StartAt.Add(time.Second)
			_, err := g.CancelPaidScheduleAtTrialEnd(t.Context(), req)
			return err
		}},
		{name: "portal", call: func(g *stripeGateway) error {
			_, err := g.CreatePortal(t.Context(), CreatePortalRequest{StripeMode: "live", CustomerID: "cus_123", ReturnURL: "/billing"})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeTrialClient{}
			gateway := newFakeStripeGateway(client)
			if err := test.call(gateway); err == nil {
				t.Fatal("invalid request error = nil")
			}
			if len(client.calls) != 0 {
				t.Fatalf("Stripe calls = %#v, want none", client.calls)
			}
		})
	}
}

func TestGatewayRejectsEmptyOrMismatchedStripeResponses(t *testing.T) {
	tests := []struct {
		name string
		call func(*stripeGateway, *fakeStripeTrialClient) error
	}{
		{name: "create checkout without ID", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.checkoutResult = &stripe.CheckoutSession{URL: "https://checkout.example/session"}
			_, err := g.CreateTrialCheckout(t.Context(), validCheckoutRequest())
			return err
		}},
		{name: "create checkout nil", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.checkoutResult = nil
			_, err := g.CreateTrialCheckout(t.Context(), validCheckoutRequest())
			return err
		}},
		{name: "create checkout without URL", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.checkoutResult = &stripe.CheckoutSession{ID: "cs_123"}
			_, err := g.CreateTrialCheckout(t.Context(), validCheckoutRequest())
			return err
		}},
		{name: "retrieve checkout mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.checkoutResult = &stripe.CheckoutSession{ID: "cs_other"}
			_, err := g.RetrieveCheckout(t.Context(), "live", "cs_123")
			return err
		}},
		{name: "expire checkout mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.checkoutResult = &stripe.CheckoutSession{ID: "cs_other"}
			_, err := g.ExpireCheckout(t.Context(), ExpireCheckoutRequest{StripeMode: "live", TrialGrantID: "grant_123", CheckoutSessionID: "cs_123"})
			return err
		}},
		{name: "retrieve subscription mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.subscriptionResult = &stripe.Subscription{ID: "sub_other"}
			_, err := g.RetrieveSubscription(t.Context(), "live", "sub_123")
			return err
		}},
		{name: "retrieve subscription nil", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.subscriptionResult = nil
			_, err := g.RetrieveSubscription(t.Context(), "live", "sub_123")
			return err
		}},
		{name: "change free subscription mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.subscriptionResult = &stripe.Subscription{ID: "sub_other"}
			_, err := g.ChangeFreeTrialPlanNow(t.Context(), validFreeChangeRequest())
			return err
		}},
		{name: "change schedule mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.scheduleUpdateResult = &stripe.SubscriptionSchedule{ID: "sub_sched_other"}
			_, err := g.ChangeScheduledTrialPlanNow(t.Context(), validScheduledChangeRequest())
			return err
		}},
		{name: "cancel free subscription mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.subscriptionResult = &stripe.Subscription{ID: "sub_other"}
			_, err := g.CancelFreeTrialAtEnd(t.Context(), validFreeCancelRequest())
			return err
		}},
		{name: "cancel paid schedule mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.scheduleUpdateResult = &stripe.SubscriptionSchedule{ID: "sub_sched_other"}
			_, err := g.CancelPaidScheduleAtTrialEnd(t.Context(), validPaidCancelRequest())
			return err
		}},
		{name: "create schedule without ID", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.scheduleCreateResult = &stripe.SubscriptionSchedule{}
			_, err := g.CreatePaidTrialSchedule(t.Context(), validPaidScheduleRequest())
			return err
		}},
		{name: "configured schedule mismatch", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.scheduleCreateResult = &stripe.SubscriptionSchedule{ID: "sub_sched_created"}
			client.scheduleUpdateResult = &stripe.SubscriptionSchedule{ID: "sub_sched_other"}
			_, err := g.CreatePaidTrialSchedule(t.Context(), validPaidScheduleRequest())
			return err
		}},
		{name: "portal without URL", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.portalResult = &stripe.BillingPortalSession{}
			_, err := g.CreatePortal(t.Context(), CreatePortalRequest{StripeMode: "live", CustomerID: "cus_123", ReturnURL: "https://app.example/billing"})
			return err
		}},
		{name: "portal whitespace URL", call: func(g *stripeGateway, client *fakeStripeTrialClient) error {
			client.portalResult = &stripe.BillingPortalSession{URL: "   "}
			_, err := g.CreatePortal(t.Context(), CreatePortalRequest{StripeMode: "live", CustomerID: "cus_123", ReturnURL: "https://app.example/billing"})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeTrialClient{}
			gateway := newFakeStripeGateway(client)
			if err := test.call(gateway, client); err == nil {
				t.Fatal("invalid Stripe response error = nil")
			}
		})
	}
}

func TestCreatePaidScheduleReturnsCreatedSnapshotWhenUpdateTimesOut(t *testing.T) {
	client := &fakeStripeTrialClient{
		scheduleCreateResult: &stripe.SubscriptionSchedule{ID: "sub_sched_created", Status: stripe.SubscriptionScheduleStatusActive},
		scheduleUpdateErr:    errors.New("request timed out"),
	}
	gateway := newFakeStripeGateway(client)

	snapshot, err := gateway.CreatePaidTrialSchedule(t.Context(), validPaidScheduleRequest())
	if err == nil || !strings.Contains(err.Error(), "sub_sched_created") {
		t.Fatalf("CreatePaidTrialSchedule() error = %v, want created schedule ID", err)
	}
	if snapshot.ID != "sub_sched_created" {
		t.Fatalf("partial snapshot ID = %q, want sub_sched_created", snapshot.ID)
	}
	if !reflect.DeepEqual(client.calls, []string{"create_schedule", "update_schedule"}) {
		t.Fatalf("Stripe calls = %#v", client.calls)
	}
	if client.scheduleCreateParams == nil {
		t.Fatal("initial schedule create params were not captured")
	}
	assertTrialMetadata(t, client.scheduleCreateParams.Metadata, "ws_paid", "team", "grant_paid", KindPaidSamePlan, "production")
}

func TestCreatePaidScheduleClassifiesEmptySnapshotErrors(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want MutationOutcome
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: MutationIndeterminate},
		{name: "transport", err: errors.New("connection reset"), want: MutationIndeterminate},
		{name: "Stripe 500", err: &stripe.Error{HTTPStatusCode: 500, Type: stripe.ErrorTypeAPI}, want: MutationIndeterminate},
		{name: "Stripe 429", err: &stripe.Error{HTTPStatusCode: 429, Type: stripe.ErrorTypeAPI}, want: MutationIndeterminate},
		{name: "Stripe 400 idempotency", err: &stripe.Error{HTTPStatusCode: 400, Type: stripe.ErrorTypeIdempotency}, want: MutationIndeterminate},
		{name: "Stripe 400", err: &stripe.Error{HTTPStatusCode: 400, Type: stripe.ErrorTypeInvalidRequest}, want: MutationRejected},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeTrialClient{scheduleCreateErr: test.err}
			_, err := newFakeStripeGateway(client).CreatePaidTrialSchedule(t.Context(), validPaidScheduleRequest())
			var mutationErr *ScheduleMutationError
			if !errors.As(err, &mutationErr) || mutationErr.Outcome != test.want {
				t.Fatalf("error=%#v, want outcome %q", err, test.want)
			}
		})
	}
}

func TestCreateTrialCheckoutClassifiesMutationOutcome(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want MutationOutcome
	}{
		{name: "deadline", err: context.DeadlineExceeded, want: MutationIndeterminate},
		{name: "transport", err: errors.New("connection reset"), want: MutationIndeterminate},
		{name: "Stripe 500", err: &stripe.Error{HTTPStatusCode: 500, Type: stripe.ErrorTypeAPI}, want: MutationIndeterminate},
		{name: "Stripe 429", err: &stripe.Error{HTTPStatusCode: 429, Type: stripe.ErrorTypeAPI}, want: MutationIndeterminate},
		{name: "Stripe idempotency", err: &stripe.Error{HTTPStatusCode: 400, Type: stripe.ErrorTypeIdempotency}, want: MutationIndeterminate},
		{name: "Stripe rejection", err: &stripe.Error{HTTPStatusCode: 400, Type: stripe.ErrorTypeInvalidRequest}, want: MutationRejected},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeTrialClient{checkoutErr: test.err}
			_, err := newFakeStripeGateway(client).CreateTrialCheckout(t.Context(), validCheckoutRequest())
			var mutationErr *CheckoutMutationError
			if !errors.As(err, &mutationErr) || mutationErr.Outcome != test.want {
				t.Fatalf("error=%#v, want outcome %q", err, test.want)
			}
		})
	}
}

func TestCreateTrialCheckoutInvalidResponseIsIndeterminate(t *testing.T) {
	client := &fakeStripeTrialClient{checkoutResult: &stripe.CheckoutSession{ID: "cs_created"}}
	_, err := newFakeStripeGateway(client).CreateTrialCheckout(t.Context(), validCheckoutRequest())
	var mutationErr *CheckoutMutationError
	if !errors.As(err, &mutationErr) || mutationErr.Outcome != MutationIndeterminate {
		t.Fatalf("error=%#v, want indeterminate", err)
	}
}

func TestConfigurePaidTrialScheduleUpdatesExistingWithoutCreate(t *testing.T) {
	client := &fakeStripeTrialClient{scheduleUpdateResult: &stripe.SubscriptionSchedule{ID: "sched_existing"}}
	gateway := newFakeStripeGateway(client)

	got, err := gateway.ConfigurePaidTrialSchedule(t.Context(), "sched_existing", validPaidScheduleRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "sched_existing" || !reflect.DeepEqual(client.calls, []string{"update_schedule"}) {
		t.Fatalf("snapshot=%#v calls=%#v", got, client.calls)
	}
	if client.scheduleUpdateID != "sched_existing" {
		t.Fatalf("updated schedule=%q", client.scheduleUpdateID)
	}
}

func TestConfigurePaidTrialScheduleRejectsInvalidIdentityWithoutCreating(t *testing.T) {
	for _, test := range []struct{ name, scheduleID, responseID string }{
		{name: "empty requested ID", scheduleID: "", responseID: ""},
		{name: "mismatched response ID", scheduleID: "sched_existing", responseID: "sched_other"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &fakeStripeTrialClient{}
			if test.responseID != "" {
				client.scheduleUpdateResult = &stripe.SubscriptionSchedule{ID: test.responseID}
			}
			gateway := newFakeStripeGateway(client)
			if _, err := gateway.ConfigurePaidTrialSchedule(t.Context(), test.scheduleID, validPaidScheduleRequest()); err == nil {
				t.Fatal("error=nil")
			}
			for _, call := range client.calls {
				if call == "create_schedule" {
					t.Fatalf("unexpected create: %#v", client.calls)
				}
			}
		})
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

func validCheckoutRequest() CreateTrialCheckoutRequest {
	return CreateTrialCheckoutRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "growth", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", CustomerID: "cus_123",
		PriceID: "price_123", DurationDays: 30,
		CheckoutAttempt: 1,
		SuccessURL:      "https://app.example/success", CancelURL: "https://app.example/cancel",
	}
}

func validFreeChangeRequest() ChangeFreeTrialPlanRequest {
	return ChangeFreeTrialPlanRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "basic", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", SubscriptionID: "sub_123",
		SubscriptionItemID: "si_123", PriceID: "price_123",
	}
}

func validScheduledChangeRequest() ChangeScheduledTrialPlanRequest {
	return ChangeScheduledTrialPlanRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "team", TrialGrantID: "grant_123",
		TrialKind: KindPaidSamePlan, Environment: "production", ScheduleID: "sub_sched_123", PriceID: "price_123",
	}
}

func validFreeCancelRequest() CancelFreeTrialRequest {
	return CancelFreeTrialRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "api", TrialGrantID: "grant_123",
		TrialKind: KindFreeToPaid, Environment: "production", SubscriptionID: "sub_123",
	}
}

func validPaidCancelRequest() CancelPaidScheduleRequest {
	start := time.Unix(1_800_000_000, 0).UTC()
	trialStart := start.AddDate(0, 0, 15)
	trialEnd := trialStart.AddDate(0, 0, 30)
	return CancelPaidScheduleRequest{
		StripeMode: "live", WorkspaceID: "ws_123", PlanID: "growth", TrialGrantID: "grant_123",
		TrialKind: KindPaidSamePlan, Environment: "production", ScheduleID: "sub_sched_123",
		RetainedPhases: []SchedulePhase{
			{PriceID: "price_123", StartAt: start, EndAt: trialStart},
			{PriceID: "price_123", StartAt: trialStart, EndAt: trialEnd, TrialEndAt: trialEnd},
		},
	}
}

func newFakeStripeGateway(client *fakeStripeTrialClient) *stripeGateway {
	manager := &billing.Manager{
		Live:    &billing.Mode{Name: "live"},
		Sandbox: &billing.Mode{Name: "sandbox"},
	}
	return &stripeGateway{
		manager: manager,
		clientForMode: func(*billing.Mode) stripeTrialClient {
			return client
		},
	}
}

type fakeStripeTrialClient struct {
	calls                []string
	checkoutResult       *stripe.CheckoutSession
	checkoutErr          error
	expireErrors         []error
	expireParams         []*stripe.CheckoutSessionExpireParams
	subscriptionResult   *stripe.Subscription
	subscriptionErr      error
	scheduleCreateResult *stripe.SubscriptionSchedule
	scheduleCreateErr    error
	scheduleCreateParams *stripe.SubscriptionScheduleParams
	scheduleUpdateResult *stripe.SubscriptionSchedule
	scheduleUpdateErr    error
	scheduleUpdateID     string
	portalResult         *stripe.BillingPortalSession
	portalErr            error
}

func (c *fakeStripeTrialClient) createCheckout(params *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	c.calls = append(c.calls, "create_checkout")
	return c.checkoutResult, c.checkoutErr
}

func (c *fakeStripeTrialClient) retrieveCheckout(_ string, _ *stripe.CheckoutSessionParams) (*stripe.CheckoutSession, error) {
	c.calls = append(c.calls, "retrieve_checkout")
	return c.checkoutResult, c.checkoutErr
}

func (c *fakeStripeTrialClient) expireCheckout(_ string, params *stripe.CheckoutSessionExpireParams) (*stripe.CheckoutSession, error) {
	c.calls = append(c.calls, "expire_checkout")
	c.expireParams = append(c.expireParams, params)
	if len(c.expireErrors) > 0 {
		err := c.expireErrors[0]
		c.expireErrors = c.expireErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	return c.checkoutResult, c.checkoutErr
}

func (c *fakeStripeTrialClient) retrieveSubscription(_ string, _ *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	c.calls = append(c.calls, "retrieve_subscription")
	return c.subscriptionResult, c.subscriptionErr
}

func (c *fakeStripeTrialClient) updateSubscription(_ string, _ *stripe.SubscriptionParams) (*stripe.Subscription, error) {
	c.calls = append(c.calls, "update_subscription")
	return c.subscriptionResult, c.subscriptionErr
}

func (c *fakeStripeTrialClient) createSchedule(params *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	c.calls = append(c.calls, "create_schedule")
	c.scheduleCreateParams = params
	return c.scheduleCreateResult, c.scheduleCreateErr
}

func (c *fakeStripeTrialClient) updateSchedule(id string, _ *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	c.calls = append(c.calls, "update_schedule")
	c.scheduleUpdateID = id
	return c.scheduleUpdateResult, c.scheduleUpdateErr
}

func (c *fakeStripeTrialClient) createPortal(_ *stripe.BillingPortalSessionParams) (*stripe.BillingPortalSession, error) {
	c.calls = append(c.calls, "create_portal")
	return c.portalResult, c.portalErr
}

func (c *recordingSchedulePlanChanger) updateSchedule(_ string, _ *stripe.SubscriptionScheduleParams) (*stripe.SubscriptionSchedule, error) {
	c.updateCalls++
	return &stripe.SubscriptionSchedule{ID: "sub_sched_123"}, nil
}

// Release intentionally exists on the fake so the behavioral assertion proves
// the plan-change helper does not choose it even when it is available.
func (c *recordingSchedulePlanChanger) Release(_ context.Context, _ string) (*stripe.SubscriptionSchedule, error) {
	c.releaseCalls++
	return &stripe.SubscriptionSchedule{ID: "released"}, nil
}
