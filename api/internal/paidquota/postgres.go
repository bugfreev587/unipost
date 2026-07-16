package paidquota

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

type postgresBeginner struct {
	pool *pgxpool.Pool
}

type postgresTransaction struct {
	tx      pgx.Tx
	queries *db.Queries
	checker *quota.Checker
}

func NewPostgresCoordinator(pool *pgxpool.Pool) Coordinator {
	return newCoordinator(&postgresBeginner{pool: pool})
}

func (b *postgresBeginner) Begin(ctx context.Context) (transaction, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	queries := db.New(tx)
	return &postgresTransaction{
		tx:      tx,
		queries: queries,
		checker: quota.NewChecker(queries),
	}, nil
}

func (t *postgresTransaction) LockPeriod(ctx context.Context, workspaceID, period string) error {
	_, err := t.tx.Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
		workspaceID,
		"paid_schedule_quota:"+period,
	)
	return err
}

func (t *postgresTransaction) Snapshot(ctx context.Context, workspaceID, period string) (quota.MonthlySnapshot, error) {
	return t.checker.MonthlySnapshotForPeriod(ctx, workspaceID, period)
}

func (t *postgresTransaction) Queries() *db.Queries {
	return t.queries
}

func (t *postgresTransaction) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *postgresTransaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}
