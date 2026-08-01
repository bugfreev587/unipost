package socialconnectioncutover

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	unipostdb "github.com/xiaoboyu/unipost-api/internal/db"
)

const cutoverIntegrationSHA = "0123456789abcdef0123456789abcdef01234567"

func TestReconcilerPromotesBindingsAndConservesDurableState(t *testing.T) {
	databaseURL := newCutoverIntegrationDatabase(t)
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	seedCutoverFixture(t, pool, false)
	if _, err := pool.Exec(context.Background(), `
		UPDATE social_connection_rollout_state
		SET phase = 'cutting_over'
		WHERE id = 'global'
	`); err != nil {
		t.Fatal(err)
	}

	reconciler := &Reconciler{
		Pool:          pool,
		CommitSHA:     cutoverIntegrationSHA,
		EnvironmentID: "local-integration",
	}
	if err := reconciler.Run(context.Background(), Report{}); err != nil {
		t.Fatalf("run cutover reconciliation: %v", err)
	}

	var bindingRows, distinctConnections, nullConnections int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			COUNT(*),
			COUNT(DISTINCT connection_id),
			COUNT(*) FILTER (WHERE connection_id IS NULL)
		FROM social_accounts
		WHERE id IN ('cutover-account-a', 'cutover-account-b')
	`).Scan(&bindingRows, &distinctConnections, &nullConnections); err != nil {
		t.Fatal(err)
	}
	if bindingRows != 2 || distinctConnections != 1 || nullConnections != 0 {
		t.Fatalf("binding promotion = (%d rows, %d connections, %d null), want (2, 1, 0)", bindingRows, distinctConnections, nullConnections)
	}

	var connectionID string
	if err := pool.QueryRow(context.Background(), `
		SELECT MIN(connection_id)
		FROM social_accounts
		WHERE id IN ('cutover-account-a', 'cutover-account-b')
	`).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}

	var phase, sha, environmentID string
	if err := pool.QueryRow(context.Background(), `
		SELECT phase, cutover_application_sha, cutover_environment_id
		FROM social_connection_rollout_state
		WHERE id = 'global'
	`).Scan(&phase, &sha, &environmentID); err != nil {
		t.Fatal(err)
	}
	if phase != "cutover" || sha != cutoverIntegrationSHA || environmentID != "local-integration" {
		t.Fatalf("cutover identity = (%q, %q, %q)", phase, sha, environmentID)
	}

	var jobConnectionID string
	var bindingVersion int64
	if err := pool.QueryRow(context.Background(), `
		SELECT connection_id, binding_version
		FROM post_delivery_jobs
		WHERE id = 'cutover-job-a'
	`).Scan(&jobConnectionID, &bindingVersion); err != nil {
		t.Fatal(err)
	}
	if jobConnectionID != connectionID || bindingVersion != 1 {
		t.Fatalf("job snapshot = (%q, %d), want (%q, 1)", jobConnectionID, bindingVersion, connectionID)
	}

	var physicalTargetCount int
	var selectedAccount, targetStatus string
	if err := pool.QueryRow(context.Background(), `
		SELECT COUNT(*), MIN(selected_social_account_id), MIN(status)
		FROM social_post_physical_targets
		WHERE post_id = 'cutover-post'
		  AND physical_target_key = 'connection:' || $1
	`, connectionID).Scan(&physicalTargetCount, &selectedAccount, &targetStatus); err != nil {
		t.Fatal(err)
	}
	if physicalTargetCount != 1 || selectedAccount != "cutover-account-a" || targetStatus != "active" {
		t.Fatalf("physical target = (%d, %q, %q)", physicalTargetCount, selectedAccount, targetStatus)
	}

	var inboxRows, canonicalRows, supersessions int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM inbox_items WHERE id IN ('cutover-inbox-a', 'cutover-inbox-b')),
			(SELECT COUNT(*) FROM inbox_items WHERE id IN ('cutover-inbox-a', 'cutover-inbox-b') AND connection_id = $1),
			(SELECT COUNT(*) FROM inbox_item_supersessions WHERE inbox_item_id IN ('cutover-inbox-a', 'cutover-inbox-b'))
	`, connectionID).Scan(&inboxRows, &canonicalRows, &supersessions); err != nil {
		t.Fatal(err)
	}
	if inboxRows != 2 || canonicalRows != 1 || supersessions != 1 {
		t.Fatalf("Inbox reconciliation = (%d rows, %d canonical, %d supersessions), want (2, 1, 1)", inboxRows, canonicalRows, supersessions)
	}

	var reservationRows, reservationUnits, operationRows, operationConnectionRows int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM physical_daily_publish_reservations WHERE workspace_id = 'cutover-workspace'),
			(SELECT COALESCE(SUM(reserved_count), 0) FROM physical_daily_publish_reservations WHERE workspace_id = 'cutover-workspace'),
			(SELECT COUNT(*) FROM physical_daily_publish_operations WHERE workspace_id = 'cutover-workspace'),
			(SELECT COUNT(*) FROM physical_daily_publish_operations WHERE workspace_id = 'cutover-workspace' AND physical_account_id = $1)
	`, connectionID).Scan(&reservationRows, &reservationUnits, &operationRows, &operationConnectionRows); err != nil {
		t.Fatal(err)
	}
	if reservationRows != 1 || reservationUnits != 5 || operationRows != 2 || operationConnectionRows != 2 {
		t.Fatalf("quota reconciliation = (%d rows, %d units, %d operations, %d mapped), want (1, 5, 2, 2)", reservationRows, reservationUnits, operationRows, operationConnectionRows)
	}
}

func TestReconcilerRollsBackWhenSiblingTargetsCannotCollapse(t *testing.T) {
	databaseURL := newCutoverIntegrationDatabase(t)
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	seedCutoverFixture(t, pool, true)
	if _, err := pool.Exec(context.Background(), `
		UPDATE social_connection_rollout_state
		SET phase = 'cutting_over'
		WHERE id = 'global'
	`); err != nil {
		t.Fatal(err)
	}

	reconciler := &Reconciler{
		Pool:          pool,
		CommitSHA:     cutoverIntegrationSHA,
		EnvironmentID: "local-integration",
	}
	err = reconciler.Run(context.Background(), Report{})
	if err == nil || !strings.Contains(err.Error(), "nonterminal sibling physical targets cannot be collapsed safely") {
		t.Fatalf("cutover error = %v, want sibling target rollback", err)
	}

	var connectionRows, classifiedBindings, snappedJobs int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT COUNT(*) FROM social_connections WHERE workspace_id = 'cutover-workspace'),
			(SELECT COUNT(*) FROM social_accounts WHERE id IN ('cutover-account-a', 'cutover-account-b') AND connection_id IS NOT NULL),
			(SELECT COUNT(*) FROM post_delivery_jobs WHERE id IN ('cutover-job-a', 'cutover-job-b') AND connection_id IS NOT NULL)
	`).Scan(&connectionRows, &classifiedBindings, &snappedJobs); err != nil {
		t.Fatal(err)
	}
	if connectionRows != 0 || classifiedBindings != 0 || snappedJobs != 0 {
		t.Fatalf("failed cutover leaked state = (%d connections, %d bindings, %d jobs)", connectionRows, classifiedBindings, snappedJobs)
	}

	var phase string
	var targetRows int
	if err := pool.QueryRow(context.Background(), `
		SELECT
			(SELECT phase FROM social_connection_rollout_state WHERE id = 'global'),
			(SELECT COUNT(*) FROM social_post_physical_targets WHERE post_id = 'cutover-post' AND physical_target_key LIKE 'legacy-account:%')
	`).Scan(&phase, &targetRows); err != nil {
		t.Fatal(err)
	}
	if phase != "cutting_over" || targetRows != 2 {
		t.Fatalf("rollback state = (%q, %d legacy targets), want (cutting_over, 2)", phase, targetRows)
	}
}

