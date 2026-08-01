//go:build integration

package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/auth"
	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/events"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
	"github.com/xiaoboyu/unipost-api/internal/quota"
	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

const restrictedDeliveryIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openRestrictedDeliveryIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(restrictedDeliveryIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to a temporary localhost PostgreSQL service", restrictedDeliveryIntegrationDatabaseEnv)
	}
	setupContext, cancelSetup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSetup()
	admin, config, err := testdbguard.OpenValidated(
		restrictedDeliveryIntegrationDatabaseEnv,
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(setupContext, validatedURL)
		},
	)
	if err != nil {
		t.Fatalf("connect temporary PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("restricted_delivery_%d", time.Now().UnixNano())
	if _, err := admin.Exec(setupContext, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(setupContext, config)
	if err != nil {
		admin.Close()
		t.Fatalf("connect isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupContext, cancelCleanup := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelCleanup()
		if _, err := admin.Exec(cleanupContext, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop isolated PostgreSQL schema: %v", err)
		}
		admin.Close()
	})
	return pool
}

func setupRestrictedDeliveryIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, `
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			caption TEXT,
			media_urls TEXT[] NOT NULL DEFAULT '{}',
			status TEXT NOT NULL,
			scheduled_at TIMESTAMPTZ,
			published_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			idempotency_key TEXT,
			workspace_id TEXT NOT NULL DEFAULT 'workspace_1',
			archived_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			source TEXT NOT NULL DEFAULT 'api',
			profile_ids TEXT[] NOT NULL DEFAULT '{}',
			quota_hold_reason TEXT,
			quota_hold_at TIMESTAMPTZ,
			quota_hold_original_scheduled_at TIMESTAMPTZ
		);
		CREATE TABLE social_post_results (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT,
			social_account_id TEXT,
			status TEXT NOT NULL,
			external_id TEXT,
			error_message TEXT,
			published_at TIMESTAMPTZ,
			caption TEXT NOT NULL DEFAULT '',
			url TEXT,
			debug_curl TEXT,
			fb_media_type TEXT,
			remotely_deleted_at TIMESTAMPTZ,
			publish_token TEXT,
			error_code TEXT,
			failure_stage TEXT,
			platform_error_code TEXT,
			is_retriable BOOLEAN,
			next_action TEXT,
			error_source TEXT,
			error_temporality TEXT,
			provider_error JSONB,
			x_credits_counted BIGINT NOT NULL DEFAULT 0,
			x_credit_operation TEXT,
			x_credit_catalog_version TEXT,
			x_credit_billing_mode TEXT
		);
		CREATE TABLE post_delivery_jobs (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT NOT NULL,
			social_post_result_id TEXT NOT NULL REFERENCES social_post_results(id),
			workspace_id TEXT NOT NULL,
			social_account_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			post_input_index INTEGER NOT NULL DEFAULT 0,
			kind TEXT NOT NULL,
			state TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			max_attempts INTEGER NOT NULL DEFAULT 5,
			failure_stage TEXT,
			error_code TEXT,
			platform_error_code TEXT,
			last_error TEXT,
			next_run_at TIMESTAMPTZ,
			last_attempt_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			finished_at TIMESTAMPTZ,
			dismissed_at TIMESTAMPTZ,
			lease_expires_at TIMESTAMPTZ,
			lease_owner TEXT,
			first_claimed_at TIMESTAMPTZ,
			platform_started_at TIMESTAMPTZ
		);
		CREATE TABLE post_failures (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			post_id TEXT NOT NULL,
			social_post_result_id TEXT,
			workspace_id TEXT NOT NULL,
			social_account_id TEXT,
			platform TEXT NOT NULL,
			failure_stage TEXT NOT NULL,
			error_code TEXT NOT NULL,
			platform_error_code TEXT,
			message TEXT NOT NULL,
			raw_error TEXT,
			is_retriable BOOLEAN NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			error_source TEXT,
			error_temporality TEXT,
			provider_error JSONB,
			restriction_cycle_id TEXT
		);
		CREATE TABLE media (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL,
			cleanup_after_at TIMESTAMPTZ,
			usage_version BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE media_processing_usages (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			media_id TEXT NOT NULL,
			cleanup_after_at TIMESTAMPTZ
		);
		CREATE TABLE media_post_usages (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			workspace_id TEXT NOT NULL,
			media_id TEXT NOT NULL,
			post_id TEXT NOT NULL,
			post_status TEXT NOT NULL,
			cleanup_after_at TIMESTAMPTZ,
			retention_reason TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (media_id, post_id)
		);
		CREATE TABLE profiles (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL
		);
		CREATE TABLE social_accounts (
			id TEXT PRIMARY KEY,
			profile_id TEXT NOT NULL DEFAULT 'profile_1' REFERENCES profiles(id),
			platform TEXT NOT NULL,
			access_token TEXT NOT NULL DEFAULT '',
			refresh_token TEXT,
			token_expires_at TIMESTAMPTZ,
			external_account_id TEXT NOT NULL DEFAULT '',
			account_name TEXT,
			account_avatar_url TEXT,
			connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			status TEXT NOT NULL DEFAULT 'active',
			disconnected_at TIMESTAMPTZ,
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			scope TEXT[] NOT NULL DEFAULT '{}',
			connection_type TEXT NOT NULL DEFAULT 'byo',
			connect_session_id TEXT,
			external_user_id TEXT,
			external_user_email TEXT,
			last_refreshed_at TIMESTAMPTZ,
			x_app_mode TEXT,
			last_connected_at TIMESTAMPTZ
		);
		CREATE TABLE platform_publishing_restrictions (
			platform TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL,
			restricted_plan_ids TEXT[] NOT NULL
		);
		CREATE TABLE subscriptions (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			current_period_start TIMESTAMPTZ,
			current_period_end TIMESTAMPTZ,
			cancel_at_period_end BOOLEAN,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			workspace_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			trial_used BOOLEAN NOT NULL DEFAULT FALSE
		);
		CREATE TABLE plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			price_cents INTEGER NOT NULL,
			post_limit INTEGER NOT NULL,
			stripe_price_id TEXT,
			created_at TIMESTAMPTZ,
			white_label BOOLEAN NOT NULL DEFAULT FALSE,
			allow_twitter BOOLEAN NOT NULL DEFAULT TRUE,
			allow_inbox BOOLEAN NOT NULL DEFAULT TRUE,
			allow_analytics BOOLEAN NOT NULL DEFAULT TRUE,
			max_profiles INTEGER,
			max_members INTEGER
		);
		CREATE TABLE admin_post_quota_resets (
			workspace_id TEXT NOT NULL,
			period TEXT NOT NULL,
			quota_kind TEXT NOT NULL,
			reset_at TIMESTAMPTZ NOT NULL
		);
		CREATE TABLE usage (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			period TEXT NOT NULL,
			post_count INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			workspace_id TEXT NOT NULL,
			UNIQUE (workspace_id, period)
		);
		CREATE TABLE terminal_post_first_publish_elections (
			workspace_id TEXT PRIMARY KEY,
			post_id TEXT NOT NULL,
			elected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE terminal_post_event_outbox (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			sequence_number BIGSERIAL NOT NULL UNIQUE,
			post_id TEXT NOT NULL,
			workspace_id TEXT NOT NULL,
			parent_status TEXT NOT NULL,
			parent_version TEXT NOT NULL,
			event_type TEXT NOT NULL DEFAULT '',
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (post_id, parent_version)
		);
		CREATE TABLE terminal_post_event_deliveries (
			event_id TEXT NOT NULL REFERENCES terminal_post_event_outbox(id) ON DELETE CASCADE,
			sink TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			attempts INTEGER NOT NULL DEFAULT 0,
			next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			lease_expires_at TIMESTAMPTZ,
			last_error TEXT,
			enqueued_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (event_id, sink)
		);
		INSERT INTO profiles (id, workspace_id) VALUES ('profile_1', 'workspace_1');
	`)
	if err != nil {
		t.Fatalf("create restricted-delivery integration tables: %v", err)
	}
}

func insertRestrictedFinalizeMediaGuardFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	suffix string,
	availableMediaIDs []string,
) db.FinalizeRestrictedPostDeliveryJobParams {
	t.Helper()
	attemptedAt := time.Date(2026, 7, 27, 1, 0, 0, 0, time.UTC)
	postID := "post_media_guard_" + suffix
	resultID := "result_media_guard_" + suffix
	jobID := "job_media_guard_" + suffix
	workspaceID := "workspace_media_guard_" + suffix
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ($1, 'publishing', $2)
	`, postID, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (
			id, post_id, social_account_id, status, external_id, error_message,
			x_credits_counted, x_credit_operation
		) VALUES ($2, $1, 'account_media_guard', 'processing', 'existing_external',
			'existing_error', 17, 'existing_operation')
	`, postID, resultID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES ($4, $1, $3, $2, 'account_media_guard', 'tiktok', 'dispatch',
			'running', 1, 'owner_media_guard', $5)
	`, postID, workspaceID, resultID, jobID, attemptedAt); err != nil {
		t.Fatal(err)
	}
	for _, mediaID := range availableMediaIDs {
		if _, err := pool.Exec(ctx, `
			INSERT INTO media (id, workspace_id, status)
			VALUES ($1, $2, 'uploaded')
		`, mediaID, workspaceID); err != nil {
			t.Fatal(err)
		}
	}
	params := restrictedFinalizeIntegrationParams(jobID, "owner_media_guard", attemptedAt)
	params.CleanupAfterAt = pgtype.Timestamptz{Time: attemptedAt.Add(60 * 24 * time.Hour), Valid: true}
	return params
}

func assertRestrictedFinalizeRetryableState(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	params db.FinalizeRestrictedPostDeliveryJobParams,
	mediaIDs []string,
) {
	t.Helper()
	var jobState, resultStatus, parentStatus string
	var finishedAt, failureStage, errorCode pgtype.Text
	var externalID, errorMessage, operation pgtype.Text
	var credits, usageVersion int64
	var usages int
	if err := pool.QueryRow(ctx, `
		SELECT job.state, job.finished_at::text, job.failure_stage, job.error_code,
		       result.status, result.external_id, result.error_message,
		       result.x_credits_counted, result.x_credit_operation,
		       parent.status,
		       (SELECT COALESCE(SUM(media.usage_version), 0)::bigint
		          FROM media WHERE media.id = ANY($2::text[])),
		       (SELECT COUNT(*)::int FROM media_post_usages usage
		          WHERE usage.post_id = job.post_id)
		FROM post_delivery_jobs job
		JOIN social_post_results result ON result.id = job.social_post_result_id
		JOIN social_posts parent ON parent.id = job.post_id
		WHERE job.id = $1
	`, params.ID, mediaIDs).Scan(
		&jobState, &finishedAt, &failureStage, &errorCode,
		&resultStatus, &externalID, &errorMessage, &credits, &operation,
		&parentStatus, &usageVersion, &usages,
	); err != nil {
		t.Fatal(err)
	}
	if jobState != "running" || finishedAt.Valid || failureStage.Valid || errorCode.Valid ||
		resultStatus != "processing" || externalID.String != "existing_external" ||
		errorMessage.String != "existing_error" || credits != 17 || operation.String != "existing_operation" ||
		parentStatus != "publishing" || usageVersion != 0 || usages != 0 {
		t.Fatalf("finalization was not retryable: job=%q finished=%#v stage=%#v code=%#v result=%q external=%#v error=%#v credits=%d operation=%#v parent=%q version=%d usages=%d",
			jobState, finishedAt, failureStage, errorCode, resultStatus, externalID,
			errorMessage, credits, operation, parentStatus, usageVersion, usages)
	}
}

func TestFinalizeRestrictedPostDeliveryJobRequiresEveryMediaBeforeMutation(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	mediaA := "media_guard_missing_a"
	mediaB := "media_guard_missing_b"
	params := insertRestrictedFinalizeMediaGuardFixture(t, ctx, pool, "missing", []string{mediaA})
	params.MediaIds = []string{mediaA, mediaB}
	if _, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing-media finalization error = %v, want pgx.ErrNoRows", err)
	}
	assertRestrictedFinalizeRetryableState(t, ctx, pool, params, params.MediaIds)

	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		SELECT $1, workspace_id, 'uploaded'
		FROM post_delivery_jobs WHERE id = $2
	`, mediaB, params.ID); err != nil {
		t.Fatal(err)
	}
	job, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params)
	if err != nil {
		t.Fatalf("retry after restoring media: %v", err)
	}
	if job.State != "dead" {
		t.Fatalf("retry job state = %q, want dead", job.State)
	}
	var usages int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM media_post_usages
		WHERE post_id = (SELECT post_id FROM post_delivery_jobs WHERE id = $1)
		  AND retention_reason = 'publishing_restriction'
	`, params.ID).Scan(&usages); err != nil {
		t.Fatal(err)
	}
	if usages != 2 {
		t.Fatalf("retained media usages after retry = %d, want 2", usages)
	}
}

func TestFinalizeRestrictedPostDeliveryJobCleanupOrdering(t *testing.T) {
	t.Run("cleanup wins", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		mediaA := "media_guard_cleanup_first_a"
		mediaB := "media_guard_cleanup_first_b"
		params := insertRestrictedFinalizeMediaGuardFixture(t, ctx, pool, "cleanup_first", []string{mediaA, mediaB})
		params.MediaIds = []string{mediaA, mediaB}

		cleanupConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanupConn.Release()
		finalizeConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer finalizeConn.Release()
		var cleanupBackend, finalizeBackend int
		if err := cleanupConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&cleanupBackend); err != nil {
			t.Fatal(err)
		}
		if err := finalizeConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&finalizeBackend); err != nil {
			t.Fatal(err)
		}
		cleanupTx, err := cleanupConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer cleanupTx.Rollback(ctx)
		deleted, err := db.New(cleanupTx).SoftDeleteUnusedMedia(ctx, db.SoftDeleteUnusedMediaParams{
			ID: mediaB, WorkspaceID: "workspace_media_guard_cleanup_first",
		})
		if err != nil || deleted != 1 {
			t.Fatalf("cleanup claim rows/error = %d/%v, want 1/nil", deleted, err)
		}
		finalized := make(chan error, 1)
		go func() {
			_, finalizeErr := db.New(finalizeConn).FinalizeRestrictedPostDeliveryJob(ctx, params)
			finalized <- finalizeErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, finalizeBackend, cleanupBackend)
		if err := cleanupTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-finalized; !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("cleanup-first finalization error = %v, want pgx.ErrNoRows", err)
		}
		assertRestrictedFinalizeRetryableState(t, ctx, pool, params, params.MediaIds)

		if _, err := pool.Exec(ctx, `UPDATE media SET status = 'uploaded', cleanup_after_at = NULL WHERE id = $1`, mediaB); err != nil {
			t.Fatal(err)
		}
		if _, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatalf("cleanup-first retry after restore: %v", err)
		}
	})

	t.Run("finalization wins", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		mediaA := "media_guard_finalize_first_a"
		mediaB := "media_guard_finalize_first_b"
		params := insertRestrictedFinalizeMediaGuardFixture(t, ctx, pool, "finalize_first", []string{mediaA, mediaB})
		params.MediaIds = []string{mediaA, mediaB}

		finalizeTx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer finalizeTx.Rollback(ctx)
		if _, err := db.New(finalizeTx).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatal(err)
		}
		deleted, err := db.New(pool).SoftDeleteUnusedMedia(ctx, db.SoftDeleteUnusedMediaParams{
			ID: mediaA, WorkspaceID: "workspace_media_guard_finalize_first",
		})
		if err != nil {
			t.Fatal(err)
		}
		if deleted != 0 {
			t.Fatalf("cleanup rows while finalization holds media locks = %d, want 0", deleted)
		}
		if err := finalizeTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var state string
		var usages int
		if err := pool.QueryRow(ctx, `
			SELECT job.state,
			       (SELECT COUNT(*)::int FROM media_post_usages usage WHERE usage.post_id = job.post_id)
			FROM post_delivery_jobs job WHERE job.id = $1
		`, params.ID).Scan(&state, &usages); err != nil {
			t.Fatal(err)
		}
		if state != "dead" || usages != 2 {
			t.Fatalf("finalization-first state/usages = %q/%d, want dead/2", state, usages)
		}
	})
}

func TestFinalizeRestrictedPostDeliveryJobAllowsEmptyMediaSet(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	params := insertRestrictedFinalizeMediaGuardFixture(t, ctx, pool, "empty", nil)
	params.MediaIds = []string{}
	job, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != "dead" {
		t.Fatalf("empty-media job state = %q, want dead", job.State)
	}
}

func TestFinalizeRestrictedPostDeliveryJobConcurrentResultsConvergeParentStatus(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	attemptedAt := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "account_parent_concurrency_a"},
		{AccountID: "account_parent_concurrency_b"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id, metadata)
		VALUES ('post_parent_concurrency', 'publishing', 'workspace_parent_concurrency', $1)
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status) VALUES
			('result_parent_concurrency_a', 'post_parent_concurrency', 'account_parent_concurrency_a', 'processing'),
			('result_parent_concurrency_b', 'post_parent_concurrency', 'account_parent_concurrency_b', 'processing')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, post_input_index, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES
			('job_parent_concurrency_a', 'post_parent_concurrency', 'result_parent_concurrency_a',
			 'workspace_parent_concurrency', 'account_parent_concurrency_a', 'tiktok', 0,
			 'dispatch', 'running', 1, 'owner_parent_concurrency_a', $1),
			('job_parent_concurrency_b', 'post_parent_concurrency', 'result_parent_concurrency_b',
			 'workspace_parent_concurrency', 'account_parent_concurrency_b', 'tiktok', 1,
			 'dispatch', 'running', 1, 'owner_parent_concurrency_b', $1)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}

	blockerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerConn.Release()
	workerA, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerA.Release()
	workerB, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerB.Release()

	blockerTx, err := blockerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerTx.Rollback(ctx)
	if _, err := blockerTx.Exec(ctx, `
		SELECT id FROM post_delivery_jobs
		WHERE id = ANY($1::text[])
		ORDER BY id
		FOR UPDATE
	`, []string{"job_parent_concurrency_a", "job_parent_concurrency_b"}); err != nil {
		t.Fatal(err)
	}

	var blockerBackend, workerABackend, workerBBackend int
	if err := blockerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBackend); err != nil {
		t.Fatal(err)
	}
	if err := workerA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&workerABackend); err != nil {
		t.Fatal(err)
	}
	if err := workerB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&workerBBackend); err != nil {
		t.Fatal(err)
	}

	post := db.SocialPost{
		ID: "post_parent_concurrency", WorkspaceID: "workspace_parent_concurrency",
		Status: "publishing", Metadata: metadata,
	}
	jobA := db.PostDeliveryJob{
		ID: "job_parent_concurrency_a", PostID: post.ID,
		SocialPostResultID: "result_parent_concurrency_a", WorkspaceID: post.WorkspaceID,
		SocialAccountID: "account_parent_concurrency_a", Platform: "tiktok",
		LeaseOwner:    pgtype.Text{String: "owner_parent_concurrency_a", Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: attemptedAt, Valid: true},
	}
	jobB := db.PostDeliveryJob{
		ID: "job_parent_concurrency_b", PostID: post.ID,
		SocialPostResultID: "result_parent_concurrency_b", WorkspaceID: post.WorkspaceID,
		SocialAccountID: "account_parent_concurrency_b", Platform: "tiktok",
		LeaseOwner:    pgtype.Text{String: "owner_parent_concurrency_b", Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: attemptedAt, Valid: true},
	}
	resultA := db.SocialPostResult{ID: jobA.SocialPostResultID, PostID: post.ID, SocialAccountID: jobA.SocialAccountID, Status: "processing"}
	resultB := db.SocialPostResult{ID: jobB.SocialPostResultID, PostID: post.ID, SocialAccountID: jobB.SocialAccountID, Status: "processing"}
	decision := publishingrestrictions.Decision{Restricted: true, Platform: "tiktok", CycleID: "cycle_parent_concurrency"}

	finalized := make(chan error, 2)
	go func() {
		finalized <- (&SocialPostHandler{queries: db.New(workerA)}).finalizeRestrictedDeliveryJob(ctx, jobA, resultA, post, decision)
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, workerABackend, blockerBackend)
	go func() {
		finalized <- (&SocialPostHandler{queries: db.New(workerB)}).finalizeRestrictedDeliveryJob(ctx, jobB, resultB, post, decision)
	}()
	waitForPostgresSessionBlocked(t, ctx, pool, workerBBackend)

	if err := blockerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-finalized; err != nil {
			t.Fatalf("concurrent restricted finalization: %v", err)
		}
	}

	var parentStatus string
	var failedResults, deadJobs, activeJobs int
	if err := pool.QueryRow(ctx, `
		SELECT parent.status,
		       COUNT(DISTINCT result.id) FILTER (WHERE result.status = 'failed')::int,
		       COUNT(DISTINCT job.id) FILTER (WHERE job.state = 'dead')::int,
		       COUNT(DISTINCT job.id) FILTER (WHERE job.state IN ('pending', 'running', 'retrying'))::int
		FROM social_posts parent
		JOIN social_post_results result ON result.post_id = parent.id
		JOIN post_delivery_jobs job ON job.post_id = parent.id
		WHERE parent.id = 'post_parent_concurrency'
		GROUP BY parent.status
	`).Scan(&parentStatus, &failedResults, &deadJobs, &activeJobs); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "failed" || failedResults != 2 || deadJobs != 2 || activeJobs != 0 {
		t.Fatalf("concurrent restricted parent/result/jobs = %q/%d/%d/%d, want failed/2/2/0",
			parentStatus, failedResults, deadJobs, activeJobs)
	}
}

func TestRestrictedAndPublishedFinalizersConvergeParentPartial(t *testing.T) {
	runMixedRestrictedTerminalParentRace(t, mixedTerminalParentRace{
		suffix:              "published",
		initialParentStatus: "publishing",
		siblingResultStatus: "published",
		includePublished:    true,
		wantParentStatus:    "partial",
	})
}

func TestRestrictedAndOrdinaryTerminalFailureConvergeParentFailed(t *testing.T) {
	runMixedRestrictedTerminalParentRace(t, mixedTerminalParentRace{
		suffix:              "failed",
		initialParentStatus: "publishing",
		siblingResultStatus: "failed",
		wantParentStatus:    "failed",
	})
}

type mixedTerminalParentRace struct {
	suffix              string
	initialParentStatus string
	siblingResultStatus string
	includePublished    bool
	wantParentStatus    string
}

func runMixedRestrictedTerminalParentRace(t *testing.T, fixture mixedTerminalParentRace) {
	t.Helper()
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	attemptedAt := time.Date(2026, 7, 27, 3, 0, 0, 0, time.UTC)
	postID := "post_mixed_terminal_" + fixture.suffix
	workspaceID := "workspace_mixed_terminal_" + fixture.suffix
	restrictedResultID := "result_mixed_restricted_" + fixture.suffix
	restrictedJobID := "job_mixed_restricted_" + fixture.suffix
	siblingResultID := "result_mixed_sibling_" + fixture.suffix
	siblingJobID := "job_mixed_sibling_" + fixture.suffix
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{
		{AccountID: "account_mixed_restricted_" + fixture.suffix},
		{AccountID: "account_mixed_sibling_" + fixture.suffix},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id, metadata)
		VALUES ($1, $2, $3, $4)
	`, postID, fixture.initialParentStatus, workspaceID, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at) VALUES
			($1, $3, $4, 'processing', NULL),
			($2, $3, $5, 'processing', NULL)
	`, restrictedResultID, siblingResultID, postID,
		"account_mixed_restricted_"+fixture.suffix,
		"account_mixed_sibling_"+fixture.suffix); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, post_input_index, kind, state, attempts, lease_owner, last_attempt_at, finished_at
		) VALUES
			($1, $3, $4, $5, $6, 'tiktok', 0, 'dispatch', 'running', 1, $7, $8, NULL),
			($2, $3, $9, $5, $10, 'instagram', 1, 'dispatch', 'running', 1, $11, $8, NULL)
	`, restrictedJobID, siblingJobID, postID, restrictedResultID, workspaceID,
		"account_mixed_restricted_"+fixture.suffix,
		"owner_mixed_restricted_"+fixture.suffix, attemptedAt,
		siblingResultID, "account_mixed_sibling_"+fixture.suffix,
		"owner_mixed_sibling_"+fixture.suffix); err != nil {
		t.Fatal(err)
	}
	if fixture.includePublished {
		publishedResultID := "result_mixed_existing_published_" + fixture.suffix
		publishedJobID := "job_mixed_existing_published_" + fixture.suffix
		if _, err := pool.Exec(ctx, `
			INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at)
			VALUES ($1, $2, $3, 'published', $4)
		`, publishedResultID, postID, "account_mixed_existing_published_"+fixture.suffix,
			attemptedAt.Add(-2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, post_input_index, kind, state, attempts, finished_at
			) VALUES ($1, $2, $3, $4, $5, 'youtube', 2, 'dispatch', 'succeeded', 1, $6)
		`, publishedJobID, postID, publishedResultID, workspaceID,
			"account_mixed_existing_published_"+fixture.suffix, attemptedAt.Add(-2*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}

	blockerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerConn.Release()
	restrictedConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer restrictedConn.Release()
	siblingConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer siblingConn.Release()

	blockerTx, err := blockerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerTx.Rollback(ctx)
	if _, err := blockerTx.Exec(ctx, `SELECT id FROM social_posts WHERE id = $1 FOR UPDATE`, postID); err != nil {
		t.Fatal(err)
	}
	var blockerBackend, restrictedBackend, siblingBackend int
	if err := blockerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBackend); err != nil {
		t.Fatal(err)
	}
	if err := restrictedConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&restrictedBackend); err != nil {
		t.Fatal(err)
	}
	if err := siblingConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&siblingBackend); err != nil {
		t.Fatal(err)
	}

	post := db.SocialPost{ID: postID, WorkspaceID: workspaceID, Status: fixture.initialParentStatus, Metadata: metadata}
	restrictedJob := db.PostDeliveryJob{
		ID: restrictedJobID, PostID: postID, SocialPostResultID: restrictedResultID,
		WorkspaceID: workspaceID, SocialAccountID: "account_mixed_restricted_" + fixture.suffix,
		Platform: "tiktok", LeaseOwner: pgtype.Text{String: "owner_mixed_restricted_" + fixture.suffix, Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: attemptedAt, Valid: true},
	}
	restrictedResult := db.SocialPostResult{
		ID: restrictedResultID, PostID: postID,
		SocialAccountID: restrictedJob.SocialAccountID, Status: "processing",
	}
	siblingJob := db.PostDeliveryJob{
		ID: siblingJobID, PostID: postID, SocialPostResultID: siblingResultID,
		WorkspaceID: workspaceID, SocialAccountID: "account_mixed_sibling_" + fixture.suffix,
		Platform: "instagram", LeaseOwner: pgtype.Text{String: "owner_mixed_sibling_" + fixture.suffix, Valid: true},
		LastAttemptAt: pgtype.Timestamptz{Time: attemptedAt, Valid: true},
		Kind:          "dispatch", State: "running", Attempts: 1, MaxAttempts: 3,
	}
	siblingResult := db.SocialPostResult{
		ID: siblingResultID, PostID: postID,
		SocialAccountID: siblingJob.SocialAccountID, Status: "processing",
	}
	decision := publishingrestrictions.Decision{Restricted: true, Platform: "tiktok", CycleID: "cycle_mixed_" + fixture.suffix}

	finalized := make(chan error, 2)
	go func() {
		finalized <- (&SocialPostHandler{queries: db.New(restrictedConn)}).finalizeRestrictedDeliveryJob(
			ctx, restrictedJob, restrictedResult, post, decision,
		)
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, restrictedBackend, blockerBackend)
	go func() {
		siblingQueries := db.New(siblingConn)
		siblingHandler := NewSocialPostHandler(siblingQueries, nil, nil, events.NoopBus{}, nil, nil, nil)
		if fixture.siblingResultStatus == "published" {
			publishedAt := time.Now()
			_, siblingErr := siblingQueries.UpdateSocialPostResultAfterRetryAndIncrementUsage(ctx,
				db.UpdateSocialPostResultAfterRetryAndIncrementUsageParams{
					ID: siblingResult.ID, Status: "published",
					ExternalID:  pgtype.Text{String: "mixed_published_" + fixture.suffix, Valid: true},
					PublishedAt: pgtype.Timestamptz{Time: publishedAt, Valid: true},
					WorkspaceID: workspaceID, Period: quota.PeriodForTime(publishedAt), PostCount: 1,
				})
			if siblingErr == nil {
				_, siblingErr = siblingQueries.MarkPostDeliveryJobSucceeded(ctx,
					markDeliveryJobSucceededParams(siblingJob, publishedAt))
			}
			if siblingErr == nil {
				siblingErr = siblingHandler.refreshTerminalParentPostStatusContext(ctx, post, nil)
			}
			finalized <- siblingErr
			return
		}
		finalized <- siblingHandler.handleJobDispatchFailure(ctx, post, siblingResult, siblingJob, publishOneOutcome{
			platform: siblingJob.Platform,
			err:      errors.New("invalid request from ordinary sibling"),
		})
	}()
	waitForPostgresSessionBlocked(t, ctx, pool, siblingBackend)

	if err := blockerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := <-finalized; err != nil {
			t.Fatalf("mixed terminal finalization: %v", err)
		}
	}
	var parentStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = $1`, postID).Scan(&parentStatus); err != nil {
		t.Fatal(err)
	}
	if parentStatus != fixture.wantParentStatus {
		t.Fatalf("mixed terminal parent status = %q, want %q", parentStatus, fixture.wantParentStatus)
	}
}

