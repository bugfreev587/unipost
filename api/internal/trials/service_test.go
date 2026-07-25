package trials

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	billing                     BillingSnapshot
	open                        *Grant
	grant                       Grant
	created                     Grant
	failed                      *FailureUpdate
	createCalls                 int
	revokeCalls                 int
	revokeErr                   error
	revokeErrs                  []error
	afterFirstRevoke            func()
	statusAtScheduleCall        Status
	expireSucceededBeforeRevoke bool
	provisioningSchedule        *ProvisioningScheduleUpdate
	markScheduledErr            error
	afterMarkScheduled          func()
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

type fakeModeResolver struct {
	mode                BillingMode
	ownerUserID, planID string
	err                 error
}

func (r *fakeModeResolver) Resolve(_ context.Context, ownerUserID, planID string) (BillingMode, error) {
	r.ownerUserID, r.planID = ownerUserID, planID
	return r.mode, r.err
}

type fakeServiceStripe struct {
	store                                                                               *fakeGrantStore
	subscription                                                                        SubscriptionSnapshot
	schedule                                                                            ScheduleSnapshot
	expiredCheckout                                                                     CheckoutSnapshot
	createScheduleErr, expireErr                                                        error
	createScheduleCalls, configureScheduleCalls, retrieveSubscriptionCalls, expireCalls int
	lastSchedule                                                                        CreatePaidTrialScheduleRequest
	lastExpire                                                                          ExpireCheckoutRequest
	lastConfiguredScheduleID                                                            string
	retrievedCheckout                                                                   CheckoutSnapshot
	retrieveCheckoutErr                                                                 error
	retrieveCheckoutCalls                                                               int
	lastRetrieveCheckoutID                                                              string
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
func (s *fakeServiceStripe) CreateTrialCheckout(context.Context, CreateTrialCheckoutRequest) (CheckoutSnapshot, error) {
	return CheckoutSnapshot{}, fmt.Errorf("unexpected CreateTrialCheckout")
}
func (s *fakeServiceStripe) RetrieveCheckout(_ context.Context, _ string, id string) (CheckoutSnapshot, error) {
	s.retrieveCheckoutCalls++
	s.lastRetrieveCheckoutID = id
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
func (s *fakeServiceStripe) ChangeFreeTrialPlanNow(context.Context, ChangeFreeTrialPlanRequest) (SubscriptionSnapshot, error) {
	return SubscriptionSnapshot{}, fmt.Errorf("unexpected ChangeFreeTrialPlanNow")
}
func (s *fakeServiceStripe) ChangeScheduledTrialPlanNow(context.Context, ChangeScheduledTrialPlanRequest) (ScheduleSnapshot, error) {
	return ScheduleSnapshot{}, fmt.Errorf("unexpected ChangeScheduledTrialPlanNow")
}
func (s *fakeServiceStripe) CancelFreeTrialAtEnd(context.Context, CancelFreeTrialRequest) (SubscriptionSnapshot, error) {
	return SubscriptionSnapshot{}, fmt.Errorf("unexpected CancelFreeTrialAtEnd")
}
func (s *fakeServiceStripe) CancelPaidScheduleAtTrialEnd(context.Context, CancelPaidScheduleRequest) (ScheduleSnapshot, error) {
	return ScheduleSnapshot{}, fmt.Errorf("unexpected CancelPaidScheduleAtTrialEnd")
}
func (s *fakeServiceStripe) CreatePortal(context.Context, CreatePortalRequest) (string, error) {
	return "", fmt.Errorf("unexpected CreatePortal")
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
