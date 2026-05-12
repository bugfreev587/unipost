package handler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestIsTrialCancellation(t *testing.T) {
	if !isTrialCancellation(stripe.Subscription{
		Status:            stripe.SubscriptionStatusTrialing,
		CancelAtPeriodEnd: true,
	}) {
		t.Fatal("trialing subscription canceled at period end should stop immediately")
	}

	cases := []stripe.Subscription{
		{Status: stripe.SubscriptionStatusActive, CancelAtPeriodEnd: true},
		{Status: stripe.SubscriptionStatusTrialing, CancelAtPeriodEnd: false},
	}
	for _, sub := range cases {
		if isTrialCancellation(sub) {
			t.Fatalf("unexpected trial cancellation match for status=%s cancel_at_period_end=%v", sub.Status, sub.CancelAtPeriodEnd)
		}
	}
}

func TestStripeUnixToTimestamptz(t *testing.T) {
	const unixTS int64 = 1_778_572_800
	got := stripeUnixToTimestamptz(unixTS)
	if !got.Valid {
		t.Fatal("expected valid timestamp")
	}
	if got.Time.UTC().Unix() != unixTS {
		t.Fatalf("unexpected unix round-trip: got %d want %d", got.Time.UTC().Unix(), unixTS)
	}
	if stripeUnixToTimestamptz(0).Valid {
		t.Fatal("zero unix timestamp should stay invalid")
	}
}

func TestShouldKeepCurrentPlanForScheduledDowngrade(t *testing.T) {
	currentPeriodEnd := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	local := db.Subscription{
		PlanID:            "team",
		CurrentPeriodEnd:  pgtype.Timestamptz{Time: currentPeriodEnd, Valid: true},
		CurrentPeriodStart: pgtype.Timestamptz{Time: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Valid: true},
	}

	if !shouldKeepCurrentPlanForScheduledDowngrade(14900, 5900, local, time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("mid-cycle downgrade should keep the current plan until period end")
	}
	if shouldKeepCurrentPlanForScheduledDowngrade(14900, 5900, local, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("new cycle downgrade should apply once the previous period has ended")
	}
	if shouldKeepCurrentPlanForScheduledDowngrade(5900, 14900, db.Subscription{PlanID: "growth", CurrentPeriodEnd: pgtype.Timestamptz{Time: currentPeriodEnd, Valid: true}}, time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("upgrades should not be delayed")
	}
}
