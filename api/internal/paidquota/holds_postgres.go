package paidquota

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/quota"
)

const listScheduledParentsForPeriodSQL = `
WITH reset_baseline AS (
  SELECT MAX(reset_at) AS reset_at
  FROM admin_post_quota_resets
  WHERE workspace_id = $1
    AND period = $2
    AND quota_kind = 'scheduled'
)
SELECT
  sp.id,
  sp.status,
  sp.scheduled_at,
  sp.created_at,
  COALESCE(
    CASE
      WHEN sp.status = 'publishing' AND EXISTS (
        SELECT 1 FROM social_post_results existing WHERE existing.post_id = sp.id
      ) THEN (
        SELECT COUNT(*)::INTEGER
        FROM social_post_results spr
        JOIN social_accounts sa ON sa.id = spr.social_account_id
        WHERE spr.post_id = sp.id
          AND spr.status NOT IN ('published', 'failed')
          AND sa.disconnected_at IS NULL
      )
      WHEN jsonb_typeof(sp.metadata->'platform_posts') = 'array' THEN (
        SELECT COUNT(*)::INTEGER
        FROM jsonb_array_elements(sp.metadata->'platform_posts') AS pp
        JOIN social_accounts sa ON sa.id = pp->>'account_id'
        WHERE sa.disconnected_at IS NULL
      )
      WHEN jsonb_typeof(sp.metadata->'account_ids') = 'array' THEN (
        SELECT COUNT(*)::INTEGER
        FROM jsonb_array_elements_text(sp.metadata->'account_ids') AS account_id
        JOIN social_accounts sa ON sa.id = account_id
        WHERE sa.disconnected_at IS NULL
      )
      ELSE 1
    END,
    0
  )::INTEGER AS units
FROM social_posts sp
CROSS JOIN reset_baseline rb
WHERE sp.workspace_id = $1
  AND sp.status IN ('scheduled', 'quota_hold', 'publishing')
  AND sp.scheduled_at IS NOT NULL
  AND sp.deleted_at IS NULL
  AND sp.scheduled_at >= ($2 || '-01')::DATE
  AND sp.scheduled_at < (($2 || '-01')::DATE + INTERVAL '1 month')
  AND sp.created_at > COALESCE(rb.reset_at, '-infinity'::timestamptz)
ORDER BY sp.scheduled_at, sp.created_at, sp.id
FOR UPDATE OF sp
`

type postgresHoldStore struct {
	pool *pgxpool.Pool
}

type postgresHoldReconciler struct {
	pool    *pgxpool.Pool
	service *HoldService
}

type postgresHoldPeriodTx struct {
	tx          pgx.Tx
	checker     *quota.Checker
	workspaceID string
	period      string
}

func NewPostgresHoldReconciler(pool *pgxpool.Pool) HoldReconciler {
	return &postgresHoldReconciler{
		pool:    pool,
		service: NewHoldService(&postgresHoldStore{pool: pool}, nil),
	}
}

func (r *postgresHoldReconciler) ReconcileWorkspace(ctx context.Context, workspaceID, reason string, effectiveAt time.Time) error {
	return r.service.ReconcileWorkspace(ctx, workspaceID, reason, effectiveAt)
}

func (r *postgresHoldReconciler) ReconcileWorkspaceForPlan(
	ctx context.Context,
	workspaceID, planID string,
	limit int,
	reason string,
	effectiveAt time.Time,
) error {
	return r.service.ReconcileWorkspaceForPlan(ctx, workspaceID, planID, limit, reason, effectiveAt)
}

func (r *postgresHoldReconciler) ApplyPlanChange(
	ctx context.Context,
	workspaceID, planID string,
	limit int,
	reason string,
	effectiveAt time.Time,
	mutation PlanChangeMutation,
) error {
	if r == nil || r.pool == nil || r.service == nil {
		return fmt.Errorf("paid quota hold reconciler is not configured")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()

	now := r.service.now().UTC()
	end := now.AddDate(0, 0, 90)
	periods := make([]string, 0, 4)
	for cursor := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC); !cursor.After(end); cursor = cursor.AddDate(0, 1, 0) {
		periods = append(periods, cursor.Format("2006-01"))
	}
	for _, period := range periods {
		if _, err := tx.Exec(
			ctx,
			"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
			workspaceID,
			"paid_schedule_quota:"+period,
		); err != nil {
			return err
		}
	}
	override := &quota.MonthlySnapshot{PlanID: planID, Limit: limit}
	queries := db.New(tx)
	for _, period := range periods {
		periodTx := &postgresHoldPeriodTx{
			tx:          tx,
			checker:     quota.NewChecker(queries),
			workspaceID: workspaceID,
			period:      period,
		}
		if err := r.service.reconcilePeriodTransaction(ctx, periodTx, reason, effectiveAt, now, override); err != nil {
			return err
		}
	}
	if mutation != nil {
		if err := mutation(queries); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *postgresHoldStore) WithinPeriod(ctx context.Context, workspaceID, period string, fn func(HoldPeriodTransaction) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback(ctx)
		}
	}()
	if _, err := tx.Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
		workspaceID,
		"paid_schedule_quota:"+period,
	); err != nil {
		return err
	}
	queries := db.New(tx)
	periodTx := &postgresHoldPeriodTx{
		tx:          tx,
		checker:     quota.NewChecker(queries),
		workspaceID: workspaceID,
		period:      period,
	}
	if err := fn(periodTx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	committed = true
	return nil
}

func (t *postgresHoldPeriodTx) Snapshot(ctx context.Context) (quota.MonthlySnapshot, error) {
	return t.checker.MonthlySnapshotForPeriod(ctx, t.workspaceID, t.period)
}

func (t *postgresHoldPeriodTx) Parents(ctx context.Context) ([]ScheduledParent, error) {
	rows, err := t.tx.Query(ctx, listScheduledParentsForPeriodSQL, t.workspaceID, t.period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	parents := make([]ScheduledParent, 0)
	for rows.Next() {
		var parent ScheduledParent
		var units int32
		if err := rows.Scan(&parent.ID, &parent.Status, &parent.ScheduledAt, &parent.CreatedAt, &units); err != nil {
			return nil, err
		}
		parent.Units = int(units)
		parents = append(parents, parent)
	}
	return parents, rows.Err()
}

func (t *postgresHoldPeriodTx) SetHold(ctx context.Context, postID, reason string) error {
	tag, err := t.tx.Exec(ctx, `
UPDATE social_posts
SET status = 'quota_hold',
    quota_hold_reason = $2,
    quota_hold_at = NOW(),
    quota_hold_original_scheduled_at = COALESCE(quota_hold_original_scheduled_at, scheduled_at)
WHERE id = $1
  AND workspace_id = $3
  AND status = 'scheduled'
  AND deleted_at IS NULL
`, postID, reason, t.workspaceID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("set quota hold for post %s: concurrent status change", postID)
	}
	return nil
}

func (t *postgresHoldPeriodTx) ReleaseHold(ctx context.Context, postID string) error {
	tag, err := t.tx.Exec(ctx, `
UPDATE social_posts
SET status = 'scheduled',
    quota_hold_reason = NULL,
    quota_hold_at = NULL
WHERE id = $1
  AND workspace_id = $2
  AND status = 'quota_hold'
  AND scheduled_at > $3
  AND deleted_at IS NULL
`, postID, t.workspaceID, time.Now().UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("release quota hold for post %s: concurrent status change", postID)
	}
	return nil
}
