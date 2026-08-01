package db

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func compactSocialConnectionSQL(source string) string {
	withoutBlockComments := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(source, " ")
	lines := strings.Split(withoutBlockComments, "\n")
	for index, line := range lines {
		if commentAt := strings.Index(line, "--"); commentAt >= 0 {
			lines[index] = line[:commentAt]
		}
	}
	return strings.Join(strings.Fields(strings.ToLower(strings.Join(lines, "\n"))), " ")
}

func readSocialConnectionsMigration(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile("migrations/138_social_connections_and_profile_bindings.sql")
	if err != nil {
		t.Fatalf("read migration 138: %v", err)
	}
	return string(body)
}

func TestSocialConnectionMigrationsAreExpandOnly(t *testing.T) {
	migration137 := compactSocialConnectionSQL(readSocialConnectionsMigration(t))

	for _, forbidden := range []string{
		"update social_accounts sa set status = 'reconnect_required'",
		"insert into social_connection_migration_audit",
		"set connection_id = sc.id",
		"lock table social_accounts in share row exclusive mode",
	} {
		if strings.Contains(migration137, forbidden) {
			t.Fatalf("migration 138 contains cutover mutation %q", forbidden)
		}
	}

	for _, required := range []string{
		"create table social_connections",
		"create table social_connection_migration_conflicts",
		"create table social_connection_migration_audit",
		"create table social_connection_rollout_state",
		"values ('global', 'expand')",
		"add column connection_id text",
		"add column binding_version bigint not null default 1",
		"add column binding_status text not null default 'active'",
		"enforce_social_connection_authority_write_gate",
		"cutover_backend_pid",
		"pg_backend_pid()",
	} {
		if !strings.Contains(migration137, required) {
			t.Fatalf("migration 138 missing Expand contract %q", required)
		}
	}

	for migration, forbiddenMutations := range map[string][]string{
		"139_delivery_job_connection_snapshot.sql": {
			"update post_delivery_jobs j set connection_id",
		},
		"140_inbox_connection_deduplication.sql": {
			"update inbox_items i set connection_id",
			"insert into inbox_item_supersessions",
		},
		"141_physical_daily_publish_reservations.sql": {
			"delete from physical_daily_publish_reservations reservation using social_accounts",
			"update physical_daily_publish_operations operation set physical_account_id = account.connection_id",
		},
	} {
		body, err := os.ReadFile("migrations/" + migration)
		if err != nil {
			t.Fatalf("read %s: %v", migration, err)
		}
		compact := compactSocialConnectionSQL(string(body))
		for _, forbidden := range forbiddenMutations {
			if strings.Contains(compact, forbidden) {
				t.Fatalf("%s contains cutover backfill %q", migration, forbidden)
			}
		}
	}
}

func TestSocialConnectionExpandGateContract(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))
	for _, required := range []string{
		"rollout_phase = 'expand'",
		"new.connection_id is null",
		"rollout_phase in ('cutting_over', 'cutover')",
		"authority.workspace_id is distinct from profile_workspace_id",
		"authority.platform is distinct from new.platform",
		"old.connection_id is not null",
		"classified social account bindings cannot change their social connection",
		"coalesce(new.binding_status, 'active') <> 'unbound'",
		"set last_legacy_write_at = now()",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("Expand authority gate missing %q", required)
		}
	}
}

func TestSocialConnectionsMigrationKeepsLegacyFallback(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))
	if strings.Contains(sql, "alter column connection_id set not null") {
		t.Fatal("Expand rollout must keep connection_id nullable")
	}
	for _, forbidden := range []string{
		"drop column access_token",
		"drop column refresh_token",
		"drop column external_user_id",
		"drop column external_account_id",
	} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("Expand migration must not %s", forbidden)
		}
	}
}

func TestSocialConnectionsMigrationPreservesReconnectIdentity(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))
	indexAt := strings.Index(sql, "create unique index social_connections_canonical_identity_unique_idx")
	if indexAt < 0 {
		t.Fatal("canonical identity unique index is missing")
	}
	indexSQL := sql[indexAt:]
	if statementEnd := strings.Index(indexSQL, ";"); statementEnd >= 0 {
		indexSQL = indexSQL[:statementEnd]
	}
	if strings.Contains(indexSQL, "status = 'active'") || strings.Contains(indexSQL, "status <> 'disconnected'") {
		t.Fatal("disconnected canonical connections must remain inside uniqueness for stable reconnect")
	}
	if !strings.Contains(indexSQL, "status <> 'migration_conflict'") {
		t.Fatal("only migration conflicts may be excluded from canonical uniqueness")
	}
}

