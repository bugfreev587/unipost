//go:build integration

package paidquota

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/quota"
)

const paidQuotaIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

type fixedSnapshotPostgresHoldTx struct {
	*postgresHoldPeriodTx
	snapshot quota.MonthlySnapshot
}

func (t *fixedSnapshotPostgresHoldTx) Snapshot(context.Context) (quota.MonthlySnapshot, error) {
	return t.snapshot, nil
}

func TestPostgresHoldServiceUsesScheduledQuotaSnapshotWithLegacyFallback(t *testing.T) {
	pool := openPaidQuotaIntegrationPool(t)
	setupPaidQuotaIntegrationSchema(t, pool)

	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	scheduledAt := now.Add(24 * time.Hour)
	tests := []struct {
		name       string
		status     string
		metadata   string
		wantStatus string
	}{
		{
			name:       "valid snapshot preserves mixed scheduled post",
			status:     "scheduled",
			metadata:   `{"scheduled_quota_units":1,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}`,
			wantStatus: "scheduled",
		},
		{
			name:       "valid snapshot releases mixed quota hold",
			status:     "quota_hold",
			metadata:   `{"scheduled_quota_units":1,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}`,
			wantStatus: "scheduled",
		},
		{
			name:       "missing snapshot keeps legacy target count",
			status:     "scheduled",
			metadata:   `{"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}`,
			wantStatus: "quota_hold",
		},
		{
			name:       "invalid snapshot keeps legacy target count",
			status:     "scheduled",
			metadata:   `{"scheduled_quota_units":-1,"platform_posts":[{"account_id":"account_tiktok"},{"account_id":"account_instagram"}]}`,
			wantStatus: "quota_hold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if _, err := pool.Exec(ctx, `TRUNCATE social_posts`); err != nil {
				t.Fatalf("reset social posts: %v", err)
			}
			if _, err := pool.Exec(ctx, `
				INSERT INTO social_posts (
					id, workspace_id, status, scheduled_at, created_at, metadata,
					quota_hold_reason, quota_hold_at, quota_hold_original_scheduled_at
				) VALUES (
					'post_under_test', 'workspace_paid', $1::text, $2::timestamptz, $3::timestamptz, $4::jsonb,
					CASE WHEN $1::text = 'quota_hold' THEN 'prior_limit' END,
					CASE WHEN $1::text = 'quota_hold' THEN $3::timestamptz END,
					CASE WHEN $1::text = 'quota_hold' THEN $2::timestamptz END
				)
			`, tt.status, scheduledAt, now.Add(-time.Hour), tt.metadata); err != nil {
				t.Fatalf("seed scheduled parent: %v", err)
			}

			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatalf("begin hold reconciliation: %v", err)
			}
			periodTx := &fixedSnapshotPostgresHoldTx{
				postgresHoldPeriodTx: &postgresHoldPeriodTx{
					tx:          tx,
					workspaceID: "workspace_paid",
					period:      "2026-07",
				},
				snapshot: quota.MonthlySnapshot{PlanID: "basic", Limit: 1},
			}
			service := NewHoldService(nil, func() time.Time { return now })
			if err := service.reconcilePeriodTransaction(ctx, periodTx, "capacity_reconciled", time.Time{}, now, nil); err != nil {
				_ = tx.Rollback(ctx)
				t.Fatalf("reconcile hold service: %v", err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatalf("commit hold reconciliation: %v", err)
			}

			var gotStatus string
			if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = 'post_under_test'`).Scan(&gotStatus); err != nil {
				t.Fatalf("load reconciled status: %v", err)
			}
			if gotStatus != tt.wantStatus {
				t.Fatalf("reconciled status = %q, want %q", gotStatus, tt.wantStatus)
			}
		})
	}
}

func openPaidQuotaIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(paidQuotaIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", paidQuotaIntegrationDatabaseEnv)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL integration URL: %v", err)
	}
	host := strings.TrimSpace(config.ConnConfig.Host)
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("%s must use a loopback host, got %q", paidQuotaIntegrationDatabaseEnv, host)
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL integration service: %v", err)
	}
	schema := fmt.Sprintf("pr270_paidquota_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatalf("connect isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop isolated PostgreSQL schema: %v", err)
		}
		admin.Close()
	})
	return pool
}

func setupPaidQuotaIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE social_accounts (
			id TEXT PRIMARY KEY,
			disconnected_at TIMESTAMPTZ
		);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL,
			scheduled_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			deleted_at TIMESTAMPTZ,
			quota_hold_reason TEXT,
			quota_hold_at TIMESTAMPTZ,
			quota_hold_original_scheduled_at TIMESTAMPTZ
		);
		CREATE TABLE social_post_results (
			id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL,
			social_account_id TEXT NOT NULL,
			status TEXT NOT NULL
		);
		CREATE TABLE admin_post_quota_resets (
			workspace_id TEXT NOT NULL,
			period TEXT NOT NULL,
			quota_kind TEXT NOT NULL,
			reset_at TIMESTAMPTZ NOT NULL
		);
		INSERT INTO social_accounts (id) VALUES ('account_tiktok'), ('account_instagram');
	`)
	if err != nil {
		t.Fatalf("setup paid quota integration schema: %v", err)
	}
}
