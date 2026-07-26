//go:build integration

package worker

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const publishingRestrictionWorkerIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openPublishingRestrictionWorkerIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionWorkerIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionWorkerIntegrationDatabaseEnv)
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

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		admin.Close()
		t.Fatalf("parse PostgreSQL integration URL: %v", err)
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
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE media_post_usages (cleanup_after_at TIMESTAMPTZ);
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
