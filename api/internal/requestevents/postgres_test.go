package requestevents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type fakePostgresDB struct {
	batchLen int
	batchErr error
	tx       *fakeEventTx
}

func (d *fakePostgresDB) SendBatch(_ context.Context, batch *pgx.Batch) pgx.BatchResults {
	d.batchLen = batch.Len()
	return &fakeBatchResults{closeErr: d.batchErr}
}

func (d *fakePostgresDB) Begin(context.Context) (pgx.Tx, error) {
	if d.tx == nil {
		d.tx = &fakeEventTx{}
	}
	return d.tx, nil
}

type fakeBatchResults struct {
	pgx.BatchResults
	closeErr error
}

func (r *fakeBatchResults) Close() error { return r.closeErr }

type fakeEventTx struct {
	pgx.Tx
	execSQL    []string
	execErrAt  int
	committed  bool
	rolledBack bool
}

func (t *fakeEventTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	t.execSQL = append(t.execSQL, sql)
	if t.execErrAt > 0 && len(t.execSQL) == t.execErrAt {
		return pgconn.CommandTag{}, errors.New("insert failed")
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (t *fakeEventTx) Commit(context.Context) error {
	t.committed = true
	return nil
}

func (t *fakeEventTx) Rollback(context.Context) error {
	t.rolledBack = true
	return nil
}

func TestPostgresStoreBatchesOnlyBaseSuccessRows(t *testing.T) {
	database := &fakePostgresDB{}
	store := NewPostgresStore(database)
	if err := store.InsertSuccessBatch(context.Background(), []Event{
		validEvent("event-1", httpStatusOK),
		validEvent("event-2", httpStatusCreated),
	}); err != nil {
		t.Fatal(err)
	}
	if database.batchLen != 2 {
		t.Fatalf("batch len = %d, want 2 base event inserts", database.batchLen)
	}
}

func TestPostgresStoreFailureEventAndDetailUseOneTransaction(t *testing.T) {
	tx := &fakeEventTx{}
	database := &fakePostgresDB{tx: tx}
	store := NewPostgresStore(database)
	event := validEvent("event-failure", httpStatusInternalServerError)
	event.HasErrorDetail = true
	detail := &Detail{EventID: event.ID, OccurredAt: event.OccurredAt, WorkspaceID: event.WorkspaceID, RedactionPolicyVersion: 1}

	if err := store.InsertFailure(context.Background(), event, detail); err != nil {
		t.Fatal(err)
	}
	if len(tx.execSQL) != 2 || !strings.Contains(strings.ToLower(tx.execSQL[0]), "insert into api_request_events") ||
		!strings.Contains(strings.ToLower(tx.execSQL[1]), "insert into api_request_error_details") {
		t.Fatalf("transaction statements = %#v", tx.execSQL)
	}
	if !tx.committed {
		t.Fatal("event and detail transaction was not committed")
	}
}

func TestPostgresStoreRollsBackWhenDetailInsertFails(t *testing.T) {
	tx := &fakeEventTx{execErrAt: 2}
	store := NewPostgresStore(&fakePostgresDB{tx: tx})
	event := validEvent("event-failure", httpStatusInternalServerError)
	event.HasErrorDetail = true
	detail := &Detail{EventID: event.ID, OccurredAt: event.OccurredAt, WorkspaceID: event.WorkspaceID, RedactionPolicyVersion: 1}

	if err := store.InsertFailure(context.Background(), event, detail); err == nil {
		t.Fatal("detail insert failure must be returned to the telemetry recorder")
	}
	if tx.committed || !tx.rolledBack {
		t.Fatalf("transaction state committed=%v rolledBack=%v", tx.committed, tx.rolledBack)
	}
}