func TestInboxConnectionMigrationRollbackPreservesSupersededEvidence(t *testing.T) {
	body, err := os.ReadFile("migrations/140_inbox_connection_deduplication.sql")
	if err != nil {
		t.Fatal(err)
	}
	downParts := strings.Split(string(body), "-- +goose Down")
	if len(downParts) != 2 {
		t.Fatal("migration 140 must have one Down section")
	}
	down := compactSocialConnectionSQL(downParts[1])
	if !strings.Contains(down, "if exists (select 1 from inbox_item_supersessions)") ||
		!strings.Contains(down, "raise exception 'refusing destructive rollback") {
		t.Fatal("migration 140 Down must fail closed while superseded Inbox evidence exists")
	}
	if strings.Contains(down, "delete from inbox_items") {
		t.Fatal("migration 140 Down must never delete preserved customer Inbox rows")
	}
}

func TestSocialConnectionsExpandAllowsLegacyAndMirrorsExistingAuthority(t *testing.T) {
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
	requireEmptyPublicSchemaForTest(t, ctx, tx)
	applyMigrationRangeForTest(t, ctx, tx, 1, 135)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email) VALUES
			('expand-user-a', 'expand-a@example.com'),
			('expand-user-b', 'expand-b@example.com');
		INSERT INTO workspaces (id, user_id, name) VALUES
			('expand-workspace-a', 'expand-user-a', 'A'),
			('expand-workspace-b', 'expand-user-b', 'B');
		INSERT INTO profiles (id, workspace_id, name) VALUES
			('expand-profile-a', 'expand-workspace-a', 'A'),
			('expand-profile-b', 'expand-workspace-b', 'B');
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type
		) VALUES (
			'legacy-before-expand', 'expand-profile-a', 'linkedin', 'legacy-token',
			'legacy-provider', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		);
	`); err != nil {
		t.Fatalf("seed Expand fixture: %v", err)
	}

	applyMigrationUp(t, ctx, tx, "migrations/138_social_connections_and_profile_bindings.sql")

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type
		) VALUES (
			'legacy-during-expand', 'expand-profile-a', 'twitter', 'legacy-token-2',
			'legacy-provider-2', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		)
	`); err != nil {
		t.Fatalf("Expand rejected legacy insert: %v", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO social_connections (
			id, workspace_id, platform, provider_identity, access_token,
			metadata, scope, status, connection_type
		) VALUES (
			'connection-a', 'expand-workspace-a', 'linkedin', 'legacy-provider',
			'legacy-token', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		);
		UPDATE social_accounts
		SET connection_id = 'connection-a'
		WHERE id = 'legacy-before-expand';
		UPDATE social_accounts
		SET access_token = 'refreshed-token'
		WHERE id = 'legacy-before-expand';
	`); err != nil {
		t.Fatalf("Expand mirror refresh: %v", err)
	}

	var authorityToken string
	if err := tx.QueryRowContext(ctx, `SELECT access_token FROM social_connections WHERE id = 'connection-a'`).Scan(&authorityToken); err != nil {
		t.Fatal(err)
	}
	if authorityToken != "refreshed-token" {
		t.Fatalf("mirrored token = %q, want refreshed-token", authorityToken)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE social_accounts
		SET binding_status = 'unbound', status = 'disconnected', disconnected_at = NOW()
		WHERE id = 'legacy-before-expand'
	`); err != nil {
		t.Fatalf("unbind update: %v", err)
	}
	var authorityStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM social_connections WHERE id = 'connection-a'`).Scan(&authorityStatus); err != nil {
		t.Fatal(err)
	}
	if authorityStatus != "active" {
		t.Fatalf("binding unbind mirrored physical status %q, want active", authorityStatus)
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT cross_workspace"); err != nil {
		t.Fatal(err)
	}
	_, scopeErr := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type, connection_id
		) VALUES (
			'cross-workspace-binding', 'expand-profile-b', 'linkedin', 'refreshed-token',
			'legacy-provider', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo', 'connection-a'
		)
	`)
	if scopeErr == nil {
		t.Fatal("cross-Workspace binding unexpectedly succeeded")
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT cross_workspace"); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE social_connection_rollout_state SET phase = 'cutover' WHERE id = 'global'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT cutover_legacy"); err != nil {
		t.Fatal(err)
	}
	_, legacyErr := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type
		) VALUES (
			'legacy-after-cutover', 'expand-profile-a', 'twitter', 'token',
			'provider', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		)
	`)
	if legacyErr == nil {
		t.Fatal("cutover accepted a null-connection binding")
	}
}
