package paidquota

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

func TestDecisionForPaidScheduleAdmission(t *testing.T) {
	tests := []struct {
		name      string
		planID    string
		current   int
		released  int
		requested int
		limit     int
		allowed   bool
	}{
		{name: "exactly 100 allowed", planID: "basic", current: 2499, requested: 1, limit: 2500, allowed: true},
		{name: "over 100 rejected", planID: "basic", current: 2500, requested: 1, limit: 2500, allowed: false},
		{name: "atomic replacement allowed", planID: "basic", current: 2500, released: 2, requested: 2, limit: 2500, allowed: true},
		{name: "api included", planID: "api", current: 999, requested: 1, limit: 1000, allowed: true},
		{name: "growth included", planID: "growth", current: 7500, requested: 1, limit: 7500, allowed: false},
		{name: "team excluded", planID: "team", current: 999999, requested: 10, limit: -1, allowed: true},
		{name: "enterprise excluded", planID: "enterprise", current: 999999, requested: 10, limit: 1000, allowed: true},
		{name: "free delegated", planID: "free", current: 100, requested: 1, limit: 100, allowed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := quota.MonthlySnapshot{
				PlanID:    tt.planID,
				Completed: tt.current,
				Limit:     tt.limit,
			}
			decision := Decide(snapshot, tt.released, tt.requested)
			if decision.Allowed != tt.allowed {
				t.Fatalf("allowed = %v, want %v; decision=%#v", decision.Allowed, tt.allowed, decision)
			}
		})
	}
}

func TestNormalizePeriodDeltasSortsAndCombinesPeriods(t *testing.T) {
	got := normalizePeriodDeltas([]PeriodDelta{
		{Period: "2026-08", RequestedUnits: 2},
		{Period: "2026-07", ReleasedUnits: 1},
		{Period: "2026-08", ReleasedUnits: 3},
		{Period: "", RequestedUnits: 99},
	})
	want := []PeriodDelta{
		{Period: "2026-07", ReleasedUnits: 1},
		{Period: "2026-08", ReleasedUnits: 3, RequestedUnits: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized deltas = %#v, want %#v", got, want)
	}
}

func TestAdmissionErrorCarriesRejectedSnapshot(t *testing.T) {
	snapshot := quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   2490,
		Scheduled:   10,
		QuotaHold:   2,
		Limit:       2500,
	}
	err := NewAdmissionError(snapshot, 1)

	if err.Snapshot != snapshot || err.RequestedUnits != 1 {
		t.Fatalf("admission error = %#v", err)
	}
	if err.Error() == "" {
		t.Fatal("expected admission error message")
	}
}

func TestDecideRejectsNetNewSchedulingWhileQuotaHoldsExist(t *testing.T) {
	snapshot := quota.MonthlySnapshot{
		WorkspaceID: "ws_123",
		PlanID:      "basic",
		Period:      "2026-07",
		Completed:   70,
		Scheduled:   10,
		QuotaHold:   5,
		Limit:       100,
	}
	if decision := Decide(snapshot, 0, 1); decision.Allowed {
		t.Fatalf("net-new schedule should be rejected while holds exist: %#v", decision)
	}
	if decision := Decide(snapshot, 3, 3); !decision.Allowed {
		t.Fatalf("capacity-neutral edit should remain allowed: %#v", decision)
	}
}

func TestCoordinatorLocksPeriodsInOrderAndCommitsAllowedMutation(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 98, Scheduled: 1, Limit: 100},
			"2026-08": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-08", Completed: 20, Scheduled: 10, Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	mutated := false

	err := coordinator.Mutate(context.Background(), "ws_123", []PeriodDelta{
		{Period: "2026-08", RequestedUnits: 1},
		{Period: "2026-07", RequestedUnits: 1},
	}, func(*db.Queries) error {
		mutated = true
		return nil
	})
	if err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if !reflect.DeepEqual(tx.locked, []string{"2026-07", "2026-08"}) {
		t.Fatalf("locked periods = %#v, want sorted periods", tx.locked)
	}
	if !mutated || !tx.committed || tx.rolledBack {
		t.Fatalf("transaction state mutated=%v committed=%v rolledBack=%v", mutated, tx.committed, tx.rolledBack)
	}
}

func TestCoordinatorRejectsOverCapBeforeMutationAndRollsBack(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 100, Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	mutated := false

	err := coordinator.Mutate(context.Background(), "ws_123", []PeriodDelta{
		{Period: "2026-07", RequestedUnits: 1},
	}, func(*db.Queries) error {
		mutated = true
		return nil
	})
	var admissionErr *AdmissionError
	if !errors.As(err, &admissionErr) {
		t.Fatalf("error = %v, want AdmissionError", err)
	}
	if mutated || tx.committed || !tx.rolledBack {
		t.Fatalf("transaction state mutated=%v committed=%v rolledBack=%v", mutated, tx.committed, tx.rolledBack)
	}
}

func TestCoordinatorRollsBackMutationFailure(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 99, Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	wantErr := errors.New("mutation failed")

	err := coordinator.Mutate(context.Background(), "ws_123", []PeriodDelta{
		{Period: "2026-07", RequestedUnits: 1},
	}, func(*db.Queries) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("transaction state committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}

func TestCoordinatorPlansDeltaAfterLocksAreHeld(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 98, Scheduled: 2, Limit: 100},
			"2026-08": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-08", Completed: 20, Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	plannedAfterLocks := false
	mutated := false

	err := coordinator.MutatePlanned(
		context.Background(),
		"ws_123",
		[]string{"2026-08", "2026-07"},
		func(*db.Queries) ([]PeriodDelta, error) {
			plannedAfterLocks = reflect.DeepEqual(tx.locked, []string{"2026-07", "2026-08"})
			return []PeriodDelta{
				{Period: "2026-07", ReleasedUnits: 2},
				{Period: "2026-08", RequestedUnits: 2},
			}, nil
		},
		func(*db.Queries) error {
			mutated = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("mutate planned: %v", err)
	}
	if !plannedAfterLocks || !mutated || !tx.committed {
		t.Fatalf("plannedAfterLocks=%v mutated=%v committed=%v", plannedAfterLocks, mutated, tx.committed)
	}
}

func TestCoordinatorRejectsPlannerDeltaForUnlockedPeriod(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})

	err := coordinator.MutatePlanned(
		context.Background(),
		"ws_123",
		[]string{"2026-07"},
		func(*db.Queries) ([]PeriodDelta, error) {
			return []PeriodDelta{{Period: "2026-08", RequestedUnits: 1}}, nil
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "unlocked period") {
		t.Fatalf("error = %v, want unlocked period rejection", err)
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("transaction state committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}

type fakeBeginner struct {
	tx  transaction
	err error
}

func (f *fakeBeginner) Begin(context.Context) (transaction, error) {
	return f.tx, f.err
}

type fakeTransaction struct {
	snapshots  map[string]quota.MonthlySnapshot
	locked     []string
	committed  bool
	rolledBack bool
}

func (f *fakeTransaction) LockPeriod(_ context.Context, _, period string) error {
	f.locked = append(f.locked, period)
	return nil
}

func (f *fakeTransaction) Snapshot(_ context.Context, _, period string) (quota.MonthlySnapshot, error) {
	snapshot, ok := f.snapshots[period]
	if !ok {
		return quota.MonthlySnapshot{}, errors.New("snapshot missing")
	}
	return snapshot, nil
}

func (f *fakeTransaction) Queries() *db.Queries {
	return nil
}

func (f *fakeTransaction) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTransaction) Rollback(context.Context) error {
	f.rolledBack = true
	return nil
}
