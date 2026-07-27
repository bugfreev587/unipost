//go:build integration

package worker

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

const publishingRestrictionWorkerIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openPublishingRestrictionWorkerIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionWorkerIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionWorkerIntegrationDatabaseEnv)
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse PostgreSQL integration URL: %v", err)
	}
	host := strings.TrimSpace(config.ConnConfig.Host)
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		t.Fatalf("%s must use a loopback host, got %q", publishingRestrictionWorkerIntegrationDatabaseEnv, host)
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL integration service: %v", err)
	}
	schema := fmt.Sprintf("pr270_worker_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		admin.Close()
		t.Fatalf("create isolated PostgreSQL schema: %v", err)
	}

	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		admin.Close()
		t.Fatalf("connect isolated PostgreSQL schema: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Errorf("drop isolated PostgreSQL schema: %v", err)
		}
		admin.Close()
	})
	return pool
}

func applyWorkerMigrationUp(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()
	raw, err := os.ReadFile("../db/migrations/" + filename)
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration %s must have exactly one Down section", filename)
	}
	up := strings.Replace(parts[0], "-- +goose Up", "", 1)
	if _, err := pool.Exec(context.Background(), up); err != nil {
		t.Fatalf("apply migration %s Up: %v", filename, err)
	}
}

func setupPublishingRestrictionWorkerSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT);
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE media_post_usages (cleanup_after_at TIMESTAMPTZ);
		CREATE TABLE subscriptions (
			workspace_id TEXT NOT NULL,
			plan_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE TABLE workspace_members (
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL
		);
		CREATE TABLE social_accounts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			status TEXT NOT NULL,
			disconnected_at TIMESTAMPTZ
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range []string{
		"092_email_send_attempts.sql",
		"122_platform_publishing_restrictions.sql",
		"124_publishing_restriction_email_send_gate.sql",
		"125_publishing_restriction_failed_recipient_retryability.sql",
	} {
		applyWorkerMigrationUp(t, pool, migration)
	}
	_, err = pool.Exec(context.Background(), `
		INSERT INTO users (id) VALUES ('worker_user_1'), ('worker_user_2');
		INSERT INTO platform_publishing_restriction_email_campaigns (
			id, restriction_id, cycle_id, campaign_type, subject_snapshot,
			body_snapshot, restriction_version
		)
		SELECT 'worker_campaign', id, 'worker_cycle', 'restriction_notice',
		       'subject', 'body', version
		FROM platform_publishing_restrictions
		WHERE platform = 'tiktok';
	`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishingRestrictionTerminalFailureManualRetryPreservesProviderIdentity(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_manual_retry", "worker_user_1", "owner@example.com", time.Now())
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restrictions
		SET enabled=TRUE, cycle_id='worker_cycle'
		WHERE platform='tiktok';
		UPDATE users SET email='owner@example.com' WHERE id='worker_user_1';
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_workspace_ids=ARRAY['workspace_1']::TEXT[]
		WHERE id='recipient_manual_retry';
		INSERT INTO workspaces (id) VALUES ('workspace_1');
		INSERT INTO subscriptions (workspace_id, plan_id) VALUES ('workspace_1', 'free');
		INSERT INTO workspace_members (workspace_id, user_id, role, status)
		VALUES ('workspace_1', 'worker_user_1', 'owner', 'active');
		INSERT INTO social_accounts (id, workspace_id, platform, status)
		VALUES ('social_1', 'workspace_1', 'tiktok', 'active');
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	firstClaim, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstClaim) != 1 {
		t.Fatalf("first claim = %+v, want one recipient", firstClaim)
	}
	first := firstClaim[0]
	firstAuditKey := fmt.Sprintf("%s:g%d:a%d", first.IdempotencyKey, first.AttemptGeneration, first.AttemptCount)
	if first.AttemptGeneration != 1 || first.AttemptCount != 1 {
		t.Fatalf("first attempt generation/count = %d/%d, want 1/1", first.AttemptGeneration, first.AttemptCount)
	}
	var nextAttemptBefore time.Time
	if err := pool.QueryRow(ctx, `
		SELECT next_attempt_at
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_manual_retry'
	`).Scan(&nextAttemptBefore); err != nil {
		t.Fatal(err)
	}

	terminalReason := "send outcome unknown after provider request; manual review required"
	if err := store.MarkPublishingRestrictionEmailRecipientTerminalFailed(ctx, first.RecipientID, terminalReason); err != nil {
		t.Fatal(err)
	}
	if err := store.RefreshPublishingRestrictionEmailCampaign(ctx, first.CampaignID); err != nil {
		t.Fatal(err)
	}
	var status, lastError string
	var retryable bool
	var claimedAt *time.Time
	var nextAttemptAfter time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, retryable, claimed_at, last_error, next_attempt_at
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_manual_retry'
	`).Scan(&status, &retryable, &claimedAt, &lastError, &nextAttemptAfter); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || retryable || claimedAt != nil || !strings.Contains(lastError, "manual review required") {
		t.Fatalf("terminal recipient status=%q retryable=%v claimed_at=%v error=%q", status, retryable, claimedAt, lastError)
	}
	if !nextAttemptAfter.Equal(nextAttemptBefore) {
		t.Fatalf("terminal failure changed next_attempt_at from %v to %v", nextAttemptBefore, nextAttemptAfter)
	}

	campaignStore := publishingrestrictions.NewPostgresStore(pool)
	if _, err := campaignStore.RetryFailedCampaign(ctx, "tiktok", first.CampaignID); err != nil {
		t.Fatalf("retry failed campaign: %v", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO email_send_attempts (
			id, event_key, recipient_email, provider, idempotency_key,
			delivery_class, status
		) VALUES (
			'attempt_1', 'email.publishing_restriction.restriction_notice.v1',
			'owner@example.com', 'loops', 'fixture-audit-attempt-key',
			'service_alert', 'pending'
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	sender := &captureRestrictionCampaignSender{}
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template")
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if len(sender.emails) != 1 {
		var diagnosticRecipientStatus, diagnosticRecipientError, diagnosticCampaignStatus string
		if diagnosticErr := pool.QueryRow(ctx, `
			SELECT recipient.status, COALESCE(recipient.last_error, ''), campaign.status
			FROM platform_publishing_restriction_email_recipients recipient
			JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
			WHERE recipient.id='recipient_manual_retry'
		`).Scan(&diagnosticRecipientStatus, &diagnosticRecipientError, &diagnosticCampaignStatus); diagnosticErr != nil {
			t.Fatalf("manual retry sender attempts = %d; diagnostics failed: %v", len(sender.emails), diagnosticErr)
		}
		t.Fatalf(
			"manual retry sender attempts = %d, want exactly 1; recipient status=%q error=%q campaign status=%q",
			len(sender.emails), diagnosticRecipientStatus, diagnosticRecipientError, diagnosticCampaignStatus,
		)
	}
	retried := sender.emails[0]
	if retried.IdempotencyKey != first.IdempotencyKey {
		t.Fatalf("provider idempotency key changed from %q to %q", first.IdempotencyKey, retried.IdempotencyKey)
	}
	if retried.Audit.AttemptIdempotencyKey == firstAuditKey {
		t.Fatalf("audit attempt key did not change from %q", firstAuditKey)
	}
	wantRetryAuditKey := first.IdempotencyKey + ":g2:a1"
	if retried.Audit.AttemptIdempotencyKey != wantRetryAuditKey {
		t.Fatalf("manual retry audit key = %q, want %q", retried.Audit.AttemptIdempotencyKey, wantRetryAuditKey)
	}
	var retriedGeneration, retriedAttemptCount int
	if err := pool.QueryRow(ctx, `
		SELECT attempt_generation, attempt_count
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_manual_retry'
	`).Scan(&retriedGeneration, &retriedAttemptCount); err != nil {
		t.Fatal(err)
	}
	if retriedGeneration != 2 || retriedAttemptCount != 1 {
		t.Fatalf("manual retry generation/count = %d/%d, want 2/1", retriedGeneration, retriedAttemptCount)
	}
}

func insertPendingRestrictionRecipient(t *testing.T, pool *pgxpool.Pool, id, userID, email string, createdAt time.Time) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			status, next_attempt_at, idempotency_key, created_at, updated_at
		) VALUES ($1, 'worker_campaign', $2, $3, $3, 'pending', NOW(), $4, $5, $5)
	`, id, userID, email, "worker_cycle:restriction_notice:"+userID, createdAt)
	if err != nil {
		t.Fatal(err)
	}
}

