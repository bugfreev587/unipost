//go:build integration

package handler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/postfailures"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

const restrictedDeliveryIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openRestrictedDeliveryIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(restrictedDeliveryIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to a temporary localhost PostgreSQL service", restrictedDeliveryIntegrationDatabaseEnv)
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL integration URL: %v", err)
	}
	host := strings.TrimSpace(config.ConnConfig.Host)
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("%s must use a loopback host, got %q", restrictedDeliveryIntegrationDatabaseEnv, host)
	}

	setupContext, cancelSetup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelSetup()
	admin, err := pgxpool.New(setupContext, databaseURL)
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
		CREATE TABLE social_post_results (
			id TEXT PRIMARY KEY,
			status TEXT NOT NULL,
			external_id TEXT,
			error_message TEXT,
			published_at TIMESTAMPTZ,
			url TEXT,
			debug_curl TEXT,
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
			id TEXT PRIMARY KEY,
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
		CREATE TABLE media (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			status TEXT NOT NULL,
			usage_version BIGINT NOT NULL DEFAULT 0
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
	`)
	if err != nil {
		t.Fatalf("create restricted-delivery integration tables: %v", err)
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
		ErrorMessage:     postfailures.ToText(publishingrestrictions.UserMessage),
		ID:               jobID,
		LeaseOwner:       postfailures.ToText(owner),
		LastAttemptAt:    pgtype.Timestamptz{Time: attemptedAt, Valid: true},
		NextAction:       postfailures.ToText(publishingrestrictions.NextAction),
		ErrorSource:      postfailures.ToText(postfailures.ErrorSourceUnipost),
		ErrorTemporality: postfailures.ToText(postfailures.ErrorTemporalityTemporary),
		PostStatus:       "publishing",
		CleanupAfterAt:   pgtype.Timestamptz{Time: attemptedAt.Add(60 * 24 * time.Hour), Valid: true},
	}
}

func TestFinalizeRestrictedPostDeliveryJobPostgresLeaseAtomicity(t *testing.T) {
	pool := openRestrictedDeliveryIntegrationPool(t)
	setupRestrictedDeliveryIntegrationSchema(t, pool)

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
			INSERT INTO social_post_results (id, status)
			VALUES ('result_stale_owner', 'pending')
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
	})

	t.Run("owned lease transitions job and result atomically", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		attemptedAt := time.Date(2026, 7, 26, 21, 0, 0, 0, time.UTC)
		_, err := pool.Exec(ctx, `
			INSERT INTO social_post_results (
				id, status, external_id, published_at, url, debug_curl, publish_token,
				platform_error_code, provider_error, x_credits_counted,
				x_credit_operation, x_credit_catalog_version, x_credit_billing_mode
			) VALUES (
				'result_owned', 'processing', 'stale_external', $1,
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
		var backendB int
		if err := ownerB.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&backendB); err != nil {
			t.Fatal(err)
		}

		attemptedAt := time.Date(2026, 7, 26, 22, 0, 0, 0, time.UTC)
		restrictionDeadline := attemptedAt.Add(60 * 24 * time.Hour)
		planDeadline := attemptedAt.Add(30 * 24 * time.Hour)
		_, err = ownerA.Exec(ctx, `
			INSERT INTO social_post_results (id, status)
			VALUES ('result_retention_race', 'processing')
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
			VALUES ('media_retention_race', 'workspace_1', 'uploaded')
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
		params.MediaIds = []string{"media_retention_race"}
		params.CleanupAfterAt = pgtype.Timestamptz{Time: restrictionDeadline, Valid: true}
		if _, err := db.New(txA).FinalizeRestrictedPostDeliveryJob(ctx, params); err != nil {
			t.Fatalf("restriction finalization: %v", err)
		}

		bStarted := make(chan struct{})
		bDone := make(chan error, 1)
		go func() {
			txB, beginErr := ownerB.Begin(ctx)
			if beginErr != nil {
				bDone <- beginErr
				return
			}
			defer txB.Rollback(ctx)
			close(bStarted)
			if _, updateErr := txB.Exec(ctx, `
				UPDATE social_post_results
				SET status = 'published', external_id = 'newer_external_id',
				    error_message = NULL, published_at = $1,
				    error_code = NULL, failure_stage = NULL
				WHERE id = 'result_retention_race'
			`, attemptedAt.Add(time.Minute)); updateErr != nil {
				bDone <- updateErr
				return
			}
			if _, updateErr := txB.Exec(ctx, `
				UPDATE media_post_usages
				SET post_status = 'published', cleanup_after_at = $1,
				    retention_reason = 'plan_status', updated_at = NOW()
				WHERE media_id = 'media_retention_race'
				  AND post_id = 'post_retention_race'
			`, planDeadline); updateErr != nil {
				bDone <- updateErr
				return
			}
			bDone <- txB.Commit(ctx)
		}()
		<-bStarted

		lockDeadline := time.Now().Add(2 * time.Second)
		for {
			var waiting bool
			if err := pool.QueryRow(ctx, `
				SELECT COALESCE(wait_event_type = 'Lock', FALSE)
				FROM pg_stat_activity WHERE pid = $1
			`, backendB).Scan(&waiting); err != nil {
				t.Fatal(err)
			}
			if waiting {
				break
			}
			if time.Now().After(lockDeadline) {
				t.Fatal("newer retry did not block on the atomic restriction result transition")
			}
			time.Sleep(10 * time.Millisecond)
		}
		if err := txA.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		if err := <-bDone; err != nil {
			t.Fatalf("newer retry transaction: %v", err)
		}

		var status, postStatus, reason string
		var cleanupAt pgtype.Timestamptz
		if err := pool.QueryRow(ctx, `
			SELECT result.status, usage.post_status, usage.retention_reason, usage.cleanup_after_at
			FROM social_post_results result
			JOIN media_post_usages usage
			  ON usage.post_id = 'post_retention_race'
			WHERE result.id = 'result_retention_race'
		`).Scan(&status, &postStatus, &reason, &cleanupAt); err != nil {
			t.Fatal(err)
		}
		if status != "published" || postStatus != "published" || reason != "plan_status" || !cleanupAt.Valid || !cleanupAt.Time.Equal(planDeadline) {
			t.Fatalf("newer durable state = status:%q usage:%q reason:%q cleanup:%#v, want published plan retention %s", status, postStatus, reason, cleanupAt, planDeadline)
		}
	})
}
