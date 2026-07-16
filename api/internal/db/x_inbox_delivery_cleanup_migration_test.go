package db

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestXInboxDeliveryCleanupMigrationCapturesWorkspaceCascadeBeforeChildrenDisappear(t *testing.T) {
	databaseURL := os.Getenv("X_INBOX_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("X_INBOX_TEST_DATABASE_URL is not configured")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	var hasXAppMode bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'social_accounts'
			  AND column_name = 'x_app_mode'
		)
	`).Scan(&hasXAppMode); err != nil {
		t.Fatal(err)
	}
	if !hasXAppMode {
		applyMigrationUp(t, ctx, tx, "migrations/108_x_inbox_oauth_and_delivery.sql")
	}

	var hasCleanupTable bool
	if err := tx.QueryRowContext(ctx, `
		SELECT to_regclass('public.x_inbox_delivery_cleanup_intents') IS NOT NULL
	`).Scan(&hasCleanupTable); err != nil {
		t.Fatal(err)
	}
	if !hasCleanupTable {
		applyMigrationUp(t, ctx, tx, "migrations/110_x_inbox_delivery_cleanup_intents.sql")
	}

	var hasWorkspaceTrigger bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_trigger
			WHERE tgrelid = 'workspaces'::regclass
			  AND tgname = 'workspaces_x_inbox_delivery_cleanup'
			  AND NOT tgisinternal
			  AND (tgtype & 2) <> 0
			  AND (tgtype & 8) <> 0
		)
	`).Scan(&hasWorkspaceTrigger); err != nil {
		t.Fatal(err)
	}
	if !hasWorkspaceTrigger {
		t.Fatal("cleanup migration must install a BEFORE DELETE trigger on workspaces")
	}

	const (
		userID      = "x-inbox-cascade-test-user"
		workspaceID = "x-inbox-cascade-test-workspace"
		profileID   = "x-inbox-cascade-test-profile"
		accountID   = "x-inbox-cascade-test-account"
	)
	seedStatements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, email) VALUES ($1, $2)`, []any{userID, userID + "@example.invalid"}},
		{`INSERT INTO workspaces (id, user_id, name) VALUES ($1, $2, 'X Inbox Cascade Test')`, []any{workspaceID, userID}},
		{`INSERT INTO profiles (id, workspace_id, name) VALUES ($1, $2, 'X Inbox Cascade Test')`, []any{profileID, workspaceID}},
		{`
			INSERT INTO platform_credentials (
				id, workspace_id, platform, client_id, client_secret, app_bearer_token
			) VALUES (
				'x-inbox-cascade-test-credential', $1, 'twitter', 'client', 'secret',
				'encrypted-workspace-bearer'
			)
		`, []any{workspaceID}},
		{`
			INSERT INTO social_accounts (
				id, profile_id, platform, access_token, external_account_id, status,
				connection_type, x_app_mode
			) VALUES (
				$1, $2, 'twitter', 'encrypted-user-token', 'x-user', 'active',
				'managed', 'workspace_x_app'
			)
		`, []any{accountID, profileID}},
		{`
			INSERT INTO x_inbox_delivery_resources (
				social_account_id, filtered_stream_rule_id, activity_dm_subscription_id
			) VALUES ($1, 'rule-exact', 'subscription-exact')
		`, []any{accountID}},
	}
	for _, statement := range seedStatements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("seed workspace topology: %v", err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID); err != nil {
		t.Fatalf("delete workspace: %v", err)
	}

	var (
		appMode        string
		appBearer      string
		ruleID         string
		subscriptionID string
	)
	if err := tx.QueryRowContext(ctx, `
		SELECT x_app_mode, app_bearer_token, filtered_stream_rule_id, activity_dm_subscription_id
		FROM x_inbox_delivery_cleanup_intents
		WHERE social_account_id = $1
	`, accountID).Scan(&appMode, &appBearer, &ruleID, &subscriptionID); err != nil {
		t.Fatalf("query workspace cleanup intent: %v", err)
	}
	if appMode != "workspace_x_app" ||
		appBearer != "encrypted-workspace-bearer" ||
		ruleID != "rule-exact" ||
		subscriptionID != "subscription-exact" {
		t.Fatalf(
			"cleanup intent = mode %q bearer %q rule %q subscription %q",
			appMode,
			appBearer,
			ruleID,
			subscriptionID,
		)
	}
}

func applyMigrationUp(t *testing.T, ctx context.Context, tx *sql.Tx, path string) {
	t.Helper()
	migration, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	upSQL := strings.Split(string(migration), "-- +goose Down")[0]
	upSQL = strings.Replace(upSQL, "-- +goose Up", "", 1)
	if _, err := tx.ExecContext(ctx, upSQL); err != nil {
		t.Fatalf("apply %s in transaction: %v", path, err)
	}
}
