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
