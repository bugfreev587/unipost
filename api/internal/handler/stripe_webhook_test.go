package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/webhook"

	"github.com/xiaoboyu/unipost-api/internal/billing"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/paidquota"
	"github.com/xiaoboyu/unipost-api/internal/runtimeenv"
	"github.com/xiaoboyu/unipost-api/internal/trials"
)

func TestStripeTrialCheckoutProjectsRetrievedSubscriptionAsTrialing(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 7900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_growth", Valid: true}, AllowAnalytics: true}
	trial := &recordingTrialWebhookService{retrieve: trials.SubscriptionSnapshot{
		StripeMode: "sandbox", ID: "sub_staging", Status: "trialing", CustomerID: "cus_staging", PriceID: "price_growth",
		TrialStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), TrialEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()),
		CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()),
		Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "growth", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"},
	}, subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "growth", Status: trials.StatusActive}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "growth", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}

	response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	if trial.retrieveMode != "sandbox" || trial.retrieveID != "sub_staging" || trial.subscriptionCalls != 1 {
		t.Fatalf("retrieve=%s/%s reconcile_calls=%d", trial.retrieveMode, trial.retrieveID, trial.subscriptionCalls)
	}
	if got := store.subscription; got.PlanID != "growth" || got.Status != "trialing" || !got.CurrentPeriodStart.Valid || !got.CurrentPeriodEnd.Valid || got.CancelAtPeriodEnd.Bool {
		t.Fatalf("subscription = %#v", got)
	}
}

func TestStripeTrialCheckoutRetrievalFailureReturns500ForRetry(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	trial := &recordingTrialWebhookService{retrieveErr: errors.New("stripe unavailable")}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestCheckoutWebhook(t, h, secret, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "unipost_environment": "staging"}, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)
	if response.Code != http.StatusInternalServerError || store.upserts != 0 {
		t.Fatalf("status=%d upserts=%d body=%s", response.Code, store.upserts, response.Body.String())
	}
}

func TestStripeSubscriptionCreatedProjectsThroughSharedProjectorBeforeCheckout(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusActive}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.created", stripe.SubscriptionStatusTrialing, false, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"})
	if response.Code != http.StatusOK || trial.subscriptionCalls != 1 || store.subscription.Status != "trialing" {
		t.Fatalf("status=%d calls=%d subscription=%#v body=%s", response.Code, trial.subscriptionCalls, store.subscription, response.Body.String())
	}
}

func TestStripeTrialWillEndSendsManagedTrialEndingEmailForBothKinds(t *testing.T) {
	for _, kind := range []trials.Kind{trials.KindFreeToPaid, trials.KindPaidSamePlan} {
		t.Run(string(kind), func(t *testing.T) {
			t.Setenv(runtimeenv.EnvVar, "staging")
			store := newStripeWebhookStore("ws_staging")
			endingAt := time.Unix(1787501017, 0).UTC()
			trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
				ID: "grant_1", WorkspaceID: "ws_staging", Kind: kind, PlanID: "basic", DurationDays: 30,
				Status: trials.StatusActive, EndsAt: &endingAt, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging",
			}}}
			syncer := &recordingStripeLifecycleSyncer{}
			h, secret := newTestStripeWebhookHandler(store, syncer)
			h.SetTrialWebhookService(trial)
			metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": string(kind), "unipost_environment": "staging"}

			first := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.trial_will_end", stripe.SubscriptionStatusTrialing, false, metadata)
			second := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.trial_will_end", stripe.SubscriptionStatusTrialing, false, metadata)

			if first.Code != http.StatusOK || second.Code != http.StatusOK {
				t.Fatalf("statuses=%d/%d first=%s second=%s", first.Code, second.Code, first.Body.String(), second.Body.String())
			}
			endingEvents := lifecycleEventsNamed(syncer.events, "billing_trial_ending")
			if len(endingEvents) != 2 {
				t.Fatalf("trial ending events=%d, want duplicate delivery attempts with one stable provider key", len(endingEvents))
			}
			for _, event := range endingEvents {
				if event.IdempotencyKey != "billing_trial_ending:grant_1:2026-08-23T16:03:37Z" {
					t.Fatalf("event=%#v", event)
				}
			}
		})
	}
}

func TestStripeTrialWillEndLoopsFailureStillAcknowledgesWebhook(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	endingAt := time.Unix(1787501017, 0).UTC()
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
		ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: 30,
		Status: trials.StatusActive, EndsAt: &endingAt, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging",
	}}}
	syncer := &recordingStripeLifecycleSyncer{err: errors.New("Loops unavailable")}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.trial_will_end", stripe.SubscriptionStatusTrialing, false, map[string]string{
		"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging",
	})
	if response.Code != http.StatusOK || len(lifecycleEventsNamed(syncer.events, "billing_trial_ending")) != 1 {
		t.Fatalf("status=%d events=%d body=%s", response.Code, len(syncer.events), response.Body.String())
	}
}

func TestStripeTrialWillEndMissingLoopsConfigAuditsFailureAndAcknowledgesWebhook(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	endingAt := time.Unix(1787501017, 0).UTC()
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
		ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: 30,
		Status: trials.StatusActive, EndsAt: &endingAt, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging",
	}}}
	audit := &recordingTrialEndingAuditStore{}
	h, secret := newTestStripeWebhookHandler(store, loops.NewSyncer(nil, loops.Options{EmailAuditStore: audit}))
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{
		"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging",
	}
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.trial_will_end", stripe.SubscriptionStatusTrialing, false, metadata)
	retry := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.trial_will_end", stripe.SubscriptionStatusTrialing, false, metadata)

	if response.Code != http.StatusOK || retry.Code != http.StatusOK {
		t.Fatalf("statuses=%d/%d body=%s retry=%s", response.Code, retry.Code, response.Body.String(), retry.Body.String())
	}
	if audit.created != 2 || audit.failed != 2 || audit.sent != 0 || audit.failureReason != "missing_transactional_id" {
		t.Fatalf("audit=%#v", audit)
	}
	if audit.attempt.EventKey != "email.billing.trial_ending.v1" || audit.attempt.IdempotencyKey != "billing_trial_ending:grant_1:2026-08-23T16:03:37Z" {
		t.Fatalf("attempt=%#v", audit.attempt)
	}
	if len(audit.attemptCounts) != 1 || audit.attemptCounts[audit.attempt.IdempotencyKey] != 2 {
		t.Fatalf("logical audit rows/attempts=%#v", audit.attemptCounts)
	}
}

