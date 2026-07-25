package trials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"
	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestValidateGrantInputAcceptsSupportedPlansAndDurationBoundaries(t *testing.T) {
	for _, planID := range []string{"api", "basic", "growth", "team"} {
		for _, days := range []int32{1, 730} {
			if err := ValidateGrantInput(planID, days); err != nil {
				t.Fatalf("ValidateGrantInput(%q, %d) = %v", planID, days, err)
			}
		}
	}
}

func TestGrantFreeCreatesPendingWithoutStripe(t *testing.T) {
	h := newServiceHarness(t)
	h.store.billing.Subscription.PlanID = "free"

	got, err := h.service.Grant(t.Context(), GrantRequest{
		WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 30, ActorUserID: "admin_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindFreeToPaid || got.Status != StatusPendingActivation {
		t.Fatalf("grant = %#v", got)
	}
	if h.stripe.createScheduleCalls != 0 || h.stripe.retrieveSubscriptionCalls != 0 {
		t.Fatalf("free grant Stripe calls: schedule=%d retrieve=%d", h.stripe.createScheduleCalls, h.stripe.retrieveSubscriptionCalls)
	}
	if h.store.created.Status != StatusPendingActivation {
		t.Fatalf("created status = %q", h.store.created.Status)
	}
	if h.store.created.StripeMode != "live" || h.modes.ownerUserID != "owner_1" {
		t.Fatalf("free offer mode=%q resolved owner=%q", h.store.created.StripeMode, h.modes.ownerUserID)
	}
}

func TestGrantPaidSchedulesCurrentPlanAfterPeriodEndUsingWorkspaceOwnerMode(t *testing.T) {
	h := newServiceHarness(t)
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	h.store.billing = BillingSnapshot{
		WorkspaceID: "ws_1", OwnerUserID: "owner_1",
		Subscription: SubscriptionRecord{PlanID: "basic", Status: "active", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"},
	}
	h.modes.mode = BillingMode{Name: "sandbox", PriceID: "price_basic_test"}
	h.stripe.subscription = SubscriptionSnapshot{
		StripeMode: "sandbox", ID: "sub_1", Status: "active", CustomerID: "cus_1", PriceID: "price_basic_test",
		CurrentPeriodStartAt: &periodStart, CurrentPeriodEndAt: &periodEnd,
	}
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "sandbox", ID: "sub_sched_1", SubscriptionID: "sub_1", CustomerID: "cus_1"}

	got, err := h.service.Grant(t.Context(), GrantRequest{
		WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 60, ActorUserID: "admin_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if h.modes.ownerUserID != "owner_1" {
		t.Fatalf("mode owner = %q, want workspace owner", h.modes.ownerUserID)
	}
	if got.Kind != KindPaidSamePlan || got.Status != StatusScheduled || got.StripeScheduleID != "sub_sched_1" {
		t.Fatalf("grant = %#v", got)
	}
	if h.store.statusAtScheduleCall != StatusProvisioning {
		t.Fatalf("status at Stripe call = %q, want provisioning", h.store.statusAtScheduleCall)
	}
	wantEnd := periodEnd.AddDate(0, 0, 60)
	if !h.stripe.lastSchedule.TrialStartAt.Equal(periodEnd) || !h.stripe.lastSchedule.TrialEndAt.Equal(wantEnd) {
		t.Fatalf("trial bounds = %s..%s, want %s..%s", h.stripe.lastSchedule.TrialStartAt, h.stripe.lastSchedule.TrialEndAt, periodEnd, wantEnd)
	}
	if h.stripe.lastSchedule.CurrentPhase.PriceID != "price_basic_test" || h.stripe.lastSchedule.CurrentPhase.TrialEndAt != (time.Time{}) {
		t.Fatalf("current phase = %#v", h.stripe.lastSchedule.CurrentPhase)
	}
}

func TestGrantPaidRejectsIneligibleBeforeCreatingGrantOrCallingStripe(t *testing.T) {
	tests := []struct {
		name   string
		planID string
		mutate func(*serviceHarness)
		want   error
	}{
		{name: "different plan", planID: "growth", want: ErrPaidPlanMismatch},
		{name: "cancel at period end", planID: "basic", mutate: func(h *serviceHarness) { h.stripe.subscription.CancelAtPeriodEnd = true }, want: ErrIneligibleSubscription},
		{name: "cancel at timestamp", planID: "basic", mutate: func(h *serviceHarness) {
			stamp := time.Now().UTC().Add(time.Hour)
			h.stripe.subscription.CancelAt = &stamp
		}, want: ErrIneligibleSubscription},
		{name: "unrelated schedule", planID: "basic", mutate: func(h *serviceHarness) { h.stripe.subscription.ScheduleID = "sched_other" }, want: ErrUnrelatedSchedule},
		{name: "not active", planID: "basic", mutate: func(h *serviceHarness) { h.stripe.subscription.Status = "past_due" }, want: ErrIneligibleSubscription},
		{name: "price mismatch", planID: "basic", mutate: func(h *serviceHarness) { h.stripe.subscription.PriceID = "price_other" }, want: ErrIneligibleSubscription},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := newPaidServiceHarness(t)
			if test.mutate != nil {
				test.mutate(h)
			}
			_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: test.planID, DurationDays: 30, ActorUserID: "admin_1"})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if h.stripe.createScheduleCalls != 0 || h.store.createCalls != 0 {
				t.Fatalf("mutated before eligibility: Stripe=%d create=%d", h.stripe.createScheduleCalls, h.store.createCalls)
			}
		})
	}
}

func TestGrantPaidLocallyCancelingSubscriptionMakesNoStripeCall(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.billing.Subscription.CancelAtPeriodEnd = true

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if !errors.Is(err, ErrIneligibleSubscription) {
		t.Fatalf("error = %v, want ErrIneligibleSubscription", err)
	}
	if h.stripe.totalCalls() != 0 || h.store.createCalls != 0 {
		t.Fatalf("locally ineligible grant called Stripe=%d create=%d", h.stripe.totalCalls(), h.store.createCalls)
	}
}

func TestGrantRejectsExistingOpenGrantAndInvalidInputWithoutStripe(t *testing.T) {
	for _, test := range []struct {
		name string
		req  GrantRequest
		open *Grant
		want error
	}{
		{name: "enterprise", req: GrantRequest{WorkspaceID: "ws_1", PlanID: "enterprise", DurationDays: 30, ActorUserID: "admin_1"}, want: ErrInvalidPlan},
		{name: "duration", req: GrantRequest{WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 731, ActorUserID: "admin_1"}, want: ErrInvalidDuration},
		{name: "open grant", req: GrantRequest{WorkspaceID: "ws_1", PlanID: "growth", DurationDays: 30, ActorUserID: "admin_1"}, open: &Grant{ID: "grant_existing", Status: StatusActive}, want: ErrOpenGrantExists},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.open = test.open
			_, err := h.service.Grant(t.Context(), test.req)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if h.stripe.totalCalls() != 0 || h.store.createCalls != 0 {
				t.Fatalf("invalid grant mutated state: Stripe=%d create=%d", h.stripe.totalCalls(), h.store.createCalls)
			}
		})
	}
}

func TestGrantPaidStripeFailureMarksProvisioningFailed(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.stripe.schedule = ScheduleSnapshot{}
	h.stripe.createScheduleErr = &ScheduleMutationError{Outcome: MutationRejected, Err: errors.New("Stripe rejected invalid phases")}

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if err == nil {
		t.Fatal("error = nil")
	}
	if h.store.failed == nil || h.store.failed.ID != "grant_1" || h.store.failed.Code != "stripe_schedule_failed" {
		t.Fatalf("failed record = %#v", h.store.failed)
	}
	if strings.Contains(h.store.failed.Message, "cardholder secret") {
		t.Fatalf("failure message leaked Stripe detail: %q", h.store.failed.Message)
	}
}

func TestGrantPaidIndeterminateCreateWithoutSnapshotStaysProvisioning(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.stripe.schedule = ScheduleSnapshot{}
	h.stripe.createScheduleErr = &ScheduleMutationError{Outcome: MutationIndeterminate, Err: context.DeadlineExceeded}

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	if h.store.grant.Status != StatusProvisioning || h.store.failed != nil || h.store.provisioningSchedule == nil {
		t.Fatalf("grant=%#v failed=%#v reconciliation=%#v", h.store.grant, h.store.failed, h.store.provisioningSchedule)
	}
	if h.store.provisioningSchedule.StripeScheduleID != "" || strings.Contains(h.store.provisioningSchedule.FailureMessage, "deadline") {
		t.Fatalf("reconciliation=%#v", h.store.provisioningSchedule)
	}
}

func TestGrantPaidStripeIdempotencyErrorStaysProvisioning(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.stripe.schedule = ScheduleSnapshot{}
	h.stripe.createScheduleErr = classifyScheduleCreateError(&stripe.Error{HTTPStatusCode: 400, Type: stripe.ErrorTypeIdempotency})

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if err == nil {
		t.Fatal("error=nil")
	}
	if h.store.grant.Status != StatusProvisioning || h.store.failed != nil || h.store.provisioningSchedule == nil {
		t.Fatalf("grant=%#v failed=%#v reconciliation=%#v", h.store.grant, h.store.failed, h.store.provisioningSchedule)
	}
}

func TestGrantPaidRetryWithUnknownAttachedScheduleReusesSameGrantCreate(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: StatusProvisioning, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"}
	h.store.grant = *h.store.open
	h.stripe.subscription.ScheduleID = "sched_unknown"
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_recovered", CustomerID: "cus_1", SubscriptionID: "sub_1"}

	got, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_retry"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusScheduled || h.stripe.createScheduleCalls != 1 || h.stripe.configureScheduleCalls != 0 {
		t.Fatalf("grant=%#v create=%d configure=%d", got, h.stripe.createScheduleCalls, h.stripe.configureScheduleCalls)
	}
	if h.stripe.lastSchedule.TrialGrantID != "grant_1" {
		t.Fatalf("retry grant ID=%q", h.stripe.lastSchedule.TrialGrantID)
	}
}

func TestGrantPaidPartialScheduleCreationPreservesProvisioningForReconciliation(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_partial", CustomerID: "cus_1", SubscriptionID: "sub_1"}
	h.stripe.createScheduleErr = errors.New("schedule update timed out")

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if err == nil {
		t.Fatal("error = nil")
	}
	if h.store.failed != nil || h.store.grant.Status != StatusProvisioning {
		t.Fatalf("partial schedule released open slot: grant=%#v failed=%#v", h.store.grant, h.store.failed)
	}
	if h.store.provisioningSchedule == nil || h.store.provisioningSchedule.StripeScheduleID != "sched_partial" {
		t.Fatalf("partial schedule was not persisted: %#v", h.store.provisioningSchedule)
	}
	if strings.Contains(h.store.provisioningSchedule.FailureMessage, "timed out") {
		t.Fatalf("partial failure leaked raw Stripe detail: %q", h.store.provisioningSchedule.FailureMessage)
	}
}

func TestGrantPaidExactRetryConfiguresPersistedScheduleWithoutCreatingAnother(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: StatusProvisioning, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_partial"}
	h.store.grant = *h.store.open
	h.stripe.subscription.ScheduleID = "sched_partial"
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_partial", CustomerID: "cus_1", SubscriptionID: "sub_1"}

	got, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_retry"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusScheduled || h.stripe.configureScheduleCalls != 1 || h.stripe.createScheduleCalls != 0 {
		t.Fatalf("grant=%#v create=%d configure=%d", got, h.stripe.createScheduleCalls, h.stripe.configureScheduleCalls)
	}
	if h.stripe.lastConfiguredScheduleID != "sched_partial" {
		t.Fatalf("configured schedule=%q", h.stripe.lastConfiguredScheduleID)
	}
}

func TestGrantPaidExactRetryFailureRepersistsSafeReconciliationState(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: StatusProvisioning, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_partial"}
	h.store.grant = *h.store.open
	h.stripe.subscription.ScheduleID = "sched_partial"
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_partial"}
	h.stripe.createScheduleErr = errors.New("raw Stripe retry timeout secret")

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_retry"})
	if err == nil {
		t.Fatal("error=nil")
	}
	if h.store.grant.Status != StatusProvisioning || h.stripe.configureScheduleCalls != 1 || h.stripe.createScheduleCalls != 0 {
		t.Fatalf("grant=%#v create=%d configure=%d", h.store.grant, h.stripe.createScheduleCalls, h.stripe.configureScheduleCalls)
	}
	if h.store.provisioningSchedule == nil || h.store.provisioningSchedule.StripeScheduleID != "sched_partial" {
		t.Fatalf("update=%#v", h.store.provisioningSchedule)
	}
	if strings.Contains(h.store.provisioningSchedule.FailureMessage, "secret") || strings.Contains(h.store.provisioningSchedule.FailureMessage, "timeout") {
		t.Fatalf("unsafe failure=%q", h.store.provisioningSchedule.FailureMessage)
	}
}

func TestGrantPaidExactRetryIndeterminateWithoutSnapshotPreservesStoredSchedule(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: StatusProvisioning, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_partial"}
	h.store.grant = *h.store.open
	h.stripe.subscription.ScheduleID = "sched_partial"
	h.stripe.schedule = ScheduleSnapshot{}
	h.stripe.createScheduleErr = &ScheduleMutationError{Outcome: MutationIndeterminate, Err: context.DeadlineExceeded}

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_retry"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v", err)
	}
	if h.store.grant.Status != StatusProvisioning || h.store.grant.StripeScheduleID != "sched_partial" {
		t.Fatalf("grant=%#v", h.store.grant)
	}
	if h.store.provisioningSchedule == nil || h.store.provisioningSchedule.StripeScheduleID != "" {
		t.Fatalf("update=%#v", h.store.provisioningSchedule)
	}
}