func TestProcessPostDeliveryJobPublishedEarlyExitRefreshesParent(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	attemptedAt := time.Date(2026, 7, 27, 4, 0, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_published_early_exit', 'publishing', 'workspace_published_early_exit')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at)
		VALUES ('result_published_early_exit', 'post_published_early_exit', 'account_published_early_exit', 'published', $1)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES (
			'job_published_early_exit', 'post_published_early_exit', 'result_published_early_exit',
			'workspace_published_early_exit', 'account_published_early_exit', 'tiktok',
			'retry', 'retrying', 2, 'owner_published_early_exit', $1
		)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	job, err := db.New(pool).GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_published_early_exit", WorkspaceID: "workspace_published_early_exit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&SocialPostHandler{queries: db.New(pool), bus: events.NoopBus{}}).ProcessPostDeliveryJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	var jobState, parentStatus string
	if err := pool.QueryRow(ctx, `
		SELECT job.state, parent.status
		FROM post_delivery_jobs job
		JOIN social_posts parent ON parent.id = job.post_id
		WHERE job.id = $1
	`, job.ID).Scan(&jobState, &parentStatus); err != nil {
		t.Fatal(err)
	}
	if jobState != "succeeded" || parentStatus != "published" {
		t.Fatalf("published early exit job/parent = %q/%q, want succeeded/published", jobState, parentStatus)
	}
}

func TestRecoverStalePublishedResultRefreshesParent(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	attemptedAt := time.Date(2026, 7, 27, 4, 15, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_stale_published_refresh', 'publishing', 'workspace_stale_published_refresh')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at)
		VALUES ('result_stale_published_refresh', 'post_stale_published_refresh', 'account_stale_published_refresh', 'published', $1)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES (
			'job_stale_published_refresh', 'post_stale_published_refresh', 'result_stale_published_refresh',
			'workspace_stale_published_refresh', 'account_stale_published_refresh', 'instagram',
			'retry', 'retrying', 3, 'owner_stale_published_refresh', $1
		)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	job, err := db.New(pool).GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_stale_published_refresh", WorkspaceID: "workspace_stale_published_refresh",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := (&SocialPostHandler{queries: db.New(pool), bus: events.NoopBus{}}).recoverStaleDeliveryJob(ctx, job); err != nil {
		t.Fatal(err)
	}
	var jobState, parentStatus string
	if err := pool.QueryRow(ctx, `
		SELECT job.state, parent.status
		FROM post_delivery_jobs job
		JOIN social_posts parent ON parent.id = job.post_id
		WHERE job.id = $1
	`, job.ID).Scan(&jobState, &parentStatus); err != nil {
		t.Fatal(err)
	}
	if jobState != "succeeded" || parentStatus != "published" {
		t.Fatalf("stale published recovery job/parent = %q/%q, want succeeded/published", jobState, parentStatus)
	}
}

func TestCancelDeliveryJobReturnsMissingParentRefreshError(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status)
		VALUES ('result_cancel_missing_parent', 'post_cancel_missing_parent', 'account_cancel_missing_parent', 'failed');
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts
		) VALUES (
			'job_cancel_missing_parent', 'post_cancel_missing_parent', 'result_cancel_missing_parent',
			'workspace_cancel_missing_parent', 'account_cancel_missing_parent', 'tiktok',
			'retry', 'pending', 1
		)
	`); err != nil {
		t.Fatal(err)
	}
	_, err := (&SocialPostHandler{queries: db.New(pool)}).CancelDeliveryJob(
		ctx, "workspace_cancel_missing_parent", "job_cancel_missing_parent",
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cancel missing-parent refresh error = %v, want pgx.ErrNoRows", err)
	}
	var state string
	if err := pool.QueryRow(ctx, `SELECT state FROM post_delivery_jobs WHERE id = 'job_cancel_missing_parent'`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != "pending" {
		t.Fatalf("missing-parent cancel rollback state = %q, want pending", state)
	}
}

func TestCancelDeliveryJobReturnsParentStatusUpdateError(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_cancel_refresh_error', 'publishing', 'workspace_cancel_refresh_error');
		INSERT INTO social_post_results (id, post_id, social_account_id, status)
		VALUES ('result_cancel_refresh_error', 'post_cancel_refresh_error', 'account_cancel_refresh_error', 'processing');
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts
		) VALUES (
			'job_cancel_refresh_error', 'post_cancel_refresh_error', 'result_cancel_refresh_error',
			'workspace_cancel_refresh_error', 'account_cancel_refresh_error', 'tiktok',
			'retry', 'pending', 1
		);
		CREATE FUNCTION reject_cancel_parent_status_update() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'injected cancel parent status update failure';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_cancel_parent_status_update
			BEFORE UPDATE OF status ON social_posts
			FOR EACH ROW EXECUTE FUNCTION reject_cancel_parent_status_update();
	`); err != nil {
		t.Fatal(err)
	}
	_, err := (&SocialPostHandler{queries: db.New(pool), bus: events.NoopBus{}}).CancelDeliveryJob(
		ctx, "workspace_cancel_refresh_error", "job_cancel_refresh_error",
	)
	if err == nil || !strings.Contains(err.Error(), "injected cancel parent status update failure") {
		t.Fatalf("cancel parent-status refresh error = %v, want injected update failure", err)
	}
	var jobState, parentStatus, resultStatus string
	if err := pool.QueryRow(ctx, `
		SELECT job.state, parent.status, result.status
		FROM post_delivery_jobs job
		JOIN social_posts parent ON parent.id = job.post_id
		JOIN social_post_results result ON result.id = job.social_post_result_id
		WHERE job.id = 'job_cancel_refresh_error'
	`).Scan(&jobState, &parentStatus, &resultStatus); err != nil {
		t.Fatal(err)
	}
	if jobState != "pending" || parentStatus != "publishing" || resultStatus != "processing" {
		t.Fatalf("cancel refresh rollback job/parent/result = %q/%q/%q, want pending/publishing/processing", jobState, parentStatus, resultStatus)
	}
}

func TestCancelDeliveryJobDoesNotOverwriteConcurrentWorkerClaim(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_cancel_worker_race', 'publishing', 'workspace_cancel_worker_race');
		INSERT INTO social_post_results (id, post_id, social_account_id, status)
		VALUES ('result_cancel_worker_race', 'post_cancel_worker_race', 'account_cancel_worker_race', 'processing');
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts
		) VALUES (
			'job_cancel_worker_race', 'post_cancel_worker_race', 'result_cancel_worker_race',
			'workspace_cancel_worker_race', 'account_cancel_worker_race', 'instagram',
			'retry', 'pending', 0
		)
	`); err != nil {
		t.Fatal(err)
	}
	workerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerConn.Release()
	cancelConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cancelConn.Release()
	workerTx, err := workerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerTx.Rollback(ctx)
	if _, err := workerTx.Exec(ctx, `
		SELECT id FROM post_delivery_jobs WHERE id = 'job_cancel_worker_race' FOR UPDATE
	`); err != nil {
		t.Fatal(err)
	}
	var workerBackend, cancelBackend int
	if err := workerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&workerBackend); err != nil {
		t.Fatal(err)
	}
	if err := cancelConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&cancelBackend); err != nil {
		t.Fatal(err)
	}
	cancelled := make(chan error, 1)
	go func() {
		_, cancelErr := NewSocialPostHandler(db.New(cancelConn), nil, nil, events.NoopBus{}, nil, nil, nil).
			CancelDeliveryJob(ctx, "workspace_cancel_worker_race", "job_cancel_worker_race")
		cancelled <- cancelErr
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, cancelBackend, workerBackend)
	if _, err := workerTx.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'running', attempts = 1, lease_owner = 'worker_won_cancel_race',
		    last_attempt_at = NOW(), first_claimed_at = NOW()
		WHERE id = 'job_cancel_worker_race'
	`); err != nil {
		t.Fatal(err)
	}
	if err := workerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-cancelled; err == nil {
		t.Fatal("concurrent cancel unexpectedly succeeded after worker claimed job")
	}
	var state, leaseOwner, parentStatus string
	if err := pool.QueryRow(ctx, `
		SELECT job.state, job.lease_owner, parent.status
		FROM post_delivery_jobs job
		JOIN social_posts parent ON parent.id = job.post_id
		WHERE job.id = 'job_cancel_worker_race'
	`).Scan(&state, &leaseOwner, &parentStatus); err != nil {
		t.Fatal(err)
	}
	if state != "running" || leaseOwner != "worker_won_cancel_race" || parentStatus != "publishing" {
		t.Fatalf("cancel/worker race job/owner/parent = %q/%q/%q, want running/worker_won_cancel_race/publishing",
			state, leaseOwner, parentStatus)
	}
}

func TestHandleJobDispatchFailureStaleLeaseHasNoSharedSideEffects(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	oldAttempt := time.Date(2026, 7, 27, 4, 20, 0, 0, time.UTC)
	newAttempt := oldAttempt.Add(time.Minute)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_dispatch_stale_lease', 'published', 'workspace_dispatch_stale_lease')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (
			id, post_id, social_account_id, status, external_id, error_message, published_at
		) VALUES (
			'result_dispatch_stale_lease', 'post_dispatch_stale_lease', 'account_dispatch_stale_lease',
			'published', 'external_new_owner', 'new owner result', $1
		)
	`, oldAttempt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, max_attempts, lease_owner, last_attempt_at
		) VALUES (
			'job_dispatch_stale_lease', 'post_dispatch_stale_lease', 'result_dispatch_stale_lease',
			'workspace_dispatch_stale_lease', 'account_dispatch_stale_lease', 'instagram',
			'dispatch', 'running', 1, 3, 'old_dispatch_owner', $1
		)
	`, oldAttempt); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	staleJob, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_dispatch_stale_lease", WorkspaceID: "workspace_dispatch_stale_lease",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET lease_owner = 'new_dispatch_owner', last_attempt_at = $2::timestamptz,
		    last_error = 'new owner job', next_run_at = $2::timestamptz + INTERVAL '10 minutes'
		WHERE id = $1
	`, staleJob.ID, newAttempt); err != nil {
		t.Fatal(err)
	}
	post, err := queries.GetSocialPostByID(ctx, staleJob.PostID)
	if err != nil {
		t.Fatal(err)
	}
	staleResult := db.SocialPostResult{
		ID: staleJob.SocialPostResultID, PostID: staleJob.PostID,
		SocialAccountID: staleJob.SocialAccountID, Status: "processing",
	}
	h := NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil)
	if err := h.handleJobDispatchFailure(ctx, post, staleResult, staleJob, publishOneOutcome{
		platform: "instagram", err: errors.New("temporarily unavailable"),
	}); err != nil {
		t.Errorf("stale dispatch owner failure handling: %v", err)
	}
	var resultStatus, resultError string
	var externalID pgtype.Text
	var jobState, leaseOwner, jobError string
	var lastAttempt time.Time
	var jobCount, failureCount int
	if err := pool.QueryRow(ctx, `
		SELECT result.status, result.external_id, result.error_message,
		       job.state, job.lease_owner, job.last_attempt_at, job.last_error,
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = job.post_id),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id = job.post_id)
		FROM social_post_results result
		JOIN post_delivery_jobs job ON job.social_post_result_id = result.id
		WHERE job.id = $1
	`, staleJob.ID).Scan(
		&resultStatus, &externalID, &resultError,
		&jobState, &leaseOwner, &lastAttempt, &jobError,
		&jobCount, &failureCount,
	); err != nil {
		t.Fatal(err)
	}
	if resultStatus != "published" || !externalID.Valid || externalID.String != "external_new_owner" || resultError != "new owner result" ||
		jobState != "running" || leaseOwner != "new_dispatch_owner" || !lastAttempt.Equal(newAttempt) ||
		jobError != "new owner job" || jobCount != 1 || failureCount != 0 {
		t.Fatalf("stale dispatch side effects result=%q/%q/%q job=%q/%q/%s/%q counts=%d/%d",
			resultStatus, externalID.String, resultError, jobState, leaseOwner, lastAttempt, jobError, jobCount, failureCount)
	}
	var parentStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = $1`, staleJob.PostID).Scan(&parentStatus); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "published" {
		t.Fatalf("stale dispatch parent status = %q, want published", parentStatus)
	}
}

