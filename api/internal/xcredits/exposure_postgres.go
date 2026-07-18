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
		SELECT id, requested_resources, reserved_units, COALESCE(actual_units, 0), status, TRUE
		FROM x_inbox_backfill_exposure_reservations
		WHERE workspace_id = $1 AND idempotency_key = $2
		FOR UPDATE
	`, req.WorkspaceID, req.IdempotencyKey).Scan(
		&existing.ID, &existing.RequestedResources, &existing.ReservedUnits,
		&existing.ActualUnits, &existing.Status, &existing.Duplicate,
	)
	if err == nil {
		existing.ReservedResources = int(existing.ReservedUnits / req.UnitsPerResource)
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return ExposureReservation{}, err
	}
	if req.AccountingEnabled {
		if _, err := tx.Exec(ctx, `
			INSERT INTO x_usage_periods (
				workspace_id, period_start, period_end, weighted_units_used, weighted_units_limit
			) VALUES ($1, $2, $3, 0, $4)
			ON CONFLICT (workspace_id, period_start, period_end)
			DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
		`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.MonthlyAllowance); err != nil {
			return ExposureReservation{}, err
		}
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
	if req.AccountingEnabled {
		if err := tx.QueryRow(ctx, `
			SELECT weighted_units_used
			FROM x_usage_periods
			WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
			FOR UPDATE
		`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd).Scan(&monthlyUsed); err != nil {
			return ExposureReservation{}, err
		}
	}
	if err := tx.QueryRow(ctx, `
		SELECT weighted_units_used, weighted_units_limit
		FROM x_inbound_daily_usage
		WHERE workspace_id = $1 AND utc_date = $2
		FOR UPDATE
	`, req.WorkspaceID, req.UTCDate).Scan(&dailyUsed, &dailyLimit); err != nil {
		return ExposureReservation{}, err
	}
	monthlyResources := int64(req.RequestedResources)
	if req.AccountingEnabled {
		monthlyResources = (req.MonthlyAllowance - monthlyUsed) / req.UnitsPerResource
	}
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
	if req.AccountingEnabled {
		if _, err := tx.Exec(ctx, `
			UPDATE x_usage_periods
			SET weighted_units_used = weighted_units_used + $4, updated_at = NOW()
			WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, units); err != nil {
			return ExposureReservation{}, err
		}
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
		Status:             "reserved",
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO x_inbox_backfill_exposure_reservations (
			workspace_id, social_account_id, operation_key, idempotency_key,
			requested_resources, reserved_units, period_start, period_end, utc_date,
			reconciliation_deadline, next_attempt_at, accounting_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11)
		RETURNING id
	`, req.WorkspaceID, req.SocialAccountID, req.OperationKey, req.IdempotencyKey,
		req.RequestedResources, units, req.PeriodStart, req.PeriodEnd, req.UTCDate,
		req.Now.Add(30*time.Minute), req.AccountingEnabled).Scan(&reservation.ID)
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

func (s *PostgresStore) MarkExposureReadStarted(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'read_started',
		    next_attempt_at = reconciliation_deadline,
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('reserved', 'read_started')
	`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X exposure read-started state was not persisted")
	}
	return nil
}

func (s *PostgresStore) MarkExposureFinalizePending(
	ctx context.Context,
	id string,
	actualUnits int64,
	message string,
) error {
	if actualUnits < 0 {
		return errors.New("actual X backfill exposure cannot be negative")
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'finalize_pending',
		    actual_units = $2,
		    last_error = LEFT($3, 1000),
		    next_attempt_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND $2 <= reserved_units
		  AND status IN ('read_started', 'finalize_pending')
	`, id, actualUnits, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X exposure finalize-pending state was not persisted")
	}
	return nil
}

func (s *PostgresStore) ReleaseExposure(ctx context.Context, id string) error {
	return s.settleExposure(ctx, id, 0, "released")
}

func (s *PostgresStore) MarkExposureReleasePending(
	ctx context.Context,
	id string,
	message string,
) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'release_pending',
		    last_error = LEFT($2, 1000),
		    next_attempt_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1
		  AND status IN ('reserved', 'read_started', 'release_pending')
	`, id, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X exposure release-pending state was not persisted")
	}
	return nil
}

