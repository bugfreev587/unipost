package xcredits

import (
	"context"
	"errors"
	"fmt"
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

func (s *PostgresStore) AdmitInbound(ctx context.Context, req StoreInboundRequest) (InboundAdmission, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InboundAdmission{}, err
	}
	defer tx.Rollback(ctx)

	lockKey := fmt.Sprintf("x-inbound-cap:%s:%s", req.WorkspaceID, req.UTCDate.Format("2006-01-02"))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return InboundAdmission{}, err
	}

	var inserted bool
	err = tx.QueryRow(ctx, `
		INSERT INTO x_inbound_event_receipts (
			workspace_id,
			social_account_id,
			upstream_resource_type,
			upstream_resource_id,
			utc_date,
			decision,
			weighted_units,
			period_start,
			period_end,
			reset_at
		) VALUES ($1, $2, $3, $4, $5, 'accepted', $6, $7, $8, $9)
		ON CONFLICT (workspace_id, social_account_id, upstream_resource_type, upstream_resource_id, utc_date) DO NOTHING
		RETURNING TRUE
	`, req.WorkspaceID, req.SocialAccountID, req.UpstreamResourceType, req.UpstreamResourceID,
		req.UTCDate, req.WeightedUnits, req.PeriodStart, req.PeriodEnd,
		req.UTCDate.AddDate(0, 0, 1)).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		admission, loadErr := loadDuplicateInboundAdmission(ctx, tx, req)
		if loadErr != nil {
			return InboundAdmission{}, loadErr
		}
		if err := tx.Commit(ctx); err != nil {
			return InboundAdmission{}, err
		}
		return admission, nil
	}
	if err != nil {
		return InboundAdmission{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO x_usage_periods (
			workspace_id, period_start, period_end, weighted_units_used, weighted_units_limit
		) VALUES ($1, $2, $3, 0, $4)
		ON CONFLICT (workspace_id, period_start, period_end)
		DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.MonthlyAllowance); err != nil {
		return InboundAdmission{}, err
	}

	dailyLimit := req.InboundDailyLimit
	err = tx.QueryRow(ctx, `
		SELECT inbound_daily_limit
		FROM x_inbound_cap_settings
		WHERE workspace_id = $1
		FOR UPDATE
	`, req.WorkspaceID).Scan(&dailyLimit)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return InboundAdmission{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO x_inbound_daily_usage (
			workspace_id, utc_date, weighted_units_used, weighted_units_limit,
			events_accepted, events_suppressed
		) VALUES ($1, $2, 0, $3, 0, 0)
		ON CONFLICT (workspace_id, utc_date)
		DO UPDATE SET weighted_units_limit = EXCLUDED.weighted_units_limit, updated_at = NOW()
	`, req.WorkspaceID, req.UTCDate, dailyLimit); err != nil {
		return InboundAdmission{}, err
	}

	var dailyUsed, accepted, suppressed int64
	if err := tx.QueryRow(ctx, `
		SELECT weighted_units_used, events_accepted, events_suppressed
		FROM x_inbound_daily_usage
		WHERE workspace_id = $1 AND utc_date = $2
		FOR UPDATE
	`, req.WorkspaceID, req.UTCDate).Scan(&dailyUsed, &accepted, &suppressed); err != nil {
		return InboundAdmission{}, err
	}

	var monthlyUsed int64
	if err := tx.QueryRow(ctx, `
		SELECT weighted_units_used
		FROM x_usage_periods
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		FOR UPDATE
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd).Scan(&monthlyUsed); err != nil {
		return InboundAdmission{}, err
	}

	admission := InboundAdmission{
		Decision:          InboundDecisionAccepted,
		WeightedUnits:     req.WeightedUnits,
		InboundDailyLimit: dailyLimit,
		ResetAt:           req.UTCDate.AddDate(0, 0, 1),
	}

	if dailyUsed+req.WeightedUnits > dailyLimit {
		admission.Decision = InboundDecisionSuppressedDailyCap
		suppressed++
		if _, err := tx.Exec(ctx, `
			UPDATE x_inbound_daily_usage
			SET events_suppressed = events_suppressed + 1, updated_at = NOW()
			WHERE workspace_id = $1 AND utc_date = $2
		`, req.WorkspaceID, req.UTCDate); err != nil {
			return InboundAdmission{}, err
		}
		claimed, err := claimInboundThreshold(ctx, tx, req.WorkspaceID, req.UTCDate, 100)
		if err != nil {
			return InboundAdmission{}, err
		}
		admission.Claimed100Percent = claimed
	} else {
		tag, err := tx.Exec(ctx, `
			UPDATE x_usage_periods
			SET weighted_units_used = weighted_units_used + $4, updated_at = NOW()
			WHERE workspace_id = $1
			  AND period_start = $2
			  AND period_end = $3
			  AND weighted_units_used + $4 <= weighted_units_limit
		`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd, req.WeightedUnits)
		if err != nil {
			return InboundAdmission{}, err
		}
		if tag.RowsAffected() != 1 {
			admission.Decision = InboundDecisionSuppressedMonthlyAllowance
			suppressed++
			if _, err := tx.Exec(ctx, `
				UPDATE x_inbound_daily_usage
				SET events_suppressed = events_suppressed + 1, updated_at = NOW()
				WHERE workspace_id = $1 AND utc_date = $2
			`, req.WorkspaceID, req.UTCDate); err != nil {
				return InboundAdmission{}, err
			}
		} else {
			dailyUsed += req.WeightedUnits
			monthlyUsed += req.WeightedUnits
			accepted++
			if _, err := tx.Exec(ctx, `
				UPDATE x_inbound_daily_usage
				SET weighted_units_used = weighted_units_used + $3,
				    events_accepted = events_accepted + 1,
				    updated_at = NOW()
				WHERE workspace_id = $1 AND utc_date = $2
			`, req.WorkspaceID, req.UTCDate, req.WeightedUnits); err != nil {
				return InboundAdmission{}, err
			}
			inboundID := fmt.Sprintf(
				"inbound:%s:%s:%s:%s",
				req.SocialAccountID,
				req.UpstreamResourceType,
				req.UpstreamResourceID,
				req.UTCDate.Format("2006-01-02"),
			)
			if _, err := tx.Exec(ctx, `
				INSERT INTO x_usage_events (
					workspace_id, social_account_id, period_start, period_end,
					operation_key, catalog_version, source, idempotency_key,
					weighted_units, status, connection_mode
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'finalized', $10)
				ON CONFLICT (workspace_id, idempotency_key) DO NOTHING
			`, req.WorkspaceID, req.SocialAccountID, req.PeriodStart, req.PeriodEnd,
				req.OperationKey, req.CatalogVersion, req.Source, inboundID,
				req.WeightedUnits, req.AppMode); err != nil {
				return InboundAdmission{}, err
			}
			if dailyLimit > 0 && dailyUsed*100 >= dailyLimit*80 {
				claimed, err := claimInboundThreshold(ctx, tx, req.WorkspaceID, req.UTCDate, 80)
				if err != nil {
					return InboundAdmission{}, err
				}
				admission.Claimed80Percent = claimed
			}
			if dailyLimit > 0 && dailyUsed >= dailyLimit {
				claimed, err := claimInboundThreshold(ctx, tx, req.WorkspaceID, req.UTCDate, 100)
				if err != nil {
					return InboundAdmission{}, err
				}
				admission.Claimed100Percent = claimed
			}
		}
	}

	admission.InboundDailyUsed = dailyUsed
	admission.EventsAccepted = accepted
	admission.EventsSuppressed = suppressed
	admission.MonthlyUsed = monthlyUsed
	admission.MonthlyRemaining = req.MonthlyAllowance - monthlyUsed
	if admission.MonthlyRemaining < 0 {
		admission.MonthlyRemaining = 0
	}
	switch {
	case admission.Decision == InboundDecisionSuppressedMonthlyAllowance || admission.MonthlyRemaining == 0:
		admission.PausePaidSources = true
		admission.PauseReason = PauseReasonMonthlyAllowance
	case admission.Decision == InboundDecisionSuppressedDailyCap:
		admission.PausePaidSources = true
		admission.PauseReason = PauseReasonDailyCap
	case remainingWithinSafetyBuffer(dailyUsed, dailyLimit):
		admission.PausePaidSources = true
		admission.PauseReason = PauseReasonDailySafetyBuffer
	}

	if _, err := tx.Exec(ctx, `
		UPDATE x_inbound_event_receipts
		SET decision = $6,
		    weighted_units = $7,
		    period_start = $8,
		    period_end = $9,
		    monthly_used_after = $10,
		    monthly_remaining_after = $11,
		    inbound_daily_used_after = $12,
		    inbound_daily_limit = $13,
		    events_accepted_after = $14,
		    events_suppressed_after = $15,
		    pause_paid_sources = $16,
		    pause_reason = $17,
		    reset_at = $18
		WHERE workspace_id = $1
		  AND social_account_id = $2
		  AND upstream_resource_type = $3
		  AND upstream_resource_id = $4
		  AND utc_date = $5
	`, req.WorkspaceID, req.SocialAccountID, req.UpstreamResourceType, req.UpstreamResourceID,
		req.UTCDate, admission.Decision, req.WeightedUnits, req.PeriodStart, req.PeriodEnd,
		admission.MonthlyUsed, admission.MonthlyRemaining, admission.InboundDailyUsed,
		admission.InboundDailyLimit, admission.EventsAccepted, admission.EventsSuppressed,
		admission.PausePaidSources, admission.PauseReason, admission.ResetAt); err != nil {
		return InboundAdmission{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return InboundAdmission{}, err
	}
	return admission, nil
}

func loadDuplicateInboundAdmission(ctx context.Context, tx pgx.Tx, req StoreInboundRequest) (InboundAdmission, error) {
	var receipt inboundReceiptSnapshot
	if err := tx.QueryRow(ctx, `
		SELECT
			decision,
			weighted_units,
			period_start,
			period_end,
			monthly_used_after,
			monthly_remaining_after,
			inbound_daily_used_after,
			inbound_daily_limit,
			events_accepted_after,
			events_suppressed_after,
			pause_paid_sources,
			pause_reason,
			reset_at
		FROM x_inbound_event_receipts
		WHERE workspace_id = $1
		  AND social_account_id = $2
		  AND upstream_resource_type = $3
		  AND upstream_resource_id = $4
		  AND utc_date = $5
		FOR UPDATE
	`, req.WorkspaceID, req.SocialAccountID, req.UpstreamResourceType, req.UpstreamResourceID,
		req.UTCDate).Scan(
		&receipt.Decision,
		&receipt.WeightedUnits,
		&receipt.PeriodStart,
		&receipt.PeriodEnd,
		&receipt.MonthlyUsedAfter,
		&receipt.MonthlyRemainingAfter,
		&receipt.InboundDailyUsedAfter,
		&receipt.InboundDailyLimit,
		&receipt.EventsAcceptedAfter,
		&receipt.EventsSuppressedAfter,
		&receipt.PausePaidSources,
		&receipt.PauseReason,
		&receipt.ResetAt,
	); err != nil {
		return InboundAdmission{}, err
	}
	return admissionFromReceipt(receipt), nil
}

func claimInboundThreshold(ctx context.Context, tx pgx.Tx, workspaceID string, utcDate time.Time, threshold int16) (bool, error) {
	var claimed int16
	err := tx.QueryRow(ctx, `
		INSERT INTO x_inbound_cap_notifications (workspace_id, utc_date, threshold)
		VALUES ($1, $2, $3)
		ON CONFLICT (workspace_id, utc_date, threshold) DO NOTHING
		RETURNING threshold
	`, workspaceID, utcDate, threshold).Scan(&claimed)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return claimed == threshold, nil
}

func (s *PostgresStore) UpdateInboundCap(ctx context.Context, req StoreUpdateInboundCapRequest) (InboundCapSetting, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return InboundCapSetting{}, err
	}
	defer tx.Rollback(ctx)

	utc := req.Now.UTC()
	utcDate := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	lockKey := fmt.Sprintf("x-inbound-cap:%s:%s", req.WorkspaceID, utcDate.Format("2006-01-02"))
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return InboundCapSetting{}, err
	}

	var monthlyUsed int64
	err = tx.QueryRow(ctx, `
		SELECT weighted_units_used
		FROM x_usage_periods
		WHERE workspace_id = $1 AND period_start = $2 AND period_end = $3
		FOR UPDATE
	`, req.WorkspaceID, req.PeriodStart, req.PeriodEnd).Scan(&monthlyUsed)
	if errors.Is(err, pgx.ErrNoRows) {
		monthlyUsed = 0
	} else if err != nil {
		return InboundCapSetting{}, err
	}
	if req.InboundDailyLimit > req.MonthlyAllowance-monthlyUsed {
		return InboundCapSetting{}, ErrInboundCapExceedsMonthlyRemaining
	}

	setting := InboundCapSetting{}
	err = tx.QueryRow(ctx, `
		INSERT INTO x_inbound_cap_settings (
			workspace_id, inbound_daily_limit, updated_by, acknowledged_exposure, updated_at
		) VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (workspace_id) DO UPDATE SET
			inbound_daily_limit = EXCLUDED.inbound_daily_limit,
			updated_by = EXCLUDED.updated_by,
			acknowledged_exposure = EXCLUDED.acknowledged_exposure,
			updated_at = EXCLUDED.updated_at
		RETURNING inbound_daily_limit, updated_by, acknowledged_exposure, updated_at
	`, req.WorkspaceID, req.InboundDailyLimit, req.UpdatedBy, req.AcknowledgedExposure, req.Now).Scan(
		&setting.InboundDailyLimit,
		&setting.UpdatedBy,
		&setting.AcknowledgedExposure,
		&setting.UpdatedAt,
	)
	if err != nil {
		return InboundCapSetting{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE x_inbound_daily_usage
		SET weighted_units_limit = $3, updated_at = NOW()
		WHERE workspace_id = $1 AND utc_date = $2
	`, req.WorkspaceID, utcDate, req.InboundDailyLimit); err != nil {
		return InboundCapSetting{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return InboundCapSetting{}, err
	}
	return setting, nil
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

	var inboundUsed, inboundAccepted, inboundSuppressed int64
	var storedInboundLimit int64
	err = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(weighted_units_used, 0),
			weighted_units_limit,
			events_accepted,
			events_suppressed
		FROM x_inbound_daily_usage
		WHERE workspace_id = $1 AND utc_date = $2
	`, workspaceID, now.UTC().Format("2006-01-02")).Scan(
		&inboundUsed,
		&storedInboundLimit,
		&inboundAccepted,
		&inboundSuppressed,
	)
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
	var customInboundLimit int64
	err = s.pool.QueryRow(ctx, `
		SELECT inbound_daily_limit
		FROM x_inbound_cap_settings
		WHERE workspace_id = $1
	`, workspaceID).Scan(&customInboundLimit)
	if err == nil {
		dailyLimit = int64Pointer(customInboundLimit)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, err
	} else if storedInboundLimit > 0 {
		dailyLimit = int64Pointer(storedInboundLimit)
	}

	utc := now.UTC()
	resetAt := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	inboundPercent := 0.0
	pausePaidSources := false
	pauseReason := ""
	if dailyLimit != nil {
		if *dailyLimit > 0 {
			inboundPercent = float64(inboundUsed) / float64(*dailyLimit) * 100
			if inboundPercent > 100 {
				inboundPercent = 100
			}
		}
		if remainingWithinSafetyBuffer(inboundUsed, *dailyLimit) {
			pausePaidSources = true
			if inboundUsed >= *dailyLimit {
				pauseReason = PauseReasonDailyCap
			} else {
				pauseReason = PauseReasonDailySafetyBuffer
			}
		}
	}
	if monthlyRemaining != nil && *monthlyRemaining == 0 {
		pausePaidSources = true
		pauseReason = PauseReasonMonthlyAllowance
	}
	return Snapshot{
		PlanID:             period.PlanID,
		PeriodStart:        period.Start,
		PeriodEnd:          period.End,
		MonthlyAllowance:   monthlyAllowance,
		MonthlyUsed:        used,
		MonthlyRemaining:   monthlyRemaining,
		InboundDailyUsed:   inboundUsed,
		InboundDailyLimit:  dailyLimit,
		InboundAccepted:    inboundAccepted,
		InboundSuppressed:  inboundSuppressed,
		InboundResetAt:     resetAt,
		InboundPercent:     inboundPercent,
		PausePaidSources:   pausePaidSources,
		InboundPauseReason: pauseReason,
		CatalogVersion:     CatalogVersion,
	}, nil
}
