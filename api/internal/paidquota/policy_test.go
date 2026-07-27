package paidquota

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

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
		{name: "free growth blocked atomically", planID: "free", current: 100, requested: 1, limit: 100, allowed: false},
		{name: "over limit net decrease allowed", planID: "basic", current: 110, released: 2, requested: 1, limit: 100, allowed: true},
		{name: "over limit net zero allowed", planID: "free", current: 110, released: 1, requested: 1, limit: 100, allowed: true},
		{name: "over limit release only allowed", planID: "basic", current: 110, released: 2, requested: 0, limit: 100, allowed: true},
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

func TestScheduledIdempotencyCoordinatorLocksKeyBeforeSortedPeriods(t *testing.T) {
	tx := &fakeTransaction{snapshots: map[string]quota.MonthlySnapshot{
		"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Limit: 100},
		"2026-08": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-08", Limit: 100},
	}}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	mutated := false
	admitted := false
	handled, err := coordinator.MutateScheduledIdempotent(
		context.Background(),
		"ws_123",
		"idem_123",
		func(TransactionContext) (bool, []PeriodDelta, error) {
			tx.lockEvents = append(tx.lockEvents, "planner")
			return false, []PeriodDelta{{Period: "2026-08", RequestedUnits: 1}, {Period: "2026-07", RequestedUnits: 1}}, nil
		},
		func(TransactionContext) error {
			admitted = true
			tx.lockEvents = append(tx.lockEvents, "enqueue")
			return nil
		},
		func(*db.Queries) error { mutated = true; return nil },
	)
	if err != nil || handled {
		t.Fatalf("MutateScheduledIdempotent handled=%v err=%v, want false/nil", handled, err)
	}
	wantLocks := []string{"idempotency:idem_123", "planner", "period:2026-07", "period:2026-08", "snapshot:2026-07", "snapshot:2026-08", "enqueue"}
	if !reflect.DeepEqual(tx.lockEvents, wantLocks) {
		t.Fatalf("lock order = %#v, want %#v", tx.lockEvents, wantLocks)
	}
	if !admitted || !mutated || !tx.committed || tx.rolledBack {
		t.Fatalf("transaction state mutated=%v committed=%v rolledBack=%v", mutated, tx.committed, tx.rolledBack)
	}
}

func TestScheduledIdempotencyCoordinatorReplayCommitsBeforeCapacityGates(t *testing.T) {
	tx := &fakeTransaction{snapshots: map[string]quota.MonthlySnapshot{
		"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 100, Limit: 100},
	}}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	mutated := false
	handled, err := coordinator.MutateScheduledIdempotent(
		context.Background(),
		"ws_123",
		"idem_123",
		func(TransactionContext) (bool, []PeriodDelta, error) { return true, nil, nil },
		nil,
		func(*db.Queries) error { mutated = true; return nil },
	)
	if err != nil || !handled {
		t.Fatalf("MutateScheduledIdempotent handled=%v err=%v, want true/nil", handled, err)
	}
	if !reflect.DeepEqual(tx.lockEvents, []string{"idempotency:idem_123"}) {
		t.Fatalf("locks = %#v, want idempotency lock only", tx.lockEvents)
	}
	if mutated || !tx.committed || tx.rolledBack {
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

func TestCoordinatorRollsBackWithIndependentBoundedContextAfterRequestCancellation(t *testing.T) {
	tx := &fakeTransaction{
		snapshots: map[string]quota.MonthlySnapshot{
			"2026-07": {WorkspaceID: "ws_123", PlanID: "basic", Period: "2026-07", Completed: 99, Limit: 100},
		},
	}
	coordinator := newCoordinator(&fakeBeginner{tx: tx})
	ctx, cancel := context.WithCancel(context.Background())
	wantErr := errors.New("request canceled during mutation")
	startedAt := time.Now()

	err := coordinator.Mutate(ctx, "ws_123", []PeriodDelta{
		{Period: "2026-07", RequestedUnits: 1},
	}, func(*db.Queries) error {
		cancel()
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want %v", err, wantErr)
	}
	if tx.rollbackContextErr != nil {
		t.Fatalf("rollback context error = %v, want nil", tx.rollbackContextErr)
	}
	if !tx.rollbackHasDeadline {
		t.Fatal("rollback context must have a deadline")
	}
	if tx.rollbackDeadline.Before(startedAt) || tx.rollbackDeadline.After(startedAt.Add(10*time.Second)) {
		t.Fatalf("rollback deadline = %v, want a live deadline within 10s of %v", tx.rollbackDeadline, startedAt)
	}
	if tx.lockHeld {
		t.Fatal("quota advisory lock remained held after rollback")
	}
	if !tx.rolledBack {
		t.Fatal("transaction was not rolled back")
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
	lockEvents []string
	committed  bool
	rolledBack bool
	lockHeld   bool

	rollbackContextErr  error
	rollbackDeadline    time.Time
	rollbackHasDeadline bool
}

func (f *fakeTransaction) LockScheduledIdempotency(_ context.Context, _, idempotencyKey string) error {
	f.lockEvents = append(f.lockEvents, "idempotency:"+idempotencyKey)
	return nil
}

func (f *fakeTransaction) LockPeriod(_ context.Context, _, period string) error {
	f.locked = append(f.locked, period)
	f.lockEvents = append(f.lockEvents, "period:"+period)
	f.lockHeld = true
	return nil
}

func (f *fakeTransaction) Snapshot(_ context.Context, _, period string) (quota.MonthlySnapshot, error) {
	f.lockEvents = append(f.lockEvents, "snapshot:"+period)
	snapshot, ok := f.snapshots[period]
	if !ok {
		return quota.MonthlySnapshot{}, errors.New("snapshot missing")
	}
	return snapshot, nil
}

func (f *fakeTransaction) DBTX() db.DBTX { return nil }

func (f *fakeTransaction) Queries() *db.Queries {
	return nil
}

func (f *fakeTransaction) Commit(context.Context) error {
	f.committed = true
	return nil
}

func (f *fakeTransaction) Rollback(ctx context.Context) error {
	f.rollbackContextErr = ctx.Err()
	f.rollbackDeadline, f.rollbackHasDeadline = ctx.Deadline()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.rolledBack = true
	f.lockHeld = false
	return nil
}
