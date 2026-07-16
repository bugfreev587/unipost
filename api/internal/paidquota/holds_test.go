package paidquota

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestScheduledParentsQueryPreservesQuotaSemantics(t *testing.T) {
	sql := strings.ToLower(listScheduledParentsForPeriodSQL)
	for _, want := range []string{
		"status in ('scheduled', 'quota_hold', 'publishing')",
		"scheduled_at is not null",
		"disconnected_at is null",
		"admin_post_quota_resets",
		"order by sp.scheduled_at, sp.created_at, sp.id",
		"for update of sp",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("scheduled parents query missing %q, got:\n%s", want, listScheduledParentsForPeriodSQL)
		}
	}
}

func TestAllocateQuotaHoldsPrioritizesCompletedGrandfatheredAndPublishing(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	parents := []ScheduledParent{
		{
			ID:          "publishing",
			Status:      "publishing",
			ScheduledAt: effectiveAt.Add(4 * time.Hour),
			CreatedAt:   effectiveAt.Add(-72 * time.Hour),
			Units:       2,
		},
		{
			ID:          "grandfathered",
			Status:      "scheduled",
			ScheduledAt: effectiveAt.Add(-time.Hour),
			CreatedAt:   effectiveAt.Add(-48 * time.Hour),
			Units:       2,
		},
		{
			ID:          "fits",
			Status:      "scheduled",
			ScheduledAt: effectiveAt.Add(24 * time.Hour),
			CreatedAt:   effectiveAt.Add(-24 * time.Hour),
			Units:       1,
		},
		{
			ID:          "held",
			Status:      "scheduled",
			ScheduledAt: effectiveAt.Add(48 * time.Hour),
			CreatedAt:   effectiveAt.Add(-12 * time.Hour),
			Units:       2,
		},
	}

	got := AllocateQuotaHolds(5, 10, effectiveAt, effectiveAt, parents)
	want := []HoldDecision{
		{PostID: "grandfathered", Status: "scheduled"},
		{PostID: "publishing", Status: "publishing"},
		{PostID: "fits", Status: "scheduled"},
		{PostID: "held", Status: "quota_hold"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

func TestAllocateQuotaHoldsKeepsParentAtomicAndDeterministic(t *testing.T) {
	effectiveAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	parents := []ScheduledParent{
		{ID: "later", Status: "scheduled", ScheduledAt: effectiveAt.Add(48 * time.Hour), CreatedAt: effectiveAt.Add(-time.Hour), Units: 1},
		{ID: "large", Status: "scheduled", ScheduledAt: effectiveAt.Add(24 * time.Hour), CreatedAt: effectiveAt.Add(-time.Hour), Units: 3},
		{ID: "small", Status: "scheduled", ScheduledAt: effectiveAt.Add(24 * time.Hour), CreatedAt: effectiveAt.Add(-2 * time.Hour), Units: 2},
	}

	got := AllocateQuotaHolds(5, 8, effectiveAt, effectiveAt, parents)
	want := []HoldDecision{
		{PostID: "small", Status: "scheduled"},
		{PostID: "large", Status: "quota_hold"},
		{PostID: "later", Status: "scheduled"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

func TestAllocateQuotaHoldsReleasesFutureHoldButNeverPastDueHold(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	parents := []ScheduledParent{
		{ID: "past", Status: "quota_hold", ScheduledAt: now.Add(-time.Hour), CreatedAt: now.Add(-48 * time.Hour), Units: 1},
		{ID: "future", Status: "quota_hold", ScheduledAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-24 * time.Hour), Units: 1},
	}

	got := AllocateQuotaHolds(0, 10, time.Time{}, now, parents)
	want := []HoldDecision{
		{PostID: "past", Status: "quota_hold"},
		{PostID: "future", Status: "scheduled"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

func TestAllocateQuotaHoldsCountsPastDueHoldBeforeReleasingFutureCapacity(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	parents := []ScheduledParent{
		{ID: "past", Status: "quota_hold", ScheduledAt: now.Add(-time.Hour), CreatedAt: now.Add(-48 * time.Hour), Units: 1},
		{ID: "future", Status: "quota_hold", ScheduledAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-24 * time.Hour), Units: 1},
	}

	got := AllocateQuotaHolds(9, 10, time.Time{}, now, parents)
	want := []HoldDecision{
		{PostID: "past", Status: "quota_hold"},
		{PostID: "future", Status: "quota_hold"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decisions = %#v, want %#v", got, want)
	}
}

func TestHoldServiceReleasesExistingHoldsOnUnlimitedEnterprisePlan(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tx := &fakeHoldPeriodTx{
		snapshot: quota.MonthlySnapshot{PlanID: "enterprise", Period: "2026-07", Limit: -1},
		parents: []ScheduledParent{
			{ID: "release", Status: "quota_hold", ScheduledAt: now.Add(24 * time.Hour), CreatedAt: now, Units: 2},
		},
	}
	service := NewHoldService(&fakeHoldStore{tx: tx}, func() time.Time { return now })

	if err := service.ReconcilePeriod(context.Background(), "ws_123", "2026-07", "plan_upgrade", time.Time{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !reflect.DeepEqual(tx.released, []string{"release"}) {
		t.Fatalf("released = %#v, want enterprise to release future holds", tx.released)
	}
}

func TestHoldServiceCanReconcileAgainstTargetPlanBeforeSubscriptionUpdate(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tx := &fakeHoldPeriodTx{
		snapshot: quota.MonthlySnapshot{
			PlanID:    "growth",
			Period:    "2026-07",
			Completed: 8,
			Limit:     7500,
		},
		parents: []ScheduledParent{
			{ID: "fits", Status: "scheduled", ScheduledAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-time.Hour), Units: 2},
			{ID: "hold", Status: "scheduled", ScheduledAt: now.Add(48 * time.Hour), CreatedAt: now, Units: 1},
		},
	}
	service := NewHoldService(&fakeHoldStore{tx: tx}, func() time.Time { return now })

	if err := service.ReconcileWorkspaceForPlan(context.Background(), "ws_123", "basic", 10, "plan_downgrade", now); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !reflect.DeepEqual(tx.held, []string{"hold"}) {
		t.Fatalf("held = %#v, want target-plan limit applied before subscription update", tx.held)
	}
}

func TestHoldServiceAppliesHoldAndReleaseDecisions(t *testing.T) {
	now := time.Date(2026, 7, 16, 12, 0, 0, 0, time.UTC)
	tx := &fakeHoldPeriodTx{
		snapshot: quota.MonthlySnapshot{
			WorkspaceID: "ws_123",
			PlanID:      "basic",
			Period:      "2026-07",
			Completed:   8,
			Scheduled:   4,
			Limit:       10,
		},
		parents: []ScheduledParent{
			{ID: "keep", Status: "scheduled", ScheduledAt: now.Add(24 * time.Hour), CreatedAt: now.Add(-2 * time.Hour), Units: 2},
			{ID: "hold", Status: "scheduled", ScheduledAt: now.Add(48 * time.Hour), CreatedAt: now.Add(-time.Hour), Units: 2},
			{ID: "release", Status: "quota_hold", ScheduledAt: now.Add(72 * time.Hour), CreatedAt: now, Units: 1},
		},
	}
	service := NewHoldService(&fakeHoldStore{tx: tx}, func() time.Time { return now })

	err := service.ReconcilePeriod(context.Background(), "ws_123", "2026-07", "plan_downgrade", now)
	if err != nil {
		t.Fatalf("reconcile period: %v", err)
	}
	if !reflect.DeepEqual(tx.held, []string{"hold"}) {
		t.Fatalf("held = %#v, want hold", tx.held)
	}
	if len(tx.released) != 0 {
		t.Fatalf("released = %#v, want none because capacity is exhausted", tx.released)
	}

	tx.snapshot.Completed = 5
	tx.held = nil
	err = service.ReconcilePeriod(context.Background(), "ws_123", "2026-07", "capacity_released", time.Time{})
	if err != nil {
		t.Fatalf("reconcile released capacity: %v", err)
	}
	if !reflect.DeepEqual(tx.released, []string{"hold", "release"}) {
		t.Fatalf("released = %#v, want future hold releases", tx.released)
	}
}

type fakeHoldStore struct {
	tx HoldPeriodTransaction
}

func (f *fakeHoldStore) WithinPeriod(_ context.Context, _, _ string, fn func(HoldPeriodTransaction) error) error {
	return fn(f.tx)
}

type fakeHoldPeriodTx struct {
	snapshot quota.MonthlySnapshot
	parents  []ScheduledParent
	held     []string
	released []string
}

func (f *fakeHoldPeriodTx) Snapshot(context.Context) (quota.MonthlySnapshot, error) {
	return f.snapshot, nil
}

func (f *fakeHoldPeriodTx) Parents(context.Context) ([]ScheduledParent, error) {
	return append([]ScheduledParent(nil), f.parents...), nil
}

func (f *fakeHoldPeriodTx) SetHold(_ context.Context, postID, _ string) error {
	f.held = append(f.held, postID)
	for i := range f.parents {
		if f.parents[i].ID == postID {
			f.parents[i].Status = "quota_hold"
		}
	}
	return nil
}

func (f *fakeHoldPeriodTx) ReleaseHold(_ context.Context, postID string) error {
	f.released = append(f.released, postID)
	for i := range f.parents {
		if f.parents[i].ID == postID {
			f.parents[i].Status = "scheduled"
		}
	}
	return nil
}
