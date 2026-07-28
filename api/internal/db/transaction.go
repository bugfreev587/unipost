package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type transactionBeginner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// WithTransaction runs fn with queries bound to one database transaction.
// The underlying DBTX must support pgx transactions; production pgx pools and
// pgx transactions do. Unsupported stores fail closed rather than silently
// running a mutation sequence without atomicity.
func (q *Queries) WithTransaction(ctx context.Context, fn func(*Queries) error) error {
	if q == nil || q.db == nil {
		return errors.New("database queries are not configured")
	}
	beginner, ok := q.db.(transactionBeginner)
	if !ok {
		return fmt.Errorf("database transaction is not supported by %T", q.db)
	}
	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(rollbackCtx)
	}()
	if err := fn(q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
