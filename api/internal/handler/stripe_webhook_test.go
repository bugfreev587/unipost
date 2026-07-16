package handler

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stripe/stripe-go/v82"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
)

func TestSubscriptionPlanChangeUsesAtomicHoldMutation(t *testing.T) {
	source, err := os.ReadFile("stripe_webhook.go")
	if err != nil {
		t.Fatalf("read stripe webhook: %v", err)
	}
	body := string(source)
	for _, want := range []string{"ApplyPlanChange(", "queries.UpdateSubscriptionStripe"} {
		if !strings.Contains(body, want) {
			t.Fatalf("plan changes must atomically reconcile holds and persist subscriptions; missing %q", want)
		}
	}
}

func TestStripeEventEffectiveAtUsesEventCreationTime(t *testing.T) {
	want := time.Date(2026, 7, 16, 20, 0, 0, 0, time.UTC)
	got := stripeEventEffectiveAt(stripe.Event{Created: want.Unix()})
	if !got.Equal(want) {
		t.Fatalf("effective time = %s, want %s", got, want)
	}
}

func TestShouldSendBillingPaymentRecovered(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "past due recovers", status: "past_due", want: true},
		{name: "active replay does not resend", status: "active", want: false},
		{name: "empty status does not send", status: "", want: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldSendBillingPaymentRecovered(tc.status); got != tc.want {
				t.Fatalf("shouldSendBillingPaymentRecovered(%q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

func TestReconcileQuotaHoldsForPlanChangeUsesDowngradeEffectiveTime(t *testing.T) {
	effectiveAt := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	reconciler := &recordingHoldReconciler{}
	h := &StripeWebhookHandler{holdReconciler: reconciler}

	err := h.reconcileQuotaHoldsForPlanChange(
		context.Background(),
		"ws_123",
		db.Plan{ID: "growth", PriceCents: 7900, PostLimit: 7500},
		db.Plan{ID: "basic", PriceCents: 1900, PostLimit: 2500},
		effectiveAt,
	)
	if err != nil {
		t.Fatalf("reconcile downgrade: %v", err)
	}
	if reconciler.calls != 1 || reconciler.reason != "plan_downgrade" || !reconciler.effectiveAt.Equal(effectiveAt) {
		t.Fatalf("reconciler = %#v", reconciler)
	}
	if reconciler.planID != "basic" || reconciler.limit != 2500 {
		t.Fatalf("target plan = %s/%d", reconciler.planID, reconciler.limit)
	}
}

func TestReconcileQuotaHoldsForPlanChangeReleasesOnUpgrade(t *testing.T) {
	reconciler := &recordingHoldReconciler{}
	h := &StripeWebhookHandler{holdReconciler: reconciler}

	err := h.reconcileQuotaHoldsForPlanChange(
		context.Background(),
		"ws_123",
		db.Plan{ID: "basic", PriceCents: 1900, PostLimit: 2500},
		db.Plan{ID: "growth", PriceCents: 7900, PostLimit: 7500},
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("reconcile upgrade: %v", err)
	}
	if reconciler.calls != 1 || reconciler.reason != "plan_upgrade" || !reconciler.effectiveAt.IsZero() {
		t.Fatalf("reconciler = %#v", reconciler)
	}
	if reconciler.planID != "growth" || reconciler.limit != 7500 {
		t.Fatalf("target plan = %s/%d", reconciler.planID, reconciler.limit)
	}
}

func TestEnterpriseToFinitePlanIsCapacityDowngradeDespitePrice(t *testing.T) {
	if !isPlanCapacityDowngrade(
		db.Plan{ID: "enterprise", PriceCents: 0, PostLimit: -1},
		db.Plan{ID: "basic", PriceCents: 1900, PostLimit: 2500},
	) {
		t.Fatal("enterprise to basic must use downgrade grandfathering")
	}
	if isPlanCapacityDowngrade(
		db.Plan{ID: "growth", PriceCents: 5900, PostLimit: 7500},
		db.Plan{ID: "enterprise", PriceCents: 0, PostLimit: -1},
	) {
		t.Fatal("growth to enterprise must release holds as an unlimited upgrade")
	}
}

type recordingHoldReconciler struct {
	calls       int
	workspaceID string
	reason      string
	effectiveAt time.Time
	planID      string
	limit       int
	err         error
}

func (r *recordingHoldReconciler) ReconcileWorkspace(_ context.Context, workspaceID, reason string, effectiveAt time.Time) error {
	r.calls++
	r.workspaceID = workspaceID
	r.reason = reason
	r.effectiveAt = effectiveAt
	return r.err
}

func (r *recordingHoldReconciler) ReconcileWorkspaceForPlan(
	_ context.Context,
	workspaceID string,
	planID string,
	limit int,
	reason string,
	effectiveAt time.Time,
) error {
	r.calls++
	r.workspaceID = workspaceID
	r.planID = planID
	r.limit = limit
	r.reason = reason
	r.effectiveAt = effectiveAt
	return r.err
}

func (r *recordingHoldReconciler) ApplyPlanChange(
	_ context.Context,
	workspaceID string,
	planID string,
	limit int,
	reason string,
	effectiveAt time.Time,
	_ paidquota.PlanChangeMutation,
) error {
	r.calls++
	r.workspaceID = workspaceID
	r.planID = planID
	r.limit = limit
	r.reason = reason
	r.effectiveAt = effectiveAt
	return r.err
}