func TestStripeShortTrialEmailsImmediatelyAfterActivationButLongTrialWaits(t *testing.T) {
	for _, tc := range []struct {
		name string
		days int32
		want int
	}{{name: "three days", days: 3, want: 1}, {name: "thirty days", days: 30, want: 0}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(runtimeenv.EnvVar, "staging")
			store := newStripeWebhookStore("ws_staging")
			endingAt := time.Unix(1784822630, 0).UTC().Add(time.Duration(tc.days) * 24 * time.Hour)
			trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
				ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: tc.days,
				Status: trials.StatusActive, EndsAt: &endingAt, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging",
			}}}
			syncer := &recordingStripeLifecycleSyncer{}
			h, secret := newTestStripeWebhookHandler(store, syncer)
			h.SetTrialWebhookService(trial)
			response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.created", stripe.SubscriptionStatusTrialing, false, map[string]string{
				"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging",
			})
			endingEvents := lifecycleEventsNamed(syncer.events, "billing_trial_ending")
			if response.Code != http.StatusOK || len(endingEvents) != tc.want {
				t.Fatalf("status=%d trial_ending_events=%d all_events=%d want=%d body=%s", response.Code, len(endingEvents), len(syncer.events), tc.want, response.Body.String())
			}
		})
	}
}

func TestStripeShortTrialEmailRequiresTrialingSubscription(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	endingAt := time.Unix(1784822630, 0).UTC().Add(72 * time.Hour)
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
		ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: 3,
		Status: trials.StatusActive, EndsAt: &endingAt, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging",
	}}}
	syncer := &recordingStripeLifecycleSyncer{}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, map[string]string{
		"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging",
	})
	if response.Code != http.StatusOK || len(lifecycleEventsNamed(syncer.events, "billing_trial_ending")) != 0 {
		t.Fatalf("status=%d events=%#v body=%s", response.Code, syncer.events, response.Body.String())
	}
}

func TestStripeShortTrialCheckoutActivationUsesSameEndingEmail(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	start := time.Unix(1784822617, 0).UTC()
	end := start.Add(48 * time.Hour)
	grant := trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "basic", DurationDays: 2, Status: trials.StatusActive, EndsAt: &end, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging"}
	trial := &recordingTrialWebhookService{
		retrieve: trials.SubscriptionSnapshot{
			StripeMode: "sandbox", ID: "sub_staging", Status: "trialing", CustomerID: "cus_staging", PriceID: "price_basic",
			TrialStartAt: &start, TrialEndAt: &end, CurrentPeriodStartAt: &start, CurrentPeriodEndAt: &end,
			Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"},
		},
		subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: grant},
	}
	syncer := &recordingStripeLifecycleSyncer{}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	h.SetTrialWebhookService(trial)
	response := postTestCheckoutWebhook(t, h, secret, trial.retrieve.Metadata, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)
	if response.Code != http.StatusOK || len(lifecycleEventsNamed(syncer.events, "billing_trial_ending")) != 1 {
		t.Fatalf("status=%d events=%#v body=%s", response.Code, syncer.events, response.Body.String())
	}
}

func TestStripeShortPaidTrialScheduleReconciliationUsesSameEndingEmail(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	end := time.Unix(1784822630, 0).UTC().Add(72 * time.Hour)
	trial := &recordingTrialWebhookService{scheduleResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
		ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindPaidSamePlan, PlanID: "basic", DurationDays: 3,
		Status: trials.StatusActive, EndsAt: &end, StripeMode: "sandbox", StripeSubscriptionID: "sub_staging", StripeScheduleID: "sched_1",
	}}}
	syncer := &recordingStripeLifecycleSyncer{}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	h.SetTrialWebhookService(trial)
	response := postTestStripeObjectWebhook(t, h, secret, "subscription_schedule.updated", map[string]interface{}{
		"id": "sched_1", "object": "subscription_schedule", "status": "active", "customer": "cus_staging", "subscription": "sub_staging",
		"metadata": map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "paid_same_plan", "unipost_environment": "staging"},
	})
	if response.Code != http.StatusOK || len(lifecycleEventsNamed(syncer.events, "billing_trial_ending")) != 1 {
		t.Fatalf("status=%d events=%#v body=%s", response.Code, syncer.events, response.Body.String())
	}
}

func TestManagedTrialCancelAtPeriodEndRetainsPlanAndAccess(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "trialing"
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, RenewalCanceled: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusActive, CanceledAt: webhookPtrTime(time.Now().UTC())}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusTrialing, true, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"})
	if response.Code != http.StatusOK || store.subscription.PlanID != "basic" || store.subscription.Status != "trialing" || store.cancelCalls != 0 {
		t.Fatalf("status=%d subscription=%#v cancel_calls=%d body=%s", response.Code, store.subscription, store.cancelCalls, response.Body.String())
	}
}

func TestManagedTrialDelayedCancelFalseDoesNotClearRecordedIntent(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "trialing"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	trial := &recordingTrialWebhookService{subscriptionResults: []trials.WebhookReconcileResult{
		{Managed: true, RenewalCanceled: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusActive}},
		{Managed: true, PreserveCancelAtPeriodEnd: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusActive}},
	}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}
	first := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusTrialing, true, metadata)
	delayed := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusTrialing, false, metadata)
	if first.Code != http.StatusOK || delayed.Code != http.StatusOK {
		t.Fatalf("statuses=%d/%d bodies=%q/%q", first.Code, delayed.Code, first.Body.String(), delayed.Body.String())
	}
	if !store.subscription.CancelAtPeriodEnd.Valid || !store.subscription.CancelAtPeriodEnd.Bool || store.upserts != 2 {
		t.Fatalf("subscription=%#v upserts=%d", store.subscription, store.upserts)
	}
}

func TestManagedTrialConfirmedPlanChangeBypassesScheduledDowngradeGuard(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   trials.Kind
		before trials.Status
	}{
		{name: "free active", kind: trials.KindFreeToPaid, before: trials.StatusActive},
		{name: "paid scheduled", kind: trials.KindPaidSamePlan, before: trials.StatusScheduled},
		{name: "paid active", kind: trials.KindPaidSamePlan, before: trials.StatusActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(runtimeenv.EnvVar, "staging")
			store := newStripeWebhookStore("ws_staging")
			store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 7900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_growth", Valid: true}}
			store.subscription.PlanID = "growth"
			store.subscription.Status = "trialing"
			store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
			store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
			store.subscription.CurrentPeriodEnd = pgtype.Timestamptz{Time: time.Unix(1789000000, 0).UTC(), Valid: true}
			trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{
				ID: "grant_1", WorkspaceID: "ws_staging", Kind: tc.kind, PlanID: "growth", Status: trials.StatusSuperseded, SupersededByPlanID: "basic",
			}}}
			h, secret := newTestStripeWebhookHandler(store, nil)
			h.SetTrialWebhookService(trial)
			metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": string(tc.kind), "unipost_environment": "staging"}

			response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, metadata)

			if response.Code != http.StatusOK || store.subscription.PlanID != "basic" || store.upserts != 1 {
				t.Fatalf("before=%s status=%d subscription=%#v upserts=%d body=%s", tc.before, response.Code, store.subscription, store.upserts, response.Body.String())
			}
		})
	}
}

