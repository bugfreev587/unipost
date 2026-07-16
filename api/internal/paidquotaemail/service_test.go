package paidquotaemail

import (
	"context"
	"reflect"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/emailpolicy"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestEvaluateCreatesHighestReachedThresholdAndSupersedesLowerOnes(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "basic",
				Period:      "2026-07",
				Completed:   108,
				Limit:       100,
			},
			UserID:     "user_123",
			OwnerEmail: "owner@example.com",
		},
		decisions: map[int]string{},
	}
	service := NewService(store, &fakePolicy{shouldSend: true}, "txn_paid_quota")

	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	wantStatuses := map[int]string{
		80:  "skipped_superseded",
		90:  "skipped_superseded",
		100: "skipped_superseded",
		105: "pending",
	}
	if !reflect.DeepEqual(store.createdStatuses(), wantStatuses) {
		t.Fatalf("created = %#v, want %#v", store.createdStatuses(), wantStatuses)
	}
	highest := store.created[105]
	if highest.EventKey != PaidAlertEventKey || highest.Severity != "alert" {
		t.Fatalf("highest decision = %#v", highest)
	}
}

func TestEvaluateSupersedesExistingLowerPendingDecision(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "basic",
				Period:      "2026-07",
				Completed:   2250,
				Limit:       2500,
			},
			UserID:     "user_123",
			OwnerEmail: "owner@example.com",
		},
		decisions: map[int]string{80: "retry_wait"},
	}
	service := NewService(store, nil, "template_123")
	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatal(err)
	}
	if got := store.decisions[80]; got != "skipped_superseded" {
		t.Fatalf("80%% decision = %q, want skipped_superseded", got)
	}
	if got := store.created[90].Status; got != "pending" {
		t.Fatalf("90%% decision = %q, want pending", got)
	}
}

func TestEvaluateWarningRespectsPreference(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "api",
				Period:      "2026-07",
				Completed:   90,
				Limit:       100,
			},
			UserID:     "user_123",
			OwnerEmail: "owner@example.com",
		},
		decisions: map[int]string{},
	}
	service := NewService(store, &fakePolicy{shouldSend: false, skipReason: emailpolicy.SkipReasonPreferenceDisabled}, "txn_paid_quota")

	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got := store.created[90].Status; got != "skipped_preference_disabled" {
		t.Fatalf("90 status = %q", got)
	}
	if got := store.created[80].Status; got != "skipped_superseded" {
		t.Fatalf("80 status = %q", got)
	}
}

func TestEvaluateRequiredAlertIgnoresDisabledOptionalPreference(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "growth",
				Period:      "2026-07",
				Completed:   100,
				Limit:       100,
			},
			UserID:     "user_123",
			OwnerEmail: "owner@example.com",
		},
		decisions: map[int]string{},
	}
	policy := &fakePolicy{shouldSend: false, skipReason: emailpolicy.SkipReasonPreferenceDisabled}
	service := NewService(store, policy, "txn_paid_quota")

	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got := store.created[100].Status; got != "pending" {
		t.Fatalf("100 status = %q, want pending required alert", got)
	}
	if policy.eventKeys[len(policy.eventKeys)-1] != PaidAlertEventKey {
		t.Fatalf("policy event keys = %#v", policy.eventKeys)
	}
}

func TestEvaluateMissingRecipientCreatesFinalSkip(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "basic",
				Period:      "2026-07",
				Completed:   80,
				Limit:       100,
			},
			UserID: "user_123",
		},
		decisions: map[int]string{},
	}
	service := NewService(store, &fakePolicy{shouldSend: true}, "txn_paid_quota")

	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if got := store.created[80].Status; got != "skipped_missing_recipient" {
		t.Fatalf("80 status = %q", got)
	}
}

func TestEvaluateIsIdempotentAndCreates120FollowUp(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "basic",
				Period:      "2026-07",
				Completed:   125,
				Limit:       100,
			},
			UserID:     "user_123",
			OwnerEmail: "owner@example.com",
		},
		decisions: map[int]string{},
	}
	service := NewService(store, &fakePolicy{shouldSend: true}, "txn_paid_quota")

	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("first evaluate: %v", err)
	}
	if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("second evaluate: %v", err)
	}
	if len(store.created) != 7 {
		t.Fatalf("created decisions = %d, want seven final threshold decisions", len(store.created))
	}
	if store.created[120].Status != "pending" || store.created[120].Severity != "critical_alert" {
		t.Fatalf("120 decision = %#v", store.created[120])
	}
	if store.followUpCalls != 2 {
		t.Fatalf("follow-up attempts = %d, want idempotent ensure on each evaluation", store.followUpCalls)
	}
}

