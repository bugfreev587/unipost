//go:build integration

package worker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	appcrypto "github.com/xiaoboyu/unipost-api/internal/crypto"
	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/platform"
	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

type fixedFacebookVideoStatusChecker struct {
	status *platform.FacebookVideoStatus
}

func (c fixedFacebookVideoStatusChecker) CheckVideoStatus(context.Context, string, string) (*platform.FacebookVideoStatus, error) {
	return c.status, nil
}

type recordingFacebookStatusEventBus struct {
	mu     sync.Mutex
	events []string
}

func (b *recordingFacebookStatusEventBus) Publish(_ context.Context, _ string, event string, _ any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
}

func (b *recordingFacebookStatusEventBus) snapshot() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]string(nil), b.events...)
}

func openFacebookStatusIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("PUBLISHING_RESTRICTION_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Fatal("PUBLISHING_RESTRICTION_TEST_DATABASE_URL is required and must point to a temporary localhost PostgreSQL service")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, config, err := testdbguard.OpenValidated(
		"PUBLISHING_RESTRICTION_TEST_DATABASE_URL",
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(ctx, validatedURL)
		},
	)
	if err != nil {
		t.Fatalf("connect temporary PostgreSQL: %v", err)
	}
	schema := fmt.Sprintf("facebook_status_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create isolated schema: %v", err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	config.MaxConns = 6
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatalf("connect isolated schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop isolated schema: %v", err)
		}
		admin.Close()
	})
	return pool
}

func setupFacebookStatusIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
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
			workspace_id TEXT NOT NULL,
			archived_at TIMESTAMPTZ,
			deleted_at TIMESTAMPTZ,
			source TEXT NOT NULL DEFAULT 'api',
			profile_ids TEXT[] NOT NULL DEFAULT '{}',
			quota_hold_reason TEXT,
			quota_hold_at TIMESTAMPTZ,
			quota_hold_original_scheduled_at TIMESTAMPTZ
		);
		CREATE TABLE social_post_results (
			id TEXT PRIMARY KEY,
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
		CREATE TABLE subscriptions (
			id TEXT PRIMARY KEY DEFAULT md5(random()::text || clock_timestamp()::text),
			stripe_customer_id TEXT,
			stripe_subscription_id TEXT,
			status TEXT NOT NULL DEFAULT 'active',
			current_period_start TIMESTAMPTZ,
			current_period_end TIMESTAMPTZ,
			cancel_at_period_end BOOLEAN,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			workspace_id TEXT NOT NULL UNIQUE,
			plan_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			trial_used BOOLEAN NOT NULL DEFAULT FALSE
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
	`)
	if err != nil {
		t.Fatalf("create facebook status integration tables: %v", err)
	}
}

func waitForFacebookStatusSessionBlockedBy(t *testing.T, ctx context.Context, pool *pgxpool.Pool, blockedBackend, blockingBackend int) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var blocked bool
		if err := pool.QueryRow(ctx, `SELECT $1::int = ANY(pg_blocking_pids($2::int))`, blockingBackend, blockedBackend).Scan(&blocked); err != nil {
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

func TestFacebookVideoStatusWorkerTerminalCoordinatorPostgres(t *testing.T) {
	pool := openFacebookStatusIntegrationPool(t)
	setupFacebookStatusIntegrationSchema(t, pool)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	encryptor, err := appcrypto.NewAESEncryptor(strings.Repeat("01", 32))
	if err != nil {
		t.Fatal(err)
	}
	encryptedToken, err := encryptor.Encrypt("facebook-access-token")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := platform.EncodePostMetadata([]platform.PlatformPostInput{{
		AccountID: "account_fb_worker", MediaIDs: []string{"media_fb_worker_b", "media_fb_worker_a"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_posts(id,status,workspace_id,metadata) VALUES
		 ('post_fb_worker_ready','publishing','workspace_fb_worker',$1),
		 ('post_fb_worker_failed','publishing','workspace_fb_worker',$1),
		 ('post_fb_worker_race','publishing','workspace_fb_worker',$1)`, metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO social_post_results(id,post_id,social_account_id,status,external_id,url) VALUES
		 ('result_fb_worker_ready','post_fb_worker_ready','account_fb_worker','processing','video_ready','https://facebook.test/video_ready'),
		 ('result_fb_worker_failed','post_fb_worker_failed','account_fb_worker','processing','video_failed','https://facebook.test/video_failed'),
		 ('result_fb_worker_race','post_fb_worker_race','account_fb_worker','processing','video_race','https://facebook.test/video_race')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO media(id,workspace_id,status) VALUES
	 ('media_fb_worker_a','workspace_fb_worker','uploaded'),
	 ('media_fb_worker_b','workspace_fb_worker','uploaded')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO subscriptions(workspace_id,plan_id) VALUES ('workspace_fb_worker','pro')`); err != nil {
		t.Fatal(err)
	}
	workerConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer workerConn.Release()
	workerBackend := 0
	if err := workerConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&workerBackend); err != nil {
		t.Fatal(err)
	}
	bus := &recordingFacebookStatusEventBus{}
	w := NewFacebookVideoStatusWorker(db.New(workerConn), encryptor, bus)
	baseRow := func(postID, resultID, externalID string) db.ListFacebookVideosAwaitingStatusRow {
		return db.ListFacebookVideosAwaitingStatusRow{
			SocialPostResultID: resultID,
			PostID:             postID,
			WorkspaceID:        "workspace_fb_worker",
			ExternalID:         pgtype.Text{String: externalID, Valid: true},
			Url:                pgtype.Text{String: "https://facebook.test/" + externalID, Valid: true},
			SocialAccountID:    "account_fb_worker",
			PageID:             "page_fb_worker",
			AccessToken:        encryptedToken,
			PostCreatedAt:      pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
		}
	}

	w.checkOne(ctx, fixedFacebookVideoStatusChecker{status: &platform.FacebookVideoStatus{
		VideoStatus: "ready", PostID: "page_fb_worker_42",
	}}, baseRow("post_fb_worker_ready", "result_fb_worker_ready", "video_ready"))
	w.checkOne(ctx, fixedFacebookVideoStatusChecker{status: &platform.FacebookVideoStatus{
		VideoStatus: "error", ErrorMessage: "encoding rejected",
	}}, baseRow("post_fb_worker_failed", "result_fb_worker_failed", "video_failed"))

	var readyResult, readyParent, failedResult, failedParent string
	var usageCount, failureCount, retainedCount int
	if err := pool.QueryRow(ctx, `
		SELECT
		 (SELECT status FROM social_post_results WHERE id='result_fb_worker_ready'),
		 (SELECT status FROM social_posts WHERE id='post_fb_worker_ready'),
		 (SELECT status FROM social_post_results WHERE id='result_fb_worker_failed'),
		 (SELECT status FROM social_posts WHERE id='post_fb_worker_failed'),
		 COALESCE((SELECT post_count FROM usage WHERE workspace_id='workspace_fb_worker'),0),
		 (SELECT COUNT(*)::int FROM post_failures WHERE post_id='post_fb_worker_failed'),
		 (SELECT COUNT(*)::int FROM media_post_usages WHERE post_id IN ('post_fb_worker_ready','post_fb_worker_failed'))
	`).Scan(&readyResult, &readyParent, &failedResult, &failedParent, &usageCount, &failureCount, &retainedCount); err != nil {
		t.Fatal(err)
	}
	if readyResult != "published" || readyParent != "published" || failedResult != "failed" || failedParent != "failed" {
		t.Fatalf("terminal state ready=%q/%q failed=%q/%q", readyResult, readyParent, failedResult, failedParent)
	}
	if usageCount != 1 || failureCount != 1 || retainedCount != 4 {
		t.Fatalf("terminal side effects usage=%d failures=%d retention=%d", usageCount, failureCount, retainedCount)
	}
	var outboxEvents []string
	var outboxDeliveries int
	if err := pool.QueryRow(ctx, `
		SELECT
		 COALESCE(ARRAY_AGG(event_type ORDER BY sequence_number), '{}'::text[]),
		 (SELECT COUNT(*)::int FROM terminal_post_event_deliveries)
		FROM terminal_post_event_outbox
	`).Scan(&outboxEvents, &outboxDeliveries); err != nil {
		t.Fatal(err)
	}
	if len(outboxEvents) != 2 || outboxEvents[0] != "post.published" || outboxEvents[1] != "post.failed" || outboxDeliveries != 6 {
		t.Fatalf("durable terminal events=%v deliveries=%d", outboxEvents, outboxDeliveries)
	}
	if got := bus.snapshot(); len(got) != 0 {
		t.Fatalf("terminal transition performed synchronous event I/O: %v", got)
	}

	lockConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockConn.Release()
	winnerTx, err := lockConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer winnerTx.Rollback(ctx)
	if err := db.New(winnerTx).LockPostDeliveryJobFinalization(ctx, "post_fb_worker_race"); err != nil {
		t.Fatal(err)
	}
	publishedAt := time.Date(2026, 7, 27, 9, 0, 0, 0, time.UTC)
	if _, err := winnerTx.Exec(ctx, `UPDATE social_post_results SET status='published',published_at=$2 WHERE id=$1`, "result_fb_worker_race", publishedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := winnerTx.Exec(ctx, `UPDATE social_posts SET status='published',published_at=$2 WHERE id=$1`, "post_fb_worker_race", publishedAt); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{}, 1)
	go func() {
		w.checkOne(ctx, fixedFacebookVideoStatusChecker{status: &platform.FacebookVideoStatus{
			VideoStatus: "error", ErrorMessage: "late poll must lose",
		}}, baseRow("post_fb_worker_race", "result_fb_worker_race", "video_race"))
		done <- struct{}{}
	}()
	winnerBackend := 0
	if err := winnerTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&winnerBackend); err != nil {
		t.Fatal(err)
	}
	waitForFacebookStatusSessionBlockedBy(t, ctx, pool, workerBackend, winnerBackend)
	if err := winnerTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	<-done
	var raceStatus, raceParent string
	var raceFailures int
	if err := pool.QueryRow(ctx, `SELECT result.status,parent.status,
	 (SELECT COUNT(*)::int FROM post_failures WHERE post_id=parent.id)
	 FROM social_post_results result JOIN social_posts parent ON parent.id=result.post_id
	 WHERE result.id='result_fb_worker_race'`).Scan(&raceStatus, &raceParent, &raceFailures); err != nil {
		t.Fatal(err)
	}
	if raceStatus != "published" || raceParent != "published" || raceFailures != 0 {
		t.Fatalf("published CAS winner result=%q parent=%q failures=%d", raceStatus, raceParent, raceFailures)
	}
	var finalOutboxEvents, finalOutboxDeliveries int
	if err := pool.QueryRow(ctx, `
		SELECT
		 (SELECT COUNT(*)::int FROM terminal_post_event_outbox),
		 (SELECT COUNT(*)::int FROM terminal_post_event_deliveries)
	`).Scan(&finalOutboxEvents, &finalOutboxDeliveries); err != nil {
		t.Fatal(err)
	}
	if finalOutboxEvents != 2 || finalOutboxDeliveries != 6 {
		t.Fatalf("late losing poll changed durable events=%d deliveries=%d", finalOutboxEvents, finalOutboxDeliveries)
	}
	if got := bus.snapshot(); len(got) != 0 {
		t.Fatalf("late losing poll performed synchronous event I/O: %v", got)
	}
}