func TestGrantPaidProvisioningRetryRejectsMismatchAndUnrelatedSchedule(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*serviceHarness)
		want   error
	}{
		{name: "duration mismatch", mutate: func(h *serviceHarness) { h.store.open.DurationDays = 60 }, want: ErrOpenGrantExists},
		{name: "mode mismatch", mutate: func(h *serviceHarness) { h.store.open.StripeMode = "sandbox" }, want: ErrOpenGrantExists},
		{name: "subscription mismatch", mutate: func(h *serviceHarness) { h.store.open.StripeSubscriptionID = "sub_other" }, want: ErrOpenGrantExists},
		{name: "unrelated schedule", mutate: func(h *serviceHarness) { h.stripe.subscription.ScheduleID = "sched_other" }, want: ErrUnrelatedSchedule},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newPaidServiceHarness(t)
			h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: StatusProvisioning, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_partial"}
			h.store.grant = *h.store.open
			h.stripe.subscription.ScheduleID = "sched_partial"
			test.mutate(h)
			_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v, want %v", err, test.want)
			}
			if h.stripe.createScheduleCalls != 0 || h.stripe.configureScheduleCalls != 0 {
				t.Fatalf("unsafe mutation: create=%d configure=%d", h.stripe.createScheduleCalls, h.stripe.configureScheduleCalls)
			}
		})
	}
}

func TestGrantPaidMarkScheduledCASConvergesWhenWebhookAlreadyProjectedSameSchedule(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.markScheduledErr = ErrConcurrentTransition
	h.store.afterMarkScheduled = func() { h.store.grant.Status = StatusScheduled; h.store.grant.StripeScheduleID = "sched_1" }

	got, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if err != nil || got.Status != StatusScheduled || got.StripeScheduleID != "sched_1" {
		t.Fatalf("grant=%#v error=%v", got, err)
	}
}

func TestGrantPaidMarkScheduledCASFailureRetainsProvisioningSchedule(t *testing.T) {
	h := newPaidServiceHarness(t)
	h.store.markScheduledErr = ErrConcurrentTransition

	_, err := h.service.Grant(t.Context(), GrantRequest{WorkspaceID: "ws_1", PlanID: "basic", DurationDays: 30, ActorUserID: "admin_1"})
	if err == nil {
		t.Fatal("error=nil")
	}
	if h.store.grant.Status != StatusProvisioning || h.store.provisioningSchedule == nil || h.store.provisioningSchedule.StripeScheduleID != "sched_1" {
		t.Fatalf("grant=%#v reconciliation=%#v", h.store.grant, h.store.provisioningSchedule)
	}
}

func TestRevokePendingIsLocalOnly(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusPendingActivation}

	got, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRevoked || h.stripe.totalCalls() != 0 {
		t.Fatalf("grant=%#v stripe calls=%d", got, h.stripe.totalCalls())
	}
}

func TestRevokeCheckoutPendingExpiresExactSessionBeforeCASRevoke(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expiredCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "expired"}

	got, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRevoked || h.stripe.lastExpire.CheckoutSessionID != "cs_1" || h.stripe.lastExpire.TrialGrantID != "grant_1" {
		t.Fatalf("grant=%#v expire=%#v", got, h.stripe.lastExpire)
	}
	if !h.store.expireSucceededBeforeRevoke {
		t.Fatal("grant revoked before Stripe confirmed expiry")
	}
}

func TestRevokeCheckoutCompletionWinsRace(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expireErr = errors.New("checkout session is complete")
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "complete"}

	_, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if !errors.Is(err, ErrRevokeConflict) {
		t.Fatalf("error = %v, want ErrRevokeConflict", err)
	}
	if h.store.grant.Status != StatusCheckoutPending || h.store.revokeCalls != 0 {
		t.Fatalf("grant=%#v revoke calls=%d", h.store.grant, h.store.revokeCalls)
	}
}

func TestRevokeExpireErrorRetrievesExactSessionAndRecoversExpired(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expireErr = context.DeadlineExceeded
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "expired"}

	got, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if err != nil || got.Status != StatusRevoked {
		t.Fatalf("grant=%#v error=%v", got, err)
	}
	if h.stripe.retrieveCheckoutCalls != 1 || h.stripe.lastRetrieveCheckoutID != "cs_1" {
		t.Fatalf("retrieve calls=%d id=%q", h.stripe.retrieveCheckoutCalls, h.stripe.lastRetrieveCheckoutID)
	}
}

func TestRevokeExpireAmbiguityClassifiesRetrievedSession(t *testing.T) {
	for _, test := range []struct {
		name, status string
		retrieveErr  error
		wantConflict bool
	}{
		{name: "complete", status: "complete", wantConflict: true},
		{name: "open", status: "open"},
		{name: "unknown", status: "unknown"},
		{name: "retrieve unavailable", retrieveErr: errors.New("network unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
			h.stripe.expireErr = context.DeadlineExceeded
			h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: test.status}
			h.stripe.retrieveCheckoutErr = test.retrieveErr
			_, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
			if test.wantConflict {
				if !errors.Is(err, ErrRevokeConflict) {
					t.Fatalf("error=%v, want conflict", err)
				}
			} else if err == nil || errors.Is(err, ErrRevokeConflict) {
				t.Fatalf("error=%v, want retryable/internal", err)
			}
			if h.store.revokeCalls != 0 {
				t.Fatalf("revoke calls=%d", h.store.revokeCalls)
			}
		})
	}
}

func TestRevokeCASConflictAfterExpiryLetsCompletionWin(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expiredCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "expired"}
	h.store.revokeErr = ErrConcurrentTransition

	_, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if !errors.Is(err, ErrRevokeConflict) {
		t.Fatalf("error = %v, want ErrRevokeConflict", err)
	}
}

func TestRevokeExpiryRaceReopensThenRevokesPendingOffer(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expiredCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "expired"}
	h.store.revokeErrs = []error{ErrConcurrentTransition, nil}
	h.store.afterFirstRevoke = func() { h.store.grant.Status = StatusPendingActivation; h.store.grant.StripeCheckoutSessionID = "" }

	got, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != StatusRevoked || h.store.revokeCalls != 2 {
		t.Fatalf("grant=%#v calls=%d", got, h.store.revokeCalls)
	}
}

func TestRevokeExpiryRaceDoesNotRevokeNewCheckoutClaim(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusCheckoutPending, StripeMode: "sandbox", StripeCheckoutSessionID: "cs_1"}
	h.stripe.expiredCheckout = CheckoutSnapshot{StripeMode: "sandbox", ID: "cs_1", Status: "expired"}
	h.store.revokeErrs = []error{ErrConcurrentTransition}
	h.store.afterFirstRevoke = func() { h.store.grant.Status = StatusCheckoutPending; h.store.grant.StripeCheckoutSessionID = "cs_2" }

	_, err := h.service.Revoke(t.Context(), RevokeRequest{WorkspaceID: "ws_1", GrantID: "grant_1", ActorUserID: "admin_1"})
	if !errors.Is(err, ErrRevokeConflict) || h.store.revokeCalls != 1 {
		t.Fatalf("error=%v calls=%d", err, h.store.revokeCalls)
	}
}

func TestPrepareCheckoutClaimsMatchingPendingGrant(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
	h.store.grant = *h.store.open
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1", CustomerID: "cus_1"}

	got, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.TrialDays != 30 || got.TrialGrantID != "grant_1" || got.URL != h.stripe.checkout.URL {
		t.Fatalf("checkout = %#v", got)
	}
	if h.store.grant.Status != StatusCheckoutPending || h.store.grant.StripeCheckoutSessionID != "cs_1" {
		t.Fatalf("grant = %#v", h.store.grant)
	}
	if h.stripe.lastCheckout.TrialGrantID != "grant_1" || h.stripe.lastCheckout.DurationDays != 30 || h.stripe.lastCheckout.CustomerID != "cus_1" {
		t.Fatalf("Stripe checkout request = %#v", h.stripe.lastCheckout)
	}
	if h.stripe.lastCheckout.Environment != "staging" || h.stripe.lastCheckout.PriceID != "price_growth_live" {
		t.Fatalf("Stripe routing = %#v", h.stripe.lastCheckout)
	}
	if h.stripe.lastCheckout.CheckoutAttempt != 1 || h.store.grant.CheckoutAttempt != 1 {
		t.Fatalf("attempt request=%d grant=%d", h.stripe.lastCheckout.CheckoutAttempt, h.store.grant.CheckoutAttempt)
	}
	if h.store.grant.StripeCustomerID != "cus_1" {
		t.Fatalf("persisted customer=%q", h.store.grant.StripeCustomerID)
	}
}

func TestPrepareCheckoutMatchingPendingGrantRequiresCustomerBeforeClaim(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live", StripeCustomerID: "cus_stale"}
	h.store.grant = *h.store.open
	req := validServiceCheckoutRequest()
	req.CustomerID = ""

	_, err := h.service.PrepareCheckout(t.Context(), req)
	if !errors.Is(err, ErrCheckoutCustomerRequired) {
		t.Fatalf("error=%v, want ErrCheckoutCustomerRequired", err)
	}
	if h.store.claimCalls != 0 || h.stripe.createCheckoutCalls != 0 {
		t.Fatalf("claim=%d Stripe=%d", h.store.claimCalls, h.stripe.createCheckoutCalls)
	}
	if h.store.grant.StripeCustomerID != "cus_stale" {
		t.Fatalf("stale customer changed during preflight: %q", h.store.grant.StripeCustomerID)
	}
}

func TestPrepareCheckoutValidatedCustomerOverwritesCustomerRetainedAfterConfirmedRejection(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
	h.store.grant = *h.store.open
	h.stripe.createCheckoutErr = &CheckoutMutationError{Outcome: MutationRejected, Err: errors.New("customer was rejected")}
	first := validServiceCheckoutRequest()
	first.CustomerID = "cus_stale"
	if _, err := h.service.PrepareCheckout(t.Context(), first); err == nil {
		t.Fatal("confirmed rejection error=nil")
	}
	if h.store.grant.Status != StatusPendingActivation || h.store.grant.StripeCustomerID != "cus_stale" {
		t.Fatalf("grant after rejection=%#v", h.store.grant)
	}

	preflight := validServiceCheckoutRequest()
	preflight.CustomerID = ""
	if _, err := h.service.PrepareCheckout(t.Context(), preflight); !errors.Is(err, ErrCheckoutCustomerRequired) {
		t.Fatalf("preflight error=%v", err)
	}
	if h.store.claimCalls != 1 {
		t.Fatalf("preflight claimed stale customer; calls=%d", h.store.claimCalls)
	}

	h.stripe.createCheckoutErr = nil
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1", CustomerID: "cus_validated"}
	validated := validServiceCheckoutRequest()
	validated.CustomerID = "cus_validated"

	_, err := h.service.PrepareCheckout(t.Context(), validated)
	if err != nil {
		t.Fatal(err)
	}
	if h.store.grant.StripeCustomerID != "cus_validated" || h.stripe.lastCheckout.CustomerID != "cus_validated" {
		t.Fatalf("grant customer=%q Checkout customer=%q", h.store.grant.StripeCustomerID, h.stripe.lastCheckout.CustomerID)
	}
}

func TestPrepareCheckoutResumesRecordedOpenSession(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_1"}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1", CustomerID: "cus_1"}

	req := validServiceCheckoutRequest()
	req.CustomerID = ""
	got, err := h.service.PrepareCheckout(t.Context(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got.URL != h.stripe.retrievedCheckout.URL || !got.Resumed || h.stripe.createCheckoutCalls != 0 {
		t.Fatalf("checkout=%#v create calls=%d", got, h.stripe.createCheckoutCalls)
	}
}

func TestPrepareCheckoutMismatchedPlanDefersToOrdinaryCheckoutWithoutTouchingGrant(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
	h.store.grant = *h.store.open
	req := validServiceCheckoutRequest()
	req.PlanID = "basic"
	req.PriceID = "price_basic_live"

	_, err := h.service.PrepareCheckout(t.Context(), req)
	if !errors.Is(err, ErrTrialCheckoutNotApplicable) {
		t.Fatalf("error = %v, want ErrTrialCheckoutNotApplicable", err)
	}
	if h.store.grant.Status != StatusPendingActivation || h.store.claimCalls != 0 || h.stripe.createCheckoutCalls != 0 {
		t.Fatalf("grant=%#v claim=%d Stripe=%d", h.store.grant, h.store.claimCalls, h.stripe.createCheckoutCalls)
	}
}

func TestPrepareCheckoutMismatchedPlanExpiresRecordedTrialSessionBeforeOrdinaryCheckout(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_1", CheckoutAttempt: 2}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "open", CustomerID: "cus_1", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}
	h.stripe.expiredCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "expired", CustomerID: "cus_1", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}

	_, err := h.service.PrepareCheckout(t.Context(), CheckoutRequest{WorkspaceID: "ws_1", PlanID: "basic", StripeMode: "live", CustomerID: "cus_1", PriceID: "price_basic", SuccessURL: "https://app/success", CancelURL: "https://app/cancel"})
	if !errors.Is(err, ErrTrialCheckoutNotApplicable) {
		t.Fatalf("error = %v, want ordinary Checkout fallback", err)
	}
	if h.stripe.expireCalls != 1 || h.stripe.lastExpire.CheckoutAttempt != 2 || h.store.releaseCalls != 1 || h.store.grant.Status != StatusPendingActivation {
		t.Fatalf("expire=%d request=%#v release=%d grant=%#v", h.stripe.expireCalls, h.stripe.lastExpire, h.store.releaseCalls, h.store.grant)
	}
}

