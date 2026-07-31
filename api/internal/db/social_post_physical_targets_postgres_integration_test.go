package db

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPhysicalTargetAllowsThreadAndRejectsSibling(t *testing.T) {
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
	applyMigrationRangeForTest(t, ctx, tx, 1, 137)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO users (id, email) VALUES ('target-user', 'target@example.com');
		INSERT INTO workspaces (id, user_id, name)
		VALUES ('target-workspace', 'target-user', 'Target');
		INSERT INTO profiles (id, workspace_id, name) VALUES
			('target-profile-a', 'target-workspace', 'A'),
			('target-profile-b', 'target-workspace', 'B');
		INSERT INTO social_connections (
			id, workspace_id, platform, provider_identity, access_token,
			metadata, scope, status, connection_type
		) VALUES (
			'target-connection', 'target-workspace', 'twitter', 'provider-1',
			'token', '{}'::jsonb, ARRAY[]::text[], 'active', 'byo'
		);
		INSERT INTO social_accounts (
			id, profile_id, platform, access_token, external_account_id,
			metadata, scope, status, connection_type, connection_id
		) VALUES
			('target-account-a', 'target-profile-a', 'twitter', 'token', 'provider-1',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'byo', 'target-connection'),
			('target-account-b', 'target-profile-b', 'twitter', 'token', 'provider-1',
				'{}'::jsonb, ARRAY[]::text[], 'active', 'byo', 'target-connection');
		INSERT INTO social_posts (id, workspace_id, status, source)
		VALUES ('target-post', 'target-workspace', 'publishing', 'api');
		INSERT INTO social_post_results (id, post_id, social_account_id, caption, status) VALUES
			('target-result-a-1', 'target-post', 'target-account-a', 'one', 'pending'),
			('target-result-a-2', 'target-post', 'target-account-a', 'two', 'pending'),
			('target-result-b', 'target-post', 'target-account-b', 'sibling', 'pending');
	`); err != nil {
		t.Fatalf("seed physical target fixture: %v", err)
	}

	insertJob := `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			connection_id, binding_version, platform, post_input_index,
			kind, state, attempts, max_attempts
		) VALUES ($1, 'target-post', $2, 'target-workspace', $3,
			'target-connection', 1, 'twitter', $4, 'dispatch', 'pending', 0, 5)
	`
	if _, err := tx.ExecContext(ctx, insertJob, "target-job-a-1", "target-result-a-1", "target-account-a", 0); err != nil {
		t.Fatalf("insert first thread job: %v", err)
	}
	if _, err := tx.ExecContext(ctx, insertJob, "target-job-a-2", "target-result-a-2", "target-account-a", 1); err != nil {
		t.Fatalf("same-binding thread job rejected: %v", err)
	}

	if _, err := tx.ExecContext(ctx, "SAVEPOINT sibling_target"); err != nil {
		t.Fatal(err)
	}
	_, siblingErr := tx.ExecContext(ctx, insertJob, "target-job-b", "target-result-b", "target-account-b", 2)
	var pgErr *pgconn.PgError
	if !errors.As(siblingErr, &pgErr) || pgErr.ConstraintName != "social_post_physical_target_binding_check" {
		t.Fatalf("sibling insert error = %v, want physical target constraint", siblingErr)
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT sibling_target"); err != nil {
		t.Fatal(err)
	}

	var targetCount int
	var selectedAccount string
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*), MIN(selected_social_account_id)
		FROM social_post_physical_targets
		WHERE post_id = 'target-post'
		  AND physical_target_key = 'connection:target-connection'
	`).Scan(&targetCount, &selectedAccount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 || selectedAccount != "target-account-a" {
		t.Fatalf("physical target = (%d, %q), want (1, target-account-a)", targetCount, selectedAccount)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO social_posts (id, workspace_id, status, source)
		VALUES ('target-batch-post', 'target-workspace', 'publishing', 'api')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "SAVEPOINT sibling_batch"); err != nil {
		t.Fatal(err)
	}
	_, batchErr := tx.ExecContext(ctx, `
		SELECT reserve_social_post_physical_targets(
			'target-batch-post',
			ARRAY['target-account-a', 'target-account-b']::text[],
			ARRAY['target-connection', 'target-connection']::text[]
		)
	`)
	if !errors.As(batchErr, &pgErr) || pgErr.ConstraintName != "social_post_physical_target_binding_check" {
		t.Fatalf("batch sibling error = %v, want physical target constraint", batchErr)
	}
	if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT sibling_batch"); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM social_post_physical_targets WHERE post_id = 'target-batch-post'
	`).Scan(&targetCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 0 {
		t.Fatalf("failed batch left %d physical targets, want zero", targetCount)
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE social_connection_rollout_state SET phase = 'draining' WHERE id = 'global'
	`); err != nil {
		t.Fatal(err)
	}
	claim, err := tx.ExecContext(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'running', lease_owner = 'old-worker', lease_expires_at = NOW() + INTERVAL '1 minute'
		WHERE id = 'target-job-a-1'
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := claim.RowsAffected(); err != nil || rows != 0 {
		t.Fatalf("draining claim affected (%d, %v), want (0, nil)", rows, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE social_connection_rollout_state SET phase = 'expand' WHERE id = 'global'
	`); err != nil {
		t.Fatal(err)
	}
	claim, err = tx.ExecContext(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'running', lease_owner = 'new-worker', lease_expires_at = NOW() + INTERVAL '1 minute'
		WHERE id = 'target-job-a-1'
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := claim.RowsAffected(); err != nil || rows != 1 {
		t.Fatalf("expanded claim affected (%d, %v), want (1, nil)", rows, err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE social_connection_rollout_state SET phase = 'draining' WHERE id = 'global'
	`); err != nil {
		t.Fatal(err)
	}
	finalize, err := tx.ExecContext(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'succeeded', finished_at = NOW()
		WHERE id = 'target-job-a-1' AND lease_owner = 'new-worker'
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := finalize.RowsAffected(); err != nil || rows != 1 {
		t.Fatalf("draining finalization affected (%d, %v), want (1, nil)", rows, err)
	}
}