func (s *PostgresStore) claimStaleReservedExposureForRelease(
	ctx context.Context,
	id string,
) (bool, error) {
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'release_pending',
		    last_error = 'Reserved X read exposure expired before the upstream call started',
		    next_attempt_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'reserved'
	`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
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
	var accountingEnabled bool
	err = tx.QueryRow(ctx, `
		SELECT workspace_id, period_start, period_end, utc_date, reserved_units, status, accounting_enabled
		FROM x_inbox_backfill_exposure_reservations
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&workspaceID, &periodStart, &periodEnd, &utcDate, &reservedUnits, &status, &accountingEnabled)
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
		if accountingEnabled {
			if _, err := tx.Exec(ctx, `
				UPDATE x_usage_periods
				SET weighted_units_used = weighted_units_used - $4, updated_at = NOW()
				WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
			`, workspaceID, periodStart, periodEnd, delta); err != nil {
				return err
			}
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
	tag, err := s.pool.Exec(ctx, `
		UPDATE x_inbox_backfill_exposure_reservations
		SET status = 'needs_reconciliation',
		    last_error = LEFT($2, 1000),
		    updated_at = NOW()
		WHERE id = $1 AND status IN ('read_started', 'needs_reconciliation')
	`, id, message)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("X exposure reconciliation state was not persisted")
	}
	return nil
}

func (s *PostgresStore) ReconcilePendingExposures(
	ctx context.Context,
	limit int,
	now time.Time,
) (ExposureReleaseReconcileStats, error) {
	stats := ExposureReleaseReconcileStats{}
	rows, err := s.pool.Query(ctx, `
		SELECT id, status, COALESCE(actual_units, reserved_units)
		FROM x_inbox_backfill_exposure_reservations
		WHERE status IN ('reserved', 'read_started', 'finalize_pending', 'release_pending')
		  AND next_attempt_at <= $1
		ORDER BY created_at
		LIMIT $2
	`, now.UTC(), limit)
	if err != nil {
		return stats, err
	}
	type pendingExposure struct {
		id          string
		status      string
		actualUnits int64
	}
	var pending []pendingExposure
	for rows.Next() {
		var exposure pendingExposure
		if err := rows.Scan(&exposure.id, &exposure.status, &exposure.actualUnits); err != nil {
			rows.Close()
			return stats, err
		}
		pending = append(pending, exposure)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return stats, err
	}
	rows.Close()
	stats.Scanned = len(pending)
	for _, exposure := range pending {
		var reconcileErr error
		switch exposure.status {
		case "reserved":
			var claimed bool
			claimed, reconcileErr = s.claimStaleReservedExposureForRelease(ctx, exposure.id)
			if reconcileErr == nil && !claimed {
				continue
			}
			if reconcileErr == nil {
				reconcileErr = s.ReleaseExposure(ctx, exposure.id)
			}
			if reconcileErr == nil {
				stats.Released++
			}
		case "release_pending":
			reconcileErr = s.ReleaseExposure(ctx, exposure.id)
			if reconcileErr == nil {
				stats.Released++
			}
		case "finalize_pending":
			reconcileErr = s.FinalizeExposure(ctx, exposure.id, exposure.actualUnits)
			if reconcileErr == nil {
				stats.Finalized++
			}
		case "read_started":
			reconcileErr = s.MarkExposureNeedsReconciliation(
				ctx, exposure.id, "X paid read outcome remained unknown past the reconciliation deadline",
			)
			if reconcileErr == nil {
				stats.NeedsReconciliation++
			}
		}
		if reconcileErr != nil {
			stats.Deferred++
			_, _ = s.pool.Exec(ctx, `
				UPDATE x_inbox_backfill_exposure_reservations
				SET reconciliation_attempts = reconciliation_attempts + 1,
				    next_attempt_at = $2,
				    last_error = LEFT($3, 1000),
				    updated_at = NOW()
				WHERE id = $1
				  AND status IN ('reserved', 'read_started', 'finalize_pending', 'release_pending')
			`, exposure.id, now.UTC().Add(time.Minute), reconcileErr.Error())
			continue
		}
	}
	return stats, nil
}