func TestManagedTrialSupersessionDoesNotBypassDowngradeGuardForDifferentTarget(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 7900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_growth", Valid: true}}
	store.subscription.PlanID = "growth"
	store.subscription.Status = "trialing"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	store.subscription.CurrentPeriodEnd = pgtype.Timestamptz{Time: time.Unix(1789000000, 0).UTC(), Valid: true}
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindFreeToPaid, PlanID: "growth", Status: trials.StatusSuperseded, SupersededByPlanID: "api"}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"})
	if response.Code != http.StatusOK || store.subscription.PlanID != "growth" {
		t.Fatalf("status=%d subscription=%#v body=%s", response.Code, store.subscription, response.Body.String())
	}
}

func TestLegacyTrialCancelAtPeriodEndStillDowngradesImmediately(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "trialing"
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	h, secret := newTestStripeWebhookHandler(store, nil)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusTrialing, true, nil)
	if response.Code != http.StatusOK || store.subscription.PlanID != "free" || store.cancelCalls != 1 {
		t.Fatalf("status=%d subscription=%#v cancel_calls=%d body=%s", response.Code, store.subscription, store.cancelCalls, response.Body.String())
	}
}

func TestTrialingPaymentSucceededDoesNotPromoteSubscriptionToActive(t *testing.T) {
	store := newStripeWebhookStore("ws_staging")
	store.subscription.Status = "trialing"
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	h := &StripeWebhookHandler{queries: db.New(store)}
	h.handlePaymentSucceeded(httptest.NewRequest(http.MethodPost, "/", nil), stripe.Event{ID: "evt_invoice", Data: &stripe.EventData{Raw: json.RawMessage(`{"id":"in_1","parent":{"subscription_details":{"subscription":"sub_staging"}}}`)}})
	if store.subscription.Status != "trialing" || store.statusUpdates != 0 {
		t.Fatalf("subscription=%#v status_updates=%d", store.subscription, store.statusUpdates)
	}
}

func TestTrialingSubscriptionIsExcludedFromAdminMRR(t *testing.T) {
	normalized := strings.ToLower(strings.Join(strings.Fields(adminStatsQuery), " "))
	if !strings.Contains(normalized, "where s.status = 'active'") {
		t.Fatalf("admin MRR must remain active-only: %s", normalized)
	}
	if strings.Contains(normalized, "s.status in ('active', 'trialing')") {
		t.Fatalf("admin MRR must not count trialing subscriptions: %s", normalized)
	}
}

func TestOrdinaryCheckoutSupersedesPendingOfferOnlyAfterProjection(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	trial := &recordingTrialWebhookService{retrieve: trials.SubscriptionSnapshot{StripeMode: "sandbox", ID: "sub_staging", Status: "active", CustomerID: "cus_staging", PriceID: "price_basic", CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()), Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestCheckoutWebhook(t, h, secret, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"}, stripe.CheckoutSessionPaymentStatusPaid)
	if response.Code != http.StatusOK || trial.ordinaryCalls != 1 || store.subscription.PlanID != "basic" {
		t.Fatalf("status=%d ordinary=%d subscription=%#v body=%s", response.Code, trial.ordinaryCalls, store.subscription, response.Body.String())
	}

	store.plans["basic"] = db.Plan{ID: "basic", Name: "Basic", PriceCents: 1900, PostLimit: 2500}
	trial.ordinaryCalls = 0
	response = postTestCheckoutWebhook(t, h, secret, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"}, stripe.CheckoutSessionPaymentStatusPaid)
	if response.Code != http.StatusInternalServerError || trial.ordinaryCalls != 0 {
		t.Fatalf("failed projection status=%d ordinary=%d body=%s", response.Code, trial.ordinaryCalls, response.Body.String())
	}
}

func TestCheckoutCompletedAfterGrantAlreadyActiveConverges(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	trial := &recordingTrialWebhookService{retrieve: trials.SubscriptionSnapshot{StripeMode: "sandbox", ID: "sub_staging", Status: "trialing", CustomerID: "cus_staging", PriceID: "price_basic", TrialStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), TrialEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()), CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()), Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}}, subscriptionResult: trials.WebhookReconcileResult{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusActive}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}
	for i := 0; i < 2; i++ {
		response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status=%d body=%s", i+1, response.Code, response.Body.String())
		}
	}
	if trial.subscriptionCalls != 2 || store.subscription.Status != "trialing" {
		t.Fatalf("reconcile_calls=%d subscription=%#v", trial.subscriptionCalls, store.subscription)
	}
}

func TestDelayedTrialingWebhookForTerminalGrantDoesNotRestoreSubscription(t *testing.T) {
	for _, status := range []trials.Status{trials.StatusCompleted, trials.StatusCanceled, trials.StatusSuperseded} {
		t.Run(string(status), func(t *testing.T) {
			t.Setenv(runtimeenv.EnvVar, "staging")
			store := newStripeWebhookStore("ws_staging")
			store.subscription.PlanID = "free"
			store.subscription.Status = "active"
			trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, DoNotProject: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: status}}}
			h, secret := newTestStripeWebhookHandler(store, nil)
			h.SetTrialWebhookService(trial)
			response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusTrialing, false, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"})
			if response.Code != http.StatusOK || store.subscription.PlanID != "free" || store.subscription.Status != "active" || store.upserts != 0 {
				t.Fatalf("status=%d subscription=%#v upserts=%d body=%s", response.Code, store.subscription, store.upserts, response.Body.String())
			}
		})
	}
}

