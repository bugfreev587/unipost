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
		"create table social_connection_migration_audit",
		"add column connection_id text",
		"add column binding_version bigint not null default 1",
		"add column binding_status text not null default 'active'",
		"metadata->>'instagram_webhook_user_id'",
		"platform <> 'instagram'",
		"external_account_id",
		"status <> 'migration_conflict'",
		"social_connections_canonical_identity_unique_idx",
		"social_accounts_profile_connection_unique_idx",
		"lock table social_accounts in share row exclusive mode",
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
		"incompatible_credential_state",
		"incompatible_scope",
		"incompatible_routing_metadata",
		"authority_evidence",
		"selected_source_account_id",
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

func TestSocialConnectionAuthorityWriteGateRejectsNewLegacyRows(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))
	for _, want := range []string{
		"tg_op = 'insert' and new.connection_id is null",
		"old.connection_id is distinct from new.connection_id",
		"before insert or update on social_accounts",
		"new.access_token is distinct from old.access_token",
		"new.access_token is distinct from authority.access_token",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("connection authority write gate missing %q: %s", want, sql)
		}
	}
	gateAt := strings.Index(sql, "create trigger social_accounts_connection_authority_write_gate")
	lockAt := strings.Index(sql, "lock table social_accounts in share row exclusive mode")
	downAt := strings.Index(sql, "do $$ begin if exists (select 1 from social_connection_migration_conflicts)")
	if lockAt < 0 || gateAt < lockAt || downAt < gateAt {
		t.Fatal("authority gate must be installed in migration 121 before its transaction releases the classification lock")
	}
}

func TestSocialConnectionMigrationAuditRedactsRoutingMetadata(t *testing.T) {
	sql := compactSocialConnectionSQL(readSocialConnectionsMigration(t))
	if !strings.Contains(sql, "routing_metadata_sha256") {
		t.Fatal("migration audit must retain a routing metadata digest")
	}
	if strings.Contains(sql, "'routing_metadata', authority_metadata") {
		t.Fatal("migration audit must not retain raw provider routing metadata")
	}
}