func TestPrepareCheckoutMismatchedPlanBlocksLiveOrIndeterminateTrialSession(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sessionID string
		status    string
		expireErr error
		wantErr   error
	}{
		{name: "missing recorded session", wantErr: ErrCheckoutStateConflict},
		{name: "completed", sessionID: "cs_1", status: "complete", wantErr: ErrCheckoutCompletionPending},
		{name: "expiry unknown", sessionID: "cs_1", status: "open", expireErr: errors.New("timeout"), wantErr: ErrCheckoutExpiryUnconfirmed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: tc.sessionID, CheckoutAttempt: 3}
			h.store.grant = *h.store.open
			h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: tc.sessionID, Status: tc.status, CustomerID: "cus_1", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}
			h.stripe.expireErr = tc.expireErr
			_, err := h.service.PrepareCheckout(t.Context(), CheckoutRequest{WorkspaceID: "ws_1", PlanID: "basic", StripeMode: "live", CustomerID: "cus_1", PriceID: "price_basic", SuccessURL: "https://app/success", CancelURL: "https://app/cancel"})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if h.store.releaseCalls != 0 {
				t.Fatalf("release calls=%d, want 0", h.store.releaseCalls)
			}
		})
	}
}

func TestReconcileOrdinaryCheckoutSupersedesOnlyPendingDifferentPlan(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusPendingActivation}
	h.store.grant = *h.store.open
	for i := 0; i < 2; i++ {
		result, err := h.service.ReconcileOrdinaryCheckout(t.Context(), "ws_1", "basic", time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC))
		if i == 0 {
			if err != nil || result.Grant.Status != StatusSuperseded {
				t.Fatalf("first result=%#v err=%v", result, err)
			}
			h.store.open = nil
		} else if !errors.Is(err, ErrGrantNotFound) {
			t.Fatalf("replay error=%v, want no open grant", err)
		}
	}
	if h.store.markSupersededCalls != 1 {
		t.Fatalf("superseded calls=%d", h.store.markSupersededCalls)
	}
}

func TestIsTerminalGrantIsReadOnlyAndWorkspaceScoped(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusSuperseded}
	terminal, err := h.service.IsTerminalGrant(t.Context(), "grant_1", "ws_1")
	if err != nil || !terminal {
		t.Fatalf("terminal=%v err=%v", terminal, err)
	}
	if h.store.markSupersededCalls != 0 || h.store.markCompletedCalls != 0 || h.store.markCanceledCalls != 0 {
		t.Fatalf("read-only lookup mutated store: superseded=%d completed=%d canceled=%d", h.store.markSupersededCalls, h.store.markCompletedCalls, h.store.markCanceledCalls)
	}
	if _, err := h.service.IsTerminalGrant(t.Context(), "grant_1", "ws_other"); !errors.Is(err, ErrGrantNotFound) {
		t.Fatalf("cross-workspace error=%v, want ErrGrantNotFound", err)
	}
}

func TestPrepareCheckoutReopensExpiredExactSessionAndStoresReplacement(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_expired", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_expired", Status: "expired", CustomerID: "cus_1"}
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_replacement", Status: "open", URL: "https://checkout.stripe.test/cs_replacement", CustomerID: "cus_1"}

	got, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "cs_replacement" || h.store.lastReleasedSessionID != "cs_expired" || h.store.releaseCalls != 1 || h.store.claimCalls != 1 {
		t.Fatalf("checkout=%#v released=%q release calls=%d claim calls=%d", got, h.store.lastReleasedSessionID, h.store.releaseCalls, h.store.claimCalls)
	}
	if h.store.grant.StripeCheckoutSessionID != "cs_replacement" || h.stripe.createCheckoutCalls != 1 {
		t.Fatalf("grant=%#v Stripe calls=%d", h.store.grant, h.stripe.createCheckoutCalls)
	}
	if h.stripe.lastCheckout.CheckoutAttempt != 2 || h.store.grant.CheckoutAttempt != 2 {
		t.Fatalf("replacement attempt request=%d grant=%d", h.stripe.lastCheckout.CheckoutAttempt, h.store.grant.CheckoutAttempt)
	}
}

func TestPrepareCheckoutCompletedSessionDefersToWebhookWithoutDuplicate(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_complete"}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_complete", Status: "complete", CustomerID: "cus_1"}

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if !errors.Is(err, ErrCheckoutCompletionPending) {
		t.Fatalf("error = %v, want ErrCheckoutCompletionPending", err)
	}
	if h.stripe.createCheckoutCalls != 0 || h.store.releaseCalls != 0 {
		t.Fatalf("create=%d release=%d", h.stripe.createCheckoutCalls, h.store.releaseCalls)
	}
}

func TestPrepareCheckoutMatchingActiveGrantDoesNotFallThroughToOrdinaryCheckout(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusActive, StripeMode: "live"}
	h.store.grant = *h.store.open

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if !errors.Is(err, ErrTrialPlanChangeRequired) {
		t.Fatalf("error=%v, want ErrTrialPlanChangeRequired", err)
	}
	if h.stripe.createCheckoutCalls != 0 {
		t.Fatalf("Stripe create calls=%d", h.stripe.createCheckoutCalls)
	}
}

func TestPrepareCheckoutManagedMutationRiskStatesRequireChangePlan(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   Kind
		status Status
	}{
		{name: "paid provisioning", kind: KindPaidSamePlan, status: StatusProvisioning},
		{name: "paid scheduled", kind: KindPaidSamePlan, status: StatusScheduled},
		{name: "paid active", kind: KindPaidSamePlan, status: StatusActive},
		{name: "free scheduled mismatched", kind: KindFreeToPaid, status: StatusScheduled},
		{name: "free active mismatched", kind: KindFreeToPaid, status: StatusActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: tc.kind, PlanID: "growth", Status: tc.status, StripeMode: "live"}
			h.store.grant = *h.store.open
			req := validServiceCheckoutRequest()
			req.PlanID, req.PriceID = "basic", "price_basic"
			_, err := h.service.PrepareCheckout(t.Context(), req)
			if !errors.Is(err, ErrTrialPlanChangeRequired) {
				t.Fatalf("error=%v, want ErrTrialPlanChangeRequired", err)
			}
			if h.stripe.createCheckoutCalls != 0 || h.stripe.expireCalls != 0 || h.store.claimCalls != 0 {
				t.Fatalf("unsafe mutation: create=%d expire=%d claim=%d", h.stripe.createCheckoutCalls, h.stripe.expireCalls, h.store.claimCalls)
			}
		})
	}
}

func TestPrepareCheckoutIndeterminateCreateKeepsClaimAndIdempotentRetryRecovers(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
	h.store.grant = *h.store.open
	h.stripe.createCheckoutErr = &CheckoutMutationError{Outcome: MutationIndeterminate, Err: context.DeadlineExceeded}

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if h.store.grant.Status != StatusCheckoutPending || h.store.reopenCalls != 0 {
		t.Fatalf("grant=%#v reopen=%d", h.store.grant, h.store.reopenCalls)
	}

	h.stripe.createCheckoutErr = nil
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_recovered", Status: "open", URL: "https://checkout.stripe.test/cs_recovered", CustomerID: "cus_1"}
	retryReq := validServiceCheckoutRequest()
	retryReq.CustomerID = ""
	got, err := h.service.PrepareCheckout(t.Context(), retryReq)
	if err != nil || got.SessionID != "cs_recovered" {
		t.Fatalf("checkout=%#v error=%v", got, err)
	}
	if h.stripe.createCheckoutCalls != 2 || h.store.grant.StripeCheckoutSessionID != "cs_recovered" {
		t.Fatalf("Stripe calls=%d grant=%#v", h.stripe.createCheckoutCalls, h.store.grant)
	}
	if len(h.stripe.checkoutAttempts) != 2 || h.stripe.checkoutAttempts[0] != 1 || h.stripe.checkoutAttempts[1] != 1 {
		t.Fatalf("retry attempts=%v", h.stripe.checkoutAttempts)
	}
}

func TestPrepareCheckoutConfirmedRejectionReopensUnrecordedClaim(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
	h.store.grant = *h.store.open
	h.stripe.createCheckoutErr = &CheckoutMutationError{Outcome: MutationRejected, Err: errors.New("invalid customer")}

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err == nil {
		t.Fatal("error = nil")
	}
	if h.store.grant.Status != StatusPendingActivation || h.store.reopenCalls != 1 {
		t.Fatalf("grant=%#v reopen=%d", h.store.grant, h.store.reopenCalls)
	}
}

func TestPrepareCheckoutConcurrentRecordConvergesOnExactSameSession(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_shared", Status: "open", URL: "https://checkout.stripe.test/cs_shared", CustomerID: "cus_1"}
	h.store.recordErr = ErrConcurrentTransition
	h.store.afterRecord = func() { h.store.grant.StripeCheckoutSessionID = "cs_shared" }

	got, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err != nil || got.SessionID != "cs_shared" {
		t.Fatalf("checkout=%#v error=%v", got, err)
	}
	if h.stripe.createCheckoutCalls != 1 || h.store.recordCalls != 1 {
		t.Fatalf("create=%d record=%d", h.stripe.createCheckoutCalls, h.store.recordCalls)
	}
}

func TestPrepareCheckoutDelayedPriorAttemptCannotRecordIntoNewAttempt(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_attempt_1", Status: "open", URL: "https://checkout.stripe.test/cs_attempt_1", CustomerID: "cus_1"}
	h.store.beforeRecord = func() { h.store.grant.CheckoutAttempt = 2 }

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if !errors.Is(err, ErrCheckoutStateConflict) {
		t.Fatalf("error=%v, want ErrCheckoutStateConflict", err)
	}
	if h.store.grant.CheckoutAttempt != 2 || h.store.grant.StripeCheckoutSessionID != "" {
		t.Fatalf("new attempt was polluted: %#v", h.store.grant)
	}
}

func TestPrepareCheckoutDelayedPriorAttemptRejectionCannotReopenNewAttempt(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.createCheckoutErr = &CheckoutMutationError{Outcome: MutationRejected, Err: errors.New("attempt one rejected")}
	h.store.beforeReopen = func() { h.store.grant.CheckoutAttempt = 2 }

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err == nil {
		t.Fatal("error=nil")
	}
	if h.store.grant.CheckoutAttempt != 2 || h.store.grant.Status != StatusCheckoutPending {
		t.Fatalf("new attempt was reopened: %#v", h.store.grant)
	}
}

func TestPrepareCheckoutRejectsCreatedSessionWithoutExactCustomerBinding(t *testing.T) {
	for _, customerID := range []string{"", "cus_other"} {
		t.Run("customer_"+customerID, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusPendingActivation, StripeMode: "live"}
			h.store.grant = *h.store.open
			h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1", CustomerID: customerID}

			_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
			if !errors.Is(err, ErrCheckoutStateConflict) {
				t.Fatalf("error=%v, want ErrCheckoutStateConflict", err)
			}
			if h.store.recordCalls != 0 || h.store.releaseCalls != 0 {
				t.Fatalf("record=%d release=%d", h.store.recordCalls, h.store.releaseCalls)
			}
		})
	}
}

func TestPrepareCheckoutRejectsRetrievedSessionWithoutExactCustomerBinding(t *testing.T) {
	for _, status := range []string{"open", "expired", "complete"} {
		for _, customerID := range []string{"", "cus_other"} {
			t.Run(status+"_customer_"+customerID, func(t *testing.T) {
				h := newServiceHarness(t)
				h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_1", CheckoutAttempt: 1}
				h.store.grant = *h.store.open
				h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_1", Status: status, URL: "https://checkout.stripe.test/cs_1", CustomerID: customerID}

				_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
				if !errors.Is(err, ErrCheckoutStateConflict) {
					t.Fatalf("error=%v, want ErrCheckoutStateConflict", err)
				}
				if h.store.recordCalls != 0 || h.store.releaseCalls != 0 {
					t.Fatalf("record=%d release=%d", h.store.recordCalls, h.store.releaseCalls)
				}
			})
		}
	}
}

func TestPrepareCheckoutRejectsRetrievedSessionWithMismatchedMetadata(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCheckoutSessionID: "cs_1", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{
		StripeMode: "live", ID: "cs_1", Status: "open", URL: "https://checkout.stripe.test/cs_1",
		Metadata: map[string]string{"workspace_id": "ws_other", "plan_id": "growth", "trial_grant_id": "grant_1", "trial_kind": string(KindFreeToPaid), "unipost_environment": "staging"},
	}

	_, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if !errors.Is(err, ErrCheckoutStateConflict) {
		t.Fatalf("error=%v, want ErrCheckoutStateConflict", err)
	}
	if h.stripe.createCheckoutCalls != 0 || h.store.releaseCalls != 0 {
		t.Fatalf("create=%d release=%d", h.stripe.createCheckoutCalls, h.store.releaseCalls)
	}
}

func TestPrepareCheckoutExpiredReleaseRaceReclaimsWebhookReopenedGrant(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1", StripeCheckoutSessionID: "cs_expired", CheckoutAttempt: 1}
	h.store.grant = *h.store.open
	h.stripe.retrievedCheckout = CheckoutSnapshot{StripeMode: "live", ID: "cs_expired", Status: "expired", CustomerID: "cus_1"}
	h.stripe.checkout = CheckoutSnapshot{StripeMode: "live", ID: "cs_replacement", Status: "open", URL: "https://checkout.stripe.test/cs_replacement", CustomerID: "cus_1"}
	h.store.releaseErr = ErrConcurrentTransition
	h.store.afterRelease = func() {
		h.store.grant.Status = StatusPendingActivation
		h.store.grant.StripeCheckoutSessionID = ""
		h.store.open = &h.store.grant
	}

	preflight := validServiceCheckoutRequest()
	preflight.CustomerID = ""
	if _, err := h.service.PrepareCheckout(t.Context(), preflight); !errors.Is(err, ErrCheckoutCustomerRequired) {
		t.Fatalf("preflight error=%v", err)
	}
	if h.store.claimCalls != 0 || h.store.grant.CheckoutAttempt != 1 {
		t.Fatalf("preflight claim=%d attempt=%d", h.store.claimCalls, h.store.grant.CheckoutAttempt)
	}
	got, err := h.service.PrepareCheckout(t.Context(), validServiceCheckoutRequest())
	if err != nil || got.SessionID != "cs_replacement" {
		t.Fatalf("checkout=%#v error=%v", got, err)
	}
	if h.store.claimCalls != 1 || h.stripe.lastCheckout.CheckoutAttempt != 2 {
		t.Fatalf("validated claim=%d attempt=%d", h.store.claimCalls, h.stripe.lastCheckout.CheckoutAttempt)
	}
}