func TestTerminalGrantProjectionRetryConvergesAfterLocalWriteFailure(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "trialing"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	store.updateErrors = []error{errors.New("database unavailable"), nil}
	trialEnd := time.Unix(1784822617, 0).UTC()
	trial := &recordingTrialWebhookService{subscriptionResults: []trials.WebhookReconcileResult{
		{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusCompleted, EndsAt: &trialEnd, CompletedAt: &trialEnd}},
		{Managed: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", PlanID: "basic", Status: trials.StatusCompleted, EndsAt: &trialEnd, CompletedAt: &trialEnd}},
	}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}

	first := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, metadata)
	second := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, metadata)
	if first.Code != http.StatusInternalServerError || second.Code != http.StatusOK {
		t.Fatalf("statuses=%d/%d bodies=%q/%q", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	if store.subscription.Status != "active" || store.subscription.PlanID != "basic" || store.upserts != 1 || trial.subscriptionCalls != 2 {
		t.Fatalf("subscription=%#v upserts=%d reconcile=%d", store.subscription, store.upserts, trial.subscriptionCalls)
	}
}

func TestCompletedPaidTrialDelayedPreTrialActiveEventPreservesCurrentPeriod(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "active"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	currentStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	currentEnd := currentStart.Add(30 * 24 * time.Hour)
	store.subscription.CurrentPeriodStart = pgtype.Timestamptz{Time: currentStart, Valid: true}
	store.subscription.CurrentPeriodEnd = pgtype.Timestamptz{Time: currentEnd, Valid: true}
	trial := &recordingTrialWebhookService{subscriptionResult: trials.WebhookReconcileResult{Managed: true, DoNotProject: true, Grant: trials.Grant{ID: "grant_1", WorkspaceID: "ws_staging", Kind: trials.KindPaidSamePlan, PlanID: "basic", Status: trials.StatusCompleted}}}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, false, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "paid_same_plan", "unipost_environment": "staging"})
	if response.Code != http.StatusOK || store.upserts != 0 || !store.subscription.CurrentPeriodStart.Time.Equal(currentStart) || !store.subscription.CurrentPeriodEnd.Time.Equal(currentEnd) {
		t.Fatalf("status=%d upserts=%d period=%s..%s body=%s", response.Code, store.upserts, store.subscription.CurrentPeriodStart.Time, store.subscription.CurrentPeriodEnd.Time, response.Body.String())
	}
}

func TestSubscriptionProjectorRejectsDelayedPeriodRegression(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.subscription.PlanID = "basic"
	store.subscription.Status = "active"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	trial := &recordingTrialWebhookService{}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)
	metadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"}
	period1 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	period2 := period1.Add(30 * 24 * time.Hour)

	first := postTestSubscriptionWebhookWithPeriod(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, metadata, period1, period2)
	second := postTestSubscriptionWebhookWithPeriod(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, metadata, period2, period2.Add(30*24*time.Hour))
	delayed := postTestSubscriptionWebhookWithPeriod(t, h, secret, "customer.subscription.updated", stripe.SubscriptionStatusActive, metadata, period1, period2)
	trial.retrieve = trials.SubscriptionSnapshot{StripeMode: "sandbox", ID: "sub_staging", Status: "active", CustomerID: "cus_staging", PriceID: "price_basic", CurrentPeriodStartAt: &period1, CurrentPeriodEndAt: &period2, Metadata: metadata}
	delayedCheckout := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)
	if first.Code != http.StatusOK || second.Code != http.StatusOK || delayed.Code != http.StatusOK || delayedCheckout.Code != http.StatusOK {
		t.Fatalf("statuses=%d/%d/%d/%d bodies=%q/%q/%q/%q", first.Code, second.Code, delayed.Code, delayedCheckout.Code, first.Body.String(), second.Body.String(), delayed.Body.String(), delayedCheckout.Body.String())
	}
	if store.upserts != 2 || trial.subscriptionCalls != 2 || trial.ordinaryCalls != 0 || !store.subscription.CurrentPeriodStart.Time.Equal(period2) || !store.subscription.CurrentPeriodEnd.Time.Equal(period2.Add(30*24*time.Hour)) {
		t.Fatalf("upserts=%d reconcile=%d ordinary=%d period=%s..%s", store.upserts, trial.subscriptionCalls, trial.ordinaryCalls, store.subscription.CurrentPeriodStart.Time, store.subscription.CurrentPeriodEnd.Time)
	}
}

func TestCheckoutCompletedRejectsSessionSubscriptionBindingMismatch(t *testing.T) {
	baseMetadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"}
	for _, tc := range []struct {
		name    string
		mutate  func(*trials.SubscriptionSnapshot)
		managed bool
	}{
		{name: "customer", mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.CustomerID = "cus_other" }},
		{name: "workspace", mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.Metadata["workspace_id"] = "ws_other" }},
		{name: "actual price plan", mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.PriceID = "price_growth" }},
		{name: "environment", mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.Metadata["unipost_environment"] = "dev" }},
		{name: "managed grant", managed: true, mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.Metadata["trial_grant_id"] = "grant_other" }},
		{name: "managed kind", managed: true, mutate: func(snapshot *trials.SubscriptionSnapshot) { snapshot.Metadata["trial_kind"] = "paid_same_plan" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(runtimeenv.EnvVar, "staging")
			store := newStripeWebhookStore("ws_staging")
			store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 7900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_growth", Valid: true}}
			metadata := cloneTestMetadata(baseMetadata)
			if tc.managed {
				metadata["trial_grant_id"], metadata["trial_kind"] = "grant_1", "free_to_paid"
			}
			snapshot := trials.SubscriptionSnapshot{StripeMode: "sandbox", ID: "sub_staging", Status: "trialing", CustomerID: "cus_staging", PriceID: "price_basic", TrialStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), TrialEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()), CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()), Metadata: cloneTestMetadata(metadata)}
			tc.mutate(&snapshot)
			trial := &recordingTrialWebhookService{retrieve: snapshot}
			h, secret := newTestStripeWebhookHandler(store, nil)
			h.SetTrialWebhookService(trial)
			response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)
			if response.Code != http.StatusInternalServerError || store.upserts != 0 || trial.subscriptionCalls != 0 || trial.ordinaryCalls != 0 {
				t.Fatalf("status=%d upserts=%d reconcile=%d ordinary=%d body=%s", response.Code, store.upserts, trial.subscriptionCalls, trial.ordinaryCalls, response.Body.String())
			}
		})
	}
}

func TestDelayedCheckoutAfterSameSubscriptionAdvancedPlanIsAcknowledgedNoOp(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.plans["growth"] = db.Plan{ID: "growth", Name: "Growth", PriceCents: 7900, PostLimit: 7500, StripePriceID: pgtype.Text{String: "price_growth", Valid: true}}
	periodStart := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.Add(30 * 24 * time.Hour)
	store.subscription.PlanID = "growth"
	store.subscription.Status = "active"
	store.subscription.StripeCustomerID = pgtype.Text{String: "cus_staging", Valid: true}
	store.subscription.StripeSubscriptionID = pgtype.Text{String: "sub_staging", Valid: true}
	store.subscription.CurrentPeriodStart = pgtype.Timestamptz{Time: periodStart, Valid: true}
	store.subscription.CurrentPeriodEnd = pgtype.Timestamptz{Time: periodEnd, Valid: true}
	trial := &recordingTrialWebhookService{terminalGrant: true, retrieve: trials.SubscriptionSnapshot{StripeMode: "sandbox", ID: "sub_staging", Status: "active", CustomerID: "cus_staging", PriceID: "price_growth", CurrentPeriodStartAt: &periodStart, CurrentPeriodEndAt: &periodEnd, Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "growth", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}}}
	syncer := &recordingStripeLifecycleSyncer{}
	quotaEvaluator := &recordingPaidQuotaEvaluator{}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	h.SetTrialWebhookService(trial).SetPaidQuotaEvaluator(quotaEvaluator)
	sessionMetadata := map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}

	response := postTestCheckoutWebhook(t, h, secret, sessionMetadata, stripe.CheckoutSessionPaymentStatusNoPaymentRequired)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if store.upserts != 0 || trial.subscriptionCalls != 0 || trial.ordinaryCalls != 0 || len(syncer.events) != 0 || len(quotaEvaluator.calls) != 0 {
		t.Fatalf("upserts=%d reconcile=%d ordinary=%d loops=%d quota=%d", store.upserts, trial.subscriptionCalls, trial.ordinaryCalls, len(syncer.events), len(quotaEvaluator.calls))
	}
}