func TestInboxConnectionMigrationRollbackPreservesSupersededEvidence(t *testing.T) {
	body, err := os.ReadFile("migrations/123_inbox_connection_deduplication.sql")
	if err != nil {
		t.Fatal(err)
	}
	downParts := strings.Split(string(body), "-- +goose Down")
	if len(downParts) != 2 {
		t.Fatal("migration 123 must have one Down section")
	}
	down := compactSocialConnectionSQL(downParts[1])
	if !strings.Contains(down, "if exists (select 1 from inbox_item_supersessions)") ||
		!strings.Contains(down, "raise exception 'refusing destructive rollback") {
		t.Fatal("migration 123 Down must fail closed while superseded Inbox evidence exists")
	}
	if strings.Contains(down, "delete from inbox_items") {
		t.Fatal("migration 123 Down must never delete preserved customer Inbox rows")
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
			('safe-b', 'connection-profile-b', 'linkedin', 'safe-token-a', 'safe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'),
			('unsafe-a', 'connection-profile-a', 'twitter', 'unsafe-token-a', 'unsafe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'),
			('unsafe-b', 'connection-profile-b', 'twitter', 'unsafe-token-b', 'unsafe-provider',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'managed', 'managed-owner-b'),
			('token-conflict-a', 'connection-profile-a', 'bluesky', 'token-a', 'did:plc:token-conflict',
				'{}'::jsonb, ARRAY['write']::text[], 'active', 'byo', NULL),
			('token-conflict-b', 'connection-profile-b', 'bluesky', 'token-b', 'did:plc:token-conflict',
				'{}'::jsonb, ARRAY['write']::text[], 'active', 'byo', NULL),
			('scope-conflict-a', 'connection-profile-a', 'youtube', 'scope-token', 'scope-provider',
				'{}'::jsonb, ARRAY['upload']::text[], 'active', 'byo', NULL),
			('scope-conflict-b', 'connection-profile-b', 'youtube', 'scope-token', 'scope-provider',
				'{}'::jsonb, ARRAY['upload', 'analytics']::text[], 'active', 'byo', NULL),
			('routing-conflict-a', 'connection-profile-a', 'facebook', 'route-token', 'route-provider',
				'{"page_id":"page-a"}'::jsonb, ARRAY[]::text[], 'active', 'byo', NULL),
			('routing-conflict-b', 'connection-profile-b', 'facebook', 'route-token', 'route-provider',
				'{"page_id":"page-b"}'::jsonb, ARRAY[]::text[], 'active', 'byo', NULL),
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
	var safeConnectionID string
	if err := tx.QueryRowContext(ctx, `
		SELECT connection_id FROM social_accounts WHERE id = 'safe-a'
	`).Scan(&safeConnectionID); err != nil {
		t.Fatal(err)
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

	for providerIdentity, reason := range map[string]string{
		"did:plc:token-conflict": "incompatible_credential_state",
		"scope-provider":         "incompatible_scope",
		"route-provider":         "incompatible_routing_metadata",
	} {
		var boundCount, conflictCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT
				COUNT(*) FILTER (WHERE sa.connection_id IS NOT NULL),
				COUNT(DISTINCT conflict.id)
			FROM social_accounts sa
			JOIN profiles p ON p.id = sa.profile_id
			LEFT JOIN social_connection_migration_conflicts conflict
			  ON conflict.workspace_id = p.workspace_id
			 AND conflict.platform = sa.platform
			 AND conflict.provider_identity = $1
			 AND conflict.reason = $2
			WHERE CASE
			  WHEN sa.platform = 'instagram' THEN sa.metadata->>'instagram_webhook_user_id'
			  ELSE sa.external_account_id
			END = $1
		`, providerIdentity, reason).Scan(&boundCount, &conflictCount); err != nil {
			t.Fatal(err)
		}
		if boundCount != 0 || conflictCount != 1 {
			t.Fatalf("%s classification = (%d bound, %d conflicts), want (0, 1)", providerIdentity, boundCount, conflictCount)
		}
	}

	var selectedSource string
	var safeAuditRows int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(selected_source_account_id)
		FROM social_connection_migration_audit
		WHERE provider_identity = 'safe-provider' AND outcome = 'promoted'
	`).Scan(&safeAuditRows, &selectedSource); err != nil {
		t.Fatal(err)
	}
	if safeAuditRows != 1 || selectedSource == "" {
		t.Fatalf("safe migration audit = (%d rows, %q selected), want one recorded winner", safeAuditRows, selectedSource)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO inbox_items (
			id, social_account_id, workspace_id, source, external_id, body, thread_key
		) VALUES
			('shared-event-a', 'safe-a', 'connection-migration-workspace', 'ig_comment', 'shared-event', 'first', 'thread-1'),
			('shared-event-b', 'safe-b', 'connection-migration-workspace', 'ig_comment', 'shared-event', 'duplicate', 'thread-1')
	`); err != nil {
		t.Fatalf("seed sibling inbox duplicates before connection dedupe migration: %v", err)
	}
	applyMigrationUp(t, ctx, tx, "migrations/122_delivery_job_connection_snapshot.sql")
	applyMigrationUp(t, ctx, tx, "migrations/123_inbox_connection_deduplication.sql")

	var authoritativeInboxRows int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM inbox_items
		WHERE source = 'ig_comment'
		  AND external_id = 'shared-event'
		  AND connection_id IS NOT NULL
	`).Scan(&authoritativeInboxRows); err != nil {
		t.Fatal(err)
	}
	if authoritativeInboxRows != 1 {
		t.Fatalf("connection-keyed historical inbox rows = %d, want one canonical row", authoritativeInboxRows)
	}
	var visibleInboxRows, supersededInboxRows int
	if err := tx.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE hidden.inbox_item_id IS NULL),
			COUNT(*) FILTER (WHERE hidden.inbox_item_id IS NOT NULL)
		FROM inbox_items item
		LEFT JOIN inbox_item_supersessions hidden ON hidden.inbox_item_id = item.id
		WHERE item.source = 'ig_comment' AND item.external_id = 'shared-event'
	`).Scan(&visibleInboxRows, &supersededInboxRows); err != nil {
		t.Fatal(err)
	}
	if visibleInboxRows != 1 || supersededInboxRows != 1 {
		t.Fatalf("historical inbox visibility = (%d visible, %d superseded), want (1, 1)", visibleInboxRows, supersededInboxRows)
	}

	applyMigrationUp(t, ctx, tx, "migrations/124_physical_daily_publish_reservations.sql")
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, refresh_token, token_expires_at,
			external_account_id, account_name, account_avatar_url, metadata, scope,
			status, connection_type, external_user_id, external_user_email,
			x_app_mode, disconnected_at, connection_id, binding_status
		)
		SELECT
			'new-safe-binding', 'connection-profile-c', platform, access_token,
			refresh_token, token_expires_at, provider_identity, account_name,
			account_avatar_url, metadata, scope, status, connection_type,
			external_user_id, external_user_email, x_app_mode, disconnected_at,
			id, 'active'
		FROM social_connections
		WHERE id = $1
	`, safeConnectionID); err != nil {
		t.Fatalf("valid connection-backed binding projection rejected by authority gate: %v", err)
	}

	reservationSQL := `SELECT reserve_physical_daily_publish($1, $2, 'linkedin', $3, 1)`
	for _, reservation := range []struct {
		operationKey string
		want         int
	}{{operationKey: "operation-a", want: 2}, {operationKey: "operation-b", want: 0}} {
		var result int
		if err := tx.QueryRowContext(ctx, reservationSQL, "connection-migration-workspace", safeConnectionID, reservation.operationKey).Scan(&result); err != nil {
			t.Fatalf("reserve %s: %v", reservation.operationKey, err)
		}
		if result != reservation.want {
			t.Fatalf("reserve %s result = %d, want %d", reservation.operationKey, result, reservation.want)
		}
	}
	var repeatedResult int
	if err := tx.QueryRowContext(ctx, reservationSQL, "connection-migration-workspace", safeConnectionID, "operation-a").Scan(&repeatedResult); err != nil {
		t.Fatal(err)
	}
	if repeatedResult != 1 {
		t.Fatalf("idempotent retry reservation result = %d, want 1", repeatedResult)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO social_posts (id, workspace_id, caption, status, profile_ids, source)
		VALUES (
			'daily-release-reconciliation-post', 'connection-migration-workspace',
			'definite provider failure', 'failed', ARRAY['connection-profile-a']::TEXT[], 'api'
		);
		INSERT INTO social_post_results (
			id, post_id, social_account_id, caption, status,
			daily_reservation_operation_key, daily_reservation_release_pending
		) VALUES (
			'daily-release-reconciliation-result', 'daily-release-reconciliation-post',
			'safe-a', 'definite provider failure', 'failed', 'operation-a', TRUE
		)
	`); err != nil {
		t.Fatalf("persist durable release reconciliation: %v", err)
	}
	var reconciledCount int
	var releasePending bool
	if err := tx.QueryRowContext(ctx, `
		SELECT reservation.reserved_count, result.daily_reservation_release_pending
		FROM physical_daily_publish_reservations reservation
		JOIN social_post_results result ON result.id = 'daily-release-reconciliation-result'
		WHERE reservation.workspace_id = 'connection-migration-workspace'
		  AND reservation.physical_account_id = $1
		  AND reservation.platform = 'linkedin'
		  AND reservation.utc_date = (NOW() AT TIME ZONE 'UTC')::DATE
	`, safeConnectionID).Scan(&reconciledCount, &releasePending); err != nil {
		t.Fatal(err)
	}
	if reconciledCount != 0 || releasePending {
		t.Fatalf("durable release reconciliation = (count %d, pending %t), want (0, false)", reconciledCount, releasePending)
	}
	var replacementResult int
	if err := tx.QueryRowContext(ctx, reservationSQL, "connection-migration-workspace", safeConnectionID, "operation-b").Scan(&replacementResult); err != nil || replacementResult != 2 {
		t.Fatalf("replacement reservation = (%d, %v), want (2, nil)", replacementResult, err)
	}
	var finalized bool
	if err := tx.QueryRowContext(ctx, `SELECT finalize_physical_daily_publish($1, $2)`, "connection-migration-workspace", "operation-b").Scan(&finalized); err != nil || !finalized {
		t.Fatalf("finalize operation-b = (%t, %v), want true", finalized, err)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE social_accounts
		SET binding_status = 'unbound', status = 'disconnected', disconnected_at = NOW()
		WHERE id = 'new-safe-binding';
		UPDATE social_accounts
		SET metadata = metadata || jsonb_build_object('dismissed_at', NOW()::TEXT)
		WHERE id = 'new-safe-binding';
		UPDATE social_accounts
		SET metadata = metadata || jsonb_build_object('disconnect_notified_at', NOW()::TEXT)
		WHERE id = 'safe-a';
	`); err != nil {
		t.Fatalf("binding-local transient metadata update rejected: %v", err)
	}
	var authorityHasTransientMetadata bool
	if err := tx.QueryRowContext(ctx, `
		SELECT metadata ?| ARRAY['dismissed_at', 'disconnect_notified_at', 'reconnect_required_at']
		FROM social_connections WHERE id = $1
	`, safeConnectionID).Scan(&authorityHasTransientMetadata); err != nil {
		t.Fatal(err)
	}
	if authorityHasTransientMetadata {
		t.Fatal("binding-local transient metadata leaked into connection authority")
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO x_inbox_outbound_requests (
			id, workspace_id, social_account_id, inbox_item_id, idempotency_key, payload_hash
		) VALUES (
			'outbound-shared-event', 'connection-migration-workspace', 'safe-a',
			'shared-event-a', 'outbound-key', 'payload-hash'
		);
		INSERT INTO social_posts (id, workspace_id, caption, status, profile_ids, source)
		VALUES (
			'legacy-daily-post', 'connection-migration-workspace', 'legacy success',
			'published', ARRAY['connection-profile-a']::TEXT[], 'api'
		);
		INSERT INTO social_post_results (
			id, post_id, social_account_id, caption, status, published_at
		) VALUES (
			'legacy-daily-result', 'legacy-daily-post', 'safe-a',
			'legacy success', 'published', NOW()
		);
	`); err != nil {
		t.Fatalf("seed durable connection-owned state: %v", err)
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT reject_new_legacy"); err != nil {
		t.Fatal(err)
	}
	_, insertErr := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type
		) VALUES (
			'post-cutover-legacy', 'connection-profile-a', 'twitter', 'token', 'post-cutover',
			'{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		)
	`)
	if insertErr == nil {
		t.Fatal("post-cutover legacy social account insert unexpectedly succeeded")
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT reject_new_legacy"); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT reject_legacy_refresh"); err != nil {
		t.Fatal(err)
	}
	_, refreshErr := tx.ExecContext(ctx, `
		UPDATE social_accounts
		SET access_token = 'legacy-refresh-drift', status = 'active', disconnected_at = NULL
		WHERE id = 'safe-a'
	`)
	if refreshErr == nil {
		t.Fatal("legacy credential refresh on a classified binding unexpectedly succeeded")
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT reject_legacy_refresh"); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT reject_legacy_upsert"); err != nil {
		t.Fatal(err)
	}
	_, upsertErr := tx.ExecContext(ctx, `
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type, external_user_id
		) VALUES (
			'legacy-upsert-attempt', 'connection-profile-a', 'linkedin',
			'legacy-upsert-drift', 'safe-provider', '{}'::jsonb,
			ARRAY[]::text[], 'active', 'managed', 'managed-owner-a'
		)
		ON CONFLICT (profile_id, platform, external_user_id)
		  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky'
		DO UPDATE SET access_token = EXCLUDED.access_token
	`)
	if upsertErr == nil {
		t.Fatal("legacy managed upsert on a classified binding unexpectedly succeeded")
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT reject_legacy_upsert"); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM social_accounts WHERE id = 'safe-a'"); err != nil {
		t.Fatalf("delete canonical sibling binding: %v", err)
	}
	var rehomedAccountID string
	if err := tx.QueryRowContext(ctx, `
		SELECT social_account_id
		FROM inbox_items
		WHERE source = 'ig_comment'
		  AND external_id = 'shared-event'
		  AND NOT EXISTS (
			SELECT 1 FROM inbox_item_supersessions hidden
			WHERE hidden.inbox_item_id = inbox_items.id
		  )
	`).Scan(&rehomedAccountID); err != nil {
		t.Fatalf("connection-owned inbox row disappeared with canonical binding: %v", err)
	}
	if rehomedAccountID != "safe-b" {
		t.Fatalf("connection-owned inbox row rehomed to %q, want safe-b", rehomedAccountID)
	}
	var rehomedOutboundAccountID, rehomedResultAccountID string
	if err := tx.QueryRowContext(ctx, `SELECT social_account_id FROM x_inbox_outbound_requests WHERE id = 'outbound-shared-event'`).Scan(&rehomedOutboundAccountID); err != nil {
		t.Fatalf("durable outbound operation disappeared with binding: %v", err)
	}
	if rehomedOutboundAccountID != "safe-b" {
		t.Fatalf("durable outbound operation rehomed to %q, want safe-b", rehomedOutboundAccountID)
	}
	if err := tx.QueryRowContext(ctx, `SELECT social_account_id FROM social_post_results WHERE id = 'legacy-daily-result'`).Scan(&rehomedResultAccountID); err != nil {
		t.Fatalf("published result disappeared with binding: %v", err)
	}
	if rehomedResultAccountID != "safe-b" {
		t.Fatalf("published result rehomed to %q, want safe-b", rehomedResultAccountID)
	}
	var reservationRows, durableReservedCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), MAX(reserved_count)
		FROM physical_daily_publish_reservations
		WHERE workspace_id = 'connection-migration-workspace'
		  AND physical_account_id = $1
	`, safeConnectionID).Scan(&reservationRows, &durableReservedCount); err != nil {
		t.Fatal(err)
	}
	if reservationRows != 1 || durableReservedCount != 2 {
		t.Fatalf("physical daily reservation after legacy success/binding delete = (%d rows, %d count), want (1, 2)", reservationRows, durableReservedCount)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE social_accounts SET account_name = 'quarantine-maintained'
		WHERE id = 'instagram-missing-webhook-id'
	`); err != nil {
		t.Fatalf("existing quarantined legacy row must remain maintainable: %v", err)
	}
}