func TestRecoverStaleDeliveryJobLostLeaseHasNoSharedSideEffects(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	oldAttempt := time.Date(2026, 7, 27, 4, 25, 0, 0, time.UTC)
	newAttempt := oldAttempt.Add(time.Minute)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_recovery_stale_lease', 'publishing', 'workspace_recovery_stale_lease')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, error_message)
		VALUES (
			'result_recovery_stale_lease', 'post_recovery_stale_lease', 'account_recovery_stale_lease',
			'processing', 'new recovery owner result'
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, max_attempts, lease_owner, last_attempt_at
		) VALUES (
			'job_recovery_stale_lease', 'post_recovery_stale_lease', 'result_recovery_stale_lease',
			'workspace_recovery_stale_lease', 'account_recovery_stale_lease', 'instagram',
			'retry', 'retrying', 3, 3, 'old_recovery_owner', $1
		)
	`, oldAttempt); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	staleJob, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_recovery_stale_lease", WorkspaceID: "workspace_recovery_stale_lease",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET lease_owner = 'new_recovery_owner', last_attempt_at = $2::timestamptz,
		    last_error = 'new recovery owner job', next_run_at = $2::timestamptz + INTERVAL '10 minutes'
		WHERE id = $1
	`, staleJob.ID, newAttempt); err != nil {
		t.Fatal(err)
	}
	h := NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil)
	if err := h.recoverStaleDeliveryJob(ctx, staleJob); err != nil {
		t.Errorf("lost-lease stale recovery: %v", err)
	}
	var resultStatus, resultError string
	var jobState, leaseOwner, jobError string
	var lastAttempt time.Time
	var jobCount, failureCount int
	if err := pool.QueryRow(ctx, `
		SELECT result.status, result.error_message,
		       job.state, job.lease_owner, job.last_attempt_at, job.last_error,
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = job.post_id),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id = job.post_id)
		FROM social_post_results result
		JOIN post_delivery_jobs job ON job.social_post_result_id = result.id
		WHERE job.id = $1
	`, staleJob.ID).Scan(
		&resultStatus, &resultError,
		&jobState, &leaseOwner, &lastAttempt, &jobError,
		&jobCount, &failureCount,
	); err != nil {
		t.Fatal(err)
	}
	if resultStatus != "processing" || resultError != "new recovery owner result" ||
		jobState != "retrying" || leaseOwner != "new_recovery_owner" || !lastAttempt.Equal(newAttempt) ||
		jobError != "new recovery owner job" || jobCount != 1 || failureCount != 0 {
		t.Fatalf("lost-lease recovery side effects result=%q/%q job=%q/%q/%s/%q counts=%d/%d",
			resultStatus, resultError, jobState, leaseOwner, lastAttempt, jobError, jobCount, failureCount)
	}
	var parentStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = $1`, staleJob.PostID).Scan(&parentStatus); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "publishing" {
		t.Fatalf("lost-lease recovery parent status = %q, want publishing", parentStatus)
	}
}

type leaseLostAfterProviderSuccessAdapter struct {
	pool             *pgxpool.Pool
	oldJobID         string
	newJobID         string
	postCalls        int
	providerExternal string
	providerURL      string
}

func (a *leaseLostAfterProviderSuccessAdapter) Platform() string { return "instagram" }
func (a *leaseLostAfterProviderSuccessAdapter) Connect(context.Context, map[string]string) (*platform.ConnectResult, error) {
	return nil, errors.New("unexpected connect")
}
func (a *leaseLostAfterProviderSuccessAdapter) Post(ctx context.Context, _ string, _ string, _ []platform.MediaItem, _ map[string]any) (*platform.PostResult, error) {
	a.postCalls++
	if _, err := a.pool.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'failed', lease_owner = NULL, lease_expires_at = NULL,
		    last_error = 'lease expired after provider accepted post', finished_at = NOW()
		WHERE id = $1
	`, a.oldJobID); err != nil {
		return nil, err
	}
	if _, err := a.pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, post_input_index, kind, state, attempts, max_attempts,
			lease_owner, last_attempt_at, first_claimed_at
		)
		SELECT $2, post_id, social_post_result_id, workspace_id, social_account_id,
		       platform, post_input_index, 'retry', 'retrying', 1, max_attempts,
		       'new_success_owner', NOW(), NOW()
		FROM post_delivery_jobs WHERE id = $1
	`, a.oldJobID, a.newJobID); err != nil {
		return nil, err
	}
	return &platform.PostResult{ExternalID: a.providerExternal, URL: a.providerURL}, nil
}
func (a *leaseLostAfterProviderSuccessAdapter) DeletePost(context.Context, string, string) error {
	return errors.New("unexpected delete")
}
func (a *leaseLostAfterProviderSuccessAdapter) RefreshToken(context.Context, string) (string, string, time.Time, error) {
	return "", "", time.Time{}, errors.New("unexpected refresh")
}

type countingPostEventBus struct {
	calls  int
	events []string
}

func (b *countingPostEventBus) Publish(_ context.Context, _ string, event string, _ any) {
	b.calls++
	b.events = append(b.events, event)
}

type blockingPostEventBus struct {
	entered chan string
	release chan struct{}
}

func (b *blockingPostEventBus) Publish(_ context.Context, _ string, event string, _ any) {
	b.entered <- event
	<-b.release
}

func TestProcessPostDeliveryJobProviderSuccessSurvivesLostLeaseAndRetryConverges(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_provider_success_lost_lease", Caption: "publish once",
	}})
	if err != nil {
		t.Fatal(err)
	}
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedToken, err := encryptor.Encrypt("provider_access_token")
	if err != nil {
		t.Fatal(err)
	}
	attemptedAt := time.Date(2026, 7, 27, 4, 35, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id, metadata)
		VALUES ('post_provider_success_lost_lease', 'publishing', 'workspace_1', $1)
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, caption)
		VALUES (
			'result_provider_success_lost_lease', 'post_provider_success_lost_lease',
			'account_provider_success_lost_lease', 'processing', 'publish once'
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, profile_id, platform, access_token, external_account_id, status)
		VALUES (
			'account_provider_success_lost_lease', 'profile_1', 'instagram', $1,
			'provider_account_success', 'active'
		)
	`, encryptedToken); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, max_attempts, lease_owner, last_attempt_at, first_claimed_at
		) VALUES (
			'job_provider_success_old_owner', 'post_provider_success_lost_lease',
			'result_provider_success_lost_lease', 'workspace_1', 'account_provider_success_lost_lease',
			'instagram', 'dispatch', 'running', 1, 3, 'old_success_owner', $1, $1
		)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	oldJob, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_provider_success_old_owner", WorkspaceID: "workspace_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &leaseLostAfterProviderSuccessAdapter{
		pool: pool, oldJobID: oldJob.ID, newJobID: "job_provider_success_new_owner",
		providerExternal: "provider_post_authoritative", providerURL: "https://example.com/provider_post_authoritative",
	}
	previous, previousErr := platform.Get("instagram")
	platform.Register(adapter)
	defer func() {
		if previousErr == nil {
			platform.Register(previous)
		}
	}()
	bus := &countingPostEventBus{}
	h := NewSocialPostHandler(queries, encryptor, nil, bus, nil, nil, nil)
	if err := h.ProcessPostDeliveryJob(ctx, oldJob); err != nil {
		t.Errorf("old owner provider success reconciliation: %v", err)
	}

	var resultStatus, externalID, providerURL, parentStatus, oldJobState, newJobState string
	var usageCount int
	if err := pool.QueryRow(ctx, `
		SELECT result.status, result.external_id, result.url, parent.status,
		       old_job.state, new_job.state,
		       COALESCE((SELECT post_count FROM usage WHERE workspace_id = 'workspace_1' AND period = $1), 0)::int
		FROM social_post_results result
		JOIN social_posts parent ON parent.id = result.post_id
		JOIN post_delivery_jobs old_job ON old_job.id = 'job_provider_success_old_owner'
		JOIN post_delivery_jobs new_job ON new_job.id = 'job_provider_success_new_owner'
		WHERE result.id = 'result_provider_success_lost_lease'
	`, quota.PeriodForTime(time.Now())).Scan(
		&resultStatus, &externalID, &providerURL, &parentStatus,
		&oldJobState, &newJobState, &usageCount,
	); err != nil {
		t.Fatal(err)
	}
	if resultStatus != "published" || externalID != adapter.providerExternal || providerURL != adapter.providerURL ||
		parentStatus != "published" || oldJobState != "failed" || newJobState != "retrying" || usageCount != 1 {
		t.Fatalf("old-owner success result=%q/%q/%q parent=%q jobs=%q/%q usage=%d",
			resultStatus, externalID, providerURL, parentStatus, oldJobState, newJobState, usageCount)
	}
	var publishedEvents, publishedDeliveries int
	if err := pool.QueryRow(ctx, `SELECT
		COUNT(DISTINCT e.id), COUNT(d.event_id)
		FROM terminal_post_event_outbox e
		LEFT JOIN terminal_post_event_deliveries d ON d.event_id=e.id
		WHERE e.post_id='post_provider_success_lost_lease' AND e.event_type='post.published'`).Scan(&publishedEvents, &publishedDeliveries); err != nil {
		t.Fatal(err)
	}
	if adapter.postCalls != 1 || bus.calls != 0 || publishedEvents != 1 || publishedDeliveries != 3 {
		t.Fatalf("before owned retry adapter/bus/outbox/deliveries = %d/%d/%d/%d, want 1/0/1/3", adapter.postCalls, bus.calls, publishedEvents, publishedDeliveries)
	}

	newJob, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: adapter.newJobID, WorkspaceID: "workspace_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ProcessPostDeliveryJob(ctx, newJob); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT parent.status, new_job.state,
		       (SELECT post_count FROM usage WHERE workspace_id = 'workspace_1' AND period = $1)::int
		FROM social_posts parent
		JOIN post_delivery_jobs new_job ON new_job.post_id = parent.id
		WHERE parent.id = 'post_provider_success_lost_lease'
		  AND new_job.id = 'job_provider_success_new_owner'
	`, quota.PeriodForTime(time.Now())).Scan(&parentStatus, &newJobState, &usageCount); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "published" || newJobState != "succeeded" || usageCount != 1 || adapter.postCalls != 1 {
		t.Fatalf("owned retry convergence parent/job/usage/adapter = %q/%q/%d/%d, want published/succeeded/1/1",
			parentStatus, newJobState, usageCount, adapter.postCalls)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM terminal_post_event_outbox WHERE post_id='post_provider_success_lost_lease' AND event_type='post.published'`).Scan(&publishedEvents); err != nil {
		t.Fatal(err)
	}
	if bus.calls != 0 || publishedEvents != 1 {
		t.Fatalf("converged direct_bus/outbox_events = %d/%d, want 0/1", bus.calls, publishedEvents)
	}
}

func TestPublishTokenCallbackLostLeaseDoesNotOverwriteNewOwnerToken(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	attemptedAt := time.Date(2026, 7, 27, 4, 40, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, publish_token)
		VALUES (
			'result_token_lost_lease', 'post_token_lost_lease', 'account_token_lost_lease',
			'processing', 'old_initial_token'
		)
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES (
			'job_token_lost_lease', 'post_token_lost_lease', 'result_token_lost_lease',
			'workspace_token_lost_lease', 'account_token_lost_lease', 'tiktok',
			'retry', 'retrying', 1, 'old_token_owner', $1
		)
	`, attemptedAt); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	job, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
		ID: "job_token_lost_lease", WorkspaceID: "workspace_token_lost_lease",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
		ID: "result_token_lost_lease", PostID: "post_token_lost_lease",
	})
	if err != nil {
		t.Fatal(err)
	}
	pp := platform.PlatformPostInput{AccountID: job.SocialAccountID}
	NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil).
		attachPublishTokenResume(ctx, &pp, result, job)
	callback, ok := pp.PlatformOptions[platform.OptOnPublishToken].(func(string) error)
	if !ok {
		t.Fatal("publish-token callback must propagate persistence failures")
	}
	if _, err := pool.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET lease_owner = 'new_token_owner', last_attempt_at = $2::timestamptz
		WHERE id = $1
	`, job.ID, attemptedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE social_post_results
		SET publish_token = 'new_owner_token'
		WHERE id = 'result_token_lost_lease'
	`); err != nil {
		t.Fatal(err)
	}
	if err := callback("late_old_owner_token"); err == nil {
		t.Fatal("lost-lease publish-token callback must fail closed")
	}
	var token string
	if err := pool.QueryRow(ctx, `
		SELECT publish_token FROM social_post_results WHERE id = 'result_token_lost_lease'
	`).Scan(&token); err != nil {
		t.Fatal(err)
	}
	if token != "new_owner_token" {
		t.Fatalf("lost-lease token callback stored %q, want new_owner_token", token)
	}
}

func TestTerminalParentRefreshWaiterPreservesCommittedPublishedAt(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	earliestPublishedAt := time.Date(2026, 7, 27, 4, 30, 0, 0, time.UTC)
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "account_refresh_parent_fresh"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id, metadata)
		VALUES ('post_refresh_parent_fresh', 'publishing', 'workspace_refresh_parent_fresh', $1)
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at) VALUES
			('result_refresh_parent_published', 'post_refresh_parent_fresh', 'account_refresh_parent_published', 'published', $1),
			('result_refresh_parent_failed', 'post_refresh_parent_fresh', 'account_refresh_parent_failed', 'failed', NULL)
	`, earliestPublishedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, finished_at
		) VALUES
			('job_refresh_parent_published', 'post_refresh_parent_fresh', 'result_refresh_parent_published',
			 'workspace_refresh_parent_fresh', 'account_refresh_parent_published', 'instagram', 'dispatch', 'succeeded', 1, $1),
			('job_refresh_parent_failed', 'post_refresh_parent_fresh', 'result_refresh_parent_failed',
			 'workspace_refresh_parent_fresh', 'account_refresh_parent_failed', 'tiktok', 'dispatch', 'dead', 1, $1)
	`, earliestPublishedAt); err != nil {
		t.Fatal(err)
	}
	ownerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ownerConn.Release()
	waiterConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer waiterConn.Release()
	ownerTx, err := ownerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer ownerTx.Rollback(ctx)
	if err := db.New(ownerTx).LockPostDeliveryJobFinalization(ctx, "post_refresh_parent_fresh"); err != nil {
		t.Fatal(err)
	}
	if _, err := ownerTx.Exec(ctx, `
		UPDATE social_posts SET status = 'partial', published_at = $2 WHERE id = $1
	`, "post_refresh_parent_fresh", earliestPublishedAt); err != nil {
		t.Fatal(err)
	}
	var ownerBackend, waiterBackend int
	if err := ownerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&ownerBackend); err != nil {
		t.Fatal(err)
	}
	if err := waiterConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&waiterBackend); err != nil {
		t.Fatal(err)
	}
	stalePost := db.SocialPost{
		ID: "post_refresh_parent_fresh", WorkspaceID: "workspace_refresh_parent_fresh",
		Status: "publishing", Metadata: metadata,
	}
	refreshed := make(chan error, 1)
	go func() {
		refreshed <- NewSocialPostHandler(db.New(waiterConn), nil, nil, events.NoopBus{}, nil, nil, nil).
			refreshTerminalParentPostStatusContext(ctx, stalePost, nil)
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, waiterBackend, ownerBackend)
	if err := ownerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-refreshed; err != nil {
		t.Fatal(err)
	}
	var status string
	var publishedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, published_at FROM social_posts WHERE id = 'post_refresh_parent_fresh'
	`).Scan(&status, &publishedAt); err != nil {
		t.Fatal(err)
	}
	if status != "partial" || !publishedAt.Equal(earliestPublishedAt) {
		t.Fatalf("fresh parent refresh = status:%q published_at:%s, want partial/%s",
			status, publishedAt, earliestPublishedAt)
	}
}

type committedParentObserverBus struct {
	pool           *pgxpool.Pool
	publishCalls   int
	observedStatus string
	err            error
}

func (b *committedParentObserverBus) Publish(ctx context.Context, _ string, _ string, data any) {
	b.publishCalls++
	response, ok := data.(socialPostResponse)
	if !ok {
		b.err = fmt.Errorf("event data type = %T, want socialPostResponse", data)
		return
	}
	b.err = b.pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = $1`, response.ID).Scan(&b.observedStatus)
}

func TestTerminalParentEventPublishesAfterStatusCommit(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id)
		VALUES ('post_event_after_commit', 'publishing', 'workspace_event_after_commit');
		INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at)
		VALUES (
			'result_event_after_commit', 'post_event_after_commit', 'account_event_after_commit',
			'published', NOW()
		);
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, finished_at
		) VALUES (
			'job_event_after_commit', 'post_event_after_commit', 'result_event_after_commit',
			'workspace_event_after_commit', 'account_event_after_commit', 'instagram',
			'dispatch', 'succeeded', 1, NOW()
		)
	`); err != nil {
		t.Fatal(err)
	}
	h := NewSocialPostHandler(db.New(pool), nil, nil, events.NoopBus{}, nil, nil, nil)
	if err := h.refreshTerminalParentPostStatusContext(ctx, db.SocialPost{ID: "post_event_after_commit"}, nil); err != nil {
		t.Fatal(err)
	}
	var parentStatus, eventStatus, eventType, payloadVersion string
	if err := pool.QueryRow(ctx, `SELECT p.status,e.parent_status,e.event_type,e.payload->>'parent_version'
		FROM social_posts p JOIN terminal_post_event_outbox e ON e.post_id=p.id
		WHERE p.id='post_event_after_commit'`).Scan(&parentStatus, &eventStatus, &eventType, &payloadVersion); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "published" || eventStatus != "published" || eventType != events.EventPostPublished || payloadVersion == "" {
		t.Fatalf("committed parent/outbox/event/version = %q/%q/%q/%q", parentStatus, eventStatus, eventType, payloadVersion)
	}
}

func TestImmediateRestrictionRetentionFailureRollsBackCreatedPostAndDeliveries(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ('account_immediate_atomic', 'tiktok', 'active');
		INSERT INTO media (id, workspace_id, status)
		VALUES
			('media_immediate_a', 'workspace_1', 'uploaded'),
			('media_immediate_b', 'workspace_1', 'uploaded');
		CREATE FUNCTION fail_immediate_second_usage() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.media_id = 'media_immediate_b' THEN
				RAISE EXCEPTION 'injected immediate second retention upsert failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER fail_immediate_second_usage_trigger
		BEFORE INSERT OR UPDATE ON media_post_usages
		FOR EACH ROW EXECUTE FUNCTION fail_immediate_second_usage();
	`); err != nil {
		t.Fatal(err)
	}

	h := &SocialPostHandler{queries: db.New(pool)}
	_, err := h.queueImmediatePost(
		ctx,
		"workspace_1",
		parsedRequest{Posts: []platform.PlatformPostInput{{
			AccountID: "account_immediate_atomic",
			Caption:   "immediate atomic retention",
			MediaIDs:  []string{"media_immediate_b", "media_immediate_a"},
		}}},
		map[string]platform.ValidateAccount{
			"account_immediate_atomic": {Platform: "tiktok"},
		},
		map[string]publishingrestrictions.Decision{
			"account_immediate_atomic": {
				Restricted: true,
				Platform:   "tiktok",
				CycleID:    "cycle_immediate_atomic",
			},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "injected immediate second retention upsert failure") {
		t.Fatalf("queue immediate error = %v, want injected second-upsert failure", err)
	}

	var posts, results, jobs, failures, usages, usageVersion int
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*)::int FROM social_posts),
			(SELECT COUNT(*)::int FROM social_post_results),
			(SELECT COUNT(*)::int FROM post_delivery_jobs),
			(SELECT COUNT(*)::int FROM post_failures),
			(SELECT COUNT(*)::int FROM media_post_usages),
			(SELECT COALESCE(SUM(usage_version), 0)::int FROM media)
	`).Scan(&posts, &results, &jobs, &failures, &usages, &usageVersion); err != nil {
		t.Fatal(err)
	}
	if posts != 0 || results != 0 || jobs != 0 || failures != 0 || usages != 0 || usageVersion != 0 {
		t.Fatalf("rolled-back immediate state = posts:%d results:%d jobs:%d failures:%d usages:%d usage_version:%d", posts, results, jobs, failures, usages, usageVersion)
	}
}