func TestForeignSubscriptionDeletedAcknowledgedWithoutMutation(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	h, secret := newTestStripeWebhookHandler(store, nil)
	response := postTestSubscriptionWebhookWithState(t, h, secret, "customer.subscription.deleted", stripe.SubscriptionStatusCanceled, false, map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "dev"})
	if response.Code != http.StatusOK || store.upserts != 0 || store.cancelCalls != 0 {
		t.Fatalf("status=%d upserts=%d cancel=%d body=%s", response.Code, store.upserts, store.cancelCalls, response.Body.String())
	}
}

func TestCheckoutExpiredAndScheduleEventsDispatchWithForeignEnvironmentGuard(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	trial := &recordingTrialWebhookService{}
	h, secret := newTestStripeWebhookHandler(store, nil)
	h.SetTrialWebhookService(trial)

	response := postTestStripeObjectWebhook(t, h, secret, "checkout.session.expired", map[string]interface{}{"id": "cs_1", "object": "checkout.session", "metadata": map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "trial_grant_id": "grant_1", "trial_kind": "free_to_paid", "unipost_environment": "staging"}})
	if response.Code != http.StatusOK || trial.checkoutCalls != 1 {
		t.Fatalf("checkout expiry status=%d calls=%d body=%s", response.Code, trial.checkoutCalls, response.Body.String())
	}
	trial.scheduleErr = trials.ErrWebhookNotApplicable
	response = postTestStripeObjectWebhook(t, h, secret, "subscription_schedule.updated", map[string]interface{}{"id": "sched_1", "object": "subscription_schedule", "status": "active", "metadata": map[string]string{"unipost_environment": "dev"}})
	if response.Code != http.StatusOK || trial.scheduleCalls != 1 {
		t.Fatalf("foreign schedule status=%d calls=%d body=%s", response.Code, trial.scheduleCalls, response.Body.String())
	}
}

func TestStripeCheckoutReplayIsIdempotent(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	syncer := &recordingStripeLifecycleSyncer{}
	h, secret := newTestStripeWebhookHandler(store, syncer)
	metadata := map[string]string{
		"workspace_id": "ws_staging",
		"plan_id":      "basic",
		"mode":         "sandbox",
	}

	first := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)
	second := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)

	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("status codes = %d/%d, want 200/200; bodies = %q/%q", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	if store.upserts != 2 {
		t.Fatalf("subscription upserts = %d, want 2 idempotent applications", store.upserts)
	}
	if got := store.subscription; got.WorkspaceID != "ws_staging" ||
		got.PlanID != "basic" ||
		got.Status != "active" ||
		got.StripeCustomerID.String != "cus_staging" ||
		got.StripeSubscriptionID.String != "sub_staging" {
		t.Fatalf("subscription = %#v", got)
	}
	if len(syncer.events) != 1 {
		t.Fatalf("plan change events = %d, want 1", len(syncer.events))
	}
}

func TestStripeCheckoutIgnoresForeignEnvironment(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id":        "ws_staging",
		"plan_id":             "basic",
		"mode":                "sandbox",
		"unipost_environment": "dev",
	}

	response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 0 || store.upserts != 0 {
		t.Fatalf("foreign event touched DB: workspace_queries=%d upserts=%d", store.workspaceQueries, store.upserts)
	}
}

func TestStripeCheckoutIgnoresLegacyForeignWorkspace(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.workspaceErr = pgx.ErrNoRows
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id": "ws_foreign",
		"plan_id":      "basic",
		"mode":         "sandbox",
	}

	response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 1 || store.upserts != 0 {
		t.Fatalf("legacy foreign event touched subscription: workspace_queries=%d upserts=%d", store.workspaceQueries, store.upserts)
	}
}

func TestStripeCheckoutReturns500ForWorkspaceLookupFailure(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.workspaceErr = errors.New("database unavailable")
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id": "ws_staging",
		"plan_id":      "basic",
		"mode":         "sandbox",
	}

	response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusPaid)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 1 || store.upserts != 0 {
		t.Fatalf("failed workspace lookup touched subscription: workspace_queries=%d upserts=%d", store.workspaceQueries, store.upserts)
	}
}

func TestStripeCheckoutIgnoresUnpaidSession(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id": "ws_staging",
		"plan_id":      "basic",
		"mode":         "sandbox",
	}

	response := postTestCheckoutWebhook(t, h, secret, metadata, stripe.CheckoutSessionPaymentStatusUnpaid)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 0 || store.upserts != 0 {
		t.Fatalf("unpaid event touched DB: workspace_queries=%d upserts=%d", store.workspaceQueries, store.upserts)
	}
}

func TestStripeSubscriptionUpdateIgnoresForeignEnvironment(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id":        "ws_foreign",
		"plan_id":             "basic",
		"unipost_environment": "dev",
	}

	response := postTestSubscriptionUpdatedWebhook(t, h, secret, metadata)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 0 || store.stripeSubscriptionQueries != 0 {
		t.Fatalf("foreign subscription update touched DB: workspace_queries=%d subscription_queries=%d", store.workspaceQueries, store.stripeSubscriptionQueries)
	}
}

func TestStripeSubscriptionUpdateIgnoresLegacyForeignSubscription(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.stripeSubscriptionErr = pgx.ErrNoRows
	h, secret := newTestStripeWebhookHandler(store, nil)

	response := postTestSubscriptionUpdatedWebhook(t, h, secret, nil)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 0 || store.stripeSubscriptionQueries != 1 {
		t.Fatalf("legacy subscription lookup = workspace:%d subscription:%d, want 0/1", store.workspaceQueries, store.stripeSubscriptionQueries)
	}
}