func TestReconcileSubscriptionActivatesCheckoutGrantAndIsIdempotent(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{
		ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth",
		Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1",
		StripeCheckoutSessionID: "cs_1",
	}
	snapshot := SubscriptionSnapshot{
		StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1",
		PriceID: "price_growth_live", TrialStartAt: &start, TrialEndAt: &end,
		CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end,
		Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging"),
	}

	for i := 0; i < 2; i++ {
		result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: snapshot, PlanID: "growth", OccurredAt: start})
		if err != nil {
			t.Fatalf("reconcile attempt %d: %v", i+1, err)
		}
		if !result.Managed || result.Grant.Status != StatusActive || result.Grant.StripeSubscriptionID != "sub_1" {
			t.Fatalf("result attempt %d = %#v", i+1, result)
		}
	}
	if h.store.markActiveCalls != 1 {
		t.Fatalf("active transitions = %d, want 1", h.store.markActiveCalls)
	}
}

func TestReconcileSubscriptionCreatedBeforeCheckoutConvergesByMetadata(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(60 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "basic", Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1"}

	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{
		Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_basic", TrialStartAt: &start, TrialEndAt: &end, Metadata: trialMetadata("ws_1", "basic", "grant_1", KindFreeToPaid, "staging")},
		PlanID:   "basic", OccurredAt: start,
	})
	if err != nil {
		t.Fatalf("reconcile created-before-checkout: %v", err)
	}
	if !result.Managed || result.Grant.Status != StatusActive || result.Grant.StripeSubscriptionID != "sub_1" {
		t.Fatalf("result = %#v", result)
	}
}

func TestReconcileSubscriptionRejectsManagedMetadataMismatchAndForeignEnvironment(t *testing.T) {
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	for _, tc := range []struct {
		name     string
		metadata map[string]string
		wantErr  error
	}{
		{name: "workspace mismatch", metadata: trialMetadata("ws_other", "growth", "grant_1", KindFreeToPaid, "staging"), wantErr: ErrWebhookStateConflict},
		{name: "plan mismatch", metadata: trialMetadata("ws_1", "basic", "grant_1", KindFreeToPaid, "staging"), wantErr: ErrWebhookStateConflict},
		{name: "foreign environment", metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "dev"), wantErr: ErrWebhookNotApplicable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusCheckoutPending, StripeMode: "live", StripeCustomerID: "cus_1"}
			_, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, Metadata: tc.metadata}, PlanID: "growth", OccurredAt: start})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if h.store.markActiveCalls != 0 {
				t.Fatalf("active transitions = %d, want 0", h.store.markActiveCalls)
			}
		})
	}
}

func TestReconcileManagedTrialCancellationRecordsIntentWithoutTerminalTransition(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusActive, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end}

	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, CancelAtPeriodEnd: true, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: start.Add(time.Hour)})
	if err != nil {
		t.Fatalf("reconcile cancellation: %v", err)
	}
	if !result.Managed || !result.RenewalCanceled || result.Grant.Status != StatusActive || result.Grant.CanceledAt == nil {
		t.Fatalf("result = %#v", result)
	}
	if h.store.markCanceledCalls != 0 || h.store.recordRenewalCancellationCalls != 1 {
		t.Fatalf("terminal=%d renewal=%d", h.store.markCanceledCalls, h.store.recordRenewalCancellationCalls)
	}
}

func TestReconcileManagedTrialPreservesRecordedCancellationOnDelayedFalse(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	intent := start.Add(time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusActive, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end, CanceledAt: &intent}
	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end, CancelAtPeriodEnd: false, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: start.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Managed || !result.PreserveCancelAtPeriodEnd || result.DoNotProject {
		t.Fatalf("result = %#v", result)
	}
}

func TestReconcileSubscriptionCASConflictRecomputesTerminalProjectionDecision(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	for _, tc := range []struct {
		name      string
		snapshot  SubscriptionSnapshot
		planID    string
		configure func(*fakeGrantStore)
	}{
		{
			name:     "concurrent canceled rejects stale trialing",
			snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end, CancelAtPeriodEnd: true, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")},
			planID:   "growth",
			configure: func(store *fakeGrantStore) {
				store.recordRenewalCancellationErr = ErrConcurrentTransition
				store.afterRecordRenewalCancellation = func() { store.grant.Status = StatusCanceled }
			},
		},
		{
			name:     "concurrent superseded rejects old active period",
			snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", PriceID: "price_growth", CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")},
			planID:   "growth",
			configure: func(store *fakeGrantStore) {
				store.markCompletedErr = ErrConcurrentTransition
				store.afterMarkCompleted = func() {
					store.grant.Status = StatusSuperseded
					store.grant.SupersededByPlanID = "team"
					store.grant.SupersededAt = &end
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusActive, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end}
			tc.configure(h.store)
			result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: tc.snapshot, PlanID: tc.planID, OccurredAt: end})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Managed || !result.DoNotProject || !result.Grant.Status.IsTerminal() {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestReconcileSubscriptionCASConflictAllowsCompatibleCompletedReplay(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	renewalEnd := end.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusActive, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end}
	h.store.markCompletedErr = ErrConcurrentTransition
	h.store.afterMarkCompleted = func() {
		h.store.grant.Status = StatusCompleted
		h.store.grant.CompletedAt = &end
	}
	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", PriceID: "price_growth", CurrentPeriodStartAt: &end, CurrentPeriodEndAt: &renewalEnd, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: end})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Managed || result.DoNotProject || result.Grant.Status != StatusCompleted {
		t.Fatalf("result = %#v, compatible replay must remain projectable", result)
	}
}

func TestTerminalCancellationPreservesEarlierRenewalIntentTime(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	intent := start.Add(time.Hour)
	terminal := end.Add(time.Minute)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusActive, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end, CanceledAt: &intent}
	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "canceled", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: terminal})
	if err != nil {
		t.Fatal(err)
	}
	if result.Grant.Status != StatusCanceled || result.Grant.CanceledAt == nil || !result.Grant.CanceledAt.Equal(intent) {
		t.Fatalf("grant = %#v, want original intent %s", result.Grant, intent)
	}
}

func TestReconcileCheckoutExpiredReleasesOnlyExactPendingAttempt(t *testing.T) {
	h := newServiceHarness(t)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusCheckoutPending, StripeMode: "live", StripeCheckoutSessionID: "cs_exact"}
	result, err := h.service.ReconcileCheckoutExpired(t.Context(), WebhookCheckoutRequest{StripeMode: "live", SessionID: "cs_exact", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging"), OccurredAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("reconcile checkout expired: %v", err)
	}
	if !result.Managed || result.Grant.Status != StatusPendingActivation || h.store.releaseCalls != 1 {
		t.Fatalf("result=%#v release_calls=%d", result, h.store.releaseCalls)
	}

	h.store.grant.Status = StatusActive
	h.store.grant.StripeCheckoutSessionID = "cs_exact"
	result, err = h.service.ReconcileCheckoutExpired(t.Context(), WebhookCheckoutRequest{StripeMode: "live", SessionID: "cs_exact", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")})
	if err != nil || !result.Managed || result.Grant.Status != StatusActive || h.store.releaseCalls != 1 {
		t.Fatalf("terminal replay result=%#v err=%v release_calls=%d", result, err, h.store.releaseCalls)
	}
}

func TestReconcileScheduleLifecycleIsGuardedAndMonotonic(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "growth", Status: StatusScheduled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, EndsAt: &end}
	snapshot := ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "completed", CustomerID: "cus_1", SubscriptionID: "sub_1", CompletedAt: &end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging"), Phases: []SchedulePhase{{PriceID: "price_growth", StartAt: start, EndAt: end, TrialEndAt: end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}}}

	for i := 0; i < 2; i++ {
		result, err := h.service.ReconcileSchedule(t.Context(), WebhookScheduleRequest{EventType: "subscription_schedule.completed", Snapshot: snapshot, OccurredAt: end})
		if err != nil {
			t.Fatalf("reconcile schedule attempt %d: %v", i+1, err)
		}
		if !result.Managed || result.Grant.Status != StatusCompleted {
			t.Fatalf("result attempt %d = %#v", i+1, result)
		}
	}
	if h.store.markActiveCalls != 1 || h.store.markCompletedCalls != 1 {
		t.Fatalf("active=%d completed=%d, want 1/1", h.store.markActiveCalls, h.store.markCompletedCalls)
	}
}

func TestReconcileScheduleCancellationRecordsIntentWithoutEndingAccess(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "growth", Status: StatusScheduled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, EndsAt: &end}
	snapshot := ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "active", EndBehavior: "cancel", CustomerID: "cus_1", SubscriptionID: "sub_1", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging"), Phases: []SchedulePhase{{PriceID: "price_growth", StartAt: start, EndAt: end, TrialEndAt: end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}}}
	for i := 0; i < 2; i++ {
		result, err := h.service.ReconcileSchedule(t.Context(), WebhookScheduleRequest{EventType: "subscription_schedule.updated", Snapshot: snapshot, OccurredAt: start})
		if err != nil {
			t.Fatal(err)
		}
		if result.Grant.Status != StatusScheduled || result.Grant.CanceledAt == nil {
			t.Fatalf("result=%#v", result)
		}
	}
	if h.store.recordRenewalCancellationCalls != 1 || h.store.markCanceledCalls != 0 {
		t.Fatalf("intent=%d terminal=%d", h.store.recordRenewalCancellationCalls, h.store.markCanceledCalls)
	}
}

func TestReconcileSchedulePlanChangeMarksSupersededAfterStripeConfirmation(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "basic", Status: StatusScheduled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, EndsAt: &end}
	snapshot := ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "active", EndBehavior: "release", CustomerID: "cus_1", SubscriptionID: "sub_1", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging"), Phases: []SchedulePhase{{PriceID: "price_growth", StartAt: start, EndAt: end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}}}
	result, err := h.service.ReconcileSchedule(t.Context(), WebhookScheduleRequest{EventType: "subscription_schedule.updated", Snapshot: snapshot, OccurredAt: start})
	if err != nil {
		t.Fatal(err)
	}
	if result.Grant.Status != StatusSuperseded || result.Grant.SupersededByPlanID != "growth" || h.store.markSupersededCalls != 1 {
		t.Fatalf("result=%#v calls=%d", result, h.store.markSupersededCalls)
	}
}

func TestReconcileSubscriptionTerminalGrantDoesNotProjectDelayedTrialingEvent(t *testing.T) {
	for _, status := range []Status{StatusCompleted, StatusCanceled, StatusSuperseded} {
		t.Run(string(status), func(t *testing.T) {
			h := newServiceHarness(t)
			start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
			end := start.Add(30 * 24 * time.Hour)
			h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: status, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end}
			result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_growth", TrialStartAt: &start, TrialEndAt: &end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: start})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Managed || !result.DoNotProject || result.Grant.Status != status {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestReconcileSubscriptionTerminalOutcomeAllowsProjectionRetry(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	renewalEnd := end.Add(30 * 24 * time.Hour)
	for _, tc := range []struct {
		name           string
		grant          Grant
		status         string
		planID         string
		metadataPlanID string
	}{
		{name: "completed active renewal", grant: Grant{Status: StatusCompleted, PlanID: "growth", CompletedAt: &end}, status: "active", planID: "growth", metadataPlanID: "growth"},
		{name: "superseded paid plan", grant: Grant{Status: StatusSuperseded, PlanID: "growth", SupersededByPlanID: "team", SupersededAt: &end}, status: "active", planID: "team", metadataPlanID: "team"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			grant := tc.grant
			grant.ID, grant.WorkspaceID, grant.Kind, grant.StripeMode = "grant_1", "ws_1", KindFreeToPaid, "live"
			grant.StripeCustomerID, grant.StripeSubscriptionID, grant.StartedAt, grant.EndsAt = "cus_1", "sub_1", &start, &end
			h.store.grant = grant
			metadata := trialMetadata("ws_1", tc.metadataPlanID, "grant_1", KindFreeToPaid, "staging")
			result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: tc.status, CustomerID: "cus_1", PriceID: "price", TrialStartAt: &start, TrialEndAt: &end, CurrentPeriodStartAt: &end, CurrentPeriodEndAt: &renewalEnd, Metadata: metadata}, PlanID: tc.planID, OccurredAt: end.Add(-time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Managed || result.DoNotProject {
				t.Fatalf("result = %#v, want projectable retry", result)
			}
		})
	}
}

func TestReconcileCompletedPaidTrialRejectsDelayedPreTrialActivePeriod(t *testing.T) {
	h := newServiceHarness(t)
	paidStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	trialStart := paidStart.Add(30 * 24 * time.Hour)
	trialEnd := trialStart.Add(30 * 24 * time.Hour)
	completedAt := trialEnd
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "growth", Status: StatusCompleted, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", ScheduledStartAt: &trialStart, StartedAt: &trialStart, EndsAt: &trialEnd, CompletedAt: &completedAt}
	result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", PriceID: "price_growth", CurrentPeriodStartAt: &paidStart, CurrentPeriodEndAt: &trialStart, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}, PlanID: "growth", OccurredAt: trialEnd.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DoNotProject {
		t.Fatalf("result = %#v, delayed pre-trial period must not project", result)
	}
}

