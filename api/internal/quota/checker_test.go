package quota

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestShouldHardBlockFreePlanQuota(t *testing.T) {
	status := QuotaStatus{Allowed: true, Usage: 99, Limit: 100}

	if !shouldHardBlockFreePlanQuota("free", status, 2) {
		t.Fatal("expected free plan to block when accepted posts would exceed quota")
	}
	if shouldHardBlockFreePlanQuota("free", status, 1) {
		t.Fatal("expected free plan to allow the final remaining post")
	}
	if shouldHardBlockFreePlanQuota("api", status, 2) {
		t.Fatal("expected paid plan to keep soft overage behavior")
	}
	if shouldHardBlockFreePlanQuota("free", QuotaStatus{Usage: 100, Limit: -1}, 1) {
		t.Fatal("expected unlimited quota to stay unblocked")
	}
}

func TestMonthlySnapshotUsesCompletedAndCommittedScheduledUsage(t *testing.T) {
	snapshot := MonthlySnapshot{
		Completed: 98,
		Scheduled: 2,
		QuotaHold: 1,
		Limit:     100,
	}

	if got := snapshot.EffectiveUsage(); got != 100 {
		t.Fatalf("effective usage = %d, want 100", got)
	}
	if got := snapshot.EffectivePercentage(); got != 100 {
		t.Fatalf("effective percentage = %v, want 100", got)
	}
	if !snapshot.Reached(100) {
		t.Fatal("expected exact 100% to reach the 100 threshold")
	}
	if snapshot.Reached(105) {
		t.Fatal("expected exact 100% not to reach the 105 threshold")
	}
}

func TestMonthlySnapshotProjectsAtomicScheduleReplacement(t *testing.T) {
	snapshot := MonthlySnapshot{
		Completed: 98,
		Scheduled: 2,
		QuotaHold: 1,
		Limit:     100,
	}

	if !snapshot.WouldExceed(0, 1) {
		t.Fatal("expected one net-new scheduled unit above 100% to exceed")
	}
	if snapshot.WouldExceed(1, 1) {
		t.Fatal("expected atomic release and reservation at 100% to remain allowed")
	}
	if snapshot.WouldExceed(2, 1) {
		t.Fatal("expected net release to remain allowed")
	}
}

func TestMonthlySnapshotUnlimitedPlanNeverExceeds(t *testing.T) {
	snapshot := MonthlySnapshot{
		Completed: 100_000,
		Scheduled: 100_000,
		Limit:     -1,
	}

	if snapshot.WouldExceed(0, 100_000) {
		t.Fatal("expected unlimited plan not to exceed")
	}
	if snapshot.Reached(80) {
		t.Fatal("expected unlimited plan not to reach finite quota thresholds")
	}
}

func TestCheckerMonthlySnapshotForPeriodIncludesScheduledAndHeldUnits(t *testing.T) {
	checker := NewChecker(db.New(&fakeQuotaDB{
		planID:         "basic",
		limit:          2500,
		usage:          2488,
		scheduledUnits: 12,
		quotaHoldUnits: 3,
	}))

	snapshot, err := checker.MonthlySnapshotForPeriod(context.Background(), "ws_123", "2026-07")
	if err != nil {
		t.Fatalf("monthly snapshot: %v", err)
	}
	if snapshot.WorkspaceID != "ws_123" || snapshot.PlanID != "basic" || snapshot.Period != "2026-07" {
		t.Fatalf("snapshot identity = %#v", snapshot)
	}
	if snapshot.Completed != 2488 || snapshot.Scheduled != 12 || snapshot.QuotaHold != 3 || snapshot.Limit != 2500 {
		t.Fatalf("snapshot counts = %#v", snapshot)
	}
}

func TestFreePlanHardBlockStatusAlwaysBlocksProjectedFreePlanOverage(t *testing.T) {
	checker := NewChecker(db.New(&fakeQuotaDB{
		planID: "free",
		limit:  100,
		usage:  99,
	}))

	status, blocked := checker.FreePlanHardBlockStatus(context.Background(), "ws_123", 2)
	if !blocked {
		t.Fatal("expected free plan to block projected overage")
	}
	if status.Usage != 99 || status.Limit != 100 {
		t.Fatalf("status = usage %d limit %d, want 99/100", status.Usage, status.Limit)
	}
}