func TestStripeSubscriptionUpdateRecoversByWorkspaceMetadata(t *testing.T) {
	t.Setenv(runtimeenv.EnvVar, "staging")
	store := newStripeWebhookStore("ws_staging")
	store.stripeSubscriptionErr = pgx.ErrNoRows
	h, secret := newTestStripeWebhookHandler(store, nil)
	metadata := map[string]string{
		"workspace_id":        "ws_staging",
		"plan_id":             "basic",
		"unipost_environment": "staging",
	}

	response := postTestSubscriptionUpdatedWebhook(t, h, secret, metadata)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %q", response.Code, response.Body.String())
	}
	if store.workspaceQueries != 1 || store.stripeSubscriptionQueries != 1 {
		t.Fatalf("recovery lookups = workspace:%d subscription:%d, want 1/1", store.workspaceQueries, store.stripeSubscriptionQueries)
	}
	if store.subscription.Status != "active" || store.upserts != 1 {
		t.Fatalf("recovered subscription=%#v upserts=%d", store.subscription, store.upserts)
	}
}

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

type recordingStripeLifecycleSyncer struct {
	events []loops.LifecycleEvent
	err    error
}

type recordingTrialEndingAuditStore struct {
	created       int
	failed        int
	sent          int
	attempt       loops.EmailSendAttempt
	failureReason string
	attemptCounts map[string]int
}

func (s *recordingTrialEndingAuditStore) CreateEmailSendAttempt(_ context.Context, attempt loops.EmailSendAttempt) (loops.EmailSendAttemptRecord, error) {
	s.created++
	s.attempt = attempt
	if s.attemptCounts == nil {
		s.attemptCounts = map[string]int{}
	}
	s.attemptCounts[attempt.IdempotencyKey]++
	return loops.EmailSendAttemptRecord{ID: "attempt_1"}, nil
}

func (s *recordingTrialEndingAuditStore) CreateSkippedEmailSendAttempt(_ context.Context, _ loops.EmailSendAttempt, _ string) (loops.EmailSendAttemptRecord, error) {
	return loops.EmailSendAttemptRecord{}, errors.New("unexpected skipped audit")
}

func (s *recordingTrialEndingAuditStore) MarkEmailSendAttemptSent(_ context.Context, _ string) error {
	s.sent++
	return nil
}

func (s *recordingTrialEndingAuditStore) MarkEmailSendAttemptFailed(_ context.Context, _ string, reason string) error {
	s.failed++
	s.failureReason = reason
	return nil
}

func (r *recordingStripeLifecycleSyncer) SendLifecycleEvent(_ context.Context, event loops.LifecycleEvent) error {
	r.events = append(r.events, event)
	return r.err
}

func lifecycleEventsNamed(events []loops.LifecycleEvent, name string) []loops.LifecycleEvent {
	var matching []loops.LifecycleEvent
	for _, event := range events {
		if event.EventName == name {
			matching = append(matching, event)
		}
	}
	return matching
}

type stripeWebhookStore struct {
	workspace                 db.Workspace
	workspaceErr              error
	workspaceQueries          int
	user                      db.User
	subscription              db.Subscription
	stripeSubscriptionErr     error
	stripeSubscriptionQueries int
	plans                     map[string]db.Plan
	upserts                   int
	cancelCalls               int
	statusUpdates             int
	updateErrors              []error
}

func newStripeWebhookStore(workspaceID string) *stripeWebhookStore {
	return &stripeWebhookStore{
		workspace: db.Workspace{
			ID:     workspaceID,
			UserID: "user_staging",
			Name:   "Staging Workspace",
		},
		user: db.User{
			ID:    "user_staging",
			Email: "staging-owner@example.com",
			Name:  pgtype.Text{String: "Staging Owner", Valid: true},
		},
		subscription: db.Subscription{
			ID:          "local_subscription",
			WorkspaceID: workspaceID,
			PlanID:      "free",
			Status:      "active",
		},
		plans: map[string]db.Plan{
			"free":  {ID: "free", Name: "Free", PriceCents: 0, PostLimit: 100, AllowInbox: false},
			"basic": {ID: "basic", Name: "Basic", PriceCents: 1900, PostLimit: 2500, StripePriceID: pgtype.Text{String: "price_basic", Valid: true}, AllowInbox: true},
		},
	}
}

func (s *stripeWebhookStore) Exec(_ context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	switch {
	case strings.Contains(query, "UPDATE subscriptions\nSET stripe_customer_id"):
		if len(s.updateErrors) > 0 {
			err := s.updateErrors[0]
			s.updateErrors = s.updateErrors[1:]
			if err != nil {
				return pgconn.CommandTag{}, err
			}
		}
		s.subscription.WorkspaceID, _ = args[0].(string)
		s.subscription.StripeCustomerID, _ = args[1].(pgtype.Text)
		s.subscription.StripeSubscriptionID, _ = args[2].(pgtype.Text)
		s.subscription.PlanID, _ = args[3].(string)
		s.subscription.Status, _ = args[4].(string)
		s.subscription.CurrentPeriodStart, _ = args[5].(pgtype.Timestamptz)
		s.subscription.CurrentPeriodEnd, _ = args[6].(pgtype.Timestamptz)
		s.subscription.CancelAtPeriodEnd, _ = args[7].(pgtype.Bool)
		s.upserts++
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "SET plan_id = 'free'"):
		s.cancelCalls++
		s.subscription.PlanID, s.subscription.Status = "free", "active"
		s.subscription.StripeSubscriptionID = pgtype.Text{}
		s.subscription.CancelAtPeriodEnd = pgtype.Bool{Bool: false, Valid: true}
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.Contains(query, "UPDATE subscriptions SET status = $2"):
		s.statusUpdates++
		s.subscription.Status, _ = args[1].(string)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	default:
		return pgconn.CommandTag{}, fmt.Errorf("unexpected Exec: %s", query)
	}
}

func (s *stripeWebhookStore) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}

