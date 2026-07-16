package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestXInboxReconciliationIndexesMigration(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("migrations/116_x_inbox_reconciliation_indexes.sql")
	if err != nil {
		t.Fatalf("read migration 116: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"-- +goose NO TRANSACTION\n-- +goose Up",
		"x_inbox_outbound_reconciliation_current_idx",
		"WHERE status NOT IN ('completed', 'succeeded')",
		"x_inbox_outbound_reconciliation_day_idx",
		"x_inbox_confirmation_running_lease_idx",
		"x_inbox_confirmation_pending_expiry_idx",
		"x_inbox_confirmation_completed_day_idx",
		"x_inbox_exposure_reconciliation_current_idx",
		"x_inbox_items_x_latency_day_idx",
		"x_usage_events_settled_day_idx",
		"x_inbound_receipts_evidence_day_idx",
		"x_inbound_notifications_reconciliation_idx",
		"x_inbox_cleanup_lease_idx",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("migration 116 missing %q", want)
		}
	}
	if got := strings.Count(text, "CREATE INDEX CONCURRENTLY IF NOT EXISTS"); got != 15 {
		t.Fatalf("migration Up has %d retry-safe concurrent indexes, want 15", got)
	}
	if got := strings.Count(text, "DROP INDEX CONCURRENTLY IF EXISTS"); got != 15 {
		t.Fatalf("migration Down has %d retry-safe concurrent drops, want 15", got)
	}
}

func TestXInboxReconciliationIndexesFreshDownUp(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	paths, err := filepath.Glob("migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	ctx := context.Background()
	for _, path := range paths {
		if filepath.Base(path) == "116_x_inbox_reconciliation_indexes.sql" {
			continue
		}
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		up := strings.Split(string(migration), "-- +goose Down")[0]
		up = strings.Replace(up, "-- +goose Up", "", 1)
		if _, execErr := database.ExecContext(ctx, up); execErr != nil {
			t.Fatalf("fresh apply %s: %v", path, execErr)
		}
	}

	if _, err := goose.EnsureDBVersion(database); err != nil {
		t.Fatalf("initialize Goose version table: %v", err)
	}
	migrations, err := goose.CollectMigrations("migrations", 115, 116)
	if err != nil {
		t.Fatal(err)
	}
	if len(migrations) != 1 || migrations[0].Version != 116 {
		t.Fatalf("Goose collected migrations = %+v, want only 116", migrations)
	}
	migration := migrations[0]
	if err := migration.UpContext(ctx, database); err != nil {
		t.Fatalf("migration 116 Goose Up: %v", err)
	}
	assertXInboxReconciliationIndexExists(t, ctx, database, true)

	if err := migration.DownContext(ctx, database); err != nil {
		t.Fatalf("migration 116 Down: %v", err)
	}
	assertXInboxReconciliationIndexExists(t, ctx, database, false)

	if err := migration.UpContext(ctx, database); err != nil {
		t.Fatalf("migration 116 re-Up: %v", err)
	}
	assertXInboxReconciliationIndexExists(t, ctx, database, true)
}

func assertXInboxReconciliationIndexExists(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	want bool,
) {
	t.Helper()
	var exists bool
	if err := database.QueryRowContext(ctx, `
		SELECT to_regclass('public.x_usage_events_settled_day_idx') IS NOT NULL
	`).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists != want {
		t.Fatalf("x_usage_events_settled_day_idx exists = %t, want %t", exists, want)
	}
}