func TestDraftRestrictionRetentionFailureRollsBackClaimAndDeliveries(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	posts := []platform.PlatformPostInput{{
		AccountID: "account_draft_atomic",
		Caption:   "draft atomic retention",
		MediaIDs:  []string{"media_draft_b", "media_draft_a"},
	}}
	metadata, err := platform.EncodePostMetadata(posts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ('account_draft_atomic', 'tiktok', 'active')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, metadata, workspace_id, profile_ids)
		VALUES ('post_draft_atomic', 'draft', $1, 'workspace_1', ARRAY['profile_1'])
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES
			('media_draft_a', 'workspace_1', 'uploaded'),
			('media_draft_b', 'workspace_1', 'uploaded');
		CREATE FUNCTION fail_draft_second_usage() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.media_id = 'media_draft_b' THEN
				RAISE EXCEPTION 'injected draft second retention upsert failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER fail_draft_second_usage_trigger
		BEFORE INSERT OR UPDATE ON media_post_usages
		FOR EACH ROW EXECUTE FUNCTION fail_draft_second_usage();
	`); err != nil {
		t.Fatal(err)
	}

	h := &SocialPostHandler{queries: db.New(pool)}
	err = h.queries.WithTransaction(ctx, func(txQueries *db.Queries) error {
		claimed, claimErr := txQueries.ClaimDraftForPublish(ctx, db.ClaimDraftForPublishParams{
			ID:          "post_draft_atomic",
			WorkspaceID: "workspace_1",
		})
		if claimErr != nil {
			return claimErr
		}
		_, enqueueErr := h.withQueueQueries(txQueries).enqueueExistingPostDeliveriesInTransaction(
			ctx,
			claimed,
			posts,
			map[string]platform.ValidateAccount{"account_draft_atomic": {Platform: "tiktok"}},
			map[string]publishingrestrictions.Decision{"account_draft_atomic": {
				Restricted: true,
				Platform:   "tiktok",
				CycleID:    "cycle_draft_atomic",
			}},
		)
		return enqueueErr
	})
	if err == nil || !strings.Contains(err.Error(), "injected draft second retention upsert failure") {
		t.Fatalf("draft transaction error = %v, want injected second-upsert failure", err)
	}

	var status string
	var results, jobs, failures, usages, usageVersion int
	if err := pool.QueryRow(ctx, `
		SELECT p.status,
		       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM media_post_usages WHERE post_id = p.id),
		       (SELECT COALESCE(SUM(usage_version), 0)::int FROM media WHERE id = ANY($2::text[]))
		FROM social_posts p WHERE p.id = $1
	`, "post_draft_atomic", []string{"media_draft_a", "media_draft_b"}).Scan(
		&status, &results, &jobs, &failures, &usages, &usageVersion,
	); err != nil {
		t.Fatal(err)
	}
	if status != "draft" || results != 0 || jobs != 0 || failures != 0 || usages != 0 || usageVersion != 0 {
		t.Fatalf("rolled-back draft state = status:%s results:%d jobs:%d failures:%d usages:%d usage_version:%d", status, results, jobs, failures, usages, usageVersion)
	}
}

func TestScheduledRestrictionRetentionFailureRollsBackClaimAndRemainsClaimable(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_scheduled_atomic",
		Caption:   "scheduled atomic retention",
		MediaIDs:  []string{"media_scheduled_b", "media_scheduled_a"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ('account_scheduled_atomic', 'tiktok', 'active')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, scheduled_at, metadata, workspace_id, profile_ids)
		VALUES ('post_scheduled_atomic', 'scheduled', NOW(), $1, 'workspace_1', ARRAY['profile_1'])
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES
			('media_scheduled_a', 'workspace_1', 'uploaded'),
			('media_scheduled_b', 'workspace_1', 'uploaded');
		CREATE FUNCTION fail_second_restriction_usage() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.media_id = 'media_scheduled_b' THEN
				RAISE EXCEPTION 'injected second retention upsert failure';
			END IF;
			RETURN NEW;
		END $$;
		CREATE TRIGGER fail_second_restriction_usage_trigger
		BEFORE INSERT OR UPDATE ON media_post_usages
		FOR EACH ROW EXECUTE FUNCTION fail_second_restriction_usage();
	`); err != nil {
		t.Fatal(err)
	}

	h := &SocialPostHandler{
		queries: db.New(pool),
		publishingRestrictions: &fakePostRestrictionEvaluator{decisions: map[string]publishingrestrictions.Decision{
			"tiktok": {
				Restricted: true,
				Platform:   "tiktok",
				CycleID:    "cycle_scheduled_atomic",
			},
		}},
	}
	err = h.ClaimAndEnqueueScheduledPost(ctx, "post_scheduled_atomic")
	if err == nil || !strings.Contains(err.Error(), "injected second retention upsert failure") {
		t.Fatalf("first claim error = %v, want injected second-upsert failure", err)
	}

	var status string
	var results, jobs, failures, usages, usageVersion int
	if err := pool.QueryRow(ctx, `
		SELECT p.status,
		       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM media_post_usages WHERE post_id = p.id),
		       (SELECT COALESCE(SUM(usage_version), 0)::int FROM media WHERE id = ANY($2::text[]))
		FROM social_posts p WHERE p.id = $1
	`, "post_scheduled_atomic", []string{"media_scheduled_a", "media_scheduled_b"}).Scan(
		&status, &results, &jobs, &failures, &usages, &usageVersion,
	); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" || results != 0 || jobs != 0 || failures != 0 || usages != 0 || usageVersion != 0 {
		t.Fatalf("rolled-back state = status:%s results:%d jobs:%d failures:%d usages:%d usage_version:%d", status, results, jobs, failures, usages, usageVersion)
	}

	if _, err := pool.Exec(ctx, `DROP TRIGGER fail_second_restriction_usage_trigger ON media_post_usages`); err != nil {
		t.Fatal(err)
	}
	if err := h.ClaimAndEnqueueScheduledPost(ctx, "post_scheduled_atomic"); err != nil {
		t.Fatalf("second claim after rollback: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT p.status,
		       (SELECT COUNT(*)::int FROM social_post_results WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM post_failures WHERE post_id = p.id),
		       (SELECT COUNT(*)::int FROM media_post_usages WHERE post_id = p.id)
		FROM social_posts p WHERE p.id = $1
	`, "post_scheduled_atomic").Scan(&status, &results, &jobs, &failures, &usages); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || results != 1 || jobs != 0 || failures != 1 || usages != 2 {
		t.Fatalf("successful retry state = status:%s results:%d jobs:%d failures:%d usages:%d", status, results, jobs, failures, usages)
	}
}

func TestScheduledOrdinaryRetentionFailureDoesNotAbortClaim(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_scheduled_best_effort",
		Caption:   "ordinary retention remains best effort",
		MediaIDs:  []string{"media_scheduled_best_effort"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ('account_scheduled_best_effort', 'instagram', 'active')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, scheduled_at, metadata, workspace_id, profile_ids)
		VALUES ('post_scheduled_best_effort', 'scheduled', NOW(), $1, 'workspace_1', ARRAY['profile_1'])
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES ('media_scheduled_best_effort', 'workspace_1', 'uploaded');
		CREATE FUNCTION fail_ordinary_scheduled_retention() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'injected ordinary scheduled retention failure';
		END $$;
		CREATE TRIGGER fail_ordinary_scheduled_retention_trigger
		BEFORE INSERT OR UPDATE ON media_post_usages
		FOR EACH ROW EXECUTE FUNCTION fail_ordinary_scheduled_retention();
	`); err != nil {
		t.Fatal(err)
	}

	h := &SocialPostHandler{queries: db.New(pool)}
	if err := h.ClaimAndEnqueueScheduledPost(ctx, "post_scheduled_best_effort"); err != nil {
		t.Fatalf("claim and enqueue with best-effort retention failure: %v", err)
	}
	var status string
	var results, jobs, usages int
	if err := pool.QueryRow(ctx, `
		SELECT post.status,
		       (SELECT COUNT(*)::int FROM social_post_results result WHERE result.post_id = post.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs job WHERE job.post_id = post.id),
		       (SELECT COUNT(*)::int FROM media_post_usages usage WHERE usage.post_id = post.id)
		FROM social_posts post WHERE post.id = 'post_scheduled_best_effort'
	`).Scan(&status, &results, &jobs, &usages); err != nil {
		t.Fatal(err)
	}
	if status != "publishing" || results != 1 || jobs != 1 || usages != 0 {
		t.Fatalf("scheduled best-effort state = status:%q results:%d jobs:%d usages:%d, want publishing/1/1/0", status, results, jobs, usages)
	}
}

func TestDraftOrdinaryRetentionFailureDoesNotAbortClaim(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_draft_best_effort",
		Caption:   "ordinary draft retention remains best effort",
		MediaURLs: []string{"https://cdn.example.com/draft-best-effort.jpg"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ('account_draft_best_effort', 'instagram', 'active')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, metadata, workspace_id, profile_ids)
		VALUES ('post_draft_best_effort', 'draft', $1, 'workspace_1', ARRAY['profile_1'])
	`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES ('media_draft_best_effort_stale', 'workspace_1', 'uploaded');
		INSERT INTO media_post_usages (
			workspace_id, media_id, post_id, post_status, cleanup_after_at, retention_reason
		) VALUES (
			'workspace_1', 'media_draft_best_effort_stale', 'post_draft_best_effort',
			'draft', NULL, 'active_post'
		);
		CREATE FUNCTION fail_ordinary_draft_retention() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'injected ordinary draft retention failure';
		END $$;
		CREATE TRIGGER fail_ordinary_draft_retention_trigger
		BEFORE DELETE ON media_post_usages
		FOR EACH ROW EXECUTE FUNCTION fail_ordinary_draft_retention();
	`); err != nil {
		t.Fatal(err)
	}

	queries := db.New(pool)
	h := NewSocialPostHandler(queries, nil, quota.NewChecker(queries), nil, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/v1/social-posts/post_draft_best_effort/publish", nil)
	req = req.WithContext(auth.SetWorkspaceID(req.Context(), "workspace_1"))
	req = withChiParam(req, "id", "post_draft_best_effort")
	rec := httptest.NewRecorder()
	h.PublishDraft(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("draft response = %d %s, want 202", rec.Code, rec.Body.String())
	}
	var status string
	var results, jobs, usages int
	if err := pool.QueryRow(ctx, `
		SELECT post.status,
		       (SELECT COUNT(*)::int FROM social_post_results result WHERE result.post_id = post.id),
		       (SELECT COUNT(*)::int FROM post_delivery_jobs job WHERE job.post_id = post.id),
		       (SELECT COUNT(*)::int FROM media_post_usages usage WHERE usage.post_id = post.id)
		FROM social_posts post WHERE post.id = 'post_draft_best_effort'
	`).Scan(&status, &results, &jobs, &usages); err != nil {
		t.Fatal(err)
	}
	if status != "publishing" || results != 1 || jobs != 1 || usages != 1 {
		t.Fatalf("draft best-effort state = status:%q results:%d jobs:%d usages:%d, want publishing/1/1/1", status, results, jobs, usages)
	}
}

func TestScheduledInvalidMetadataCommitsTerminalFailure(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, scheduled_at, metadata, workspace_id)
		VALUES (
			'post_scheduled_invalid_metadata', 'scheduled', NOW(),
			'{"schema_version":2,"platform_posts":"invalid"}'::jsonb, 'workspace_1'
		)
	`); err != nil {
		t.Fatal(err)
	}
	h := &SocialPostHandler{queries: db.New(pool)}
	firstErr := h.ClaimAndEnqueueScheduledPost(ctx, "post_scheduled_invalid_metadata")
	if firstErr == nil || !strings.Contains(firstErr.Error(), "decode post metadata") {
		t.Fatalf("first invalid metadata outcome = %v, want terminal decode error", firstErr)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id = 'post_scheduled_invalid_metadata'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("invalid metadata post status = %q, want failed", status)
	}
	if err := h.ClaimAndEnqueueScheduledPost(ctx, "post_scheduled_invalid_metadata"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("second invalid metadata claim = %v, want pgx.ErrNoRows", err)
	}
}

type restrictedResultSnapshot struct {
	Status                string
	ExternalID            pgtype.Text
	ErrorMessage          pgtype.Text
	PublishedAt           pgtype.Timestamptz
	URL                   pgtype.Text
	DebugCurl             pgtype.Text
	PublishToken          pgtype.Text
	ErrorCode             pgtype.Text
	FailureStage          pgtype.Text
	PlatformErrorCode     pgtype.Text
	IsRetriable           pgtype.Bool
	NextAction            pgtype.Text
	ErrorSource           pgtype.Text
	ErrorTemporality      pgtype.Text
	ProviderError         []byte
	XCreditsCounted       int64
	XCreditOperation      pgtype.Text
	XCreditCatalogVersion pgtype.Text
	XCreditBillingMode    pgtype.Text
}

func loadRestrictedResultSnapshot(t *testing.T, ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, resultID string) restrictedResultSnapshot {
	t.Helper()
	var result restrictedResultSnapshot
	err := querier.QueryRow(ctx, `
		SELECT status, external_id, error_message, published_at, url, debug_curl,
		       publish_token, error_code, failure_stage, platform_error_code,
		       is_retriable, next_action, error_source, error_temporality,
		       provider_error, x_credits_counted, x_credit_operation,
		       x_credit_catalog_version, x_credit_billing_mode
		FROM social_post_results
		WHERE id = $1
	`, resultID).Scan(
		&result.Status, &result.ExternalID, &result.ErrorMessage, &result.PublishedAt,
		&result.URL, &result.DebugCurl, &result.PublishToken, &result.ErrorCode,
		&result.FailureStage, &result.PlatformErrorCode, &result.IsRetriable,
		&result.NextAction, &result.ErrorSource, &result.ErrorTemporality,
		&result.ProviderError, &result.XCreditsCounted, &result.XCreditOperation,
		&result.XCreditCatalogVersion, &result.XCreditBillingMode,
	)
	if err != nil {
		t.Fatalf("load social post result %s: %v", resultID, err)
	}
	return result
}

func restrictedFinalizeIntegrationParams(jobID, owner string, attemptedAt time.Time) db.FinalizeRestrictedPostDeliveryJobParams {
	return db.FinalizeRestrictedPostDeliveryJobParams{
		FailureStage:     postfailures.ToText(publishingrestrictions.FailureStage),
		ErrorCode:        postfailures.ToText(publishingrestrictions.NormalizedCode),
		ErrorMessage:     publishingrestrictions.UserMessage,
		ID:               jobID,
		LeaseOwner:       postfailures.ToText(owner),
		LastAttemptAt:    pgtype.Timestamptz{Time: attemptedAt, Valid: true},
		NextAction:       postfailures.ToText(publishingrestrictions.NextAction),
		ErrorSource:      postfailures.ToText(postfailures.ErrorSourceUnipost),
		ErrorTemporality: postfailures.ToText(postfailures.ErrorTemporalityTemporary),
		CleanupAfterAt:   pgtype.Timestamptz{Time: attemptedAt.Add(60 * 24 * time.Hour), Valid: true},
	}
}

func insertRetryAccountAvailabilityFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string) db.CreateRetryPostDeliveryJobWithMediaActivationParams {
	t.Helper()
	accountID := "account_retry_availability_" + suffix
	resultID := "result_retry_availability_" + suffix
	postID := "post_retry_availability_" + suffix
	mediaID := "media_retry_availability_" + suffix
	workspaceID := "workspace_retry_availability_" + suffix
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_accounts (id, platform, status)
		VALUES ($1, 'tiktok', 'active')
	`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id, post_id, social_account_id, status)
		VALUES ($1, $2, $3, 'failed')
	`, resultID, postID, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES ($1, $2, 'uploaded')
	`, mediaID, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO platform_publishing_restrictions (platform, enabled, restricted_plan_ids)
		VALUES ('tiktok', FALSE, ARRAY[]::text[])
		ON CONFLICT (platform) DO NOTHING
	`); err != nil {
		t.Fatal(err)
	}
	return db.CreateRetryPostDeliveryJobWithMediaActivationParams{
		PostID:             postID,
		SocialPostResultID: resultID,
		WorkspaceID:        workspaceID,
		SocialAccountID:    accountID,
		Platform:           "tiktok",
		PostInputIndex:     0,
		MaxAttempts:        5,
		NextRunAt:          pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
		MediaIds:           []string{mediaID},
	}
}

func waitForPostgresSessionBlockedBy(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	blockedBackend int,
	blockingBackend int,
) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT $1::int = ANY(pg_blocking_pids($2::int))`, blockingBackend, blockedBackend).Scan(&blocked)
		if err != nil {
			t.Fatalf("inspect PostgreSQL lock graph: %v", err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PostgreSQL backend %d did not block behind backend %d: %v", blockedBackend, blockingBackend, ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitForPostgresSessionBlocked(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	blockedBackend int,
) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT CARDINALITY(pg_blocking_pids($1::int)) > 0`, blockedBackend).Scan(&blocked)
		if err != nil {
			t.Fatalf("inspect PostgreSQL lock graph: %v", err)
		}
		if blocked {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("PostgreSQL backend %d did not block: %v", blockedBackend, ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestCreateRetryPostDeliveryJobRejectsUnavailableAccountAtomically(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)

	tests := []struct {
		name       string
		status     string
		disconnect bool
	}{
		{name: "disconnected timestamp", status: "active", disconnect: true},
		{name: "reconnect required status", status: "reconnect_required"},
		{name: "disconnected status", status: "disconnected"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			params := insertRetryAccountAvailabilityFixture(t, ctx, pool, fmt.Sprintf("direct_%d", index))
			var disconnectedAt any
			if test.disconnect {
				disconnectedAt = time.Now().UTC()
			}
			if _, err := pool.Exec(ctx, `
				UPDATE social_accounts SET status = $2, disconnected_at = $3 WHERE id = $1
			`, params.SocialAccountID, test.status, disconnectedAt); err != nil {
				t.Fatal(err)
			}

			_, err := db.New(pool).CreateRetryPostDeliveryJobWithMediaActivation(ctx, params)
			if !errors.Is(err, pgx.ErrNoRows) {
				t.Fatalf("retry enqueue error = %v, want pgx.ErrNoRows", err)
			}
			var jobs int
			if err := pool.QueryRow(ctx, `
				SELECT COUNT(*)::int FROM post_delivery_jobs WHERE social_post_result_id = $1
			`, params.SocialPostResultID).Scan(&jobs); err != nil {
				t.Fatal(err)
			}
			if jobs != 0 {
				t.Fatalf("retry jobs = %d, want 0 for unavailable account", jobs)
			}
		})
	}
}

func TestCreateRetryPostDeliveryJobSerializesWithAccountDisconnect(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)

	t.Run("disconnect commits first and retry is rejected", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		params := insertRetryAccountAvailabilityFixture(t, ctx, pool, "disconnect_first")
		disconnectConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer disconnectConn.Release()
		retryConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryConn.Release()
		var disconnectBackend, retryBackend int
		if err := disconnectConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&disconnectBackend); err != nil {
			t.Fatal(err)
		}
		if err := retryConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&retryBackend); err != nil {
			t.Fatal(err)
		}
		disconnectTx, err := disconnectConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer disconnectTx.Rollback(ctx)
		if _, err := disconnectTx.Exec(ctx, `
			UPDATE social_accounts
			SET status = 'disconnected', disconnected_at = NOW()
			WHERE id = $1
		`, params.SocialAccountID); err != nil {
			t.Fatal(err)
		}
		retryTx, err := retryConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryTx.Rollback(ctx)
		retried := make(chan error, 1)
		go func() {
			_, retryErr := db.New(retryTx).CreateRetryPostDeliveryJobWithMediaActivation(ctx, params)
			retried <- retryErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, retryBackend, disconnectBackend)
		if err := disconnectTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case retryErr := <-retried:
			if !errors.Is(retryErr, pgx.ErrNoRows) {
				t.Fatalf("retry after disconnect commit error = %v, want pgx.ErrNoRows", retryErr)
			}
		case <-ctx.Done():
			t.Fatalf("retry did not resume after disconnect commit: %v", ctx.Err())
		}
	})

	t.Run("retry commits first and disconnect waits", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		params := insertRetryAccountAvailabilityFixture(t, ctx, pool, "retry_first")
		retryConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryConn.Release()
		disconnectConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer disconnectConn.Release()
		var retryBackend, disconnectBackend int
		if err := retryConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&retryBackend); err != nil {
			t.Fatal(err)
		}
		if err := disconnectConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&disconnectBackend); err != nil {
			t.Fatal(err)
		}
		retryTx, err := retryConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryTx.Rollback(ctx)
		job, err := db.New(retryTx).CreateRetryPostDeliveryJobWithMediaActivation(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		disconnectTx, err := disconnectConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer disconnectTx.Rollback(ctx)
		disconnected := make(chan error, 1)
		go func() {
			_, disconnectErr := disconnectTx.Exec(ctx, `
				UPDATE social_accounts
				SET status = 'disconnected', disconnected_at = NOW()
				WHERE id = $1
			`, params.SocialAccountID)
			disconnected <- disconnectErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, disconnectBackend, retryBackend)
		if err := retryTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case disconnectErr := <-disconnected:
			if disconnectErr != nil {
				t.Fatal(disconnectErr)
			}
		case <-ctx.Done():
			t.Fatalf("disconnect did not resume after retry commit: %v", ctx.Err())
		}
		if err := disconnectTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var state string
		if err := pool.QueryRow(ctx, `SELECT state FROM post_delivery_jobs WHERE id = $1`, job.ID).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "pending" {
			t.Fatalf("linearized retry state = %q, want pending", state)
		}
	})
}

func insertSharedResultDeliveryRaceFixture(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	suffix string,
	attemptedAt time.Time,
) (resultID, postID, restrictedJobID, successJobID, mediaID string) {
	t.Helper()
	resultID = "result_shared_" + suffix
	postID = "post_shared_" + suffix
	restrictedJobID = "job_restricted_" + suffix
	successJobID = "job_success_" + suffix
	mediaID = "media_shared_" + suffix
	_, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, published_at, workspace_id)
		VALUES ($1, 'publishing', NULL, 'workspace_shared')
	`, postID)
	if err != nil {
		t.Fatalf("insert shared parent race fixture %s: %v", suffix, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO social_post_results (
			id, post_id, social_account_id, status, external_id, error_message,
			published_at, url, debug_curl, publish_token, error_code, failure_stage,
			platform_error_code, is_retriable, next_action, error_source,
			error_temporality, provider_error, x_credits_counted,
			x_credit_operation, x_credit_catalog_version, x_credit_billing_mode
		) VALUES (
			$1, $2, 'account_shared', 'processing', 'stale_external', 'stale_error',
			$3, 'https://social.example.com/stale', 'stale_debug', 'stale_publish_token',
			'stale_error_code', 'stale_stage', 'stale_platform_code', TRUE,
			'stale_action', 'provider', 'temporary', '{"stale":true}'::jsonb, 19,
			'stale_operation', 'stale_catalog', 'stale_billing'
		)
	`, resultID, postID, attemptedAt)
	if err != nil {
		t.Fatalf("insert shared result race fixture %s: %v", suffix, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO post_delivery_jobs (
			id, post_id, social_post_result_id, workspace_id, social_account_id,
			platform, kind, state, attempts, lease_owner, last_attempt_at
		) VALUES
			($3, $2, $1, 'workspace_shared', 'account_shared', 'tiktok',
			 'dispatch', 'running', 1, 'owner_restricted', $5),
			($4, $2, $1, 'workspace_shared', 'account_shared', 'tiktok',
			 'retry', 'running', 1, 'owner_success', $5)
	`, resultID, postID, restrictedJobID, successJobID, attemptedAt)
	if err != nil {
		t.Fatalf("insert shared delivery jobs race fixture %s: %v", suffix, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO media (id, workspace_id, status)
		VALUES ($1, 'workspace_shared', 'uploaded')
	`, mediaID)
	if err != nil {
		t.Fatalf("insert shared media race fixture %s: %v", suffix, err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO media_post_usages (
			workspace_id, media_id, post_id, post_status, cleanup_after_at, retention_reason
		) VALUES (
			'workspace_shared', $1, $2, 'publishing', NULL, 'active_post'
		)
	`, mediaID, postID)
	if err != nil {
		t.Fatalf("insert shared media usage race fixture %s: %v", suffix, err)
	}
	return resultID, postID, restrictedJobID, successJobID, mediaID
}

func publishSharedResultInTransaction(
	ctx context.Context,
	tx pgx.Tx,
	resultID string,
	successJobID string,
	mediaID string,
	postID string,
	publishedAt time.Time,
) error {
	_, err := tx.Exec(ctx, `
		UPDATE social_post_results
		SET status = 'published', external_id = 'winning_external_id',
			error_message = NULL, published_at = $2,
			url = 'https://social.example.com/winning_external_id', debug_curl = NULL,
			publish_token = 'winning_publish_token', error_code = NULL,
			failure_stage = NULL, platform_error_code = NULL, is_retriable = NULL,
			next_action = NULL, error_source = NULL, error_temporality = NULL,
			provider_error = '{"provider":"success"}'::jsonb,
			x_credits_counted = 41, x_credit_operation = 'winning_operation',
			x_credit_catalog_version = 'winning_catalog',
			x_credit_billing_mode = 'winning_billing'
		WHERE id = $1
	`, resultID, publishedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE post_delivery_jobs
		SET state = 'succeeded', failure_stage = NULL, error_code = NULL,
			platform_error_code = NULL, last_error = NULL, next_run_at = NULL,
			finished_at = $2, updated_at = $2
		WHERE id = $1
	`, successJobID, publishedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE social_posts
		SET status = 'published', published_at = $2
		WHERE id = $1
	`, postID, publishedAt)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE media
		SET usage_version = usage_version + 1
		WHERE id = $1
	`, mediaID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE media_post_usages
		SET post_status = 'published', cleanup_after_at = $3::timestamptz + INTERVAL '30 days',
			retention_reason = 'plan_status', updated_at = $3
		WHERE media_id = $1 AND post_id = $2
	`, mediaID, postID, publishedAt)
	return err
}