func (s *stripeWebhookStore) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "FROM workspaces WHERE id = $1"):
		s.workspaceQueries++
		return stripeTestRow(func(dest ...interface{}) error {
			if s.workspaceErr != nil {
				return s.workspaceErr
			}
			return scanStripeWorkspace(dest, s.workspace)
		})
	case strings.Contains(query, "FROM users WHERE id = $1"):
		return stripeTestRow(func(dest ...interface{}) error {
			return scanStripeUser(dest, s.user)
		})
	case strings.Contains(query, "FROM subscriptions WHERE workspace_id = $1"):
		return stripeTestRow(func(dest ...interface{}) error {
			if s.subscription.WorkspaceID == "" {
				return pgx.ErrNoRows
			}
			return scanStripeSubscription(dest, s.subscription)
		})
	case strings.Contains(query, "FROM subscriptions WHERE stripe_subscription_id = $1"):
		s.stripeSubscriptionQueries++
		return stripeTestRow(func(dest ...interface{}) error {
			if s.stripeSubscriptionErr != nil {
				return s.stripeSubscriptionErr
			}
			return scanStripeSubscription(dest, s.subscription)
		})
	case strings.Contains(query, "FROM plans WHERE id = $1"):
		planID, _ := args[0].(string)
		return stripeTestRow(func(dest ...interface{}) error {
			plan, ok := s.plans[planID]
			if !ok {
				return pgx.ErrNoRows
			}
			return scanStripePlan(dest, plan)
		})
	case strings.Contains(query, "FROM plans WHERE stripe_price_id = $1"):
		price, _ := args[0].(pgtype.Text)
		return stripeTestRow(func(dest ...interface{}) error {
			for _, plan := range s.plans {
				if plan.StripePriceID.Valid && plan.StripePriceID.String == price.String {
					return scanStripePlan(dest, plan)
				}
			}
			return pgx.ErrNoRows
		})
	case strings.Contains(query, "INSERT INTO subscriptions"):
		s.upserts++
		s.subscription.WorkspaceID, _ = args[0].(string)
		s.subscription.PlanID, _ = args[1].(string)
		s.subscription.StripeCustomerID, _ = args[2].(pgtype.Text)
		s.subscription.StripeSubscriptionID, _ = args[3].(pgtype.Text)
		s.subscription.Status, _ = args[4].(string)
		return stripeTestRow(func(dest ...interface{}) error {
			return scanStripeSubscription(dest, s.subscription)
		})
	default:
		return stripeTestRow(func(...interface{}) error {
			return fmt.Errorf("unexpected QueryRow: %s", query)
		})
	}
}

type stripeTestRow func(...interface{}) error

func (r stripeTestRow) Scan(dest ...interface{}) error {
	return r(dest...)
}

func scanStripeWorkspace(dest []interface{}, value db.Workspace) error {
	if len(dest) != 8 {
		return fmt.Errorf("workspace scan destinations = %d", len(dest))
	}
	*dest[0].(*string) = value.ID
	*dest[1].(*string) = value.UserID
	*dest[2].(*string) = value.Name
	*dest[3].(*pgtype.Int4) = value.PerAccountMonthlyLimit
	*dest[4].(*pgtype.Timestamptz) = value.CreatedAt
	*dest[5].(*pgtype.Timestamptz) = value.UpdatedAt
	*dest[6].(*[]string) = value.UsageModes
	*dest[7].(*pgtype.Text) = value.CustomPlatformSlot
	return nil
}

func scanStripeUser(dest []interface{}, value db.User) error {
	if len(dest) != 13 {
		return fmt.Errorf("user scan destinations = %d", len(dest))
	}
	*dest[0].(*string) = value.ID
	*dest[1].(*string) = value.Email
	*dest[2].(*pgtype.Text) = value.Name
	*dest[3].(*pgtype.Timestamptz) = value.CreatedAt
	*dest[4].(*pgtype.Timestamptz) = value.UpdatedAt
	*dest[5].(*pgtype.Text) = value.DefaultProfileID
	*dest[6].(*pgtype.Text) = value.LastProfileID
	*dest[7].(*bool) = value.OnboardingCompleted
	*dest[8].(*pgtype.Text) = value.OnboardingIntent
	*dest[9].(*pgtype.Timestamptz) = value.OnboardingShownAt
	*dest[10].(*pgtype.Timestamptz) = value.OnboardingCompletedAt
	*dest[11].(*pgtype.Timestamptz) = value.ActivationCompletedAt
	*dest[12].(*pgtype.Timestamptz) = value.ActivationGuideDismissedAt
	return nil
}

func scanStripeSubscription(dest []interface{}, value db.Subscription) error {
	if len(dest) != 12 {
		return fmt.Errorf("subscription scan destinations = %d", len(dest))
	}
	*dest[0].(*string) = value.ID
	*dest[1].(*string) = value.PlanID
	*dest[2].(*pgtype.Text) = value.StripeCustomerID
	*dest[3].(*pgtype.Text) = value.StripeSubscriptionID
	*dest[4].(*string) = value.Status
	*dest[5].(*pgtype.Timestamptz) = value.CurrentPeriodStart
	*dest[6].(*pgtype.Timestamptz) = value.CurrentPeriodEnd
	*dest[7].(*pgtype.Bool) = value.CancelAtPeriodEnd
	*dest[8].(*pgtype.Timestamptz) = value.CreatedAt
	*dest[9].(*pgtype.Timestamptz) = value.UpdatedAt
	*dest[10].(*bool) = value.TrialUsed
	*dest[11].(*string) = value.WorkspaceID
	return nil
}

func scanStripePlan(dest []interface{}, value db.Plan) error {
	if len(dest) != 12 {
		return fmt.Errorf("plan scan destinations = %d", len(dest))
	}
	*dest[0].(*string) = value.ID
	*dest[1].(*string) = value.Name
	*dest[2].(*int32) = value.PriceCents
	*dest[3].(*int32) = value.PostLimit
	*dest[4].(*pgtype.Text) = value.StripePriceID
	*dest[5].(*pgtype.Timestamptz) = value.CreatedAt
	*dest[6].(*bool) = value.WhiteLabel
	*dest[7].(*bool) = value.AllowTwitter
	*dest[8].(*bool) = value.AllowInbox
	*dest[9].(*bool) = value.AllowAnalytics
	*dest[10].(*pgtype.Int4) = value.MaxProfiles
	*dest[11].(*pgtype.Int4) = value.MaxMembers
	return nil
}

func newTestStripeWebhookHandler(store *stripeWebhookStore, syncer loopsLifecycleSyncer) (*StripeWebhookHandler, string) {
	const secret = "whsec_staging_test"
	manager := &billing.Manager{
		Live: &billing.Mode{},
		Sandbox: &billing.Mode{
			Name:          "sandbox",
			WebhookSecret: secret,
		},
	}
	handler := NewStripeWebhookHandler(db.New(store), manager, events.NoopBus{}, "https://staging-app.unipost.dev")
	handler.SetTrialWebhookService(&recordingTrialWebhookService{retrieve: trials.SubscriptionSnapshot{
		StripeMode: "sandbox", ID: "sub_staging", Status: "active", CustomerID: "cus_staging", PriceID: "price_basic",
		CurrentPeriodStartAt: webhookPtrTime(time.Unix(1784822617, 0).UTC()), CurrentPeriodEndAt: webhookPtrTime(time.Unix(1787501017, 0).UTC()),
		Metadata: map[string]string{"workspace_id": "ws_staging", "plan_id": "basic", "unipost_environment": "staging"},
	}})
	if syncer != nil {
		handler.SetLoopsSyncer(syncer)
	}
	return handler, secret
}

