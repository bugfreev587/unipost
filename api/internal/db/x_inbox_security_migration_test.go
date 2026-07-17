package db

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestXInboxSecurityMigrationAddsRouteKeyAndDateIndependentReceiptIdentity(t *testing.T) {
	source, err := os.ReadFile("migrations/112_x_inbox_webhook_security_and_atomic_receipts.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(source), "-- +goose Down")[0]
	for _, required := range []string{
		"ADD COLUMN webhook_route_key TEXT",
		"platform_credentials_webhook_route_key_idx",
		"DROP CONSTRAINT x_inbound_event_receipts_pkey",
		"PRIMARY KEY (workspace_id, social_account_id, upstream_resource_type, upstream_resource_id)",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	if strings.Contains(up, "consumer_secret") && strings.Contains(up, "digest(") {
		t.Fatal("migration must not attempt to derive route keys from encrypted consumer secrets")
	}
}

func TestXInboxRouteRotationMigrationTracksWebhookGenerationsAndCleanupSecrets(t *testing.T) {
	source, err := os.ReadFile("migrations/113_x_inbox_webhook_route_rotation.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(source), "-- +goose Down")[0]
	for _, required := range []string{
		"activity_webhook_route_key",
		"webhook_route_key",
		"consumer_secret",
		"x_inbox_delivery_cleanup_intents",
		"augment_replaced_workspace_x_inbox_cleanup_route",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("route rotation migration missing %q", required)
		}
	}
}

func TestXInboxSecurityMigrationExecutesAndInstallsDateIndependentReceiptKey(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	ctx := context.Background()
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	var hasReceipts bool
	if err := tx.QueryRowContext(ctx, `
		SELECT to_regclass('public.x_inbound_event_receipts') IS NOT NULL
	`).Scan(&hasReceipts); err != nil {
		t.Fatal(err)
	}
	if !hasReceipts {
		applyMigrationUp(t, ctx, tx, "migrations/109_x_inbound_usage_controls.sql")
	}

	var hasRouteKey bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'platform_credentials'
			  AND column_name = 'webhook_route_key'
		)
	`).Scan(&hasRouteKey); err != nil {
		t.Fatal(err)
	}
	if !hasRouteKey {
		applyMigrationUp(t, ctx, tx, "migrations/112_x_inbox_webhook_security_and_atomic_receipts.sql")
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT a.attname
		FROM pg_constraint c
		JOIN unnest(c.conkey) WITH ORDINALITY AS key(attnum, ordinality) ON TRUE
		JOIN pg_attribute a
		  ON a.attrelid = c.conrelid
		 AND a.attnum = key.attnum
		WHERE c.conrelid = 'x_inbound_event_receipts'::regclass
		  AND c.contype = 'p'
		ORDER BY key.ordinality
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var primaryKey []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		primaryKey = append(primaryKey, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{"workspace_id", "social_account_id", "upstream_resource_type", "upstream_resource_id"}
	if !reflect.DeepEqual(primaryKey, want) {
		t.Fatalf("receipt primary key = %v, want %v", primaryKey, want)
	}
}
