package trials

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
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
		StatusCheckoutPending:   {StatusPendingActivation, StatusActive, StatusRevoked, StatusSuperseded, StatusFailed},
		StatusScheduled:         {StatusActive, StatusCanceled, StatusSuperseded, StatusFailed},
		StatusActive:            {StatusCompleted, StatusCanceled, StatusSuperseded, StatusFailed},
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
		{StatusCheckoutPending, StatusFailed}:            {},
		{StatusScheduled, StatusActive}:                  {},
		{StatusScheduled, StatusCanceled}:                {},
		{StatusScheduled, StatusSuperseded}:              {},
		{StatusScheduled, StatusFailed}:                  {},
		{StatusActive, StatusCompleted}:                  {},
		{StatusActive, StatusCanceled}:                   {},
		{StatusActive, StatusSuperseded}:                 {},
		{StatusActive, StatusFailed}:                     {},
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
	price := int64(2900)

	projection := NewTrialProjection(grant, ProjectionOptions{
		PostTrialPriceCents: &price,
		CancelAtPeriodEnd:   true,
	})

	if projection.ID != grant.ID || projection.Kind != KindFreeToPaid || projection.PlanID != grant.PlanID || projection.DurationDays != grant.DurationDays || projection.Status != StatusActive {
		t.Fatalf("identity projection = %#v", projection)
	}
	if projection.PostTrialPriceCents == nil || *projection.PostTrialPriceCents != price {
		t.Fatalf("post-trial price = %#v", projection.PostTrialPriceCents)
	}
	if !projection.CancelAtPeriodEnd || !projection.ChangingPlanForfeitsTrial {
		t.Fatalf("billing flags = cancel:%t forfeits:%t", projection.CancelAtPeriodEnd, projection.ChangingPlanForfeitsTrial)
	}
	if projection.GrantedAt == nil || projection.ScheduledStartAt == nil || projection.StartedAt == nil || projection.EndsAt == nil || projection.ActivatedAt == nil {
		t.Fatalf("user-facing dates missing: %#v", projection)
	}
	assertProjectionRedactsInternalFields(t, projection)
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
			price := int64(4900)
			projection := NewHistoryProjection(grant, ProjectionOptions{PostTrialPriceCents: &price})

			if projection.TerminalReason != test.want {
				t.Fatalf("terminal reason = %q, want %q", projection.TerminalReason, test.want)
			}
			if projection.PostTrialPriceCents == nil || *projection.PostTrialPriceCents != price {
				t.Fatalf("post-trial price = %#v", projection.PostTrialPriceCents)
			}
			assertProjectionRedactsInternalFields(t, projection)
		})
	}
}

func TestNewServiceReturnsServiceSkeleton(t *testing.T) {
	store := &fakeStore{}
	stripe := &fakeStripeGateway{}
	service := NewService(store, stripe)
	if service == nil {
		t.Fatal("NewService returned nil")
	}
}

type fakeStore struct{}

type fakeStripeGateway struct{}

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
