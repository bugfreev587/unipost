package xcredits

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) ReserveExposure(
	ctx context.Context,
	req StoreExposureReservationRequest,
) (ExposureReservation, error) {
	if req.RequestedResources <= 0 || req.MinimumResources <= 0 ||
		req.UnitsPerResource <= 0 || req.MinimumResources > req.RequestedResources {
		return ExposureReservation{}, errors.New("invalid X backfill exposure reservation")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ExposureReservation{}, err
	}
	defer tx.Rollback(ctx)
	lockKey := fmt.Sprintf("x-inbound-cap:%s:%s", req.WorkspaceID, req.UTCDate.Format("2006-01-02"))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return ExposureReservation{}, err
	}
	var existing ExposureReservation
	err = tx.QueryRow(ctx, `
		SELECT id, requested_resources, reserved_units, TRUE
		FROM x_inbox_backfill_exposure_reservations
		WHERE workspace_id = $1 AND idempotency_key = $2
		FOR UPDATE
	`, req.WorkspaceID, req.IdempotencyKey).Scan(
		&existing.ID, &existing.RequestedResources, &existing.ReservedUnits, &existing.Duplicate,
	)
	if err == nil {
		existing.ReservedResources = int(existing.ReservedUnits / req.UnitsPerResource)
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ExposureReservation{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO x_usage_periods (
			workspace_id, period_start, period_end, weighted_units_used, weighted_units_limit
		) VALUES ($1, $2, $3, 0, $4)
		ON CONFLICT (workspace_id, period_start, period_end)
		DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.MonthlyAllowance); err != nil {
		return ExposureReservation{}, err
	}
	effectiveDailyLimit := req.InboundDailyLimit
	err = tx.QueryRow(ctx, `
		SELECT inbound_daily_limit
		FROM x_inbound_cap_settings
		WHERE workspace_id = $1
		FOR UPDATE
	`, req.WorkspaceID).Scan(&effectiveDailyLimit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ExposureReservation{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO x_inbound_daily_usage (
			workspace_id, utc_date, weighted_units_used, weighted_units_limit,
			events_accepted, events_suppressed
		) VALUES ($1, $2, 0, $3, 0, 0)
		ON CONFLICT (workspace_id, utc_date)
		DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
	`, req.WorkspaceID, req.UTCDate, effectiveDailyLimit); err != nil {
		return ExposureReservation{}, err
	}
	var monthlyUsed, dailyUsed, dailyLimit int64
	if err := tx.QueryRow(ctx, `
		SELECT weighted_units_used
		FROM x_usage_periods
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		FOR UPDATE
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd).Scan(&monthlyUsed); err != nil {
		return ExposureReservation{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT weighted_units_used, weighted_units_limit
		FROM x_inbound_daily_usage
		WHERE workspace_id = $1 AND utc_date = $2
		FOR UPDATE
	`, req.WorkspaceID, req.UTCDate).Scan(&dailyUsed, &dailyLimit); err != nil {
		return ExposureReservation{}, err
	}
	monthlyResources := (req.MonthlyAllowance - monthlyUsed) / req.UnitsPerResource
	safetyBuffer := dailyLimit / 10
	if safetyBuffer < 20 {
		safetyBuffer = 20
	}
	dailyRemaining := dailyLimit - dailyUsed - safetyBuffer
	if dailyRemaining < 0 {
		dailyRemaining = 0
	}
	dailyResources := dailyRemaining / req.UnitsPerResource
	resources := int64(req.RequestedResources)
	if monthlyResources < resources {
		resources = monthlyResources
	}
	if dailyResources < resources {
		resources = dailyResources
	}
	if resources < int64(req.MinimumResources) {
		if monthlyResources < int64(req.MinimumResources) {
			return ExposureReservation{}, ErrMonthlyLimitExceeded
		}
		return ExposureReservation{}, ErrInboundDailyCapExceeded
	}
	units := resources * req.UnitsPerResource
	if _, err := tx.Exec(ctx, `
		UPDATE x_usage_periods
		SET weighted_units_used = weighted_units_used + $4, updated_at = NOW()
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, units); err != nil {
		return ExposureReservation{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE x_inbound_daily_usage
		SET weighted_units_used = weighted_units_used + $3, updated_at = NOW()
		WHERE workspace_id = $1 AND utc_date = $2
	`, req.WorkspaceID, req.UTCDate, units); err != nil {
		return ExposureReservation{}, err
	}
	reservation := ExposureReservation{
		RequestedResources: req.RequestedResources,
		ReservedResources:  int(resources),
		ReservedUnits:      units,
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO x_inbox_backfill_exposure_reservations (
			workspace_id, social_account_id, operation_key, idempotency_key,
			requested_resources, reserved_units, period_start, period_end, utc_date,
			reconciliation_deadline
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`, req.WorkspaceID, req.SocialAccountID, req.OperationKey, req.IdempotencyKey,
		req.RequestedResources, units, req.PeriodStart, req.PeriodEnd, req.UTCDate,
		req.Now.Add(30*time.Minute)).Scan(&reservation.ID)
	if err != nil {
		return ExposureReservation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ExposureReservation{}, err
	}
	return reservation, nil
}

func (s *PostgresStore) FinalizeExposure(ctx context.Context, id string, actualUnits int64) error {
	return s.settleExposure(ctx, id, actualUnits, "finalized")
}

func (s *PostgresStore) ReleaseExposure(ctx context.Context, id string) error {
	return s.settleExposure(ctx, id, 0, "released")
}

func (s *PostgresStore) settleExposure(
	ctx context.Context,
	id string,
	actualUnits int64,
	finalStatus string,
) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var workspaceID, status string
	var periodStart, periodEnd time.Time
	var utcDate time.Time
	var reservedUnits int64
	err = tx.QueryRow(ctx, `
		SELECT workspace_id, period_start, period_end, utc_date, reserved_units, status
		FROM x_inbox_backfill_exposure_reservations
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&workspaceID, &periodStart, &periodEnd, &utcDate, &reservedUnits, &status)
	if err != nil {
		return err
	}
	if status == "finalized" || status == "released" {
		return tx.Commit(ctx)
	}
	if actualUnits < 0 || actualUnits > reservedUnits {
		return errors.New("actual X backfill exposure exceeds reservation")
	}
	delta := reservedUnits - actualUnits
	if delta > 0 {
		if _, err := tx.Exec(ctx, `
			UPDATE x_usage_periods
			SET weighted_units_used = weighted_units_used - $4, updated_at = NOW()
			WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		`, workspaceID, periodStart, periodEnd, delta); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE x_inbound_daily_usage
			SET weighted_units_used = weighted_units_used - $3, updated_at = NOW()
			WHERE workspace_id = $1 AND utc_date = $2
		`, workspaceID, utcDate, delta); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET actual_units = $2, status = $3, last_error = NULL, updated_at = NOW()
		WHERE id = $1
	`, id, actualUnits, finalStatus); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) MarkExposureNeedsReconciliation(
	ctx context.Context,
	id string,
	message string,
) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'needs_reconciliation',
		    last_error = LEFT($2, 1000),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'reserved'
	`, id, message)
	return err
}
