package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/loops"
)

func TestPaidPlanQuotaEmailWorkerMarksSuccessfulSend(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	store := &fakePaidQuotaDeliveryStore{
		deliveries: []PaidQuotaDelivery{paidQuotaDeliveryFixture(1)},
	}
	sender := &fakePaidQuotaSender{}
	worker := NewPaidPlanQuotaEmailWorker(store, sender, nil, func() time.Time { return now })
	worker.SetAppBaseURL("https://dev-app.unipost.dev/")

	if err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("process once: %v", err)
	}
	if len(sender.sent) != 1 {
		t.Fatalf("sent = %d, want 1", len(sender.sent))
	}
	if sender.sent[0].IdempotencyKey != "paid_plan_quota:ws_123:2026-07:100" {
		t.Fatalf("idempotency key = %q", sender.sent[0].IdempotencyKey)
	}
	if sender.sent[0].Audit.EventKey != "email.quota.paid_plan_alert.v1" ||
		sender.sent[0].Audit.WorkspaceID != "ws_123" {
		t.Fatalf("audit = %#v", sender.sent[0].Audit)
	}
	if got := sender.sent[0].DataVariables["billing_url"]; got != "https://dev-app.unipost.dev/settings/billing" {
		t.Fatalf("billing_url = %#v", got)
	}
	if got := sender.sent[0].DataVariables["scheduling_allowed"]; got != "false" {
		t.Fatalf("scheduling_allowed = %#v, want Loops-compatible string", got)
	}
	if len(store.sentIDs) != 1 || store.sentIDs[0] != "notification_1" {
		t.Fatalf("sent ids = %#v", store.sentIDs)
	}
}

func TestPaidPlanQuotaEmailWorkerUsesApprovedRetrySchedule(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		attempt int
		delay   time.Duration
		failed  bool
	}{
		{attempt: 1, delay: 5 * time.Minute},
		{attempt: 2, delay: time.Hour},
		{attempt: 3, delay: 6 * time.Hour},
		{attempt: 4, failed: true},
	}
	for _, tt := range tests {
		t.Run(time.Duration(tt.attempt).String(), func(t *testing.T) {
			store := &fakePaidQuotaDeliveryStore{
				deliveries: []PaidQuotaDelivery{paidQuotaDeliveryFixture(tt.attempt)},
			}
			sender := &fakePaidQuotaSender{err: errors.New("provider unavailable")}
			worker := NewPaidPlanQuotaEmailWorker(store, sender, nil, func() time.Time { return now })

			if err := worker.ProcessOnce(context.Background()); err != nil {
				t.Fatalf("process once: %v", err)
			}
			if tt.failed {
				if len(store.failedIDs) != 1 {
					t.Fatalf("failed ids = %#v", store.failedIDs)
				}
				return
			}
			if len(store.retries) != 1 {
				t.Fatalf("retries = %#v", store.retries)
			}
			if got := store.retries[0].NextAttemptAt; !got.Equal(now.Add(tt.delay)) {
				t.Fatalf("next attempt = %s, want %s", got, now.Add(tt.delay))
			}
		})
	}
}

func TestPaidPlanQuotaEmailWorkerHonorsWarningPreferenceAtDeliveryTime(t *testing.T) {
	delivery := paidQuotaDeliveryFixture(1)
	delivery.ThresholdPercent = 90
	delivery.EventKey = "email.quota.paid_plan_warning.v1"
	store := &fakePaidQuotaDeliveryStore{deliveries: []PaidQuotaDelivery{delivery}}
	sender := &fakePaidQuotaSender{}
	worker := NewPaidPlanQuotaEmailWorker(store, sender, nil, nil).
		SetEmailPolicy(fakePaidQuotaDeliveryPolicy{shouldSend: false})

	if err := worker.ProcessOnce(context.Background()); err != nil {
		t.Fatalf("process once: %v", err)
	}
	if len(sender.sent) != 0 {
		t.Fatalf("sent = %d, want warning skipped after preference opt-out", len(sender.sent))
	}
	if len(store.preferenceSkippedIDs) != 1 || store.preferenceSkippedIDs[0] != delivery.ID {
		t.Fatalf("preference skipped ids = %#v", store.preferenceSkippedIDs)
	}
}