func assertSharedResultPublishedWinner(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	resultID string,
	restrictedJobID string,
	successJobID string,
	mediaID string,
	postID string,
	wantRestrictedJobState string,
	publishedAt time.Time,
) {
	t.Helper()
	result := loadRestrictedResultSnapshot(t, ctx, pool, resultID)
	if result.Status != "published" || result.ExternalID.String != "winning_external_id" ||
		!result.PublishedAt.Valid || !result.PublishedAt.Time.Equal(publishedAt) ||
		result.URL.String != "https://social.example.com/winning_external_id" ||
		result.PublishToken.String != "winning_publish_token" || result.ErrorMessage.Valid ||
		result.ErrorCode.Valid || result.FailureStage.Valid || result.PlatformErrorCode.Valid ||
		result.IsRetriable.Valid || result.NextAction.Valid || result.ErrorSource.Valid ||
		result.ErrorTemporality.Valid || result.XCreditsCounted != 41 ||
		result.XCreditOperation.String != "winning_operation" ||
		result.XCreditCatalogVersion.String != "winning_catalog" ||
		result.XCreditBillingMode.String != "winning_billing" {
		t.Fatalf("shared result terminal published state changed: %+v", result)
	}
	if string(result.ProviderError) != `{"provider": "success"}` {
		t.Fatalf("shared result provider response = %s, want published response", result.ProviderError)
	}

	var restrictedState, successState string
	var restrictedFailureStage, restrictedErrorCode, restrictedLastError pgtype.Text
	if err := pool.QueryRow(ctx, `
		SELECT restricted.state, restricted.failure_stage, restricted.error_code,
		       restricted.last_error, success.state
		FROM post_delivery_jobs restricted
		JOIN post_delivery_jobs success ON success.id = $2
		WHERE restricted.id = $1
	`, restrictedJobID, successJobID).Scan(
		&restrictedState, &restrictedFailureStage, &restrictedErrorCode,
		&restrictedLastError, &successState,
	); err != nil {
		t.Fatal(err)
	}
	if restrictedState != wantRestrictedJobState || successState != "succeeded" {
		t.Fatalf("shared-result job states = restricted:%q success:%q, want %q/succeeded", restrictedState, successState, wantRestrictedJobState)
	}
	if wantRestrictedJobState == "succeeded" && (restrictedFailureStage.Valid || restrictedErrorCode.Valid || restrictedLastError.Valid) {
		t.Fatalf("converged restricted job retained failure diagnostics: stage=%#v code=%#v error=%#v", restrictedFailureStage, restrictedErrorCode, restrictedLastError)
	}

	var usageStatus, retentionReason string
	var cleanupAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `
		SELECT post_status, retention_reason, cleanup_after_at
		FROM media_post_usages
		WHERE media_id = $1 AND post_id = $2
	`, mediaID, postID).Scan(&usageStatus, &retentionReason, &cleanupAt); err != nil {
		t.Fatal(err)
	}
	if usageStatus != "published" || retentionReason != "plan_status" || !cleanupAt.Valid || !cleanupAt.Time.Equal(publishedAt.Add(30*24*time.Hour)) {
		t.Fatalf("shared-result media retention = status:%q reason:%q cleanup:%#v, want published/plan_status/%s", usageStatus, retentionReason, cleanupAt, publishedAt.Add(30*24*time.Hour))
	}
	var parentStatus string
	var parentPublishedAt pgtype.Timestamptz
	if err := pool.QueryRow(ctx, `
		SELECT status, published_at FROM social_posts WHERE id = $1
	`, postID).Scan(&parentStatus, &parentPublishedAt); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "published" || !parentPublishedAt.Valid || !parentPublishedAt.Time.Equal(publishedAt) {
		t.Fatalf("shared-result parent = status:%q published_at:%#v, want published/%s", parentStatus, parentPublishedAt, publishedAt)
	}
}

