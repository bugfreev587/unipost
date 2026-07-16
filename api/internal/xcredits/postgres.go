package xcredits

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

type PostgresStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func shouldSkipUsageSettlement(queryErr error, status string) (bool, error) {
	if errors.Is(queryErr, pgx.ErrNoRows) {
		return true, nil
	}
	if queryErr != nil {
		return false, queryErr
	}
	return status != UsageStatusProvisional, nil
}

func int64Pointer(value int64) *int64 {
	return &value
}

func NewPostgresService(pool *pgxpool.Pool, queries *db.Queries) *Service {
	return NewService(&PostgresStore{pool: pool, queries: queries})
}

func (s *PostgresStore) ResolveWorkspacePeriod(ctx context.Context, workspaceID string, now time.Time) (WorkspacePeriod, error) {
	start, end := CalendarMonthPeriod(now)
	period := WorkspacePeriod{PlanID: "free", Start: start, End: end}
	sub, err := s.queries.GetSubscriptionByWorkspace(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return period, nil
		}
		return WorkspacePeriod{}, err
	}
	if sub.PlanID != "" {
		period.PlanID = sub.PlanID
	}
	if sub.CurrentPeriodStart.Valid && sub.CurrentPeriodEnd.Valid && sub.CurrentPeriodEnd.Time.After(sub.CurrentPeriodStart.Time) {
		period.Start = sub.CurrentPeriodStart.Time.UTC()
		period.End = sub.CurrentPeriodEnd.Time.UTC()
	}
	if period.PlanID == "enterprise" {
		var monthlyAllowance, inboundDailyLimit int64
		err := s.pool.QueryRow(ctx, `
			SELECT monthly_allowance, inbound_daily_limit
			FROM x_workspace_allowances
			WHERE workspace_id = $1
		`, workspaceID).Scan(&monthlyAllowance, &inboundDailyLimit)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return WorkspacePeriod{}, err
		}
		if err == nil {
			period.MonthlyAllowance = int64Pointer(monthlyAllowance)
			period.InboundDailyLimit = int64Pointer(inboundDailyLimit)
		}
	}
	return period, nil
}

func (s *PostgresStore) Reserve(ctx context.Context, req StoreReserveRequest) (UsageEvent, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return UsageEvent{}, err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO x_usage_periods (
			workspace_id, period_start, period_end, weighted_units_used, weighted_units_limit
		) VALUES ($1, $2, $3, 0, $4)
		ON CONFLICT (workspace_id, period_start, period_end)
		DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.WeightedUnitsLimit)
	if err != nil {
		return UsageEvent{}, err
	}

	event := UsageEvent{
		Status:         UsageStatusProvisional,
		OperationKey:   req.OperationKey,
		CatalogVersion: req.CatalogVersion,
		WeightedUnits:  req.WeightedUnits,
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO x_usage_events (
			workspace_id, social_account_id, period_start, period_end,
			operation_key, catalog_version, source, idempotency_key,
			weighted_units, status, connection_mode
		) VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6, $7, $8, $9, 'provisional', $10)
		ON CONFLICT (workspace_id, idempotency_key) DO NOTHING
		RETURNING id
	`, req.WorkspaceID, req.SocialAccountID, req.PeriodStart, req.PeriodEnd,
		req.OperationKey, req.CatalogVersion, req.Source, req.IdempotencyKey,
		req.WeightedUnits, req.AppMode,
	).Scan(&event.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		var existing UsageEvent
		err = tx.QueryRow(ctx, `
			SELECT id, status, operation_key, catalog_version, weighted_units
			FROM x_usage_events
			WHERE workspace_id = $1 AND idempotency_key = $2
			FOR UPDATE
		`, req.WorkspaceID, req.IdempotencyKey).Scan(
			&existing.ID,
			&existing.Status,
			&existing.OperationKey,
			&existing.CatalogVersion,
			&existing.WeightedUnits,
		)
		if err != nil {
			return UsageEvent{}, err
		}
		if existing.Status != UsageStatusReversed {
			existing.Duplicate = true
			if err := tx.Commit(ctx); err != nil {
				return UsageEvent{}, err
			}
			return existing, nil
		}
		event.ID = existing.ID
	} else if err != nil {
		return UsageEvent{}, err
	}

	tag, err := tx.Exec(ctx, `
		UPDATE x_usage_periods
		SET weighted_units_used = weighted_units_used + $4, updated_at = NOW()
		WHERE workspace_id = $1
		  AND period_start = $2
		  AND period_end = $3
		  AND weighted_units_used + $4 <= weighted_units_limit
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.WeightedUnits)
	if err != nil {
		return UsageEvent{}, err
	}
	if tag.RowsAffected() != 1 {
		return UsageEvent{}, ErrMonthlyLimitExceeded
	}

	_, err = tx.Exec(ctx, `
		UPDATE x_usage_events
		SET status = 'provisional',
		    operation_key = $2,
		    catalog_version = $3,
		    source = $4,
		    weighted_units = $5,
		    updated_at = NOW()
		WHERE id = $1
	`, event.ID, req.OperationKey, req.CatalogVersion, req.Source, req.WeightedUnits)
	if err != nil {
		return UsageEvent{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return UsageEvent{}, err
	}
	return event, nil
}