func TestReconcileCanceledGrantNeverProjectsOrdinarySubscriptionSnapshot(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	canceledAt := end
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, PlanID: "growth", Status: StatusCanceled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end, CanceledAt: &canceledAt}
	for _, status := range []string{"trialing", "active", "canceled"} {
		result, err := h.service.ReconcileSubscription(t.Context(), WebhookSubscriptionRequest{Snapshot: SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: status, CustomerID: "cus_1", PriceID: "price_growth", CurrentPeriodStartAt: &end, CurrentPeriodEndAt: webhookPtr(end.Add(30 * 24 * time.Hour)), Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}, PlanID: "growth", OccurredAt: end.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if !result.DoNotProject {
			t.Fatalf("status %s result=%#v, canceled grant must not project", status, result)
		}
	}
}

func TestReconcileReleasedScheduleAfterTrialEndCompletesOutOfOrder(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "growth", Status: StatusScheduled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, EndsAt: &end}
	snapshot := ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "released", EndBehavior: "release", CustomerID: "cus_1", ReleasedSubscriptionID: "sub_1", ReleasedAt: &end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging"), Phases: []SchedulePhase{{PriceID: "price_growth", StartAt: start, EndAt: end, TrialEndAt: end, Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}}}
	result, err := h.service.ReconcileSchedule(t.Context(), WebhookScheduleRequest{EventType: "subscription_schedule.released", Snapshot: snapshot, OccurredAt: end})
	if err != nil {
		t.Fatal(err)
	}
	if result.Grant.Status != StatusCompleted || h.store.markActiveCalls != 1 || h.store.markCompletedCalls != 1 || h.store.markSupersededCalls != 0 {
		t.Fatalf("result=%#v active=%d completed=%d superseded=%d", result, h.store.markActiveCalls, h.store.markCompletedCalls, h.store.markSupersededCalls)
	}
}

func TestReconcileEarlyReleasedScheduleIsSuperseded(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, PlanID: "growth", Status: StatusScheduled, StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, EndsAt: &end}
	snapshot := ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "released", EndBehavior: "release", CustomerID: "cus_1", ReleasedSubscriptionID: "sub_1", ReleasedAt: webhookPtr(start.Add(time.Hour)), Metadata: trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")}
	result, err := h.service.ReconcileSchedule(t.Context(), WebhookScheduleRequest{EventType: "subscription_schedule.released", Snapshot: snapshot, OccurredAt: start.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if result.Grant.Status != StatusSuperseded || h.store.markSupersededCalls != 1 {
		t.Fatalf("result=%#v superseded=%d", result, h.store.markSupersededCalls)
	}
}

func webhookPtr(value time.Time) *time.Time { return &value }

func validServiceCheckoutRequest() CheckoutRequest {
	return CheckoutRequest{
		WorkspaceID: "ws_1", PlanID: "growth", StripeMode: "live",
		CustomerID: "cus_1", PriceID: "price_growth_live",
		SuccessURL: "https://app.unipost.dev/settings/billing?status=success",
		CancelURL:  "https://app.unipost.dev/settings/billing?status=canceled",
	}
}

type serviceHarness struct {
	service *Service
	store   *fakeGrantStore
	stripe  *fakeServiceStripe
	modes   *fakeModeResolver
}

func newServiceHarness(t *testing.T) *serviceHarness {
	t.Helper()
	store := &fakeGrantStore{billing: BillingSnapshot{WorkspaceID: "ws_1", OwnerUserID: "owner_1", Subscription: SubscriptionRecord{PlanID: "free", Status: "active"}}}
	stripe := &fakeServiceStripe{store: store}
	modes := &fakeModeResolver{mode: BillingMode{Name: "live", PriceID: "price_growth_live"}}
	service := NewService(store, stripe, modes, "staging", func() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) })
	return &serviceHarness{service: service, store: store, stripe: stripe, modes: modes}
}

