package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

// leaseNoopDB satisfies db.DBTX. The heartbeat's renewal tick
// (leaseRenewInterval) is far longer than this test runs, so no query is
// actually issued; the fake just needs to exist to build db.Queries.
type leaseNoopDB struct{}

func (leaseNoopDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errors.New("unexpected Exec")
}
func (leaseNoopDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected Query")
}
func (leaseNoopDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return errRow{}
}

type errRow struct{}

func (errRow) Scan(...interface{}) error { return errors.New("unexpected QueryRow") }

// The heartbeat must keep renewing every job the worker still owns —
// including jobs waiting their turn in the serial loop — and stop renewing
// a job only once it has been processed. This is what prevents stale
// recovery from reaping a slow-but-alive worker's queued jobs.
func TestLeaseHeartbeatRenewsOwnedJobsUntilDone(t *testing.T) {
	jobs := []db.PostDeliveryJob{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	hb := startLeaseHeartbeat(context.Background(), db.New(leaseNoopDB{}), jobs)
	defer hb.stop()

	hb.mu.Lock()
	got := len(hb.remaining)
	hb.mu.Unlock()
	if got != 3 {
		t.Fatalf("heartbeat should track all %d claimed jobs, got %d", len(jobs), got)
	}

	hb.done("b")

	hb.mu.Lock()
	defer hb.mu.Unlock()
	if _, ok := hb.remaining["b"]; ok {
		t.Fatal("a processed job must stop being renewed")
	}
	if _, ok := hb.remaining["a"]; !ok {
		t.Fatal("jobs still owned (queued/in-flight) must keep their lease renewed")
	}
	if _, ok := hb.remaining["c"]; !ok {
		t.Fatal("jobs still owned (queued/in-flight) must keep their lease renewed")
	}
}