func postTestCheckoutWebhook(
	t *testing.T,
	handler *StripeWebhookHandler,
	secret string,
	metadata map[string]string,
	paymentStatus stripe.CheckoutSessionPaymentStatus,
) *httptest.ResponseRecorder {
	t.Helper()
	metadata = cloneTestMetadata(metadata)
	if metadata["unipost_environment"] == "" {
		metadata["unipost_environment"] = runtimeenv.Current()
	}
	payload, err := json.Marshal(map[string]interface{}{
		"id":      "evt_checkout_basic",
		"object":  "event",
		"created": int64(1784822622),
		"type":    "checkout.session.completed",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":             "cs_test_basic",
				"object":         "checkout.session",
				"mode":           "subscription",
				"status":         "complete",
				"payment_status": paymentStatus,
				"customer":       "cus_staging",
				"subscription":   "sub_staging",
				"metadata":       metadata,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
	request := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	request.Header.Set("Stripe-Signature", signed.Header)
	response := httptest.NewRecorder()

	handler.HandleStripe(response, request)

	return response
}

func postTestSubscriptionUpdatedWebhook(
	t *testing.T,
	handler *StripeWebhookHandler,
	secret string,
	metadata map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{
		"id":      "evt_subscription_updated",
		"object":  "event",
		"created": int64(1784822630),
		"type":    "customer.subscription.updated",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":                   "sub_staging",
				"object":               "subscription",
				"status":               "active",
				"customer":             "cus_staging",
				"cancel_at_period_end": false,
				"metadata":             metadata,
				"items": map[string]interface{}{
					"data": []map[string]interface{}{
						{
							"current_period_start": int64(1784822617),
							"current_period_end":   int64(1787501017),
							"price": map[string]interface{}{
								"id": "price_basic",
							},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal webhook: %v", err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  secret,
	})
	request := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	request.Header.Set("Stripe-Signature", signed.Header)
	response := httptest.NewRecorder()

	handler.HandleStripe(response, request)

	return response
}

func postTestSubscriptionWebhookWithState(
	t *testing.T,
	handler *StripeWebhookHandler,
	secret string,
	eventType string,
	status stripe.SubscriptionStatus,
	cancelAtPeriodEnd bool,
	metadata map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	return postTestSubscriptionWebhookWithPeriod(t, handler, secret, eventType, status, metadata, time.Unix(1784822617, 0).UTC(), time.Unix(1787501017, 0).UTC(), cancelAtPeriodEnd)
}

func postTestSubscriptionWebhookWithPeriod(
	t *testing.T,
	handler *StripeWebhookHandler,
	secret string,
	eventType string,
	status stripe.SubscriptionStatus,
	metadata map[string]string,
	periodStart time.Time,
	periodEnd time.Time,
	cancelAtPeriodEnd ...bool,
) *httptest.ResponseRecorder {
	t.Helper()
	cancelAtEnd := false
	if len(cancelAtPeriodEnd) > 0 {
		cancelAtEnd = cancelAtPeriodEnd[0]
	}
	payload, err := json.Marshal(map[string]interface{}{
		"id": "evt_subscription_state", "object": "event", "created": int64(1784822630), "type": eventType,
		"data": map[string]interface{}{"object": map[string]interface{}{
			"id": "sub_staging", "object": "subscription", "status": status, "customer": "cus_staging",
			"cancel_at_period_end": cancelAtEnd, "trial_start": int64(1784822617), "trial_end": int64(1787501017), "metadata": metadata,
			"items": map[string]interface{}{"data": []map[string]interface{}{{"id": "si_staging", "current_period_start": periodStart.Unix(), "current_period_end": periodEnd.Unix(), "price": map[string]interface{}{"id": "price_basic"}}}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret})
	request := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	request.Header.Set("Stripe-Signature", signed.Header)
	response := httptest.NewRecorder()
	handler.HandleStripe(response, request)
	return response
}

func postTestStripeObjectWebhook(t *testing.T, handler *StripeWebhookHandler, secret, eventType string, object map[string]interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(map[string]interface{}{"id": "evt_object", "object": "event", "created": int64(1784822630), "type": eventType, "data": map[string]interface{}{"object": object}})
	if err != nil {
		t.Fatal(err)
	}
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{Payload: payload, Secret: secret})
	request := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(payload))
	request.Header.Set("Stripe-Signature", signed.Header)
	response := httptest.NewRecorder()
	handler.HandleStripe(response, request)
	return response
}

type recordingTrialWebhookService struct {
	retrieve            trials.SubscriptionSnapshot
	retrieveErr         error
	retrieveMode        string
	retrieveID          string
	subscriptionResult  trials.WebhookReconcileResult
	subscriptionResults []trials.WebhookReconcileResult
	subscriptionErr     error
	subscriptionCalls   int
	checkoutResult      trials.WebhookReconcileResult
	checkoutErr         error
	scheduleResult      trials.WebhookReconcileResult
	scheduleErr         error
	ordinaryResult      trials.WebhookReconcileResult
	ordinaryErr         error
	checkoutCalls       int
	scheduleCalls       int
	ordinaryCalls       int
	terminalGrant       bool
	terminalGrantErr    error
}

func (s *recordingTrialWebhookService) RetrieveSubscription(_ context.Context, mode, id string) (trials.SubscriptionSnapshot, error) {
	s.retrieveMode, s.retrieveID = mode, id
	return s.retrieve, s.retrieveErr
}
func (s *recordingTrialWebhookService) ReconcileSubscription(_ context.Context, _ trials.WebhookSubscriptionRequest) (trials.WebhookReconcileResult, error) {
	s.subscriptionCalls++
	if len(s.subscriptionResults) > 0 {
		result := s.subscriptionResults[0]
		s.subscriptionResults = s.subscriptionResults[1:]
		return result, s.subscriptionErr
	}
	return s.subscriptionResult, s.subscriptionErr
}
func (s *recordingTrialWebhookService) ReconcileCheckoutExpired(_ context.Context, _ trials.WebhookCheckoutRequest) (trials.WebhookReconcileResult, error) {
	s.checkoutCalls++
	return s.checkoutResult, s.checkoutErr
}
func (s *recordingTrialWebhookService) ReconcileSchedule(_ context.Context, _ trials.WebhookScheduleRequest) (trials.WebhookReconcileResult, error) {
	s.scheduleCalls++
	return s.scheduleResult, s.scheduleErr
}
func (s *recordingTrialWebhookService) ReconcileOrdinaryCheckout(_ context.Context, _, _ string, _ time.Time) (trials.WebhookReconcileResult, error) {
	s.ordinaryCalls++
	return s.ordinaryResult, s.ordinaryErr
}
func (s *recordingTrialWebhookService) IsTerminalGrant(_ context.Context, _, _ string) (bool, error) {
	return s.terminalGrant, s.terminalGrantErr
}

func webhookPtrTime(value time.Time) *time.Time { return &value }

func cloneTestMetadata(input map[string]string) map[string]string {
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
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