func newPaidServiceHarness(t *testing.T) *serviceHarness {
	t.Helper()
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	h.store.billing.Subscription = SubscriptionRecord{PlanID: "basic", Status: "active", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"}
	h.modes.mode = BillingMode{Name: "live", PriceID: "price_basic_live"}
	h.stripe.subscription = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", PriceID: "price_basic_live", CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end}
	h.stripe.schedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_1", CustomerID: "cus_1", SubscriptionID: "sub_1"}
	return h
}

type fakeGrantStore struct {
	billing                        BillingSnapshot
	open                           *Grant
	grant                          Grant
	created                        Grant
	failed                         *FailureUpdate
	createCalls                    int
	revokeCalls                    int
	revokeErr                      error
	revokeErrs                     []error
	afterFirstRevoke               func()
	statusAtScheduleCall           Status
	expireSucceededBeforeRevoke    bool
	provisioningSchedule           *ProvisioningScheduleUpdate
	markScheduledErr               error
	afterMarkScheduled             func()
	claimCalls                     int
	recordCalls                    int
	releaseCalls                   int
	reopenCalls                    int
	lastReleasedSessionID          string
	recordErr                      error
	afterRecord                    func()
	beforeRecord                   func()
	beforeReopen                   func()
	releaseErr                     error
	afterRelease                   func()
	markActiveCalls                int
	markCanceledCalls              int
	markCompletedCalls             int
	markSupersededCalls            int
	recordRenewalCancellationCalls int
	recordRenewalCancellationErr   error
	afterRecordRenewalCancellation func()
	markCompletedErr               error
	afterMarkCompleted             func()
}

func (s *fakeGrantStore) GetBilling(context.Context, string) (BillingSnapshot, error) {
	return s.billing, nil
}
func (s *fakeGrantStore) GetOpenGrant(context.Context, string) (Grant, error) {
	if s.open == nil {
		return Grant{}, ErrGrantNotFound
	}
	return *s.open, nil
}
func (s *fakeGrantStore) GetGrant(_ context.Context, id string) (Grant, error) {
	if s.grant.ID != id {
		return Grant{}, ErrGrantNotFound
	}
	return s.grant, nil
}
func (s *fakeGrantStore) CreateGrant(_ context.Context, input CreateGrantInput) (Grant, error) {
	s.createCalls++
	s.created = Grant{ID: "grant_1", WorkspaceID: input.WorkspaceID, Kind: input.Kind, PlanID: input.PlanID, DurationDays: input.DurationDays, Status: input.Status, ActorUserID: input.ActorUserID, StripeMode: input.StripeMode, StripeCustomerID: input.StripeCustomerID, StripeSubscriptionID: input.StripeSubscriptionID, GrantedAt: input.GrantedAt}
	s.grant = s.created
	return s.created, nil
}
func (s *fakeGrantStore) MarkScheduled(_ context.Context, update ScheduledUpdate) (Grant, error) {
	if s.markScheduledErr != nil {
		if s.afterMarkScheduled != nil {
			s.afterMarkScheduled()
		}
		return Grant{}, s.markScheduledErr
	}
	s.grant.Status, s.grant.StripeScheduleID = StatusScheduled, update.StripeScheduleID
	s.grant.ScheduledStartAt, s.grant.EndsAt = &update.ScheduledStartAt, &update.EndsAt
	return s.grant, nil
}
func (s *fakeGrantStore) MarkFailed(_ context.Context, update FailureUpdate) (Grant, error) {
	s.failed = &update
	s.grant.Status = StatusFailed
	return s.grant, nil
}
func (s *fakeGrantStore) RecordProvisioningSchedule(_ context.Context, update ProvisioningScheduleUpdate) (Grant, error) {
	s.provisioningSchedule = &update
	if update.StripeScheduleID != "" {
		s.grant.StripeScheduleID = update.StripeScheduleID
	}
	s.grant.FailureCode = update.FailureCode
	s.grant.FailureMessage = update.FailureMessage
	return s.grant, nil
}
func (s *fakeGrantStore) MarkRevoked(_ context.Context, id string, expected Status, at time.Time) (Grant, error) {
	s.revokeCalls++
	if len(s.revokeErrs) > 0 {
		err := s.revokeErrs[0]
		s.revokeErrs = s.revokeErrs[1:]
		if s.revokeCalls == 1 && s.afterFirstRevoke != nil {
			s.afterFirstRevoke()
		}
		if err != nil {
			return Grant{}, err
		}
	}
	if s.revokeErr != nil {
		return Grant{}, s.revokeErr
	}
	if s.grant.ID != id || s.grant.Status != expected {
		return Grant{}, ErrConcurrentTransition
	}
	if expected == StatusCheckoutPending && !s.expireSucceededBeforeRevoke {
		return Grant{}, errors.New("revoked before expiry")
	}
	s.grant.Status, s.grant.RevokedAt = StatusRevoked, &at
	return s.grant, nil
}
func (s *fakeGrantStore) ClaimCheckout(_ context.Context, id, workspaceID, planID, customerID string) (Grant, error) {
	s.claimCalls++
	if s.grant.ID != id || s.grant.WorkspaceID != workspaceID || s.grant.PlanID != planID || s.grant.Status != StatusPendingActivation {
		return Grant{}, ErrConcurrentTransition
	}
	s.grant.Status = StatusCheckoutPending
	s.grant.CheckoutAttempt++
	s.grant.StripeCustomerID = customerID
	s.open = &s.grant
	return s.grant, nil
}
func (s *fakeGrantStore) RecordCheckoutSession(_ context.Context, id, sessionID string, expectedAttempt int32) (Grant, error) {
	s.recordCalls++
	if s.beforeRecord != nil {
		s.beforeRecord()
	}
	if s.recordErr != nil {
		if s.afterRecord != nil {
			s.afterRecord()
		}
		return Grant{}, s.recordErr
	}
	if s.grant.ID != id || s.grant.Status != StatusCheckoutPending || s.grant.StripeCheckoutSessionID != "" || s.grant.CheckoutAttempt != expectedAttempt {
		return Grant{}, ErrConcurrentTransition
	}
	s.grant.StripeCheckoutSessionID = sessionID
	s.open = &s.grant
	return s.grant, nil
}
func (s *fakeGrantStore) ReleaseCheckout(_ context.Context, id, sessionID string, _ time.Time) (Grant, error) {
	s.releaseCalls++
	s.lastReleasedSessionID = sessionID
	if s.releaseErr != nil {
		if s.afterRelease != nil {
			s.afterRelease()
		}
		return Grant{}, s.releaseErr
	}
	if s.grant.ID != id || s.grant.Status != StatusCheckoutPending || s.grant.StripeCheckoutSessionID != sessionID {
		return Grant{}, ErrConcurrentTransition
	}
	s.grant.Status = StatusPendingActivation
	s.grant.StripeCheckoutSessionID = ""
	s.open = &s.grant
	return s.grant, nil
}
func (s *fakeGrantStore) ReopenUnrecordedCheckout(_ context.Context, id string, expectedAttempt int32) (Grant, error) {
	s.reopenCalls++
	if s.beforeReopen != nil {
		s.beforeReopen()
	}
	if s.grant.ID != id || s.grant.Status != StatusCheckoutPending || s.grant.StripeCheckoutSessionID != "" || s.grant.CheckoutAttempt != expectedAttempt {
		return Grant{}, ErrConcurrentTransition
	}
	s.grant.Status = StatusPendingActivation
	s.open = &s.grant
	return s.grant, nil
}
func (s *fakeGrantStore) GetGrantByCheckoutSession(_ context.Context, id string) (Grant, error) {
	if s.grant.StripeCheckoutSessionID != id {
		return Grant{}, ErrGrantNotFound
	}
	return s.grant, nil
}
func (s *fakeGrantStore) GetGrantBySubscription(_ context.Context, id string) (Grant, error) {
	if s.grant.StripeSubscriptionID != id {
		return Grant{}, ErrGrantNotFound
	}
	return s.grant, nil
}
func (s *fakeGrantStore) GetGrantBySchedule(_ context.Context, id string) (Grant, error) {
	if s.grant.StripeScheduleID != id {
		return Grant{}, ErrGrantNotFound
	}
	return s.grant, nil
}
func (s *fakeGrantStore) MarkActive(_ context.Context, update ActiveUpdate) (Grant, error) {
	if s.grant.ID != update.ID || s.grant.Status != update.ExpectedStatus {
		return Grant{}, ErrConcurrentTransition
	}
	s.markActiveCalls++
	s.grant.Status = StatusActive
	s.grant.StripeCustomerID = update.StripeCustomerID
	s.grant.StripeSubscriptionID = update.StripeSubscriptionID
	s.grant.StartedAt, s.grant.EndsAt, s.grant.ActivatedAt = &update.StartedAt, &update.EndsAt, &update.ActivatedAt
	return s.grant, nil
}
func (s *fakeGrantStore) RecordRenewalCancellation(_ context.Context, id string, at time.Time) (Grant, error) {
	if s.grant.ID != id || (s.grant.Status != StatusScheduled && s.grant.Status != StatusActive) {
		return Grant{}, ErrConcurrentTransition
	}
	s.recordRenewalCancellationCalls++
	if s.recordRenewalCancellationErr != nil {
		if s.afterRecordRenewalCancellation != nil {
			s.afterRecordRenewalCancellation()
		}
		return Grant{}, s.recordRenewalCancellationErr
	}
	s.grant.CanceledAt = &at
	return s.grant, nil
}
func (s *fakeGrantStore) MarkCanceled(_ context.Context, id string, expected Status, at time.Time) (Grant, error) {
	if s.grant.ID != id || s.grant.Status != expected {
		return Grant{}, ErrConcurrentTransition
	}
	s.markCanceledCalls++
	s.grant.Status = StatusCanceled
	if s.grant.CanceledAt == nil {
		s.grant.CanceledAt = &at
	}
	return s.grant, nil
}
func (s *fakeGrantStore) MarkCompleted(_ context.Context, id string, expected Status, at time.Time) (Grant, error) {
	if s.grant.ID != id || s.grant.Status != expected {
		return Grant{}, ErrConcurrentTransition
	}
	s.markCompletedCalls++
	if s.markCompletedErr != nil {
		if s.afterMarkCompleted != nil {
			s.afterMarkCompleted()
		}
		return Grant{}, s.markCompletedErr
	}
	s.grant.Status, s.grant.CompletedAt = StatusCompleted, &at
	return s.grant, nil
}
func (s *fakeGrantStore) MarkSuperseded(_ context.Context, id string, expected Status, planID string, at time.Time) (Grant, error) {
	if s.grant.ID != id || s.grant.Status != expected {
		return Grant{}, ErrConcurrentTransition
	}
	s.markSupersededCalls++
	s.grant.Status, s.grant.SupersededAt, s.grant.SupersededByPlanID = StatusSuperseded, &at, planID
	s.open = nil
	return s.grant, nil
}

type fakeModeResolver struct {
	mode                BillingMode
	prices              map[string]string
	ownerUserID, planID string
	err                 error
}

func (r *fakeModeResolver) Resolve(_ context.Context, ownerUserID, planID string) (BillingMode, error) {
	r.ownerUserID, r.planID = ownerUserID, planID
	mode := r.mode
	if priceID, ok := r.prices[planID]; ok {
		mode.PriceID = priceID
	}
	return mode, r.err
}

type fakeServiceStripe struct {
	store                                                                                   *fakeGrantStore
	subscription                                                                            SubscriptionSnapshot
	schedule                                                                                ScheduleSnapshot
	expiredCheckout                                                                         CheckoutSnapshot
	createScheduleErr, expireErr                                                            error
	createScheduleCalls, configureScheduleCalls, retrieveSubscriptionCalls, expireCalls     int
	lastSchedule                                                                            CreatePaidTrialScheduleRequest
	lastExpire                                                                              ExpireCheckoutRequest
	lastConfiguredScheduleID                                                                string
	retrievedCheckout                                                                       CheckoutSnapshot
	retrieveCheckoutErr                                                                     error
	retrieveCheckoutCalls                                                                   int
	lastRetrieveCheckoutID                                                                  string
	checkout                                                                                CheckoutSnapshot
	createCheckoutErr                                                                       error
	createCheckoutCalls                                                                     int
	lastCheckout                                                                            CreateTrialCheckoutRequest
	checkoutAttempts                                                                        []int32
	changeFreeResult                                                                        SubscriptionSnapshot
	changeScheduleResult                                                                    ScheduleSnapshot
	retrievedSchedule                                                                       ScheduleSnapshot
	retrieveScheduleErr                                                                     error
	cancelFreeResult                                                                        SubscriptionSnapshot
	cancelScheduleResult                                                                    ScheduleSnapshot
	portalURL                                                                               string
	changeFreeErr, changeScheduleErr, cancelFreeErr, cancelScheduleErr, portalErr           error
	changeFreeCalls, changeScheduleCalls, cancelFreeCalls, cancelScheduleCalls, portalCalls int
	retrieveScheduleCalls                                                                   int
	lastChangeFree                                                                          ChangeFreeTrialPlanRequest
	lastChangeSchedule                                                                      ChangeScheduledTrialPlanRequest
	lastCancelFree                                                                          CancelFreeTrialRequest
	lastCancelSchedule                                                                      CancelPaidScheduleRequest
	lastPortal                                                                              CreatePortalRequest
}

func (s *fakeServiceStripe) totalCalls() int {
	return s.createScheduleCalls + s.configureScheduleCalls + s.retrieveSubscriptionCalls + s.expireCalls
}
func (s *fakeServiceStripe) CreatePaidTrialSchedule(_ context.Context, req CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error) {
	s.createScheduleCalls++
	s.lastSchedule = req
	s.store.statusAtScheduleCall = s.store.grant.Status
	return s.schedule, s.createScheduleErr
}
func (s *fakeServiceStripe) ConfigurePaidTrialSchedule(_ context.Context, scheduleID string, req CreatePaidTrialScheduleRequest) (ScheduleSnapshot, error) {
	s.configureScheduleCalls++
	s.lastConfiguredScheduleID = scheduleID
	s.lastSchedule = req
	s.store.statusAtScheduleCall = s.store.grant.Status
	return s.schedule, s.createScheduleErr
}
func (s *fakeServiceStripe) CreateTrialCheckout(_ context.Context, req CreateTrialCheckoutRequest) (CheckoutSnapshot, error) {
	s.createCheckoutCalls++
	s.lastCheckout = req
	s.checkoutAttempts = append(s.checkoutAttempts, req.CheckoutAttempt)
	if s.checkout.Metadata == nil && s.checkout.ID != "" {
		s.checkout.Metadata = trialMetadata(req.WorkspaceID, req.PlanID, req.TrialGrantID, req.TrialKind, req.Environment)
	}
	return s.checkout, s.createCheckoutErr
}
func (s *fakeServiceStripe) RetrieveCheckout(_ context.Context, _ string, id string) (CheckoutSnapshot, error) {
	s.retrieveCheckoutCalls++
	s.lastRetrieveCheckoutID = id
	if s.retrievedCheckout.Metadata == nil && s.retrievedCheckout.ID != "" && s.store != nil {
		grant := s.store.grant
		s.retrievedCheckout.Metadata = trialMetadata(grant.WorkspaceID, grant.PlanID, grant.ID, grant.Kind, "staging")
	}
	if s.retrieveCheckoutErr == nil && s.retrievedCheckout.Status == "expired" {
		s.store.expireSucceededBeforeRevoke = true
	}
	return s.retrievedCheckout, s.retrieveCheckoutErr
}
func (s *fakeServiceStripe) ExpireCheckout(_ context.Context, req ExpireCheckoutRequest) (CheckoutSnapshot, error) {
	s.expireCalls++
	s.lastExpire = req
	if s.expireErr == nil {
		s.store.expireSucceededBeforeRevoke = true
	}
	return s.expiredCheckout, s.expireErr
}
func (s *fakeServiceStripe) RetrieveSubscription(context.Context, string, string) (SubscriptionSnapshot, error) {
	s.retrieveSubscriptionCalls++
	return s.subscription, nil
}
func (s *fakeServiceStripe) RetrieveSchedule(context.Context, string, string) (ScheduleSnapshot, error) {
	s.retrieveScheduleCalls++
	return s.retrievedSchedule, s.retrieveScheduleErr
}
func (s *fakeServiceStripe) ChangeFreeTrialPlanNow(_ context.Context, req ChangeFreeTrialPlanRequest) (SubscriptionSnapshot, error) {
	s.changeFreeCalls++
	s.lastChangeFree = req
	return s.changeFreeResult, s.changeFreeErr
}
func (s *fakeServiceStripe) ChangeScheduledTrialPlanNow(_ context.Context, req ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	s.changeScheduleCalls++
	s.lastChangeSchedule = req
	return s.changeScheduleResult, s.changeScheduleErr
}
func (s *fakeServiceStripe) CancelFreeTrialAtEnd(_ context.Context, req CancelFreeTrialRequest) (SubscriptionSnapshot, error) {
	s.cancelFreeCalls++
	s.lastCancelFree = req
	return s.cancelFreeResult, s.cancelFreeErr
}
func (s *fakeServiceStripe) CancelPaidScheduleAtTrialEnd(_ context.Context, req CancelPaidScheduleRequest) (ScheduleSnapshot, error) {
	s.cancelScheduleCalls++
	s.lastCancelSchedule = req
	return s.cancelScheduleResult, s.cancelScheduleErr
}
func (s *fakeServiceStripe) CreatePortal(_ context.Context, req CreatePortalRequest) (string, error) {
	s.portalCalls++
	s.lastPortal = req
	return s.portalURL, s.portalErr
}

func TestChangePlanFreeTrialUsesOneSubscriptionUpdateAndWaitsForWebhook(t *testing.T) {
	h := newServiceHarness(t)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindFreeToPaid, Status: StatusActive, PlanID: "basic", StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StartedAt: &start, EndsAt: &end}
	h.store.open = &h.store.grant
	h.store.billing = BillingSnapshot{WorkspaceID: "ws_1", OwnerUserID: "owner_1", Subscription: SubscriptionRecord{PlanID: "basic", Status: "trialing", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"}}
	h.modes.mode = BillingMode{Name: "live", PriceID: "price_growth"}
	h.modes.prices = map[string]string{"basic": "price_basic", "growth": "price_growth"}
	h.stripe.subscription = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", ItemID: "si_1", PriceID: "price_basic", TrialStartAt: &start, TrialEndAt: &end, Metadata: trialMetadata("ws_1", "basic", "grant_1", KindFreeToPaid, "staging")}
	h.stripe.changeFreeResult = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", ItemID: "si_1", PriceID: "price_growth", Metadata: trialMetadata("ws_1", "growth", "grant_1", KindFreeToPaid, "staging")}

	got, err := h.service.ChangePlan(t.Context(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
	if err != nil {
		t.Fatal(err)
	}
	if h.stripe.changeFreeCalls != 1 || h.stripe.changeScheduleCalls != 0 {
		t.Fatalf("free=%d schedule=%d", h.stripe.changeFreeCalls, h.stripe.changeScheduleCalls)
	}
	if h.stripe.lastChangeFree.SubscriptionItemID != "si_1" || h.stripe.lastChangeFree.PriceID != "price_growth" {
		t.Fatalf("request=%#v", h.stripe.lastChangeFree)
	}
	if got.Status != StatusActive || h.store.markSupersededCalls != 0 {
		t.Fatalf("grant=%#v superseded calls=%d", got, h.store.markSupersededCalls)
	}
}

func TestChangePlanPaidTrialUsesOneScheduleUpdateAndNeverMarksBeforeWebhook(t *testing.T) {
	for _, status := range []Status{StatusScheduled, StatusActive} {
		t.Run(string(status), func(t *testing.T) {
			h := newPaidPlanChangeHarness(t, status)
			got, err := h.service.ChangePlan(t.Context(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
			if err != nil {
				t.Fatal(err)
			}
			if h.stripe.retrieveSubscriptionCalls != 1 || h.stripe.retrieveScheduleCalls != 1 || h.stripe.changeScheduleCalls != 1 || h.stripe.changeFreeCalls != 0 {
				t.Fatalf("retrieve sub/schedule=%d/%d change schedule/free=%d/%d", h.stripe.retrieveSubscriptionCalls, h.stripe.retrieveScheduleCalls, h.stripe.changeScheduleCalls, h.stripe.changeFreeCalls)
			}
			if h.stripe.lastChangeSchedule.ScheduleID != "sched_1" || h.stripe.lastChangeSchedule.PriceID != "price_growth" {
				t.Fatalf("request=%#v", h.stripe.lastChangeSchedule)
			}
			if got.Status != status || h.store.markSupersededCalls != 0 {
				t.Fatalf("grant=%#v superseded calls=%d", got, h.store.markSupersededCalls)
			}
		})
	}
}

func TestChangePlanPaidTrialStillWorksAfterRenewalWasCanceled(t *testing.T) {
	h := newPaidPlanChangeHarness(t, StatusActive)
	canceledAt := time.Date(2026, 7, 24, 10, 0, 0, 0, time.UTC)
	h.store.grant.CanceledAt = &canceledAt
	h.store.open = &h.store.grant
	h.stripe.retrievedSchedule.EndBehavior = "cancel"
	h.stripe.retrievedSchedule.Phases = h.stripe.retrievedSchedule.Phases[1:2]
	if _, err := h.service.ChangePlan(t.Context(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"}); err != nil {
		t.Fatal(err)
	}
	if h.stripe.changeScheduleCalls != 1 {
		t.Fatalf("schedule mutations=%d", h.stripe.changeScheduleCalls)
	}
}

func TestChangePlanPaidTrialFailsClosedBeforeMutationOnRemoteDrift(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*serviceHarness)
	}{
		{name: "subscription id", mutate: func(h *serviceHarness) { h.stripe.subscription.ID = "sub_other" }},
		{name: "subscription customer", mutate: func(h *serviceHarness) { h.stripe.subscription.CustomerID = "cus_other" }},
		{name: "subscription schedule", mutate: func(h *serviceHarness) { h.stripe.subscription.ScheduleID = "sched_other" }},
		{name: "subscription price", mutate: func(h *serviceHarness) { h.stripe.subscription.PriceID = "price_other" }},
		{name: "schedule id", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.ID = "sched_other" }},
		{name: "schedule mode", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.StripeMode = "sandbox" }},
		{name: "schedule customer", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.CustomerID = "cus_other" }},
		{name: "schedule subscription", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.SubscriptionID = "sub_other" }},
		{name: "schedule released", mutate: func(h *serviceHarness) {
			h.stripe.retrievedSchedule.Status = "released"
			h.stripe.retrievedSchedule.ReleasedSubscriptionID = "sub_1"
		}},
		{name: "schedule canceled", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.Status = "canceled" }},
		{name: "schedule metadata", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.Metadata[metadataEnvironment] = "production" }},
		{name: "trial phase missing", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.Phases[1].TrialEndAt = time.Time{} }},
		{name: "resumed phase missing", mutate: func(h *serviceHarness) { h.stripe.retrievedSchedule.Phases = h.stripe.retrievedSchedule.Phases[:2] }},
		{name: "current phase metadata", mutate: func(h *serviceHarness) {
			h.stripe.retrievedSchedule.Phases[0].Metadata[metadataTrialGrant] = "grant_other"
		}},
		{name: "current phase boundary", mutate: func(h *serviceHarness) {
			h.stripe.retrievedSchedule.CurrentPhaseEndAt = webhookPtr(h.stripe.retrievedSchedule.CurrentPhaseEndAt.Add(time.Hour))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newPaidPlanChangeHarness(t, StatusScheduled)
			tc.mutate(h)
			_, err := h.service.ChangePlan(t.Context(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
			if !errors.Is(err, ErrTrialMutationNotApplicable) {
				t.Fatalf("error=%v, want ErrTrialMutationNotApplicable", err)
			}
			if h.stripe.changeScheduleCalls != 0 {
				t.Fatalf("schedule mutations=%d, want zero", h.stripe.changeScheduleCalls)
			}
		})
	}
}

