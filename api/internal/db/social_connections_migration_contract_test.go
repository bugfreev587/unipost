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
	body, err := os.ReadFile("migrations/121_social_connections_and_profile_bindings.sql")
	if err != nil {
		t.Fatalf("read migration 121: %v", err)
	}
	return string(body)
}

func TestSocialConnectionsMigrationContract(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))

	for _, want := range []string{
		"create table social_connections",
		"external_user_id text",
		"provider_identity text",
		"status text not null",
		"create table social_connection_migration_conflicts",
		"add column connection_id text",
		"add column binding_version bigint not null default 1",
		"add column binding_status text not null default 'active'",
		"metadata->>'instagram_webhook_user_id'",
		"platform <> 'instagram'",
		"external_account_id",
		"status <> 'migration_conflict'",
		"social_connections_canonical_identity_unique_idx",
		"social_accounts_profile_connection_unique_idx",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("migration missing %q", want)
		}
	}
}

func TestSocialConnectionsMigrationNeverMergesManagedOwners(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))

	for _, want := range []string{
		"count(distinct external_user_id) filter",
		"count(distinct connection_type)",
		"social_connection_migration_conflicts",
		"cross_managed_user",
		"mixed_ownership",
		"missing_provider_identity",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("unsafe-group classification missing %q", want)
		}
	}
}

func TestSocialConnectionsMigrationKeepsLegacyFallback(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))

	if strings.Contains(sql, "alter column connection_id set not null") {
		t.Fatal("dual-read rollout must keep connection_id nullable")
	}
	for _, forbidden := range []string{
		"drop column access_token",
		"drop column refresh_token",
		"drop column external_user_id",
		"drop column external_account_id",
	} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("dual-read migration must not %s", forbidden)
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

func TestSocialConnectionsMigrationBackfillSeparatesUnsafeOwnership(t *testing.T) {
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
	bootstrapMigrationBaselineIfEmptyForTest(t, ctx, tx, 120)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email) VALUES
			('connection-migration-user', 'connection-migration@example.com');
		INSERT INTO workspaces (id, user_id, name) VALUES
			('connection-migration-workspace', 'connection-migration-user', 'Migration Fixture');
		INSERT INTO profiles (id, workspace_id, name) VALUES
			('connection-profile-a', 'connection-migration-workspace', 'A'),
			('connection-profile-b', 'connection-migration-workspace', 'B'),
			('connection-profile-c', 'connection-migration-workspace', 'C');
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type, external_user_id
		) VALUES
			('safe-a', 'connection-profile-a', 'linkedin', 'safe-token-a', 'safe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'),
			('safe-b', 'connection-profile-b', 'linkedin', 'safe-token-b', 'safe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'),
			('unsafe-a', 'connection-profile-a', 'twitter', 'unsafe-token-a', 'unsafe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'),
			('unsafe-b', 'connection-profile-b', 'twitter', 'unsafe-token-b', 'unsafe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-b'),
			('instagram-missing-webhook-id', 'connection-profile-c', 'instagram', 'ig-token', 'app-scoped-id',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'byo', NULL)
	`); err != nil {
		t.Fatalf("seed social connection migration fixture: %v", err)
	}

	applyMigrationUp(t, ctx, tx, "migrations/121_social_connections_and_profile_bindings.sql")

	var safeConnectionCount int
	var safeDistinctBindings int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(DISTINCT sc.id),
			COUNT(DISTINCT sa.connection_id)
		FROM social_accounts sa
		JOIN social_connections sc ON sc.id = sa.connection_id
		WHERE sa.id IN ('safe-a', 'safe-b')
	`).Scan(&safeConnectionCount, &safeDistinctBindings); err != nil {
		t.Fatal(err)
	}
	if safeConnectionCount != 1 || safeDistinctBindings != 1 {
		t.Fatalf("safe duplicate backfill = (%d connections, %d bindings), want one shared connection", safeConnectionCount, safeDistinctBindings)
	}

	var unsafeBoundCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM social_accounts
		WHERE id IN ('unsafe-a', 'unsafe-b') AND connection_id IS NOT NULL
	`).Scan(&unsafeBoundCount); err != nil {
		t.Fatal(err)
	}
	if unsafeBoundCount != 0 {
		t.Fatalf("cross-managed-owner rows bound to %d connections, want zero", unsafeBoundCount)
	}

	var crossOwnerConflicts int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM social_connection_migration_conflicts
		WHERE provider_identity = 'unsafe-provider'
		  AND reason = 'cross_managed_user'
	`).Scan(&crossOwnerConflicts); err != nil {
		t.Fatal(err)
	}
	if crossOwnerConflicts != 1 {
		t.Fatalf("cross-managed-owner conflict count = %d, want 1", crossOwnerConflicts)
	}

	var missingInstagramConflicts int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM social_connection_migration_conflicts
		WHERE source_account_ids = ARRAY['instagram-missing-webhook-id']::text[]
		  AND reason = 'missing_provider_identity'
	`).Scan(&missingInstagramConflicts); err != nil {
		t.Fatal(err)
	}
	if missingInstagramConflicts != 1 {
		t.Fatalf("missing Instagram webhook identity conflicts = %d, want 1", missingInstagramConflicts)
	}
}