func TestFinalizeRestrictedPostDeliveryJobPostgresLeaseAtomicity(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)

	t.Run("published shared result commits before restriction finalizer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 27, 2, 0, 0, 0, time.UTC)
		publishedAt := attemptedAt.Add(time.Minute)
		resultID, postID, restrictedJobID, successJobID, mediaID := insertSharedResultDeliveryRaceFixture(
			t, ctx, pool, "published_first", attemptedAt,
		)

		restrictedConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer restrictedConn.Release()
		successConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer successConn.Release()
		var restrictedBackend, successBackend int
		if err := restrictedConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&restrictedBackend); err != nil {
			t.Fatal(err)
		}
		if err := successConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&successBackend); err != nil {
			t.Fatal(err)
		}
		if restrictedBackend == successBackend {
			t.Fatalf("shared-result race requires two PostgreSQL sessions, both used backend %d", restrictedBackend)
		}

		successTx, err := successConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer successTx.Rollback(ctx)
		if err := publishSharedResultInTransaction(ctx, successTx, resultID, successJobID, mediaID, postID, publishedAt); err != nil {
			t.Fatalf("stage published winner: %v", err)
		}

		restrictedTx, err := restrictedConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer restrictedTx.Rollback(ctx)
		type finalizeResult struct {
			job db.FinalizeRestrictedPostDeliveryJobRow
			err error
		}
		finalized := make(chan finalizeResult, 1)
		go func() {
			params := restrictedFinalizeIntegrationParams(restrictedJobID, "owner_restricted", attemptedAt)
			params.MediaIds = []string{mediaID}
			job, finalizeErr := db.New(restrictedTx).FinalizeRestrictedPostDeliveryJob(ctx, params)
			finalized <- finalizeResult{job: job, err: finalizeErr}
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, restrictedBackend, successBackend)
		if err := successTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		var result finalizeResult
		select {
		case result = <-finalized:
		case <-ctx.Done():
			t.Fatalf("restriction finalizer did not resume after published commit: %v", ctx.Err())
		}
		if result.err != nil {
			t.Fatalf("restriction finalizer after published commit: %v", result.err)
		}
		if result.job.State != "succeeded" {
			t.Fatalf("restriction finalizer convergence state = %q, want succeeded", result.job.State)
		}
		if err := restrictedTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		assertSharedResultPublishedWinner(
			t, ctx, pool, resultID, restrictedJobID, successJobID, mediaID, postID, "succeeded", publishedAt,
		)
	})

	t.Run("restriction finalizer commits before published shared result", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 27, 2, 30, 0, 0, time.UTC)
		publishedAt := attemptedAt.Add(time.Minute)
		resultID, postID, restrictedJobID, successJobID, mediaID := insertSharedResultDeliveryRaceFixture(
			t, ctx, pool, "restricted_first", attemptedAt,
		)

		restrictedConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer restrictedConn.Release()
		successConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer successConn.Release()
		var restrictedBackend, successBackend int
		if err := restrictedConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&restrictedBackend); err != nil {
			t.Fatal(err)
		}
		if err := successConn.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&successBackend); err != nil {
			t.Fatal(err)
		}
		if restrictedBackend == successBackend {
			t.Fatalf("shared-result race requires two PostgreSQL sessions, both used backend %d", restrictedBackend)
		}

		restrictedTx, err := restrictedConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer restrictedTx.Rollback(ctx)
		params := restrictedFinalizeIntegrationParams(restrictedJobID, "owner_restricted", attemptedAt)
		params.MediaIds = []string{mediaID}
		finalizedJob, err := db.New(restrictedTx).FinalizeRestrictedPostDeliveryJob(ctx, params)
		if err != nil {
			t.Fatalf("stage restriction finalization: %v", err)
		}
		if finalizedJob.State != "dead" {
			t.Fatalf("first restriction state = %q, want dead", finalizedJob.State)
		}

		successTx, err := successConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer successTx.Rollback(ctx)
		published := make(chan error, 1)
		go func() {
			published <- publishSharedResultInTransaction(ctx, successTx, resultID, successJobID, mediaID, postID, publishedAt)
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, successBackend, restrictedBackend)
		if err := restrictedTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		select {
		case err := <-published:
			if err != nil {
				t.Fatalf("published winner after restriction commit: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("published winner did not resume after restriction commit: %v", ctx.Err())
		}
		if err := successTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		assertSharedResultPublishedWinner(
			t, ctx, pool, resultID, restrictedJobID, successJobID, mediaID, postID, "dead", publishedAt,
		)
	})

	t.Run("invalid metadata preserves job result billing and existing usage", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 26, 18, 30, 0, 0, time.UTC)
		_, err := pool.Exec(ctx, `
			INSERT INTO social_post_results (
				id, post_id, social_account_id, status, x_credits_counted,
				x_credit_operation, x_credit_catalog_version, x_credit_billing_mode
			) VALUES (
				'result_invalid_metadata', 'post_invalid_metadata', 'account_invalid_metadata',
				'processing', 29, 'existing_operation', 'existing_catalog', 'existing_billing'
			)
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_invalid_metadata', 'post_invalid_metadata', 'result_invalid_metadata',
				'workspace_invalid_metadata', 'account_invalid_metadata', 'tiktok',
				'dispatch', 'running', 1, 'owner_invalid_metadata', $1
			)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO media (id, workspace_id, status)
			VALUES ('media_invalid_metadata', 'workspace_invalid_metadata', 'uploaded');
			INSERT INTO media_post_usages (
				workspace_id, media_id, post_id, post_status, cleanup_after_at, retention_reason
			) VALUES (
				'workspace_invalid_metadata', 'media_invalid_metadata', 'post_invalid_metadata',
				'publishing', NULL, 'active_post'
			)
		`)
		if err != nil {
			t.Fatal(err)
		}

		queries := db.New(pool)
		h := NewSocialPostHandler(queries, nil, nil, nil, nil, nil, nil)
		job := db.PostDeliveryJob{
			ID:                 "job_invalid_metadata",
			PostID:             "post_invalid_metadata",
			SocialPostResultID: "result_invalid_metadata",
			WorkspaceID:        "workspace_invalid_metadata",
			SocialAccountID:    "account_invalid_metadata",
			Platform:           "tiktok",
			State:              "running",
			LeaseOwner:         pgtype.Text{String: "owner_invalid_metadata", Valid: true},
			LastAttemptAt:      pgtype.Timestamptz{Time: attemptedAt, Valid: true},
		}
		result := db.SocialPostResult{
			ID:                    "result_invalid_metadata",
			PostID:                "post_invalid_metadata",
			SocialAccountID:       "account_invalid_metadata",
			Status:                "processing",
			XCreditsCounted:       29,
			XCreditOperation:      pgtype.Text{String: "existing_operation", Valid: true},
			XCreditCatalogVersion: pgtype.Text{String: "existing_catalog", Valid: true},
			XCreditBillingMode:    pgtype.Text{String: "existing_billing", Valid: true},
		}
		post := db.SocialPost{
			ID:          "post_invalid_metadata",
			WorkspaceID: "workspace_invalid_metadata",
			Status:      "publishing",
			Metadata:    []byte(`{"schema_version":2,"platform_posts":[`),
		}
		err = h.finalizeRestrictedDeliveryJob(ctx, job, result, post, publishingrestrictions.Decision{
			Restricted: true,
			Platform:   "tiktok",
			CycleID:    "cycle_invalid_metadata_pg",
		})
		if !errors.Is(err, errInvalidPostMediaMetadata) {
			t.Fatalf("finalizeRestrictedDeliveryJob error = %v, want media metadata error", err)
		}

		var jobState, resultStatus string
		var credits int64
		var operation, catalog, billing pgtype.Text
		if err := pool.QueryRow(ctx, `SELECT state FROM post_delivery_jobs WHERE id = 'job_invalid_metadata'`).Scan(&jobState); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `
			SELECT status, x_credits_counted, x_credit_operation,
			       x_credit_catalog_version, x_credit_billing_mode
			FROM social_post_results WHERE id = 'result_invalid_metadata'
		`).Scan(&resultStatus, &credits, &operation, &catalog, &billing); err != nil {
			t.Fatal(err)
		}
		if jobState != "running" || resultStatus != "processing" || credits != 29 || operation.String != "existing_operation" || catalog.String != "existing_catalog" || billing.String != "existing_billing" {
			t.Fatalf("durable state changed = job:%q result:%q credits:%d operation:%#v catalog:%#v billing:%#v", jobState, resultStatus, credits, operation, catalog, billing)
		}
		var usageCount int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM media_post_usages
			WHERE media_id = 'media_invalid_metadata'
			  AND post_id = 'post_invalid_metadata'
			  AND retention_reason = 'active_post'
			  AND cleanup_after_at IS NULL
		`).Scan(&usageCount); err != nil {
			t.Fatal(err)
		}
		if usageCount != 1 {
			t.Fatalf("preserved active media usages = %d, want 1", usageCount)
		}
	})

	t.Run("stale owner preserves newer completed job and published result", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ownerA, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerA.Release()
		ownerB, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerB.Release()
		var backendA, backendB int
		if err := ownerA.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backendA); err != nil {
			t.Fatal(err)
		}
		if err := ownerB.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backendB); err != nil {
			t.Fatal(err)
		}
		if backendA == backendB {
			t.Fatalf("integration fixture requires two PostgreSQL sessions, both used backend %d", backendA)
		}

		attemptA := time.Date(2026, 7, 26, 19, 0, 0, 0, time.UTC)
		attemptB := attemptA.Add(time.Minute)
		completedB := attemptB.Add(time.Minute)
		publishedAt := completedB.Add(-10 * time.Second)
		_, err = ownerA.Exec(ctx, `
			INSERT INTO social_posts (id, status, published_at)
			VALUES ('post_stale_owner', 'published', $1)
		`, publishedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ownerA.Exec(ctx, `
			INSERT INTO social_post_results (id, post_id, social_account_id, status)
			VALUES ('result_stale_owner', 'post_stale_owner', 'account_1', 'pending')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ownerA.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_stale_owner', 'post_stale_owner', 'result_stale_owner', 'workspace_1',
				'account_1', 'tiktok', 'dispatch', 'running', 1, 'owner_a', $1
			)
		`, attemptA)
		if err != nil {
			t.Fatal(err)
		}
		txB, err := ownerB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txB.Rollback(ctx)
		_, err = txB.Exec(ctx, `
			UPDATE post_delivery_jobs
			SET state = 'succeeded', lease_owner = 'owner_b', last_attempt_at = $2,
			    finished_at = $3, updated_at = $3
			WHERE id = 'job_stale_owner' AND lease_owner = 'owner_a' AND last_attempt_at = $1
		`, attemptA, attemptB, completedB)
		if err != nil {
			t.Fatal(err)
		}
		_, err = txB.Exec(ctx, `
			UPDATE social_post_results
			SET status = 'published', external_id = 'newer_external_id',
			    error_message = NULL, published_at = $1,
			    url = 'https://social.example.com/newer_external_id',
			    debug_curl = 'newer_debug', publish_token = 'newer_publish_token',
			    error_code = NULL, failure_stage = NULL, platform_error_code = NULL,
			    is_retriable = NULL, next_action = NULL, error_source = NULL,
			    error_temporality = NULL, provider_error = NULL,
			    x_credits_counted = 23, x_credit_operation = 'newer_operation',
			    x_credit_catalog_version = 'newer_catalog', x_credit_billing_mode = 'newer_billing'
			WHERE id = 'result_stale_owner'
		`, publishedAt)
		if err != nil {
			t.Fatal(err)
		}
		if err := txB.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		before := loadRestrictedResultSnapshot(t, ctx, ownerB, "result_stale_owner")
		queriesA := db.New(ownerA)
		_, err = queriesA.FinalizeRestrictedPostDeliveryJob(ctx, restrictedFinalizeIntegrationParams("job_stale_owner", "owner_a", attemptA))
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatalf("stale finalization error = %v, want pgx.ErrNoRows", err)
		}
		after := loadRestrictedResultSnapshot(t, ctx, ownerB, "result_stale_owner")
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("newer published result changed after stale finalization:\n before=%+v\n after=%+v", before, after)
		}

		var state string
		var owner pgtype.Text
		var attemptedAt, finishedAt pgtype.Timestamptz
		if err := ownerB.QueryRow(ctx, `
			SELECT state, lease_owner, last_attempt_at, finished_at
			FROM post_delivery_jobs WHERE id = 'job_stale_owner'
		`).Scan(&state, &owner, &attemptedAt, &finishedAt); err != nil {
			t.Fatal(err)
		}
		if state != "succeeded" || owner.String != "owner_b" || !attemptedAt.Time.Equal(attemptB) || !finishedAt.Time.Equal(completedB) {
			t.Fatalf("newer job changed after stale finalization: state=%q owner=%#v attempted=%#v finished=%#v", state, owner, attemptedAt, finishedAt)
		}
		var parentStatus string
		var parentPublishedAt pgtype.Timestamptz
		if err := ownerB.QueryRow(ctx, `
			SELECT status, published_at FROM social_posts WHERE id = 'post_stale_owner'
		`).Scan(&parentStatus, &parentPublishedAt); err != nil {
			t.Fatal(err)
		}
		if parentStatus != "published" || !parentPublishedAt.Valid || !parentPublishedAt.Time.Equal(publishedAt) {
			t.Fatalf("parent changed after stale finalization: status=%q published_at=%#v", parentStatus, parentPublishedAt)
		}
	})

	t.Run("owned lease transitions job and result atomically", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 26, 21, 0, 0, 0, time.UTC)
		_, err := pool.Exec(ctx, `
			INSERT INTO social_posts (id, status, published_at)
			VALUES ('post_owned', 'publishing', $1)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO social_post_results (
				id, post_id, social_account_id, status, external_id, published_at, url, debug_curl, publish_token,
				platform_error_code, provider_error, x_credits_counted,
				x_credit_operation, x_credit_catalog_version, x_credit_billing_mode
			) VALUES (
				'result_owned', 'post_owned', 'account_1', 'processing', 'stale_external', $1,
				'https://social.example.com/stale', 'stale_debug', 'stale_publish_token',
				'stale_provider_code', '{"stale":true}'::jsonb, 31,
				'stale_operation', 'stale_catalog', 'stale_billing'
			)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_owned', 'post_owned', 'result_owned', 'workspace_1', 'account_1',
				'tiktok', 'dispatch', 'running', 1, 'owner_current', $1
			)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}

		transitioned, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, restrictedFinalizeIntegrationParams("job_owned", "owner_current", attemptedAt))
		if err != nil {
			t.Fatalf("owned finalization: %v", err)
		}
		if transitioned.State != "dead" || !transitioned.FinishedAt.Valid {
			t.Fatalf("transitioned job state=%q finished_at=%#v, want dead with timestamp", transitioned.State, transitioned.FinishedAt)
		}
		result := loadRestrictedResultSnapshot(t, ctx, pool, "result_owned")
		if result.Status != "failed" || result.ExternalID.Valid || result.PublishedAt.Valid || result.URL.Valid || result.DebugCurl.Valid || result.PublishToken.Valid {
			t.Fatalf("owned restricted result core fields = %+v", result)
		}
		if result.ErrorMessage.String != publishingrestrictions.UserMessage ||
			result.ErrorCode.String != publishingrestrictions.NormalizedCode ||
			result.FailureStage.String != publishingrestrictions.FailureStage ||
			!result.IsRetriable.Valid || result.IsRetriable.Bool ||
			result.NextAction.String != publishingrestrictions.NextAction ||
			result.ErrorSource.String != postfailures.ErrorSourceUnipost ||
			result.ErrorTemporality.String != postfailures.ErrorTemporalityTemporary ||
			result.PlatformErrorCode.Valid || result.ProviderError != nil {
			t.Fatalf("owned restricted result diagnostics = %+v", result)
		}
		if result.XCreditsCounted != 0 || result.XCreditOperation.Valid || result.XCreditCatalogVersion.Valid || result.XCreditBillingMode.Valid {
			t.Fatalf("owned restricted X billing metadata = count:%d operation:%#v catalog:%#v billing:%#v, want cleared", result.XCreditsCounted, result.XCreditOperation, result.XCreditCatalogVersion, result.XCreditBillingMode)
		}
		var parentStatus string
		var parentPublishedAt pgtype.Timestamptz
		if err := pool.QueryRow(ctx, `
			SELECT status, published_at FROM social_posts WHERE id = 'post_owned'
		`).Scan(&parentStatus, &parentPublishedAt); err != nil {
			t.Fatal(err)
		}
		if parentStatus != "failed" || parentPublishedAt.Valid {
			t.Fatalf("owned restricted parent = status:%q published_at:%#v, want failed/null", parentStatus, parentPublishedAt)
		}
	})

	t.Run("atomic restriction retention removes obsolete usage and derives final status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 26, 21, 30, 0, 0, time.UTC)
		cleanupAt := attemptedAt.Add(60 * 24 * time.Hour)
		_, err := pool.Exec(ctx, `
			INSERT INTO social_post_results (id, post_id, social_account_id, status)
			VALUES ('result_stale_usage', 'post_stale_usage', 'account_stale_usage', 'processing')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_stale_usage', 'post_stale_usage', 'result_stale_usage', 'workspace_1',
				'account_stale_usage', 'tiktok', 'dispatch', 'running', 1, 'owner_stale_usage', $1
			)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO media (id, workspace_id, status)
			VALUES
				('media_current_usage', 'workspace_1', 'uploaded'),
				('media_obsolete_usage', 'workspace_1', 'uploaded'),
				('media_obsolete_later_usage', 'workspace_1', 'uploaded')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = pool.Exec(ctx, `
			INSERT INTO media_post_usages (
				workspace_id, media_id, post_id, post_status, cleanup_after_at, retention_reason
			) VALUES
				('workspace_1', 'media_current_usage', 'post_stale_usage', 'publishing', NULL, 'active_post'),
				('workspace_1', 'media_obsolete_usage', 'post_stale_usage', 'publishing', NULL, 'active_post'),
				('workspace_1', 'media_obsolete_later_usage', 'post_stale_usage', 'published', $1, 'plan_status')
		`, attemptedAt.Add(365*24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}

		params := restrictedFinalizeIntegrationParams("job_stale_usage", "owner_stale_usage", attemptedAt)
		params.MediaIds = []string{"media_current_usage"}
		params.CleanupAfterAt = pgtype.Timestamptz{Time: cleanupAt, Valid: true}
		if _, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatalf("restriction finalization: %v", err)
		}

		var currentStatus, currentReason string
		var currentCleanup pgtype.Timestamptz
		if err := pool.QueryRow(ctx, `
			SELECT post_status, retention_reason, cleanup_after_at
			FROM media_post_usages
			WHERE media_id = 'media_current_usage' AND post_id = 'post_stale_usage'
		`).Scan(&currentStatus, &currentReason, &currentCleanup); err != nil {
			t.Fatal(err)
		}
		if currentStatus != "failed" || currentReason != "publishing_restriction" || !currentCleanup.Valid || !currentCleanup.Time.Equal(cleanupAt) {
			t.Fatalf("current restriction usage = status:%q reason:%q cleanup:%#v, want failed publishing_restriction %s", currentStatus, currentReason, currentCleanup, cleanupAt)
		}
		var obsoleteCount int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM media_post_usages
			WHERE media_id = ANY($1::text[]) AND post_id = 'post_stale_usage'
		`, []string{"media_obsolete_usage", "media_obsolete_later_usage"}).Scan(&obsoleteCount); err != nil {
			t.Fatal(err)
		}
		if obsoleteCount != 0 {
			t.Fatalf("obsolete media usages = %d, want 0", obsoleteCount)
		}
	})

	t.Run("atomic restriction retention derives partial parent status", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 26, 21, 45, 0, 0, time.UTC)
		publishedAt := attemptedAt.Add(-37 * time.Minute)
		_, err := pool.Exec(ctx, `
			INSERT INTO social_posts (id, status)
			VALUES ('post_partial_restricted', 'publishing');
			INSERT INTO social_post_results (id, post_id, social_account_id, status, published_at) VALUES
				('result_partial_restricted', 'post_partial_restricted', 'account_partial_restricted', 'processing', NULL),
				('result_partial_published', 'post_partial_restricted', 'account_partial_published', 'published', TIMESTAMPTZ '2026-07-26 21:08:00+00');
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_partial_restricted', 'post_partial_restricted', 'result_partial_restricted',
				'workspace_1', 'account_partial_restricted', 'tiktok', 'dispatch', 'running',
				1, 'owner_partial_restricted', TIMESTAMPTZ '2026-07-26 21:45:00+00'
			);
			INSERT INTO media (id, workspace_id, status)
			VALUES ('media_partial_restricted', 'workspace_1', 'uploaded')
		`)
		if err != nil {
			t.Fatal(err)
		}
		params := restrictedFinalizeIntegrationParams("job_partial_restricted", "owner_partial_restricted", attemptedAt)
		params.MediaIds = []string{"media_partial_restricted"}
		if _, err := db.New(pool).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatal(err)
		}
		var status, reason string
		if err := pool.QueryRow(ctx, `
			SELECT post_status, retention_reason FROM media_post_usages
			WHERE media_id = 'media_partial_restricted' AND post_id = 'post_partial_restricted'
		`).Scan(&status, &reason); err != nil {
			t.Fatal(err)
		}
		if status != "partial" || reason != "publishing_restriction" {
			t.Fatalf("partial restriction usage = status:%q reason:%q, want partial/publishing_restriction", status, reason)
		}
		var parentStatus string
		var parentPublishedAt pgtype.Timestamptz
		if err := pool.QueryRow(ctx, `
			SELECT status, published_at FROM social_posts WHERE id = 'post_partial_restricted'
		`).Scan(&parentStatus, &parentPublishedAt); err != nil {
			t.Fatal(err)
		}
		if parentStatus != "partial" || !parentPublishedAt.Valid || !parentPublishedAt.Time.Equal(publishedAt) {
			t.Fatalf("partial restricted parent = status:%q published_at:%#v, want partial/%s", parentStatus, parentPublishedAt, publishedAt)
		}
	})

	t.Run("newer retry plan retention wins after atomic restriction retention", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ownerA, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerA.Release()
		ownerB, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerB.Release()
		attemptedAt := time.Date(2026, 7, 26, 22, 0, 0, 0, time.UTC)
		restrictionDeadline := attemptedAt.Add(60 * 24 * time.Hour)
		planDeadline := attemptedAt.Add(30 * 24 * time.Hour)
		_, err = ownerA.Exec(ctx, `
			INSERT INTO social_post_results (id, post_id, social_account_id, status)
			VALUES ('result_retention_race', 'post_retention_race', 'account_1', 'processing')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ownerA.Exec(ctx, `
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_retention_race', 'post_retention_race', 'result_retention_race',
				'workspace_1', 'account_1', 'tiktok', 'dispatch', 'running', 1,
				'owner_a', $1
			)
		`, attemptedAt)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ownerA.Exec(ctx, `
			INSERT INTO media (id, workspace_id, status)
			VALUES
				('media_retention_race_a', 'workspace_1', 'uploaded'),
				('media_retention_race_b', 'workspace_1', 'uploaded')
		`)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ownerA.Exec(ctx, `
			INSERT INTO social_accounts (id, platform)
			VALUES ('account_1', 'tiktok');
			INSERT INTO platform_publishing_restrictions (platform, enabled, restricted_plan_ids)
			VALUES ('tiktok', FALSE, ARRAY[]::text[])
		`)
		if err != nil {
			t.Fatal(err)
		}

		txA, err := ownerA.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txA.Rollback(ctx)
		params := restrictedFinalizeIntegrationParams("job_retention_race", "owner_a", attemptedAt)
		params.MediaIds = []string{"media_retention_race_b", "media_retention_race_a"}
		params.CleanupAfterAt = pgtype.Timestamptz{Time: restrictionDeadline, Valid: true}
		if _, err := db.New(txA).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatalf("restriction finalization: %v", err)
		}
		if err := txA.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		txB, err := ownerB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txB.Rollback(ctx)
		retryJob, err := db.New(txB).CreateRetryPostDeliveryJobWithMediaActivation(ctx, db.CreateRetryPostDeliveryJobWithMediaActivationParams{
			PostID:             "post_retention_race",
			SocialPostResultID: "result_retention_race",
			WorkspaceID:        "workspace_1",
			SocialAccountID:    "account_1",
			Platform:           "tiktok",
			PostInputIndex:     0,
			MaxAttempts:        5,
			NextRunAt:          pgtype.Timestamptz{Time: attemptedAt.Add(time.Minute), Valid: true},
			MediaIds:           []string{"media_retention_race_a", "media_retention_race_b"},
		})
		if err != nil {
			t.Fatalf("create real retry job with media activation: %v", err)
		}
		if retryJob.State != "pending" || retryJob.Kind != "retry" {
			t.Fatalf("retry job state/kind = %q/%q, want pending/retry", retryJob.State, retryJob.Kind)
		}
		rows, err := txB.Query(ctx, `
				SELECT media.id, media.usage_version, usage.retention_reason,
				       usage.cleanup_after_at
				FROM media
				JOIN media_post_usages usage
				  ON usage.media_id = media.id
				 AND usage.post_id = 'post_retention_race'
				WHERE media.id = ANY($1::text[])
				ORDER BY media.id
		`, []string{"media_retention_race_a", "media_retention_race_b"})
		if err != nil {
			t.Fatal(err)
		}
		activatedCount := 0
		for rows.Next() {
			var mediaID, reason string
			var version int64
			var cleanup pgtype.Timestamptz
			if err := rows.Scan(&mediaID, &version, &reason, &cleanup); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if version != 2 || reason != "active_post" || cleanup.Valid {
				rows.Close()
				t.Fatalf("activated media %s = version:%d reason:%q cleanup:%#v, want 2/active_post/null", mediaID, version, reason, cleanup)
			}
			activatedCount++
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		rows.Close()
		if activatedCount != 2 {
			t.Fatalf("activated media rows = %d, want 2", activatedCount)
		}
		if _, err := txB.Exec(ctx, `
				UPDATE social_post_results
				SET status = 'published', external_id = 'newer_external_id',
				    error_message = NULL, published_at = $1,
				    error_code = NULL, failure_stage = NULL
				WHERE id = 'result_retention_race'
		`, attemptedAt.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := txB.Exec(ctx, `
				UPDATE post_delivery_jobs
				SET state = 'succeeded', finished_at = NOW(), updated_at = NOW()
				WHERE id = $1
		`, retryJob.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := txB.Exec(ctx, `
				UPDATE media_post_usages
				SET post_status = 'published', cleanup_after_at = $1,
				    retention_reason = 'plan_status', updated_at = NOW()
				WHERE media_id = ANY($2::text[])
				  AND post_id = 'post_retention_race'
		`, planDeadline, []string{"media_retention_race_a", "media_retention_race_b"}); err != nil {
			t.Fatal(err)
		}
		if err := txB.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		var status string
		if err := pool.QueryRow(ctx, `SELECT status FROM social_post_results WHERE id = 'result_retention_race'`).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status != "published" {
			t.Fatalf("newer result status = %q, want published", status)
		}
		rows, err = pool.Query(ctx, `
			SELECT post_status, retention_reason, cleanup_after_at
			FROM media_post_usages
			WHERE media_id = ANY($1::text[]) AND post_id = 'post_retention_race'
			ORDER BY media_id
		`, []string{"media_retention_race_a", "media_retention_race_b"})
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		retainedCount := 0
		for rows.Next() {
			var postStatus, reason string
			var cleanupAt pgtype.Timestamptz
			if err := rows.Scan(&postStatus, &reason, &cleanupAt); err != nil {
				t.Fatal(err)
			}
			if postStatus != "published" || reason != "plan_status" || !cleanupAt.Valid || !cleanupAt.Time.Equal(planDeadline) {
				t.Fatalf("newer durable usage = status:%q reason:%q cleanup:%#v, want published plan retention %s", postStatus, reason, cleanupAt, planDeadline)
			}
			retainedCount++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if retainedCount != 2 {
			t.Fatalf("newer retained media rows = %d, want 2", retainedCount)
		}
	})

	t.Run("opposite media input orders serialize without deadlock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ownerA, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerA.Release()
		ownerB, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer ownerB.Release()
		attemptedAt := time.Date(2026, 7, 26, 23, 0, 0, 0, time.UTC)

		_, err = pool.Exec(ctx, `
			INSERT INTO social_post_results (id, post_id, social_account_id, status) VALUES
				('result_lock_finalize', 'post_lock_finalize', 'account_lock_finalize', 'processing'),
				('result_lock_retry', 'post_lock_retry', 'account_lock_retry', 'failed');
			INSERT INTO post_delivery_jobs (
				id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, lease_owner, last_attempt_at
			) VALUES (
				'job_lock_finalize', 'post_lock_finalize', 'result_lock_finalize',
				'workspace_lock_order', 'account_lock_finalize', 'tiktok', 'dispatch',
				'running', 1, 'owner_lock_finalize', TIMESTAMPTZ '2026-07-26 23:00:00+00'
			);
			INSERT INTO media (id, workspace_id, status) VALUES
				('media_lock_a', 'workspace_lock_order', 'uploaded'),
				('media_lock_b', 'workspace_lock_order', 'uploaded');
			INSERT INTO social_accounts (id, platform) VALUES
				('account_lock_retry', 'tiktok');
			INSERT INTO platform_publishing_restrictions (platform, enabled, restricted_plan_ids)
			VALUES ('tiktok', FALSE, ARRAY[]::text[])
			ON CONFLICT (platform) DO UPDATE SET enabled = FALSE, restricted_plan_ids = ARRAY[]::text[]
		`)
		if err != nil {
			t.Fatal(err)
		}

		txA, err := ownerA.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txA.Rollback(ctx)
		txB, err := ownerB.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer txB.Rollback(ctx)

		type mediaLockResult struct {
			owner string
			job   db.PostDeliveryJob
			err   error
		}
		start := make(chan struct{})
		done := make(chan mediaLockResult, 2)
		go func() {
			<-start
			params := restrictedFinalizeIntegrationParams("job_lock_finalize", "owner_lock_finalize", attemptedAt)
			params.MediaIds = []string{"media_lock_b", "media_lock_a"}
			params.CleanupAfterAt = pgtype.Timestamptz{Time: attemptedAt.Add(60 * 24 * time.Hour), Valid: true}
			_, finalizeErr := db.New(txA).FinalizeRestrictedPostDeliveryJob(ctx, params)
			done <- mediaLockResult{owner: "a", err: finalizeErr}
		}()
		go func() {
			<-start
			job, retryErr := db.New(txB).CreateRetryPostDeliveryJobWithMediaActivation(ctx, db.CreateRetryPostDeliveryJobWithMediaActivationParams{
				PostID:             "post_lock_retry",
				SocialPostResultID: "result_lock_retry",
				WorkspaceID:        "workspace_lock_order",
				SocialAccountID:    "account_lock_retry",
				Platform:           "tiktok",
				PostInputIndex:     0,
				MaxAttempts:        5,
				NextRunAt:          pgtype.Timestamptz{Time: attemptedAt, Valid: true},
				MediaIds:           []string{"media_lock_a", "media_lock_b"},
			})
			done <- mediaLockResult{owner: "b", job: job, err: retryErr}
		}()
		close(start)

		awaitResult := func() mediaLockResult {
			t.Helper()
			select {
			case result := <-done:
				return result
			case <-ctx.Done():
				t.Fatalf("media lock query timeout: %v", ctx.Err())
				return mediaLockResult{}
			}
		}
		commitOwner := func(owner string) error {
			if owner == "a" {
				return txA.Commit(ctx)
			}
			return txB.Commit(ctx)
		}

		first := awaitResult()
		if first.err != nil {
			t.Fatalf("first media lock query (%s): %v", first.owner, first.err)
		}
		if err := commitOwner(first.owner); err != nil {
			t.Fatal(err)
		}
		second := awaitResult()
		if second.err != nil {
			t.Fatalf("second media lock query (%s): %v", second.owner, second.err)
		}
		if err := commitOwner(second.owner); err != nil {
			t.Fatal(err)
		}
		retryJob := first.job
		if second.owner == "b" {
			retryJob = second.job
		}
		if retryJob.State != "pending" || retryJob.Kind != "retry" {
			t.Fatalf("concurrent retry job = state:%q kind:%q, want pending/retry", retryJob.State, retryJob.Kind)
		}

		rows, err := pool.Query(ctx, `
			SELECT id, usage_version FROM media
			WHERE id = ANY($1::text[])
			ORDER BY id
		`, []string{"media_lock_a", "media_lock_b"})
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		count := 0
		for rows.Next() {
			var mediaID string
			var version int64
			if err := rows.Scan(&mediaID, &version); err != nil {
				t.Fatal(err)
			}
			if version != 2 {
				t.Fatalf("media %s usage_version = %d, want 2 after both atomic paths", mediaID, version)
			}
			count++
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("locked media rows = %d, want 2", count)
		}
	})
}

func TestOrdinaryFinalizationPublishedRaceAndFacebookProcessingAuthority(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)

	t.Run("published commit wins blocked dispatch failure without retry or failure evidence", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 27, 6, 0, 0, 0, time.UTC)
		publishedAt := attemptedAt.Add(17 * time.Second)
		if _, err := pool.Exec(ctx, `
			INSERT INTO social_posts (id, status, workspace_id) VALUES ('post_dispatch_publish_race', 'publishing', 'workspace_race');
			INSERT INTO social_post_results (id, post_id, social_account_id, status)
			VALUES ('result_dispatch_publish_race', 'post_dispatch_publish_race', 'account_race', 'processing');
			INSERT INTO post_delivery_jobs (id, post_id, social_post_result_id, workspace_id, social_account_id,
				platform, kind, state, attempts, max_attempts, lease_owner, last_attempt_at)
			VALUES ('job_dispatch_publish_race', 'post_dispatch_publish_race', 'result_dispatch_publish_race',
				'workspace_race', 'account_race', 'instagram', 'dispatch', 'running', 1, 5, 'owner_race', TIMESTAMPTZ '2026-07-27 06:00:00+00')
		`); err != nil {
			t.Fatal(err)
		}
		winnerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer winnerConn.Release()
		loserConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer loserConn.Release()
		winnerTx, err := winnerConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer winnerTx.Rollback(ctx)
		if err := db.New(winnerTx).LockPostDeliveryJobFinalization(ctx, "post_dispatch_publish_race"); err != nil {
			t.Fatal(err)
		}
		if _, err := winnerTx.Exec(ctx, `UPDATE social_post_results SET status='published', external_id='provider_winner', published_at=$2 WHERE id=$1`, "result_dispatch_publish_race", publishedAt); err != nil {
			t.Fatal(err)
		}
		var winnerBackend, loserBackend int
		if err := winnerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&winnerBackend); err != nil {
			t.Fatal(err)
		}
		if err := loserConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&loserBackend); err != nil {
			t.Fatal(err)
		}
		job, err := db.New(loserConn).GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{ID: "job_dispatch_publish_race", WorkspaceID: "workspace_race"})
		if err != nil {
			t.Fatal(err)
		}
		post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
		stale := db.SocialPostResult{ID: job.SocialPostResultID, PostID: job.PostID, SocialAccountID: job.SocialAccountID, Status: "processing"}
		done := make(chan error, 1)
		go func() {
			done <- NewSocialPostHandler(db.New(loserConn), nil, nil, events.NoopBus{}, nil, nil, nil).
				handleJobDispatchFailure(ctx, post, stale, job, publishOneOutcome{platform: "instagram", err: errors.New("late dispatch failure")})
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, loserBackend, winnerBackend)
		if err := winnerTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		var resultStatus, externalID, jobState, parentStatus string
		var parentPublishedAt pgtype.Timestamptz
		var retries, failures int
		if err := pool.QueryRow(ctx, `
			SELECT result.status, result.external_id, job.state, parent.status, parent.published_at,
			 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=parent.id AND kind='retry'),
			 (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
			FROM social_post_results result JOIN post_delivery_jobs job ON job.social_post_result_id=result.id
			JOIN social_posts parent ON parent.id=result.post_id WHERE job.id='job_dispatch_publish_race'
		`).Scan(&resultStatus, &externalID, &jobState, &parentStatus, &parentPublishedAt, &retries, &failures); err != nil {
			t.Fatal(err)
		}
		if resultStatus != "published" || externalID != "provider_winner" || jobState != "succeeded" || parentStatus != "published" ||
			!parentPublishedAt.Valid || !parentPublishedAt.Time.Equal(publishedAt) || retries != 0 || failures != 0 {
			t.Fatalf("race result/job/parent/time/retries/failures=%q/%q/%q/%q/%#v/%d/%d", resultStatus, externalID, jobState, parentStatus, parentPublishedAt, retries, failures)
		}
	})

	t.Run("facebook provider accepted processing rejects generic failure and stale retry", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, `
			INSERT INTO social_posts (id, status, workspace_id) VALUES ('post_fb_accepted', 'publishing', 'workspace_fb');
			INSERT INTO social_post_results (id, post_id, social_account_id, status, external_id, url)
			VALUES ('result_fb_accepted', 'post_fb_accepted', 'account_fb', 'processing', 'fb_video_accepted', 'https://fb.example/video');
			INSERT INTO post_delivery_jobs (id, post_id, social_post_result_id, workspace_id, social_account_id,
			 platform, kind, state, attempts, max_attempts, lease_owner, last_attempt_at)
			VALUES ('job_fb_accepted', 'post_fb_accepted', 'result_fb_accepted', 'workspace_fb', 'account_fb',
			 'facebook', 'retry', 'retrying', 2, 5, 'owner_fb', TIMESTAMPTZ '2026-07-27 06:30:00+00')
		`); err != nil {
			t.Fatal(err)
		}
		queries := db.New(pool)
		job, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{ID: "job_fb_accepted", WorkspaceID: "workspace_fb"})
		if err != nil {
			t.Fatal(err)
		}
		result, err := queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: job.SocialPostResultID, PostID: job.PostID})
		if err != nil {
			t.Fatal(err)
		}
		post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
		if err := NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil).
			handleJobDispatchFailure(ctx, post, result, job, publishOneOutcome{platform: "facebook", err: errors.New("generic timeout")}); err != nil {
			t.Fatal(err)
		}
		var state, status, externalID, url string
		var retryCount, failureCount int
		if err := pool.QueryRow(ctx, `SELECT job.state, result.status, result.external_id, result.url,
		 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=job.post_id AND kind='retry' AND id<>job.id),
		 (SELECT COUNT(*)::int FROM post_failures WHERE post_id=job.post_id)
		 FROM post_delivery_jobs job JOIN social_post_results result ON result.id=job.social_post_result_id
		 WHERE job.id='job_fb_accepted'`).Scan(&state, &status, &externalID, &url, &retryCount, &failureCount); err != nil {
			t.Fatal(err)
		}
		if state != "succeeded" || status != "processing" || externalID != "fb_video_accepted" || url != "https://fb.example/video" || retryCount != 0 || failureCount != 0 {
			t.Fatalf("facebook accepted convergence=%q/%q/%q/%q retries=%d failures=%d", state, status, externalID, url, retryCount, failureCount)
		}
	})

	t.Run("published commit wins blocked stale recovery without requeue", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 27, 6, 45, 0, 0, time.UTC)
		publishedAt := attemptedAt.Add(23 * time.Second)
		if _, err := pool.Exec(ctx, `
			INSERT INTO social_posts(id,status,workspace_id) VALUES('post_stale_publish_barrier','publishing','workspace_stale_barrier');
			INSERT INTO social_post_results(id,post_id,social_account_id,status) VALUES('result_stale_publish_barrier','post_stale_publish_barrier','account_stale_barrier','processing');
			INSERT INTO post_delivery_jobs(id,post_id,social_post_result_id,workspace_id,social_account_id,platform,kind,state,attempts,max_attempts,lease_owner,last_attempt_at)
			VALUES('job_stale_publish_barrier','post_stale_publish_barrier','result_stale_publish_barrier','workspace_stale_barrier','account_stale_barrier','tiktok','retry','retrying',5,5,'stale_barrier_owner',TIMESTAMPTZ '2026-07-27 06:45:00+00')
		`); err != nil {
			t.Fatal(err)
		}
		winnerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer winnerConn.Release()
		loserConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer loserConn.Release()
		winnerTx, err := winnerConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer winnerTx.Rollback(ctx)
		if err := db.New(winnerTx).LockPostDeliveryJobFinalization(ctx, "post_stale_publish_barrier"); err != nil {
			t.Fatal(err)
		}
		if _, err := winnerTx.Exec(ctx, `UPDATE social_post_results SET status='published',external_id='stale_provider_winner',published_at=$2 WHERE id=$1`, "result_stale_publish_barrier", publishedAt); err != nil {
			t.Fatal(err)
		}
		var winnerBackend, loserBackend int
		if err := winnerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&winnerBackend); err != nil {
			t.Fatal(err)
		}
		if err := loserConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&loserBackend); err != nil {
			t.Fatal(err)
		}
		job, err := db.New(loserConn).GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{ID: "job_stale_publish_barrier", WorkspaceID: "workspace_stale_barrier"})
		if err != nil {
			t.Fatal(err)
		}
		done := make(chan error, 1)
		go func() {
			done <- NewSocialPostHandler(db.New(loserConn), nil, nil, events.NoopBus{}, nil, nil, nil).recoverStaleDeliveryJob(ctx, job)
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, loserBackend, winnerBackend)
		if err := winnerTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		var resultStatus, jobState, parentStatus string
		var retries, failures int
		if err := pool.QueryRow(ctx, `SELECT result.status,job.state,parent.status,
		 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=parent.id AND state='pending'),
		 (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
		 FROM social_post_results result JOIN post_delivery_jobs job ON job.social_post_result_id=result.id JOIN social_posts parent ON parent.id=result.post_id
		 WHERE job.id='job_stale_publish_barrier'`).Scan(&resultStatus, &jobState, &parentStatus, &retries, &failures); err != nil {
			t.Fatal(err)
		}
		if resultStatus != "published" || jobState != "succeeded" || parentStatus != "published" || retries != 0 || failures != 0 {
			t.Fatalf("stale barrier=%q/%q/%q retries=%d failures=%d", resultStatus, jobState, parentStatus, retries, failures)
		}
	})
}

func TestDispatchFailureParentUpdateErrorRollsBackEntireTerminalTransition(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts(id,status,workspace_id) VALUES('post_failure_killpoint','publishing','workspace_failure_killpoint');
		INSERT INTO social_post_results(id,post_id,social_account_id,status) VALUES('result_failure_killpoint','post_failure_killpoint','account_failure_killpoint','processing');
		INSERT INTO post_delivery_jobs(id,post_id,social_post_result_id,workspace_id,social_account_id,platform,kind,state,attempts,max_attempts,lease_owner,last_attempt_at)
		VALUES('job_failure_killpoint','post_failure_killpoint','result_failure_killpoint','workspace_failure_killpoint','account_failure_killpoint','tiktok','dispatch','running',1,5,'failure_killpoint_owner',TIMESTAMPTZ '2026-07-27 07:15:00+00');
		CREATE FUNCTION reject_failure_parent_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected ordinary parent update failure'; END $$;
		CREATE TRIGGER reject_failure_parent_update_trigger BEFORE UPDATE OF status ON social_posts FOR EACH ROW EXECUTE FUNCTION reject_failure_parent_update()
	`); err != nil {
		t.Fatal(err)
	}
	queries := db.New(pool)
	job, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{ID: "job_failure_killpoint", WorkspaceID: "workspace_failure_killpoint"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: job.SocialPostResultID, PostID: job.PostID})
	if err != nil {
		t.Fatal(err)
	}
	post := db.SocialPost{ID: job.PostID, WorkspaceID: job.WorkspaceID, Status: "publishing"}
	err = NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil).handleJobDispatchFailure(ctx, post, result, job, publishOneOutcome{platform: "tiktok", err: errors.New("temporary provider outage")})
	if err == nil || !strings.Contains(err.Error(), "injected ordinary parent update failure") {
		t.Fatalf("killpoint error=%v", err)
	}
	var jobState, resultStatus, parentStatus string
	var retryCount, failureCount int
	if err := pool.QueryRow(ctx, `SELECT job.state,result.status,parent.status,
	 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=parent.id AND id<>job.id),
	 (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
	 FROM post_delivery_jobs job JOIN social_post_results result ON result.id=job.social_post_result_id JOIN social_posts parent ON parent.id=job.post_id
	 WHERE job.id='job_failure_killpoint'`).Scan(&jobState, &resultStatus, &parentStatus, &retryCount, &failureCount); err != nil {
		t.Fatal(err)
	}
	if jobState != "running" || resultStatus != "processing" || parentStatus != "publishing" || retryCount != 0 || failureCount != 0 {
		t.Fatalf("killpoint durable state=%q/%q/%q retry=%d failure=%d", jobState, resultStatus, parentStatus, retryCount, failureCount)
	}
}