func TestPublishingRestrictionRecipientClaimSkipsRowLockedByConcurrentTransaction(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_locked", "worker_user_1", "locked@example.com", time.Now().Add(-time.Minute))
	insertPendingRestrictionRecipient(t, pool, "recipient_available", "worker_user_2", "available@example.com", time.Now())

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx1.Rollback(ctx)
	var tx1Recipient string
	err = tx1.QueryRow(ctx, `
		WITH candidate AS MATERIALIZED (
			SELECT recipient.id
			FROM platform_publishing_restriction_email_recipients recipient
			JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
			WHERE campaign.status IN ('queued','running')
			  AND recipient.status='pending'
			ORDER BY recipient.created_at
			FOR UPDATE OF recipient SKIP LOCKED
			LIMIT 1
		)
		UPDATE platform_publishing_restriction_email_recipients recipient
		SET status='sending', attempt_count=attempt_count+1, retryable=FALSE,
		    claimed_at=NOW(), updated_at=NOW()
		FROM candidate
		WHERE recipient.id=candidate.id
		RETURNING recipient.id
	`).Scan(&tx1Recipient)
	if err != nil {
		t.Fatal(err)
	}
	if tx1Recipient != "recipient_locked" {
		t.Fatalf("tx1 recipient = %q, want recipient_locked", tx1Recipient)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	tx2Context, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	work, err := store.ClaimPublishingRestrictionEmailRecipients(tx2Context, 1)
	if err != nil {
		t.Fatalf("tx2 claim: %v", err)
	}
	if len(work) != 1 || work[0].RecipientID != "recipient_available" {
		t.Fatalf("tx2 work = %+v, want only recipient_available while recipient_locked is held", work)
	}
}

func TestPublishingRestrictionStaleSendingTerminatesAndCannotBeReclaimed(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO email_send_attempts (
			id, event_key, recipient_email, provider, idempotency_key,
			delivery_class, status
		) VALUES (
			'attempt_stale', 'email.publishing_restriction.restriction_notice.v1',
			'stale@example.com', 'loops', 'attempt-stale-provider-key',
			'service_alert', 'pending'
		);

		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			status, attempt_count, next_attempt_at, claimed_at, idempotency_key,
			email_send_attempt_id, retryable, created_at, updated_at
		) VALUES (
			'recipient_stale', 'worker_campaign', 'worker_user_1',
			'stale@example.com', 'stale@example.com', 'sending', 1,
			NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '20 minutes',
			'worker_cycle:restriction_notice:worker_user_1', 'attempt_stale',
			FALSE, NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '20 minutes'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("first claim returned stale send work: %+v", work)
	}

	var recipientStatus string
	var retryable bool
	var claimedAt *time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, retryable, claimed_at
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_stale'
	`).Scan(&recipientStatus, &retryable, &claimedAt); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "failed" || retryable || claimedAt != nil {
		t.Fatalf("stale recipient status=%q retryable=%v claimed_at=%v, want failed/false/NULL", recipientStatus, retryable, claimedAt)
	}

	var auditStatus, auditError string
	if err := pool.QueryRow(ctx, `SELECT status, last_error FROM email_send_attempts WHERE id='attempt_stale'`).Scan(&auditStatus, &auditError); err != nil {
		t.Fatal(err)
	}
	if auditStatus != "failed" || !strings.Contains(auditError, "outcome unknown") {
		t.Fatalf("stale audit status=%q error=%q, want terminal failed unknown-outcome evidence", auditStatus, auditError)
	}

	work, err = store.ClaimPublishingRestrictionEmailRecipients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("second claim reclaimed terminal stale recipient: %+v", work)
	}
}
