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
	"github.com/xiaoboyu/unipost-api/internal/platform"
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
			x_app_mode TEXT
		);
		CREATE TABLE platform_publishing_restrictions (
			platform TEXT PRIMARY KEY,
			enabled BOOLEAN NOT NULL,
			restricted_plan_ids TEXT[] NOT NULL
		);
		CREATE TABLE subscriptions (
			workspace_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		INSERT INTO profiles (id, workspace_id) VALUES ('profile_1', 'workspace_1');
	`)
	if err != nil {
		t.Fatalf("create restricted-delivery integration tables: %v", err)
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
		_, err := pool.Exec(ctx, `
			INSERT INTO social_posts (id, status)
			VALUES ('post_partial_restricted', 'publishing');
			INSERT INTO social_post_results (id, post_id, social_account_id, status) VALUES
				('result_partial_restricted', 'post_partial_restricted', 'account_partial_restricted', 'processing'),
				('result_partial_published', 'post_partial_restricted', 'account_partial_published', 'published');
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
		if parentStatus != "partial" || !parentPublishedAt.Valid {
			t.Fatalf("partial restricted parent = status:%q published_at:%#v, want partial/non-null", parentStatus, parentPublishedAt)
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