func TestDispatchFailureDiagnosticPanicCannotChangeBusinessOutcome(t *testing.T) {
	type outcome struct {
		jobState     string
		resultStatus string
		parentStatus string
		retryCount   int
		failureCount int
		legacyDebug  pgtype.Text
	}
	run := func(t *testing.T, withPanickingSink bool) outcome {
		t.Helper()
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := pool.Exec(ctx, `
			INSERT INTO social_posts(id,status,workspace_id)
			VALUES('post_observability_fault','publishing','workspace_observability_fault');
			INSERT INTO social_post_results(id,post_id,social_account_id,status)
			VALUES('result_observability_fault','post_observability_fault','account_observability_fault','processing');
			INSERT INTO post_delivery_jobs(
				id,post_id,social_post_result_id,workspace_id,social_account_id,
				platform,kind,state,attempts,max_attempts,lease_owner,last_attempt_at
			) VALUES(
				'job_observability_fault','post_observability_fault','result_observability_fault',
				'workspace_observability_fault','account_observability_fault','tiktok',
				'dispatch','running',1,5,'observability_fault_owner',NOW()
			)
		`); err != nil {
			t.Fatal(err)
		}
		queries := db.New(pool)
		job, err := queries.GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{
			ID: "job_observability_fault", WorkspaceID: "workspace_observability_fault",
		})
		if err != nil {
			t.Fatal(err)
		}
		result, err := queries.GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{
			ID: job.SocialPostResultID, PostID: job.PostID,
		})
		if err != nil {
			t.Fatal(err)
		}
		post, err := queries.GetSocialPostByID(ctx, job.PostID)
		if err != nil {
			t.Fatal(err)
		}
		h := NewSocialPostHandler(queries, nil, nil, events.NoopBus{}, nil, nil, nil)
		if withPanickingSink {
			h.SetPostFailureDebugSink(panickingPostFailureDebugSink{})
		}
		if err := h.handleJobDispatchFailure(ctx, post, result, job, publishOneOutcome{
			platform:  "tiktok",
			err:       errors.New("generic provider rejection"),
			debugCurl: "safe diagnostic that must not enter the business transaction",
		}); err != nil {
			t.Fatal(err)
		}
		var got outcome
		if err := pool.QueryRow(ctx, `
			SELECT job.state, result.status, parent.status,
			  (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=parent.id AND id<>job.id),
			  (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id),
			  result.debug_curl
			FROM post_delivery_jobs job
			JOIN social_post_results result ON result.id=job.social_post_result_id
			JOIN social_posts parent ON parent.id=job.post_id
			WHERE job.id='job_observability_fault'
		`).Scan(
			&got.jobState,
			&got.resultStatus,
			&got.parentStatus,
			&got.retryCount,
			&got.failureCount,
			&got.legacyDebug,
		); err != nil {
			t.Fatal(err)
		}
		return got
	}

	baseline := run(t, false)
	withFault := run(t, true)
	if !reflect.DeepEqual(withFault, baseline) {
		t.Fatalf("observability panic changed business outcome:\n baseline=%#v\n with fault=%#v", baseline, withFault)
	}
	if baseline.legacyDebug.Valid {
		t.Fatalf("legacy business debug column was populated: %#v", baseline.legacyDebug)
	}
}

func TestFacebookProviderPollDefinitiveFailureAndPublishedRace(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_posts (id, status, workspace_id) VALUES
		 ('post_fb_poll_failure', 'publishing', 'workspace_fb_poll'),
		 ('post_fb_poll_race', 'publishing', 'workspace_fb_poll');
		INSERT INTO social_post_results (id, post_id, social_account_id, status, external_id) VALUES
		 ('result_fb_poll_failure', 'post_fb_poll_failure', 'account_fb_poll', 'processing', 'fb_video_failure'),
		 ('result_fb_poll_race', 'post_fb_poll_race', 'account_fb_poll', 'processing', 'fb_video_race')
	`); err != nil {
		t.Fatal(err)
	}
	h := NewSocialPostHandler(db.New(pool), nil, nil, events.NoopBus{}, nil, nil, nil)
	failedResult, err := db.New(pool).GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: "result_fb_poll_failure", PostID: "post_fb_poll_failure"})
	if err != nil {
		t.Fatal(err)
	}
	applied, err := h.finalizeFacebookProviderPoll(ctx, db.SocialPost{ID: "post_fb_poll_failure", WorkspaceID: "workspace_fb_poll", Status: "publishing"}, failedResult,
		"workspace_fb_poll", "failed", failedResult.ExternalID, pgtype.Text{}, "provider definitive failure", time.Time{})
	if err != nil || !applied {
		t.Fatalf("definitive poll failure applied=%v err=%v", applied, err)
	}
	var status, parentStatus string
	var failures int
	if err := pool.QueryRow(ctx, `SELECT result.status, parent.status, (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
	 FROM social_post_results result JOIN social_posts parent ON parent.id=result.post_id WHERE result.id='result_fb_poll_failure'`).Scan(&status, &parentStatus, &failures); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || parentStatus != "failed" || failures != 1 {
		t.Fatalf("definitive failure=%q/%q/%d", status, parentStatus, failures)
	}

	tracerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tracerConn.Release()
	winnerTx, err := tracerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer winnerTx.Rollback(ctx)
	if err := db.New(winnerTx).LockPostDeliveryJobFinalization(ctx, "post_fb_poll_race"); err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, 7, 27, 7, 0, 0, 0, time.UTC)
	if _, err := winnerTx.Exec(ctx, `UPDATE social_post_results SET status='published', published_at=$2 WHERE id=$1`, "result_fb_poll_race", publishedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := winnerTx.Exec(ctx, `UPDATE social_posts SET status='published', published_at=$2 WHERE id=$1`, "post_fb_poll_race", publishedAt); err != nil {
		t.Fatal(err)
	}
	raceResult := db.SocialPostResult{ID: "result_fb_poll_race", PostID: "post_fb_poll_race", SocialAccountID: "account_fb_poll", Status: "processing", ExternalID: pgtype.Text{String: "fb_video_race", Valid: true}}
	done := make(chan struct {
		applied bool
		err     error
	}, 1)
	go func() {
		a, e := h.finalizeFacebookProviderPoll(ctx, db.SocialPost{ID: "post_fb_poll_race", WorkspaceID: "workspace_fb_poll", Status: "publishing"}, raceResult,
			"workspace_fb_poll", "failed", raceResult.ExternalID, pgtype.Text{}, "late provider error", time.Time{})
		done <- struct {
			applied bool
			err     error
		}{a, e}
	}()
	time.Sleep(50 * time.Millisecond)
	if err := winnerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	outcome := <-done
	if outcome.err != nil || outcome.applied {
		t.Fatalf("late poll race applied=%v err=%v", outcome.applied, outcome.err)
	}
	if err := pool.QueryRow(ctx, `SELECT result.status, parent.status, (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
	 FROM social_post_results result JOIN social_posts parent ON parent.id=result.post_id WHERE result.id='result_fb_poll_race'`).Scan(&status, &parentStatus, &failures); err != nil {
		t.Fatal(err)
	}
	if status != "published" || parentStatus != "published" || failures != 0 {
		t.Fatalf("poll published winner=%q/%q/%d", status, parentStatus, failures)
	}
}

func insertManualRetryFinalizerRaceFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool, suffix string, siblingProcessing bool) (string, string, string, db.SocialPostResult) {
	t.Helper()
	workspaceID := "workspace_manual_retry_race_" + suffix
	postID := "post_manual_retry_race_" + suffix
	resultID := "result_manual_retry_race_" + suffix
	siblingID := "result_manual_retry_sibling_" + suffix
	accountID := "account_manual_retry_race_" + suffix
	mediaID := "media_manual_retry_race_" + suffix
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: accountID, MediaIDs: []string{mediaID},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,status,workspace_id,metadata) VALUES($1,'publishing',$2,$3)`, postID, workspaceID, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_accounts(id,platform,status) VALUES($1,'facebook','active')`, accountID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_post_results(id,post_id,social_account_id,status,external_id) VALUES($1,$2,$3,'failed','failed_provider_id')`, resultID, postID, accountID); err != nil {
		t.Fatal(err)
	}
	if siblingProcessing {
		if _, err := pool.Exec(ctx, `INSERT INTO social_post_results(id,post_id,social_account_id,status,external_id) VALUES($1,$2,$3,'processing','sibling_provider_id')`, siblingID, postID, accountID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO media(id,workspace_id,status) VALUES($1,$2,'uploaded')`, mediaID, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions(workspace_id,plan_id) VALUES($1,'free')`, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO platform_publishing_restrictions(platform,enabled,restricted_plan_ids)
	 VALUES('facebook',FALSE,ARRAY[]::text[]) ON CONFLICT(platform) DO UPDATE SET enabled=FALSE,restricted_plan_ids=ARRAY[]::text[]`); err != nil {
		t.Fatal(err)
	}
	return workspaceID, postID, resultID, db.SocialPostResult{
		ID: siblingID, PostID: postID, SocialAccountID: accountID, Status: "processing",
		ExternalID: pgtype.Text{String: "sibling_provider_id", Valid: true},
	}
}

func TestEnqueueRetryAndTerminalFinalizerSerializePostProjection(t *testing.T) {
	t.Run("retry lock wins before finalizer snapshot", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		workspaceID, postID, resultID, sibling := insertManualRetryFinalizerRaceFixture(t, ctx, pool, "retry_first", true)

		blockerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer blockerConn.Release()
		blockerBackend := 0
		if err := blockerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBackend); err != nil {
			t.Fatal(err)
		}
		blockerTx, err := blockerConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer blockerTx.Rollback(ctx)
		if _, err := blockerTx.Exec(ctx, `SELECT id FROM media WHERE id=$1 FOR UPDATE`, "media_manual_retry_race_retry_first"); err != nil {
			t.Fatal(err)
		}

		retryConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryConn.Release()
		retryBackend := 0
		if err := retryConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retryBackend); err != nil {
			t.Fatal(err)
		}
		retryDone := make(chan error, 1)
		go func() {
			_, retryErr := NewSocialPostHandler(db.New(retryConn), nil, nil, events.NoopBus{}, nil, nil, nil).
				EnqueueRetryForResult(ctx, workspaceID, postID, resultID)
			retryDone <- retryErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, retryBackend, blockerBackend)

		finalizerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer finalizerConn.Release()
		finalizerBackend := 0
		if err := finalizerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&finalizerBackend); err != nil {
			t.Fatal(err)
		}
		bus := &countingPostEventBus{}
		finalizerDone := make(chan error, 1)
		go func() {
			_, finalizeErr := NewSocialPostHandler(db.New(finalizerConn), nil, nil, bus, nil, nil, nil).
				finalizeFacebookProviderPoll(ctx, db.SocialPost{ID: postID, WorkspaceID: workspaceID, Status: "publishing"}, sibling,
					workspaceID, "failed", sibling.ExternalID, pgtype.Text{}, "sibling definitive failure", time.Time{})
			finalizerDone <- finalizeErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, finalizerBackend, retryBackend)
		if err := blockerTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-retryDone; err != nil {
			t.Fatal(err)
		}
		if err := <-finalizerDone; err != nil {
			t.Fatal(err)
		}
		var parentStatus string
		var activeJobs int
		if err := pool.QueryRow(ctx, `SELECT status,
		 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=$1 AND state IN ('pending','running','retrying'))
		 FROM social_posts WHERE id=$1`, postID).Scan(&parentStatus, &activeJobs); err != nil {
			t.Fatal(err)
		}
		if parentStatus != "publishing" || activeJobs != 1 || bus.calls != 0 {
			t.Fatalf("retry-first projection parent=%q active=%d terminal_events=%d", parentStatus, activeJobs, bus.calls)
		}
	})

	t.Run("finalizer lock wins before retry eligibility", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		workspaceID, postID, resultID, _ := insertManualRetryFinalizerRaceFixture(t, ctx, pool, "finalizer_first", false)
		if _, err := pool.Exec(ctx, `UPDATE social_post_results SET status='processing' WHERE id=$1`, resultID); err != nil {
			t.Fatal(err)
		}

		finalizerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer finalizerConn.Release()
		finalizerBackend := 0
		if err := finalizerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&finalizerBackend); err != nil {
			t.Fatal(err)
		}
		finalizerTx, err := finalizerConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer finalizerTx.Rollback(ctx)
		finalizerQueries := db.New(finalizerTx)
		if err := finalizerQueries.LockPostDeliveryJobFinalization(ctx, postID); err != nil {
			t.Fatal(err)
		}
		if _, err := finalizerTx.Exec(ctx, `UPDATE social_post_results SET status='failed',error_message='definitive failure' WHERE id=$1`, resultID); err != nil {
			t.Fatal(err)
		}
		if _, err := finalizerTx.Exec(ctx, `UPDATE social_posts SET status='failed' WHERE id=$1`, postID); err != nil {
			t.Fatal(err)
		}

		retryConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryConn.Release()
		retryBackend := 0
		if err := retryConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retryBackend); err != nil {
			t.Fatal(err)
		}
		retryDone := make(chan error, 1)
		go func() {
			_, retryErr := NewSocialPostHandler(db.New(retryConn), nil, nil, events.NoopBus{}, nil, nil, nil).
				EnqueueRetryForResult(ctx, workspaceID, postID, resultID)
			retryDone <- retryErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, retryBackend, finalizerBackend)
		if err := finalizerTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-retryDone; err != nil {
			t.Fatal(err)
		}
		var parentStatus, resultStatus string
		var activeJobs int
		if err := pool.QueryRow(ctx, `SELECT parent.status,result.status,
		 (SELECT COUNT(*)::int FROM post_delivery_jobs WHERE post_id=parent.id AND state IN ('pending','running','retrying'))
		 FROM social_posts parent JOIN social_post_results result ON result.post_id=parent.id
		 WHERE parent.id=$1 AND result.id=$2`, postID, resultID).Scan(&parentStatus, &resultStatus, &activeJobs); err != nil {
			t.Fatal(err)
		}
		if resultStatus != "failed" || parentStatus != "publishing" || activeJobs != 1 {
			t.Fatalf("finalizer-first projection result=%q parent=%q active=%d", resultStatus, parentStatus, activeJobs)
		}
	})

	t.Run("terminal retention snapshot cannot overwrite retry activation", func(t *testing.T) {
		pool := openRestrictedDeliveryIntegrationPool(t)
		setupRestrictedDeliveryIntegrationSchema(t, pool)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		workspaceID, postID, resultID, _ := insertManualRetryFinalizerRaceFixture(t, ctx, pool, "retention_retry", false)
		if _, err := pool.Exec(ctx, `UPDATE social_posts SET status='failed' WHERE id=$1`, postID); err != nil {
			t.Fatal(err)
		}

		blockerConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer blockerConn.Release()
		blockerBackend := 0
		if err := blockerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerBackend); err != nil {
			t.Fatal(err)
		}
		blockerTx, err := blockerConn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer blockerTx.Rollback(ctx)
		if _, err := blockerTx.Exec(ctx, `SELECT id FROM media WHERE id=$1 FOR UPDATE`, "media_manual_retry_race_retention_retry"); err != nil {
			t.Fatal(err)
		}

		retentionConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retentionConn.Release()
		retentionBackend := 0
		if err := retentionConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retentionBackend); err != nil {
			t.Fatal(err)
		}
		retentionDone := make(chan struct{}, 1)
		go func() {
			NewSocialPostHandler(db.New(retentionConn), nil, nil, events.NoopBus{}, nil, nil, nil).
				syncTerminalPostRetentionAfterCommit(ctx, postID)
			retentionDone <- struct{}{}
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, retentionBackend, blockerBackend)

		retryConn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer retryConn.Release()
		retryBackend := 0
		if err := retryConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retryBackend); err != nil {
			t.Fatal(err)
		}
		retryDone := make(chan error, 1)
		go func() {
			_, retryErr := NewSocialPostHandler(db.New(retryConn), nil, nil, events.NoopBus{}, nil, nil, nil).
				EnqueueRetryForResult(ctx, workspaceID, postID, resultID)
			retryDone <- retryErr
		}()
		waitForPostgresSessionBlockedBy(t, ctx, pool, retryBackend, retentionBackend)
		var retentionHoldsPostLock, retryWaitsForPostLock bool
		if err := pool.QueryRow(ctx, `SELECT
		 EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted),
		 EXISTS(SELECT 1 FROM pg_locks WHERE pid=$2 AND locktype='advisory' AND NOT granted)
		`, retentionBackend, retryBackend).Scan(&retentionHoldsPostLock, &retryWaitsForPostLock); err != nil {
			t.Fatal(err)
		}
		if !retentionHoldsPostLock || !retryWaitsForPostLock {
			t.Fatalf("retention/retry advisory lock state holder=%v waiter=%v", retentionHoldsPostLock, retryWaitsForPostLock)
		}
		if err := blockerTx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		<-retentionDone
		if err := <-retryDone; err != nil {
			t.Fatal(err)
		}
		var parentStatus, usageStatus, retentionReason string
		var cleanupAfter pgtype.Timestamptz
		if err := pool.QueryRow(ctx, `SELECT parent.status,usage.post_status,usage.retention_reason,usage.cleanup_after_at
		 FROM social_posts parent JOIN media_post_usages usage ON usage.post_id=parent.id WHERE parent.id=$1`, postID).
			Scan(&parentStatus, &usageStatus, &retentionReason, &cleanupAfter); err != nil {
			t.Fatal(err)
		}
		if parentStatus != "publishing" || usageStatus != "publishing" || retentionReason != "active_post" || cleanupAfter.Valid {
			t.Fatalf("retention/retry final state parent=%q usage=%q reason=%q cleanup=%v", parentStatus, usageStatus, retentionReason, cleanupAfter)
		}
	})
}

func TestTerminalParentEventSerializesBeforeManualRetry(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	workspaceID, postID, resultID, _ := insertManualRetryFinalizerRaceFixture(t, ctx, pool, "event_first", false)
	if _, err := pool.Exec(ctx, `UPDATE social_post_results SET status='processing',external_id='event_first_video' WHERE id=$1`, resultID); err != nil {
		t.Fatal(err)
	}

	finalizerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer finalizerConn.Release()
	finalizerBackend := 0
	if err := finalizerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&finalizerBackend); err != nil {
		t.Fatal(err)
	}
	result, err := db.New(pool).GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: resultID, PostID: postID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE FUNCTION block_terminal_event_insert() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(7727001);
			RETURN NEW;
		END $$;
		CREATE TRIGGER block_terminal_event_insert_trigger
		BEFORE INSERT ON terminal_post_event_outbox
		FOR EACH ROW EXECUTE FUNCTION block_terminal_event_insert();
	`); err != nil {
		t.Fatal(err)
	}
	blockerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerConn.Release()
	blockerTx, err := blockerConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blockerTx.Rollback(ctx)
	var blockerBackend int
	if err := blockerTx.QueryRow(ctx, `SELECT pg_backend_pid() FROM pg_advisory_xact_lock(7727001)`).Scan(&blockerBackend); err != nil {
		t.Fatal(err)
	}
	finalizerDone := make(chan error, 1)
	go func() {
		_, finalizeErr := NewSocialPostHandler(db.New(finalizerConn), nil, nil, events.NoopBus{}, nil, nil, nil).
			finalizeFacebookProviderPoll(ctx, db.SocialPost{ID: postID, WorkspaceID: workspaceID, Status: "publishing"}, result,
				workspaceID, "failed", result.ExternalID, pgtype.Text{}, "definitive event-first failure", time.Time{})
		finalizerDone <- finalizeErr
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, finalizerBackend, blockerBackend)

	retryConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer retryConn.Release()
	retryBackend := 0
	if err := retryConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retryBackend); err != nil {
		t.Fatal(err)
	}
	retryDone := make(chan error, 1)
	go func() {
		_, retryErr := NewSocialPostHandler(db.New(retryConn), nil, nil, events.NoopBus{}, nil, nil, nil).
			EnqueueRetryForResult(ctx, workspaceID, postID, resultID)
		retryDone <- retryErr
	}()
	blockCtx, blockCancel := context.WithTimeout(ctx, time.Second)
	defer blockCancel()
	waitForPostgresSessionBlockedBy(t, blockCtx, pool, retryBackend, finalizerBackend)
	if err := blockerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-finalizerDone; err != nil {
		t.Fatal(err)
	}
	if err := <-retryDone; err != nil {
		t.Fatal(err)
	}
	var parentStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id=$1`, postID).Scan(&parentStatus); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "publishing" {
		t.Fatalf("event-first final parent=%q", parentStatus)
	}
	rows, err := pool.Query(ctx, `
		SELECT parent_status,event_type,parent_version,
		       (SELECT COUNT(*) FROM terminal_post_event_deliveries d WHERE d.event_id=e.id)
		FROM terminal_post_event_outbox e
		WHERE post_id=$1
		ORDER BY sequence_number
	`, postID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var statuses, eventTypes, versions []string
	var deliveryCounts []int
	for rows.Next() {
		var status, eventType, version string
		var deliveries int
		if err := rows.Scan(&status, &eventType, &version, &deliveries); err != nil {
			t.Fatal(err)
		}
		statuses = append(statuses, status)
		eventTypes = append(eventTypes, eventType)
		versions = append(versions, version)
		deliveryCounts = append(deliveryCounts, deliveries)
	}
	if !reflect.DeepEqual(statuses, []string{"failed", "publishing"}) ||
		!reflect.DeepEqual(eventTypes, []string{events.EventPostFailed, ""}) ||
		!reflect.DeepEqual(deliveryCounts, []int{3, 0}) || len(versions) != 2 || versions[0] == versions[1] {
		t.Fatalf("ordered transition outbox statuses=%v events=%v versions=%v deliveries=%v", statuses, eventTypes, versions, deliveryCounts)
	}
}

func TestTerminalParentEventSkipsStaleProjectionWhenRetryWins(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	workspaceID, postID, resultID, _ := insertManualRetryFinalizerRaceFixture(t, ctx, pool, "event_retry_first", false)

	terminalConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer terminalConn.Release()
	terminalBackend := 0
	if err := terminalConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&terminalBackend); err != nil {
		t.Fatal(err)
	}
	terminalTx, err := terminalConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer terminalTx.Rollback(ctx)
	if err := db.New(terminalTx).LockPostDeliveryJobFinalization(ctx, postID); err != nil {
		t.Fatal(err)
	}
	if _, err := terminalTx.Exec(ctx, `UPDATE social_posts SET status='failed' WHERE id=$1`, postID); err != nil {
		t.Fatal(err)
	}
	var terminalVersion string
	if err := terminalTx.QueryRow(ctx, `SELECT xmin::text FROM social_posts WHERE id=$1`, postID).Scan(&terminalVersion); err != nil {
		t.Fatal(err)
	}
	_ = terminalVersion

	retryConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer retryConn.Release()
	retryBackend := 0
	if err := retryConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&retryBackend); err != nil {
		t.Fatal(err)
	}
	retryDone := make(chan error, 1)
	go func() {
		_, retryErr := NewSocialPostHandler(db.New(retryConn), nil, nil, events.NoopBus{}, nil, nil, nil).
			EnqueueRetryForResult(ctx, workspaceID, postID, resultID)
		retryDone <- retryErr
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, retryBackend, terminalBackend)
	if err := terminalTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-retryDone; err != nil {
		t.Fatal(err)
	}
	var parentStatus string
	if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id=$1`, postID).Scan(&parentStatus); err != nil {
		t.Fatal(err)
	}
	if parentStatus != "publishing" {
		t.Fatalf("retry-first parent=%q", parentStatus)
	}
	var failedEvents, publishingTransitions int
	if err := pool.QueryRow(ctx, `SELECT
		COUNT(*) FILTER (WHERE event_type='post.failed'),
		COUNT(*) FILTER (WHERE parent_status='publishing' AND event_type='')
		FROM terminal_post_event_outbox WHERE post_id=$1`, postID).Scan(&failedEvents, &publishingTransitions); err != nil {
		t.Fatal(err)
	}
	if failedEvents != 0 || publishingTransitions != 1 {
		t.Fatalf("retry-first outbox failed_events=%d publishing_transitions=%d", failedEvents, publishingTransitions)
	}
}