func TestFreePlanHardBlockStatusKeepsPaidPlansSoftOverage(t *testing.T) {
	checker := NewChecker(db.New(&fakeQuotaDB{
		planID: "api",
		limit:  100,
		usage:  99,
	}))

	_, blocked := checker.FreePlanHardBlockStatus(context.Background(), "ws_123", 2)
	if blocked {
		t.Fatal("expected API plan to keep soft-overage behavior")
	}
}

func TestFreePlanHardBlockStatusIncludesScheduledReservations(t *testing.T) {
	checker := NewChecker(db.New(&fakeQuotaDB{
		planID:         "free",
		limit:          100,
		usage:          98,
		scheduledUnits: 2,
	}))

	status, blocked := checker.FreePlanHardBlockStatus(context.Background(), "ws_123", 1)
	if !blocked {
		t.Fatal("expected free plan hard cap to include already scheduled posts")
	}
	if status.Reserved != 2 {
		t.Fatalf("reserved = %d, want 2", status.Reserved)
	}
}

func TestFreePlanHardBlockGateProjectsBulkAccumulation(t *testing.T) {
	gate := FreePlanHardBlockGate{
		Status:  QuotaStatus{Allowed: true, Usage: 0, Limit: 100},
		planID:  "free",
		enabled: true,
	}

	accepted := 0
	for i := 0; i < 100; i++ {
		if gate.Blocked(accepted + 1) {
			t.Fatalf("post %d should still be accepted", i+1)
		}
		accepted++
	}
	if !gate.Blocked(accepted + 1) {
		t.Fatal("101st projected post should be blocked")
	}
}

type fakeQuotaDB struct {
	planID         string
	limit          int32
	usage          int32
	scheduledUnits int32
	quotaHoldUnits int32
}

func (f *fakeQuotaDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (f *fakeQuotaDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}

func (f *fakeQuotaDB) QueryRow(_ context.Context, sql string, _ ...interface{}) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM subscriptions"):
		return fakeQuotaRow{values: []any{
			"sub_123",
			f.planID,
			pgtype.Text{},
			pgtype.Text{},
			"active",
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			pgtype.Bool{},
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			false,
			"ws_123",
		}}
	case strings.Contains(sql, "FROM plans"):
		return fakeQuotaRow{values: []any{
			f.planID,
			"Free",
			int32(0),
			f.limit,
			pgtype.Text{},
			pgtype.Timestamptz{},
			false,
			false,
			false,
			false,
			pgtype.Int4{},
			pgtype.Int4{},
		}}
	case strings.Contains(sql, "FROM usage"):
		return fakeQuotaRow{values: []any{
			"usage_123",
			currentPeriod(),
			f.usage,
			pgtype.Timestamptz{},
			pgtype.Timestamptz{},
			"ws_123",
		}}
	case strings.Contains(sql, "sp.status = 'quota_hold'"):
		return fakeQuotaRow{values: []any{f.quotaHoldUnits}}
	case strings.Contains(sql, "FROM social_posts"):
		return fakeQuotaRow{values: []any{f.scheduledUnits}}
	default:
		return fakeQuotaRow{err: errors.New("unexpected query row")}
	}
}

type fakeQuotaRow struct {
	values []any
	err    error
}

func (r fakeQuotaRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected scan destination count")
	}
	for i, value := range r.values {
		switch d := dest[i].(type) {
		case *string:
			*d = value.(string)
		case *int32:
			*d = value.(int32)
		case *bool:
			*d = value.(bool)
		case *pgtype.Text:
			*d = value.(pgtype.Text)
		case *pgtype.Timestamptz:
			*d = value.(pgtype.Timestamptz)
		case *pgtype.Bool:
			*d = value.(pgtype.Bool)
		case *pgtype.Int4:
			*d = value.(pgtype.Int4)
		default:
			return errors.New("unsupported scan destination")
		}
	}
	return nil
}