func TestChangePlanPaidTrialRejectsInconsistentMutationResponse(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ScheduleSnapshot)
	}{
		{name: "mode", mutate: func(s *ScheduleSnapshot) { s.StripeMode = "sandbox" }},
		{name: "customer", mutate: func(s *ScheduleSnapshot) { s.CustomerID = "cus_other" }},
		{name: "subscription", mutate: func(s *ScheduleSnapshot) { s.SubscriptionID = "sub_other" }},
		{name: "end behavior", mutate: func(s *ScheduleSnapshot) { s.EndBehavior = "cancel" }},
		{name: "multiple phases", mutate: func(s *ScheduleSnapshot) { s.Phases = append(s.Phases, s.Phases[0]) }},
		{name: "target price", mutate: func(s *ScheduleSnapshot) { s.Phases[0].PriceID = "price_other" }},
		{name: "trial retained", mutate: func(s *ScheduleSnapshot) { s.Phases[0].TrialEndAt = s.Phases[0].StartAt.Add(time.Hour) }},
		{name: "anchor", mutate: func(s *ScheduleSnapshot) { s.Phases[0].BillingCycleAnchor = "automatic" }},
		{name: "phase metadata", mutate: func(s *ScheduleSnapshot) { s.Phases[0].Metadata[metadataPlanID] = "team" }},
		{name: "schedule metadata", mutate: func(s *ScheduleSnapshot) { s.Metadata[metadataEnvironment] = "production" }},
		{name: "start", mutate: func(s *ScheduleSnapshot) { s.Phases[0].StartAt = s.Phases[0].StartAt.Add(-time.Hour) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newPaidPlanChangeHarness(t, StatusActive)
			tc.mutate(&h.stripe.changeScheduleResult)
			_, err := h.service.ChangePlan(t.Context(), ChangePlanRequest{WorkspaceID: "ws_1", TargetPlanID: "growth"})
			if !errors.Is(err, ErrTrialMutationNotApplicable) {
				t.Fatalf("error=%v, want inconsistent response", err)
			}
			if h.stripe.changeScheduleCalls != 1 {
				t.Fatalf("schedule mutations=%d, want one", h.stripe.changeScheduleCalls)
			}
		})
	}
}

func newPaidPlanChangeHarness(t *testing.T, status Status) *serviceHarness {
	t.Helper()
	h := newServiceHarness(t)
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	paidStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	trialStart := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	trialEnd := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	metadata := trialMetadata("ws_1", "basic", "grant_1", KindPaidSamePlan, "staging")
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, Status: status, PlanID: "basic", StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &trialStart, EndsAt: &trialEnd}
	h.store.open = &h.store.grant
	h.store.billing = BillingSnapshot{WorkspaceID: "ws_1", OwnerUserID: "owner_1", Subscription: SubscriptionRecord{PlanID: "basic", Status: "active", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"}}
	h.modes.mode = BillingMode{Name: "live", PriceID: "price_growth"}
	h.modes.prices = map[string]string{"basic": "price_basic", "growth": "price_growth"}
	h.stripe.subscription = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "active", CustomerID: "cus_1", ScheduleID: "sched_1", PriceID: "price_basic", CurrentPeriodStartAt: &paidStart, CurrentPeriodEndAt: &trialStart, Metadata: cloneMetadata(metadata)}
	h.stripe.retrievedSchedule = ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "active", EndBehavior: "release", CustomerID: "cus_1", SubscriptionID: "sub_1", CurrentPhaseStartAt: &paidStart, CurrentPhaseEndAt: &trialStart, Metadata: cloneMetadata(metadata), Phases: []SchedulePhase{
		{PriceID: "price_basic", StartAt: paidStart, EndAt: trialStart, Metadata: cloneMetadata(metadata)},
		{PriceID: "price_basic", StartAt: trialStart, EndAt: trialEnd, TrialEndAt: trialEnd, Metadata: cloneMetadata(metadata)},
		{PriceID: "price_basic", StartAt: trialEnd, EndAt: trialEnd.AddDate(0, 1, 0), Metadata: cloneMetadata(metadata)},
	}}
	if status == StatusActive {
		h.store.grant.StartedAt = &trialStart
		h.store.open = &h.store.grant
		h.stripe.subscription.Status = "trialing"
		h.stripe.subscription.TrialStartAt, h.stripe.subscription.TrialEndAt = &trialStart, &trialEnd
		h.stripe.subscription.CurrentPeriodStartAt, h.stripe.subscription.CurrentPeriodEndAt = &trialStart, &trialEnd
		h.stripe.retrievedSchedule.CurrentPhaseStartAt, h.stripe.retrievedSchedule.CurrentPhaseEndAt = &trialStart, &trialEnd
	}
	targetMetadata := trialMetadata("ws_1", "growth", "grant_1", KindPaidSamePlan, "staging")
	h.stripe.changeScheduleResult = ScheduleSnapshot{StripeMode: "live", ID: "sched_1", Status: "active", SubscriptionID: "sub_1", CustomerID: "cus_1", EndBehavior: "release", Metadata: cloneMetadata(targetMetadata), Phases: []SchedulePhase{{PriceID: "price_growth", StartAt: now, BillingCycleAnchor: "phase_start", Metadata: cloneMetadata(targetMetadata)}}}
	return h
}

func TestCancelRenewalDispatchesByKindAndKeepsGrantOpen(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	for _, tc := range []struct {
		name                   string
		kind                   Kind
		status                 Status
		wantFree, wantSchedule int
	}{
		{name: "free active", kind: KindFreeToPaid, status: StatusActive, wantFree: 1},
		{name: "paid scheduled", kind: KindPaidSamePlan, status: StatusScheduled, wantSchedule: 1},
		{name: "paid active", kind: KindPaidSamePlan, status: StatusActive, wantSchedule: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: tc.kind, Status: tc.status, PlanID: "basic", StripeMode: "live", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1", StripeScheduleID: "sched_1", ScheduledStartAt: &start, StartedAt: &start, EndsAt: &end}
			h.store.open = &h.store.grant
			h.store.billing = BillingSnapshot{WorkspaceID: "ws_1", OwnerUserID: "owner_1", Subscription: SubscriptionRecord{PlanID: "basic", Status: "trialing", StripeCustomerID: "cus_1", StripeSubscriptionID: "sub_1"}}
			h.modes.mode = BillingMode{Name: "live", PriceID: "price_basic"}
			h.stripe.subscription = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", ItemID: "si_1", PriceID: "price_basic", CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end, TrialStartAt: &start, TrialEndAt: &end, ScheduleID: "sched_1", Metadata: trialMetadata("ws_1", "basic", "grant_1", tc.kind, "staging")}
			if tc.status == StatusScheduled {
				periodStart := start.Add(-30 * 24 * time.Hour)
				h.stripe.subscription.Status = "active"
				h.stripe.subscription.CurrentPeriodStartAt = &periodStart
				h.stripe.subscription.CurrentPeriodEndAt = &start
				h.stripe.subscription.TrialStartAt = nil
				h.stripe.subscription.TrialEndAt = nil
			}
			h.stripe.cancelFreeResult = SubscriptionSnapshot{StripeMode: "live", ID: "sub_1", Status: "trialing", CustomerID: "cus_1", PriceID: "price_basic", TrialStartAt: &start, TrialEndAt: &end, CancelAtPeriodEnd: true}
			h.stripe.cancelScheduleResult = ScheduleSnapshot{StripeMode: "live", ID: "sched_1", SubscriptionID: "sub_1", CustomerID: "cus_1", EndBehavior: "cancel"}

			got, err := h.service.CancelRenewal(t.Context(), CancelRequest{WorkspaceID: "ws_1", GrantID: "grant_1"})
			if err != nil {
				t.Fatal(err)
			}
			if h.stripe.cancelFreeCalls != tc.wantFree || h.stripe.cancelScheduleCalls != tc.wantSchedule {
				t.Fatalf("free=%d schedule=%d", h.stripe.cancelFreeCalls, h.stripe.cancelScheduleCalls)
			}
			if got.Status != tc.status || got.CanceledAt == nil {
				t.Fatalf("grant=%#v", got)
			}
		})
	}
}

func TestCancelRenewalRepeatIsIdempotentWithoutAnotherStripeMutation(t *testing.T) {
	h := newServiceHarness(t)
	canceled := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	h.store.grant = Grant{ID: "grant_1", WorkspaceID: "ws_1", Kind: KindPaidSamePlan, Status: StatusScheduled, PlanID: "basic", StripeMode: "live", StripeScheduleID: "sched_1", CanceledAt: &canceled}
	h.store.open = &h.store.grant
	got, err := h.service.CancelRenewal(t.Context(), CancelRequest{WorkspaceID: "ws_1", GrantID: "grant_1"})
	if err != nil || got.CanceledAt == nil {
		t.Fatalf("grant=%#v error=%v", got, err)
	}
	if h.stripe.cancelFreeCalls != 0 || h.stripe.cancelScheduleCalls != 0 {
		t.Fatalf("free=%d schedule=%d", h.stripe.cancelFreeCalls, h.stripe.cancelScheduleCalls)
	}
}

func TestCreateTrialSafePortalRequiresExactGrantModeConfiguration(t *testing.T) {
	for _, status := range []Status{StatusScheduled, StatusActive} {
		t.Run(string(status), func(t *testing.T) {
			h := newServiceHarness(t)
			h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: status, StripeMode: "sandbox", StripeCustomerID: "cus_1"}
			h.store.grant = *h.store.open
			h.stripe.portalURL = "https://billing.stripe.test/session"
			url, handled, err := h.service.CreateTrialSafePortal(t.Context(), PortalRequest{WorkspaceID: "ws_1", StripeMode: "sandbox", CustomerID: "cus_1", ReturnURL: "https://app.example/settings/billing"})
			if err != nil || !handled || url == "" {
				t.Fatalf("url=%q handled=%v error=%v", url, handled, err)
			}
			if h.stripe.portalCalls != 1 || !h.stripe.lastPortal.RequireTrialSafeConfiguration || h.stripe.lastPortal.StripeMode != "sandbox" {
				t.Fatalf("portal=%#v calls=%d", h.stripe.lastPortal, h.stripe.portalCalls)
			}
		})
	}
}

func TestCreateTrialSafePortalLeavesOrdinaryPortalUnchangedAndFailsClosedForModeMismatch(t *testing.T) {
	h := newServiceHarness(t)
	h.store.open = nil
	if _, handled, err := h.service.CreateTrialSafePortal(t.Context(), PortalRequest{WorkspaceID: "ws_1", StripeMode: "live", CustomerID: "cus_1", ReturnURL: "https://app.example/settings/billing"}); err != nil || handled || h.stripe.portalCalls != 0 {
		t.Fatalf("handled=%v error=%v calls=%d", handled, err, h.stripe.portalCalls)
	}

	h.store.open = &Grant{ID: "grant_1", WorkspaceID: "ws_1", Status: StatusActive, StripeMode: "sandbox", StripeCustomerID: "cus_1"}
	h.store.grant = *h.store.open
	if _, handled, err := h.service.CreateTrialSafePortal(t.Context(), PortalRequest{WorkspaceID: "ws_1", StripeMode: "live", CustomerID: "cus_1", ReturnURL: "https://app.example/settings/billing"}); err == nil || !handled || h.stripe.portalCalls != 0 {
		t.Fatalf("handled=%v error=%v calls=%d", handled, err, h.stripe.portalCalls)
	}
}

func TestValidateGrantInputRejectsUnsupportedPlans(t *testing.T) {
	for _, planID := range []string{"", "free", "enterprise", "Growth"} {
		err := ValidateGrantInput(planID, 30)
		if !errors.Is(err, ErrInvalidPlan) {
			t.Fatalf("ValidateGrantInput(%q, 30) = %v, want ErrInvalidPlan", planID, err)
		}
	}
}

func TestValidateGrantInputRejectsDurationsOutsideBoundaries(t *testing.T) {
	for _, days := range []int32{-1, 0, 731} {
		err := ValidateGrantInput("growth", days)
		if !errors.Is(err, ErrInvalidDuration) {
			t.Fatalf("ValidateGrantInput(growth, %d) = %v, want ErrInvalidDuration", days, err)
		}
	}
}

func TestCanTransitionAllowsEveryDocumentedTransition(t *testing.T) {
	allowed := map[Status][]Status{
		StatusProvisioning:      {StatusScheduled, StatusFailed},
		StatusPendingActivation: {StatusCheckoutPending, StatusRevoked, StatusSuperseded},
		StatusCheckoutPending:   {StatusPendingActivation, StatusActive, StatusRevoked, StatusSuperseded},
		StatusScheduled:         {StatusActive, StatusCanceled, StatusSuperseded},
		StatusActive:            {StatusCompleted, StatusCanceled, StatusSuperseded},
	}

	for from, destinations := range allowed {
		for _, to := range destinations {
			if !CanTransition(from, to) {
				t.Errorf("CanTransition(%q, %q) = false, want true", from, to)
			}
			result, err := ValidateTransition(from, to)
			if err != nil || result != TransitionApplied {
				t.Errorf("ValidateTransition(%q, %q) = (%q, %v), want (%q, nil)", from, to, result, err, TransitionApplied)
			}
		}
	}
}

