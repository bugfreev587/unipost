//go:build integration

package handler

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/xcredits"
)

func TestXReadExposureRecoveryPurposeAndConcurrentSettlementPostgres(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	ctx := context.Background()
	periodStart := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := periodStart.AddDate(0, 1, 0)
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

	if _, err := pool.Exec(ctx, `
		CREATE TABLE x_usage_periods (
			workspace_id TEXT NOT NULL,
			period_start TIMESTAMPTZ NOT NULL,
			period_end TIMESTAMPTZ NOT NULL,
			weighted_units_used BIGINT NOT NULL,
			weighted_units_limit BIGINT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (workspace_id, period_start, period_end)
		);
		CREATE TABLE x_inbox_backfill_exposure_reservations (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			social_account_id TEXT NOT NULL,
			operation_key TEXT NOT NULL,
			idempotency_key TEXT NOT NULL,
			requested_resources INTEGER NOT NULL,
			reserved_units BIGINT NOT NULL,
			actual_units BIGINT,
			period_start TIMESTAMPTZ NOT NULL,
			period_end TIMESTAMPTZ NOT NULL,
			utc_date DATE NOT NULL,
			status TEXT NOT NULL,
			reconciliation_deadline TIMESTAMPTZ,
			reconciliation_attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TIMESTAMPTZ NOT NULL,
			last_error TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			accounting_enabled BOOLEAN NOT NULL,
			purpose TEXT NOT NULL,
			external_user_id TEXT,
			safety_policy TEXT NOT NULL,
			units_per_resource BIGINT NOT NULL,
			catalog_version TEXT NOT NULL,
			app_mode TEXT NOT NULL,
			bypass_reason TEXT,
			finalized_at TIMESTAMPTZ,
			UNIQUE (workspace_id, idempotency_key)
		);
		CREATE VIEW x_read_exposures AS
			SELECT * FROM x_inbox_backfill_exposure_reservations;
		CREATE TABLE x_read_settlement_audit (
			exposure_id TEXT PRIMARY KEY,
			settled BOOLEAN NOT NULL
		);
	`); err != nil {
		t.Fatalf("create X read exposure schema: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO x_usage_periods (
			workspace_id, period_start, period_end, weighted_units_used, weighted_units_limit
		) VALUES ('ws_1', $1, $2, 10, 1000)
	`, periodStart, periodEnd); err != nil {
		t.Fatalf("seed X Credits usage: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO x_read_exposures (
			id, workspace_id, social_account_id, operation_key, idempotency_key,
			requested_resources, reserved_units, actual_units, period_start, period_end,
			utc_date, status, reconciliation_deadline, next_attempt_at,
			accounting_enabled, purpose, safety_policy, units_per_resource,
			catalog_version, app_mode
		) VALUES
			('inbox_exposure', 'ws_1', 'sa_1', 'post.read', 'inbox_key',
			 10, 10, NULL, $1, $2, $3, 'release_pending', $4, $4,
			 FALSE, 'inbox_backfill', 'request_limits', 1, 'catalog', 'unipost_managed_app'),
			('account_exposure', 'ws_1', 'sa_1', 'post.read', 'account_key',
			 10, 10, 7, $1, $2, $3, 'finalize_pending', $4, $4,
			 TRUE, 'account_post_history', 'request_limits', 1, 'catalog', 'unipost_managed_app')
	`, periodStart, periodEnd, now, now.Add(-time.Minute)); err != nil {
		t.Fatalf("seed X read exposures: %v", err)
	}

	store := xcredits.NewPostgresService(pool, db.New(pool))
	stats, err := store.ReconcilePendingExposures(ctx, 10, now)
	if err != nil {
		t.Fatalf("reconcile Inbox exposures: %v", err)
	}
	if stats.Scanned != 1 || stats.Released != 1 || stats.Finalized != 0 || stats.Deferred != 0 {
		t.Fatalf("Inbox recovery stats = %+v", stats)
	}
	var inboxStatus, accountStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM x_read_exposures WHERE id = 'inbox_exposure'`).Scan(&inboxStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM x_read_exposures WHERE id = 'account_exposure'`).Scan(&accountStatus); err != nil {
		t.Fatal(err)
	}
	if inboxStatus != "released" || accountStatus != "finalize_pending" {
		t.Fatalf("statuses = inbox:%q account:%q", inboxStatus, accountStatus)
	}

	start := make(chan struct{})
	errors := make(chan error, 2)
	go func() {
		<-start
		errors <- store.FinalizeExposure(ctx, "account_exposure", 7)
	}()
	go func() {
		<-start
		errors <- store.FinalizeExposureWithMutation(
			ctx,
			"account_exposure",
			7,
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `
					INSERT INTO x_read_settlement_audit (exposure_id, settled)
					VALUES ('account_exposure', TRUE)
					ON CONFLICT (exposure_id) DO UPDATE SET settled = EXCLUDED.settled
				`)
				return err
			},
		)
	}()
	close(start)
	for range 2 {
		if err := <-errors; err != nil {
			t.Fatalf("concurrent settlement: %v", err)
		}
	}

	var used int64
	var actual int64
	var settled bool
	if err := pool.QueryRow(ctx, `
		SELECT p.weighted_units_used, e.status, e.actual_units, a.settled
		FROM x_usage_periods p
		JOIN x_read_exposures e ON e.workspace_id = p.workspace_id
		JOIN x_read_settlement_audit a ON a.exposure_id = e.id
		WHERE e.id = 'account_exposure'
	`).Scan(&used, &accountStatus, &actual, &settled); err != nil {
		t.Fatalf("load concurrent settlement: %v", err)
	}
	if used != 7 || accountStatus != "finalized" || actual != 7 || !settled {
		t.Fatalf("settlement = used:%d status:%q actual:%d receipt:%t", used, accountStatus, actual, settled)
	}
}