func TestPaidPlanQuotaEmailWorkerReconcilesEligibleWorkspaces(t *testing.T) {
	store := &fakePaidQuotaDeliveryStore{workspaceIDs: []string{"ws_1", "ws_2"}}
	evaluator := &fakePaidQuotaEvaluator{}
	now := time.Date(2026, 7, 16, 0, 15, 0, 0, time.UTC)
	worker := NewPaidPlanQuotaEmailWorker(store, &fakePaidQuotaSender{}, evaluator, func() time.Time { return now })

	if err := worker.ReconcileCurrentPeriod(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(evaluator.calls) != 2 {
		t.Fatalf("evaluation calls = %#v", evaluator.calls)
	}
	for _, call := range evaluator.calls {
		if call.period != "2026-07" {
			t.Fatalf("period = %q", call.period)
		}
	}
}

func paidQuotaDeliveryFixture(attempt int) PaidQuotaDelivery {
	return PaidQuotaDelivery{
		ID:               "notification_1",
		WorkspaceID:      "ws_123",
		WorkspaceName:    "Example",
		UserID:           "user_123",
		OwnerEmail:       "owner@example.com",
		OwnerName:        "Owner",
		PlanID:           "basic",
		PlanName:         "Basic",
		Period:           "2026-07",
		ThresholdPercent: 100,
		Severity:         "alert",
		EventKey:         "email.quota.paid_plan_alert.v1",
		TransactionalID:  "txn_paid_quota",
		IdempotencyKey:   "paid_plan_quota:ws_123:2026-07:100",
		CompletedUsage:   90,
		ScheduledUsage:   10,
		QuotaHoldUsage:   2,
		EffectiveUsage:   100,
		PostLimit:        100,
		AttemptCount:     attempt,
	}
}

type fakePaidQuotaDeliveryStore struct {
	deliveries           []PaidQuotaDelivery
	sentIDs              []string
	retries              []PaidQuotaRetry
	failedIDs            []string
	preferenceSkippedIDs []string
	workspaceIDs         []string
}

func (f *fakePaidQuotaDeliveryStore) Claim(context.Context, int) ([]PaidQuotaDelivery, error) {
	out := append([]PaidQuotaDelivery(nil), f.deliveries...)
	f.deliveries = nil
	return out, nil
}

func (f *fakePaidQuotaDeliveryStore) MarkSent(_ context.Context, id string) error {
	f.sentIDs = append(f.sentIDs, id)
	return nil
}

func (f *fakePaidQuotaDeliveryStore) MarkRetry(_ context.Context, retry PaidQuotaRetry) error {
	f.retries = append(f.retries, retry)
	return nil
}

func (f *fakePaidQuotaDeliveryStore) MarkFailed(_ context.Context, id, _ string) error {
	f.failedIDs = append(f.failedIDs, id)
	return nil
}

func (f *fakePaidQuotaDeliveryStore) MarkPreferenceDisabled(_ context.Context, id string) error {
	f.preferenceSkippedIDs = append(f.preferenceSkippedIDs, id)
	return nil
}

func (f *fakePaidQuotaDeliveryStore) ReconciliationWorkspaces(context.Context) ([]string, error) {
	return append([]string(nil), f.workspaceIDs...), nil
}

type fakePaidQuotaSender struct {
	sent []loops.TransactionalEmail
	err  error
}

func (f *fakePaidQuotaSender) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	f.sent = append(f.sent, email)
	return f.err
}

type fakePaidQuotaEvaluator struct {
	calls []struct {
		workspaceID string
		period      string
	}
}

func (f *fakePaidQuotaEvaluator) Evaluate(_ context.Context, workspaceID, period string) error {
	f.calls = append(f.calls, struct {
		workspaceID string
		period      string
	}{workspaceID: workspaceID, period: period})
	return nil
}

type fakePaidQuotaDeliveryPolicy struct {
	shouldSend bool
}

func (f fakePaidQuotaDeliveryPolicy) Prepare(_ context.Context, request emailpolicy.Request) (emailpolicy.Decision, error) {
	return emailpolicy.Decision{
		ShouldSend:    f.shouldSend,
		SkipReason:    emailpolicy.SkipReasonPreferenceDisabled,
		DataVariables: request.DataVariables,
	}, nil
}