func TestTerminalRetentionSortsSharedMediaLocksAcrossPosts(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	metadataAB, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "account_media_order_ab", MediaIDs: []string{"media_order_a", "media_order_b"}}})
	if err != nil {
		t.Fatal(err)
	}
	metadataBA, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "account_media_order_ba", MediaIDs: []string{"media_order_b", "media_order_a"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,status,workspace_id,metadata) VALUES
		 ('post_media_order_ab','failed','workspace_media_order',$1),
		 ('post_media_order_ba','failed','workspace_media_order',$2)`, metadataAB, metadataBA); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results(id,post_id,social_account_id,status) VALUES
		 ('result_media_order_ab','post_media_order_ab','account_media_order_ab','failed'),
		 ('result_media_order_ba','post_media_order_ba','account_media_order_ba','failed');
		INSERT INTO media(id,workspace_id,status) VALUES
		 ('media_order_a','workspace_media_order','uploaded'),
		 ('media_order_b','workspace_media_order','uploaded');
		INSERT INTO subscriptions(workspace_id,plan_id) VALUES ('workspace_media_order','free');
		CREATE FUNCTION wait_media_order_gate() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
		  IF NEW.id = 'media_order_a' THEN
		    PERFORM pg_advisory_xact_lock(20260727, 101);
		  ELSIF NEW.id = 'media_order_b' THEN
		    PERFORM pg_advisory_xact_lock(20260727, 102);
		  END IF;
		  RETURN NEW;
		END $$;
		CREATE TRIGGER wait_media_order_gate_trigger BEFORE UPDATE OF usage_version ON media
		 FOR EACH ROW EXECUTE FUNCTION wait_media_order_gate();
	`); err != nil {
		t.Fatal(err)
	}

	gateConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gateConn.Release()
	gateBackend := 0
	if err := gateConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&gateBackend); err != nil {
		t.Fatal(err)
	}
	if _, err := gateConn.Exec(ctx, `SELECT pg_advisory_lock(20260727,101),pg_advisory_lock(20260727,102)`); err != nil {
		t.Fatal(err)
	}
	defer gateConn.Exec(context.Background(), `SELECT pg_advisory_unlock(20260727,101),pg_advisory_unlock(20260727,102)`)

	connAB, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connAB.Release()
	connBA, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer connBA.Release()
	backendAB, backendBA := 0, 0
	if err := connAB.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendAB); err != nil {
		t.Fatal(err)
	}
	if err := connBA.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&backendBA); err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 2)
	go func() {
		NewSocialPostHandler(db.New(connAB), nil, nil, events.NoopBus{}, nil, nil, nil).
			syncTerminalPostRetentionAfterCommit(ctx, "post_media_order_ab")
		done <- "ab"
	}()
	waitForPostgresSessionBlockedBy(t, ctx, pool, backendAB, gateBackend)
	go func() {
		NewSocialPostHandler(db.New(connBA), nil, nil, events.NoopBus{}, nil, nil, nil).
			syncTerminalPostRetentionAfterCommit(ctx, "post_media_order_ba")
		done <- "ba"
	}()
	// Both metadata blobs must normalize to A then B. The BA post therefore
	// queues behind AB's A row lock, rather than independently reaching B.
	waitForPostgresSessionBlockedBy(t, ctx, pool, backendBA, backendAB)
	if _, err := gateConn.Exec(ctx, `SELECT pg_advisory_unlock(20260727,101)`); err != nil {
		t.Fatal(err)
	}
	for {
		var waitingOnB bool
		if err := pool.QueryRow(ctx, `SELECT $1::int = ANY(pg_blocking_pids($2::int)) OR $1::int = ANY(pg_blocking_pids($3::int))`, gateBackend, backendAB, backendBA).Scan(&waitingOnB); err != nil {
			t.Fatal(err)
		}
		if waitingOnB {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("neither retention transaction reached ordered media B gate: %v", ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if _, err := gateConn.Exec(ctx, `SELECT pg_advisory_unlock(20260727,102)`); err != nil {
		t.Fatal(err)
	}
	first, second := <-done, <-done
	if first == second {
		t.Fatalf("retention completions=%q/%q", first, second)
	}
	var usages int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM media_post_usages
	 WHERE post_id IN ('post_media_order_ab','post_media_order_ba')
	   AND media_id IN ('media_order_a','media_order_b')
	   AND post_status='failed' AND retention_reason='plan_status' AND cleanup_after_at IS NOT NULL`).Scan(&usages); err != nil {
		t.Fatal(err)
	}
	if usages != 4 {
		t.Fatalf("ordered shared-media retention ledger rows=%d, want 4", usages)
	}
}

func TestCancelDeliveryJobTerminalResultAndPostCommitRetention(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "account_cancel_terminal", MediaIDs: []string{"media_cancel_terminal"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts (id,status,workspace_id,metadata) VALUES ($1,'publishing',$2,$3)`, "post_cancel_terminal", "workspace_cancel_terminal", metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO social_post_results (id,post_id,social_account_id,status) VALUES ('result_cancel_terminal','post_cancel_terminal','account_cancel_terminal','processing');
		INSERT INTO post_delivery_jobs (id,post_id,social_post_result_id,workspace_id,social_account_id,platform,kind,state,attempts)
		VALUES ('job_cancel_terminal','post_cancel_terminal','result_cancel_terminal','workspace_cancel_terminal','account_cancel_terminal','tiktok','retry','pending',1);
		INSERT INTO media (id,workspace_id,status) VALUES ('media_cancel_terminal','workspace_cancel_terminal','uploaded');
		CREATE FUNCTION reject_cancel_retention() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected post-commit retention failure'; END $$;
		CREATE TRIGGER reject_cancel_retention_trigger BEFORE INSERT ON media_post_usages FOR EACH ROW EXECUTE FUNCTION reject_cancel_retention()
	`); err != nil {
		t.Fatal(err)
	}
	h := NewSocialPostHandler(db.New(pool), nil, nil, events.NoopBus{}, nil, nil, nil)
	job, err := h.CancelDeliveryJob(ctx, "workspace_cancel_terminal", "job_cancel_terminal")
	if err != nil {
		t.Fatalf("post-commit retention error leaked from cancel: %v", err)
	}
	if job.State != "cancelled" {
		t.Fatalf("cancel state=%q", job.State)
	}
	var resultStatus, parentStatus, errorCode, failureStage string
	var retriable bool
	if err := pool.QueryRow(ctx, `SELECT result.status,parent.status,result.error_code,result.failure_stage,result.is_retriable
	 FROM social_post_results result JOIN social_posts parent ON parent.id=result.post_id WHERE result.id='result_cancel_terminal'`).
		Scan(&resultStatus, &parentStatus, &errorCode, &failureStage, &retriable); err != nil {
		t.Fatal(err)
	}
	if resultStatus != "failed" || parentStatus != "failed" || errorCode != "delivery_cancelled" || failureStage != "queue_cancelled" || retriable {
		t.Fatalf("cancel terminal result=%q parent=%q code=%q stage=%q retryable=%v", resultStatus, parentStatus, errorCode, failureStage, retriable)
	}
	if _, err := pool.Exec(ctx, `UPDATE post_delivery_jobs SET state='retrying',lease_owner='active_owner',last_attempt_at=NOW(),platform_started_at=NOW() WHERE id='job_cancel_terminal'`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.CancelDeliveryJob(ctx, "workspace_cancel_terminal", "job_cancel_terminal"); err == nil || !strings.Contains(err.Error(), "not started provider delivery") {
		t.Fatalf("active provider retry cancel error=%v, want rejection", err)
	}
}

func TestRestrictedFinalizationPublishesParentTerminalEventExactlyOnce(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)
	for _, tc := range []struct {
		name, suffix, wantStatus, wantEvent string
		withPublishedSibling                bool
	}{
		{name: "failed", suffix: "failed_event", wantStatus: "failed", wantEvent: events.EventPostFailed},
		{name: "partial", suffix: "partial_event", wantStatus: "partial", wantEvent: events.EventPostPartial, withPublishedSibling: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			attemptedAt := time.Date(2026, 7, 27, 7, 30, 0, 0, time.UTC)
			postID, resultID, jobID := "post_"+tc.suffix, "result_"+tc.suffix, "job_"+tc.suffix
			metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{AccountID: "account_" + tc.suffix}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,status,workspace_id,metadata) VALUES($1,'publishing',$2,$3)`, postID, "workspace_"+tc.suffix, metadata); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO social_post_results(id,post_id,social_account_id,status) VALUES($1,$2,$3,'processing')`, resultID, postID, "account_"+tc.suffix); err != nil {
				t.Fatal(err)
			}
			if tc.withPublishedSibling {
				if _, err := pool.Exec(ctx, `INSERT INTO social_post_results(id,post_id,social_account_id,status,published_at) VALUES($1,$2,$3,'published',$4)`, resultID+"_published", postID, "account_"+tc.suffix+"_published", attemptedAt.Add(-time.Hour)); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, `INSERT INTO post_delivery_jobs(id,post_id,social_post_result_id,workspace_id,social_account_id,platform,kind,state,attempts,lease_owner,last_attempt_at)
			 VALUES($1,$2,$3,$4,$5,'tiktok','dispatch','running',1,'event_owner',$6)`, jobID, postID, resultID, "workspace_"+tc.suffix, "account_"+tc.suffix, attemptedAt); err != nil {
				t.Fatal(err)
			}
			job, err := db.New(pool).GetPostDeliveryJobByIDAndWorkspace(ctx, db.GetPostDeliveryJobByIDAndWorkspaceParams{ID: jobID, WorkspaceID: "workspace_" + tc.suffix})
			if err != nil {
				t.Fatal(err)
			}
			result, err := db.New(pool).GetSocialPostResultByIDAndPost(ctx, db.GetSocialPostResultByIDAndPostParams{ID: resultID, PostID: postID})
			if err != nil {
				t.Fatal(err)
			}
			post, err := db.New(pool).GetSocialPostByID(ctx, postID)
			if err != nil {
				t.Fatal(err)
			}
			h := NewSocialPostHandler(db.New(pool), nil, nil, events.NoopBus{}, nil, nil, nil)
			decision := publishingrestrictions.Decision{Restricted: true, Platform: "tiktok", CycleID: "cycle_" + tc.suffix}
			if err := h.finalizeRestrictedDeliveryJob(ctx, job, result, post, decision); err != nil {
				t.Fatal(err)
			}
			if err := h.finalizeRestrictedDeliveryJob(ctx, job, result, post, decision); err != nil {
				t.Fatal(err)
			}
			var status string
			if err := pool.QueryRow(ctx, `SELECT status FROM social_posts WHERE id=$1`, postID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			var eventCount, deliveryCount int
			if err := pool.QueryRow(ctx, `SELECT COUNT(DISTINCT e.id),COUNT(d.event_id)
				FROM terminal_post_event_outbox e LEFT JOIN terminal_post_event_deliveries d ON d.event_id=e.id
				WHERE e.post_id=$1 AND e.event_type=$2`, postID, tc.wantEvent).Scan(&eventCount, &deliveryCount); err != nil {
				t.Fatal(err)
			}
			if status != tc.wantStatus || eventCount != 1 || deliveryCount != 3 {
				t.Fatalf("restricted terminal status/outbox/deliveries=%q/%d/%d want %q/1/3", status, eventCount, deliveryCount, tc.wantStatus)
			}
		})
	}
}