func TestEvaluateExcludesFreeTeamAndEnterprise(t *testing.T) {
	for _, planID := range []string{"free", "team", "enterprise"} {
		t.Run(planID, func(t *testing.T) {
			store := &fakeStore{
				snapshot: Snapshot{
					MonthlySnapshot: quota.MonthlySnapshot{
						WorkspaceID: "ws_123",
						PlanID:      planID,
						Period:      "2026-07",
						Completed:   200,
						Limit:       100,
					},
					UserID:     "user_123",
					OwnerEmail: "owner@example.com",
				},
				decisions: map[int]string{},
			}
			service := NewService(store, &fakePolicy{shouldSend: true}, "txn_paid_quota")
			if err := service.Evaluate(context.Background(), "ws_123", "2026-07"); err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			if len(store.created) != 0 {
				t.Fatalf("created = %#v, want none", store.created)
			}
		})
	}
}

func TestResolveFollowUpsBelowLimitAfterPlanUpgrade(t *testing.T) {
	store := &fakeStore{
		snapshot: Snapshot{
			MonthlySnapshot: quota.MonthlySnapshot{
				WorkspaceID: "ws_123",
				PlanID:      "growth",
				Period:      "2026-07",
				Completed:   100,
				Scheduled:   20,
				Limit:       7500,
			},
		},
	}
	service := NewService(store, nil, "txn_paid_quota")

	if err := service.ResolveFollowUpsBelowLimit(context.Background(), "ws_123", "2026-07"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if store.resolveCalls != 1 {
		t.Fatalf("resolve calls = %d, want 1", store.resolveCalls)
	}
}

type fakeStore struct {
	snapshot      Snapshot
	decisions     map[int]string
	created       map[int]Decision
	followUpCalls int
	resolveCalls  int
}

func (f *fakeStore) Snapshot(context.Context, string, string) (Snapshot, error) {
	return f.snapshot, nil
}

func (f *fakeStore) Decisions(context.Context, string, string) (map[int]string, error) {
	out := make(map[int]string, len(f.decisions)+len(f.created))
	for threshold, status := range f.decisions {
		out[threshold] = status
	}
	for threshold, decision := range f.created {
		out[threshold] = decision.Status
	}
	return out, nil
}

func (f *fakeStore) CreateDecision(_ context.Context, decision Decision) (bool, error) {
	if f.created == nil {
		f.created = map[int]Decision{}
	}
	if _, exists := f.decisions[decision.ThresholdPercent]; exists {
		return false, nil
	}
	if _, exists := f.created[decision.ThresholdPercent]; exists {
		return false, nil
	}
	f.created[decision.ThresholdPercent] = decision
	return true, nil
}

func (f *fakeStore) MarkLowerPendingSuperseded(_ context.Context, _ string, _ string, threshold int) error {
	for candidate, status := range f.decisions {
		if candidate < threshold && (status == "pending" || status == "retry_wait") {
			f.decisions[candidate] = "skipped_superseded"
		}
	}
	return nil
}

func (f *fakeStore) EnsureFollowUp(context.Context, Snapshot, Decision) error {
	f.followUpCalls++
	return nil
}

func (f *fakeStore) ResolveFollowUpsBelowLimit(context.Context, string, string) error {
	f.resolveCalls++
	return nil
}

func (f *fakeStore) createdStatuses() map[int]string {
	out := make(map[int]string, len(f.created))
	for threshold, decision := range f.created {
		out[threshold] = decision.Status
	}
	return out
}

type fakePolicy struct {
	shouldSend bool
	skipReason string
	eventKeys  []string
}

func (f *fakePolicy) Prepare(_ context.Context, request emailpolicy.Request) (emailpolicy.Decision, error) {
	f.eventKeys = append(f.eventKeys, request.EventKey)
	return emailpolicy.Decision{
		ShouldSend: f.shouldSend,
		SkipReason: f.skipReason,
	}, nil
}
