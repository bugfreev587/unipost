package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type transactionTestDB struct {
	tx *transactionTestTx
}

func (f *transactionTestDB) Begin(context.Context) (pgx.Tx, error) { return f.tx, nil }
func (*transactionTestDB) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (*transactionTestDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (*transactionTestDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return transactionTestRow{err: errors.New("unexpected query row")}
}

type transactionTestTx struct {
	commits            int
	rollbacks          int
	commitErr          error
	rollbackContextErr error
}

func (f *transactionTestTx) Begin(context.Context) (pgx.Tx, error) { return f, nil }
func (f *transactionTestTx) Commit(context.Context) error {
	f.commits++
	return f.commitErr
}
func (f *transactionTestTx) Rollback(ctx context.Context) error {
	f.rollbacks++
	f.rollbackContextErr = ctx.Err()
	return nil
}
func (*transactionTestTx) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errors.New("unexpected copy")
}
func (*transactionTestTx) SendBatch(context.Context, *pgx.Batch) pgx.BatchResults { return nil }
func (*transactionTestTx) LargeObjects() pgx.LargeObjects                         { return pgx.LargeObjects{} }
func (*transactionTestTx) Prepare(context.Context, string, string) (*pgconn.StatementDescription, error) {
	return nil, errors.New("unexpected prepare")
}
func (*transactionTestTx) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (*transactionTestTx) Query(context.Context, string, ...interface{}) (pgx.Rows, error) {
	return nil, errors.New("unexpected query")
}
func (*transactionTestTx) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return transactionTestRow{err: errors.New("unexpected query row")}
}
func (*transactionTestTx) Conn() *pgx.Conn { return nil }

type transactionTestRow struct{ err error }

func (r transactionTestRow) Scan(...interface{}) error { return r.err }

func TestQueriesWithTransactionCommitsOnlyAfterCallbackSuccess(t *testing.T) {
	tx := &transactionTestTx{}
	queries := New(&transactionTestDB{tx: tx})
	callbackCalls := 0

	err := queries.WithTransaction(context.Background(), func(txQueries *Queries) error {
		callbackCalls++
		if txQueries == nil || txQueries == queries {
			t.Fatal("callback must receive transaction-bound queries")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if callbackCalls != 1 || tx.commits != 1 || tx.rollbacks != 1 {
		t.Fatalf("callback/commit/rollback = %d/%d/%d, want 1/1/1", callbackCalls, tx.commits, tx.rollbacks)
	}
}

func TestQueriesWithTransactionRollsBackCallbackFailure(t *testing.T) {
	wantErr := errors.New("strict retention failed")
	tx := &transactionTestTx{}
	queries := New(&transactionTestDB{tx: tx})

	err := queries.WithTransaction(context.Background(), func(*Queries) error { return wantErr })
	if !errors.Is(err, wantErr) {
		t.Fatalf("transaction error = %v, want %v", err, wantErr)
	}
	if tx.commits != 0 || tx.rollbacks != 1 {
		t.Fatalf("commit/rollback = %d/%d, want 0/1", tx.commits, tx.rollbacks)
	}
}

func TestQueriesWithTransactionRollsBackCommitFailureWithLiveContext(t *testing.T) {
	wantErr := errors.New("commit failed")
	tx := &transactionTestTx{commitErr: wantErr}
	queries := New(&transactionTestDB{tx: tx})
	ctx, cancel := context.WithCancel(context.Background())

	err := queries.WithTransaction(ctx, func(*Queries) error {
		cancel()
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("transaction error = %v, want %v", err, wantErr)
	}
	if tx.commits != 1 || tx.rollbacks != 1 {
		t.Fatalf("commit/rollback = %d/%d, want 1/1", tx.commits, tx.rollbacks)
	}
	if tx.rollbackContextErr != nil {
		t.Fatalf("rollback context error = %v, want nil", tx.rollbackContextErr)
	}
}