func (s *PostgresStore) Finalize(ctx context.Context, eventID string, finalUnits int64) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workspaceID, status string
	var start, end time.Time
	var currentUnits int64
	err = tx.QueryRow(ctx, `
		SELECT workspace_id, period_start, period_end, weighted_units, status
		FROM x_usage_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&workspaceID, &start, &end, &currentUnits, &status)
	skip, err := shouldSkipUsageSettlement(err, status)
	if err != nil {
		return err
	}
	if skip {
		return tx.Commit(ctx)
	}
	if finalUnits > currentUnits {
		return errors.New("final X usage cannot exceed provisional usage")
	}

	delta := currentUnits - finalUnits
	if delta > 0 {
		_, err = tx.Exec(ctx, `
			UPDATE x_usage_periods
			SET weighted_units_used = weighted_units_used - $4, updated_at = NOW()
			WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		`, workspaceID, start, end, delta)
		if err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE x_usage_events
		SET status = 'finalized', weighted_units = $2, updated_at = NOW()
		WHERE id = $1
	`, eventID, finalUnits)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Reverse(ctx context.Context, eventID string) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workspaceID, status string
	var start, end time.Time
	var units int64
	err = tx.QueryRow(ctx, `
		SELECT workspace_id, period_start, period_end, weighted_units, status
		FROM x_usage_events
		WHERE id = $1
		FOR UPDATE
	`, eventID).Scan(&workspaceID, &start, &end, &units, &status)
	skip, err := shouldSkipUsageSettlement(err, status)
	if err != nil {
		return err
	}
	if skip {
		return tx.Commit(ctx)
	}

	_, err = tx.Exec(ctx, `
		UPDATE x_usage_periods
		SET weighted_units_used = weighted_units_used - $4, updated_at = NOW()
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
	`, workspaceID, start, end, units)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE x_usage_events
		SET status = 'reversed', updated_at = NOW()
		WHERE id = $1
	`, eventID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) Snapshot(ctx context.Context, workspaceID string, now time.Time) (Snapshot, error) {
	period, err := s.ResolveWorkspacePeriod(ctx, workspaceID, now)
	if err != nil {
		return Snapshot{}, err
	}
	allowance, allowanceConfigured := PlanAllowance(period.PlanID)
	if period.MonthlyAllowance != nil {
		allowance = *period.MonthlyAllowance
		allowanceConfigured = true
	}
	inboundLimit, inboundLimitConfigured := InboundDailyLimit(period.PlanID)
	if period.InboundDailyLimit != nil {
		inboundLimit = *period.InboundDailyLimit
		inboundLimitConfigured = true
	}

	var used int64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(weighted_units_used, 0)
		FROM x_usage_periods
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
	`, workspaceID, period.Start, period.End).Scan(&used)
	if errors.Is(err, pgx.ErrNoRows) {
		used = 0
	} else if err != nil {
		return Snapshot{}, err
	}

	var inboundUsed int64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(weighted_units_used, 0)
		FROM x_inbound_daily_usage
		WHERE workspace_id = $1 AND utc_date = $2
	`, workspaceID, now.UTC().Format("2006-01-02")).Scan(&inboundUsed)
	if errors.Is(err, pgx.ErrNoRows) {
		inboundUsed = 0
	} else if err != nil {
		return Snapshot{}, err
	}

	var monthlyAllowance, monthlyRemaining, dailyLimit *int64
	if allowanceConfigured {
		remaining := allowance - used
		if remaining < 0 {
			remaining = 0
		}
		monthlyAllowance = int64Pointer(allowance)
		monthlyRemaining = int64Pointer(remaining)
	}
	if inboundLimitConfigured {
		dailyLimit = int64Pointer(inboundLimit)
	}
	return Snapshot{
		PlanID:            period.PlanID,
		PeriodStart:       period.Start,
		PeriodEnd:         period.End,
		MonthlyAllowance:  monthlyAllowance,
		MonthlyUsed:       used,
		MonthlyRemaining:  monthlyRemaining,
		InboundDailyUsed:  inboundUsed,
		InboundDailyLimit: dailyLimit,
		CatalogVersion:    CatalogVersion,
	}, nil
}