func TestTransitionMatrixCoversEveryStatusPair(t *testing.T) {
	statuses := []Status{
		StatusProvisioning,
		StatusPendingActivation,
		StatusCheckoutPending,
		StatusScheduled,
		StatusActive,
		StatusCompleted,
		StatusCanceled,
		StatusRevoked,
		StatusSuperseded,
		StatusFailed,
	}
	approvedEdges := map[[2]Status]struct{}{
		{StatusProvisioning, StatusScheduled}:            {},
		{StatusProvisioning, StatusFailed}:               {},
		{StatusPendingActivation, StatusCheckoutPending}: {},
		{StatusPendingActivation, StatusRevoked}:         {},
		{StatusPendingActivation, StatusSuperseded}:      {},
		{StatusCheckoutPending, StatusPendingActivation}: {},
		{StatusCheckoutPending, StatusActive}:            {},
		{StatusCheckoutPending, StatusRevoked}:           {},
		{StatusCheckoutPending, StatusSuperseded}:        {},
		{StatusScheduled, StatusActive}:                  {},
		{StatusScheduled, StatusCanceled}:                {},
		{StatusScheduled, StatusSuperseded}:              {},
		{StatusActive, StatusCompleted}:                  {},
		{StatusActive, StatusCanceled}:                   {},
		{StatusActive, StatusSuperseded}:                 {},
	}

	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(string(from)+"_to_"+string(to), func(t *testing.T) {
				result, err := ValidateTransition(from, to)
				canTransition := CanTransition(from, to)

				if from == to {
					if result != TransitionIdempotent || err != nil {
						t.Fatalf("ValidateTransition(%q, %q) = (%q, %v), want (%q, nil)", from, to, result, err, TransitionIdempotent)
					}
					if canTransition {
						t.Fatalf("CanTransition(%q, %q) = true, want false for idempotent projection", from, to)
					}
					return
				}

				if _, approved := approvedEdges[[2]Status{from, to}]; approved {
					if result != TransitionApplied || err != nil {
						t.Fatalf("ValidateTransition(%q, %q) = (%q, %v), want (%q, nil)", from, to, result, err, TransitionApplied)
					}
					if !canTransition {
						t.Fatalf("CanTransition(%q, %q) = false, want true", from, to)
					}
					return
				}

				if result != TransitionInvalid || !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("ValidateTransition(%q, %q) = (%q, %v), want (%q, ErrInvalidTransition)", from, to, result, err, TransitionInvalid)
				}
				if canTransition {
					t.Fatalf("CanTransition(%q, %q) = true, want false", from, to)
				}
			})
		}
	}
}

func TestCanTransitionRejectsTerminalAndBackwardTransitions(t *testing.T) {
	allStatuses := []Status{
		StatusProvisioning,
		StatusPendingActivation,
		StatusCheckoutPending,
		StatusScheduled,
		StatusActive,
		StatusCompleted,
		StatusCanceled,
		StatusRevoked,
		StatusSuperseded,
		StatusFailed,
	}
	for _, terminal := range []Status{StatusCompleted, StatusCanceled, StatusRevoked, StatusSuperseded, StatusFailed} {
		for _, to := range allStatuses {
			if terminal != to && CanTransition(terminal, to) {
				t.Errorf("terminal transition %q -> %q was allowed", terminal, to)
			}
		}
	}

	for _, pair := range [][2]Status{
		{StatusPendingActivation, StatusProvisioning},
		{StatusScheduled, StatusProvisioning},
		{StatusScheduled, StatusPendingActivation},
		{StatusActive, StatusScheduled},
		{StatusActive, StatusCheckoutPending},
		{StatusRevoked, StatusPendingActivation},
	} {
		if CanTransition(pair[0], pair[1]) {
			t.Errorf("backward transition %q -> %q was allowed", pair[0], pair[1])
		}
		result, err := ValidateTransition(pair[0], pair[1])
		if result != TransitionInvalid || !errors.Is(err, ErrInvalidTransition) {
			t.Errorf("ValidateTransition(%q, %q) = (%q, %v), want (%q, ErrInvalidTransition)", pair[0], pair[1], result, err, TransitionInvalid)
		}
	}
}

func TestCheckoutExpiryIsTheOnlyReopenTransition(t *testing.T) {
	if !CanTransition(StatusCheckoutPending, StatusPendingActivation) {
		t.Fatal("checkout expiry must reopen the offer")
	}
	if CanTransition(StatusRevoked, StatusPendingActivation) {
		t.Fatal("a revoked offer must not reopen")
	}
}

func TestValidateTransitionTreatsKnownSameStateAsIdempotent(t *testing.T) {
	for _, status := range []Status{StatusProvisioning, StatusActive, StatusCompleted, StatusRevoked} {
		result, err := ValidateTransition(status, status)
		if err != nil || result != TransitionIdempotent {
			t.Errorf("ValidateTransition(%q, %q) = (%q, %v), want (%q, nil)", status, status, result, err, TransitionIdempotent)
		}
		if CanTransition(status, status) {
			t.Errorf("CanTransition(%q, %q) = true; idempotency must not be a forward transition", status, status)
		}
	}

	result, err := ValidateTransition(Status("unknown"), Status("unknown"))
	if result != TransitionInvalid || !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("unknown same-state result = (%q, %v), want invalid", result, err)
	}
}

func TestStatusClassification(t *testing.T) {
	for _, status := range []Status{StatusProvisioning, StatusPendingActivation, StatusCheckoutPending, StatusScheduled, StatusActive} {
		if !status.IsOpen() || status.IsTerminal() {
			t.Errorf("status %q classification: open=%t terminal=%t", status, status.IsOpen(), status.IsTerminal())
		}
	}
	for _, status := range []Status{StatusCompleted, StatusCanceled, StatusRevoked, StatusSuperseded, StatusFailed} {
		if status.IsOpen() || !status.IsTerminal() {
			t.Errorf("status %q classification: open=%t terminal=%t", status, status.IsOpen(), status.IsTerminal())
		}
	}
	if Status("unknown").IsOpen() || Status("unknown").IsTerminal() {
		t.Fatal("unknown status must be neither open nor terminal")
	}
}

func TestTrialProjectionIsUserSafeAndIncludesSuppliedBillingFlags(t *testing.T) {
	grant := populatedGrant(StatusActive)
	projection := NewTrialProjection(grant, 2900, true)

	if projection.ID != grant.ID || projection.Kind != KindFreeToPaid || projection.PlanID != grant.PlanID || projection.DurationDays != grant.DurationDays || projection.Status != StatusActive {
		t.Fatalf("identity projection = %#v", projection)
	}
	if projection.PostTrialPriceCents != 2900 {
		t.Fatalf("post-trial price = %d", projection.PostTrialPriceCents)
	}
	if !projection.CancelAtPeriodEnd || !projection.ChangingPlanForfeitsTrial {
		t.Fatalf("billing flags = cancel:%t forfeits:%t", projection.CancelAtPeriodEnd, projection.ChangingPlanForfeitsTrial)
	}
	if projection.GrantedAt == nil || projection.ScheduledStartAt == nil || projection.StartedAt == nil || projection.EndsAt == nil || projection.ActivatedAt == nil {
		t.Fatalf("user-facing dates missing: %#v", projection)
	}
	assertProjectionRedactsInternalFields(t, projection)
}

func TestTrialProjectionRequiresPriceAndKeepsExactJSONContract(t *testing.T) {
	grant := db.WorkspaceTrialGrant{
		ID:           "trial_zero",
		Kind:         string(KindFreeToPaid),
		PlanID:       "api",
		DurationDays: 1,
		Status:       string(StatusPendingActivation),
		GrantedAt: pgtype.Timestamptz{
			Time:  time.Date(2026, 7, 24, 14, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60)),
			Valid: true,
		},
	}

	encoded, err := json.Marshal(NewTrialProjection(grant, 0, false))
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal projection: %v", err)
	}
	assertExactJSONKeys(t, payload,
		"id", "kind", "plan_id", "duration_days", "status", "granted_at",
		"post_trial_price_cents", "cancel_at_period_end", "changing_plan_forfeits_trial",
	)
	if got, ok := payload["post_trial_price_cents"].(float64); !ok || got != 0 {
		t.Fatalf("post_trial_price_cents = %#v, want numeric zero", payload["post_trial_price_cents"])
	}
	if got := payload["granted_at"]; got != "2026-07-24T12:30:00Z" {
		t.Fatalf("granted_at = %#v, want UTC timestamp", got)
	}
}

func TestHistoryProjectionIsUserSafeAndNormalizesTerminalReasons(t *testing.T) {
	tests := []struct {
		status Status
		want   TerminalReason
	}{
		{StatusCompleted, TerminalReasonCompleted},
		{StatusCanceled, TerminalReasonRenewalCanceled},
		{StatusRevoked, TerminalReasonOfferRevoked},
		{StatusSuperseded, TerminalReasonPlanChanged},
		{StatusFailed, TerminalReasonUnavailable},
		{StatusActive, TerminalReasonNone},
	}

	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			grant := populatedGrant(test.status)
			grant.FailureCode = pgtype.Text{String: "secret_" + string(test.status), Valid: true}
			grant.FailureMessage = pgtype.Text{String: "internal detail " + string(test.status), Valid: true}
			projection := NewHistoryProjection(grant)

			if projection.TerminalReason != test.want {
				t.Fatalf("terminal reason = %q, want %q", projection.TerminalReason, test.want)
			}
			assertProjectionRedactsInternalFields(t, projection)
		})
	}
}

func TestHistoryProjectionHasExactJSONContract(t *testing.T) {
	projection := NewHistoryProjection(populatedGrant(StatusSuperseded))
	encoded, err := json.Marshal(projection)
	if err != nil {
		t.Fatalf("marshal history projection: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal history projection: %v", err)
	}
	assertExactJSONKeys(t, payload,
		"id", "kind", "plan_id", "duration_days", "status",
		"granted_at", "scheduled_start_at", "started_at", "ends_at", "activated_at",
		"canceled_at", "revoked_at", "superseded_at", "completed_at",
		"superseded_by_plan_id", "terminal_reason",
	)
	if got := payload["superseded_by_plan_id"]; got != "team" {
		t.Fatalf("superseded_by_plan_id = %#v, want team", got)
	}
}

func TestHistoryProjectionOmitsInvalidNullableValuesAndConvertsDatesToUTC(t *testing.T) {
	grant := db.WorkspaceTrialGrant{
		ID:           "trial_nullable",
		Kind:         string(KindPaidSamePlan),
		PlanID:       "basic",
		DurationDays: 30,
		Status:       string(StatusCompleted),
		StartedAt: pgtype.Timestamptz{
			Time:  time.Date(2026, 8, 2, 4, 0, 0, 0, time.FixedZone("UTC-7", -7*60*60)),
			Valid: true,
		},
	}

	encoded, err := json.Marshal(NewHistoryProjection(grant))
	if err != nil {
		t.Fatalf("marshal history projection: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal history projection: %v", err)
	}
	assertExactJSONKeys(t, payload,
		"id", "kind", "plan_id", "duration_days", "status", "started_at", "terminal_reason",
	)
	if got := payload["started_at"]; got != "2026-08-02T11:00:00Z" {
		t.Fatalf("started_at = %#v, want UTC timestamp", got)
	}
}

func populatedGrant(status Status) db.WorkspaceTrialGrant {
	stamp := func(day int) pgtype.Timestamptz {
		return pgtype.Timestamptz{Time: time.Date(2026, 7, day, 12, 0, 0, 0, time.FixedZone("test", 2*60*60)), Valid: true}
	}
	return db.WorkspaceTrialGrant{
		ID:                      "trial_1",
		WorkspaceID:             "ws_1",
		Kind:                    string(KindFreeToPaid),
		PlanID:                  "growth",
		DurationDays:            30,
		Status:                  string(status),
		GrantedByUserID:         "admin_secret",
		StripeMode:              pgtype.Text{String: "live_secret", Valid: true},
		StripeCustomerID:        pgtype.Text{String: "cus_secret", Valid: true},
		StripeSubscriptionID:    pgtype.Text{String: "sub_secret", Valid: true},
		StripeScheduleID:        pgtype.Text{String: "sched_secret", Valid: true},
		StripeCheckoutSessionID: pgtype.Text{String: "cs_secret", Valid: true},
		GrantedAt:               stamp(1),
		ScheduledStartAt:        stamp(2),
		StartedAt:               stamp(3),
		EndsAt:                  stamp(30),
		ActivatedAt:             stamp(4),
		CanceledAt:              stamp(5),
		RevokedAt:               stamp(6),
		SupersededAt:            stamp(7),
		CompletedAt:             stamp(8),
		SupersededByPlanID:      pgtype.Text{String: "team", Valid: true},
		FailureCode:             pgtype.Text{String: "secret_failure_code", Valid: true},
		FailureMessage:          pgtype.Text{String: "secret failure message", Valid: true},
		CreatedAt:               stamp(1),
		UpdatedAt:               stamp(9),
	}
}

func assertProjectionRedactsInternalFields(t *testing.T, value any) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	jsonText := string(encoded)
	for _, forbidden := range []string{
		"granted_by_user_id",
		"failure_code",
		"failure_message",
		"stripe_mode",
		"stripe_customer_id",
		"stripe_subscription_id",
		"stripe_schedule_id",
		"stripe_checkout_session_id",
		"admin_secret",
		"secret_failure_code",
		"secret failure message",
		"live_secret",
		"cus_secret",
		"sub_secret",
		"sched_secret",
		"cs_secret",
	} {
		if strings.Contains(jsonText, forbidden) {
			t.Errorf("projection leaked %q: %s", forbidden, jsonText)
		}
	}
}

func assertExactJSONKeys(t *testing.T, payload map[string]any, expected ...string) {
	t.Helper()
	want := make(map[string]struct{}, len(expected))
	for _, key := range expected {
		want[key] = struct{}{}
	}
	for key := range payload {
		if _, ok := want[key]; !ok {
			t.Errorf("unexpected JSON key %q", key)
		}
	}
	for key := range want {
		if _, ok := payload[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
}