func newCutoverIntegrationDatabase(t *testing.T) string {
	t.Helper()
	adminURL := os.Getenv("SOCIAL_CONNECTION_CUTOVER_TEST_ADMIN_URL")
	if adminURL == "" {
		t.Skip("SOCIAL_CONNECTION_CUTOVER_TEST_ADMIN_URL is not configured")
	}
	config, err := pgx.ParseConfig(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	if config.Host != "127.0.0.1" && config.Host != "localhost" && config.Host != "::1" {
		t.Fatalf("cutover integration database must be local, got host %q", config.Host)
	}

	admin, err := pgx.ConnectConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })

	databaseName := fmt.Sprintf("unipost_cutover_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{databaseName}.Sanitize()
	if _, err := admin.Exec(context.Background(), "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), `
			SELECT pg_terminate_backend(pid)
			FROM pg_stat_activity
			WHERE datname = $1 AND pid <> pg_backend_pid()
		`, databaseName)
		_, _ = admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+identifier)
	})

	databaseLocation, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	databaseLocation.Path = "/" + databaseName
	databaseURL := databaseLocation.String()
	if err := unipostdb.RunMigrations(databaseURL); err != nil {
		t.Fatalf("migrate cutover integration database: %v", err)
	}
	return databaseURL
}

func seedCutoverFixture(t *testing.T, pool *pgxpool.Pool, includeSiblingJob bool) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email) VALUES ('cutover-user', 'cutover@example.com');
		INSERT INTO workspaces (id, user_id, name)
		VALUES ('cutover-workspace', 'cutover-user', 'Cutover');
		INSERT INTO profiles (id, workspace_id, name) VALUES
			('cutover-profile-a', 'cutover-workspace', 'A'),
			('cutover-profile-b', 'cutover-workspace', 'B');
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, refresh_token,
			external_account_id, metadata, scope, status, connection_type
		) VALUES
			('cutover-account-a', 'cutover-profile-a', 'twitter', 'shared-token', 'shared-refresh',
				'physical-provider', '{}'::jsonb, ARRAY['write', 'read']::text[], 'active', 'byo'),
			('cutover-account-b', 'cutover-profile-b', 'twitter', 'shared-token', 'shared-refresh',
				'physical-provider', '{}'::jsonb, ARRAY['read', 'write']::text[], 'active', 'byo');
		INSERT INTO social_posts (id, workspace_id, status, source)
		VALUES ('cutover-post', 'cutover-workspace', 'publishing', 'api');
		INSERT INTO social_post_results (id, post_id, social_account_id, caption, status) VALUES
			('cutover-result-a', 'cutover-post', 'cutover-account-a', 'one', 'pending'),
			('cutover-result-b', 'cutover-post', 'cutover-account-b', 'two', 'pending');
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, post_input_index, kind, state, attempts, max_attempts
		) VALUES (
			'cutover-job-a', 'cutover-post', 'cutover-result-a', 'cutover-workspace',
			'cutover-account-a', 'twitter', 0, 'dispatch', 'pending', 0, 5
		);
		INSERT INTO inbox_items (
			id, social_account_id, workspace_id, source, external_id, thread_key, body,
			received_at, created_at
		) VALUES
			('cutover-inbox-a', 'cutover-account-a', 'cutover-workspace', 'twitter_reply', 'reply-1', 'reply-1', 'first', NOW() - INTERVAL '2 minutes', NOW() - INTERVAL '2 minutes'),
			('cutover-inbox-b', 'cutover-account-b', 'cutover-workspace', 'twitter_reply', 'reply-1', 'reply-1', 'duplicate', NOW() - INTERVAL '1 minute', NOW() - INTERVAL '1 minute');
		INSERT INTO physical_daily_publish_reservations (
			workspace_id, physical_account_id, platform, utc_date, reserved_count
		) VALUES
			('cutover-workspace', 'cutover-account-a', 'twitter', (NOW() AT TIME ZONE 'UTC')::date, 2),
			('cutover-workspace', 'cutover-account-b', 'twitter', (NOW() AT TIME ZONE 'UTC')::date, 3);
		INSERT INTO physical_daily_publish_operations (
			workspace_id, operation_key, physical_account_id, platform, utc_date, status
		) VALUES
			('cutover-workspace', 'operation-a', 'cutover-account-a', 'twitter', (NOW() AT TIME ZONE 'UTC')::date, 'reserved'),
			('cutover-workspace', 'operation-b', 'cutover-account-b', 'twitter', (NOW() AT TIME ZONE 'UTC')::date, 'finalized');
	`); err != nil {
		t.Fatalf("seed cutover fixture: %v", err)
	}
	if !includeSiblingJob {
		return
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, post_input_index, kind, state, attempts, max_attempts
		) VALUES (
			'cutover-job-b', 'cutover-post', 'cutover-result-b', 'cutover-workspace',
			'cutover-account-b', 'twitter', 1, 'dispatch', 'pending', 0, 5
		)
	`); err != nil {
		t.Fatalf("seed sibling delivery job: %v", err)
	}
}
