//go:build integration

package worker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/db"
	"github.com/xiaoboyu/unipost-api/internal/loops"
	"github.com/xiaoboyu/unipost-api/internal/publishingrestrictions"
)

const publishingRestrictionWorkerIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

type integrationUnknownLifecycleClient struct {
	sends int
}

func (*integrationUnknownLifecycleClient) Enabled() bool { return true }
func (*integrationUnknownLifecycleClient) UpsertContact(context.Context, loops.Contact) error {
	return nil
}
func (*integrationUnknownLifecycleClient) SendEvent(context.Context, loops.Event) error { return nil }
func (c *integrationUnknownLifecycleClient) SendTransactional(context.Context, loops.TransactionalEmail) error {
	c.sends++
	return &loops.SendOutcomeUnknownError{Err: fmt.Errorf("response lost")}
}

type integrationSuccessLifecycleClient struct {
	sends  int
	emails []loops.TransactionalEmail
}

func (*integrationSuccessLifecycleClient) Enabled() bool { return true }
func (*integrationSuccessLifecycleClient) UpsertContact(context.Context, loops.Contact) error {
	return nil
}
func (*integrationSuccessLifecycleClient) SendEvent(context.Context, loops.Event) error { return nil }
func (c *integrationSuccessLifecycleClient) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	c.sends++
	c.emails = append(c.emails, email)
	return nil
}

type integrationDefinitiveFailureLifecycleClient struct {
	cancel context.CancelFunc
	sends  int
	emails []loops.TransactionalEmail
}

func (*integrationDefinitiveFailureLifecycleClient) Enabled() bool { return true }
func (*integrationDefinitiveFailureLifecycleClient) UpsertContact(context.Context, loops.Contact) error {
	return nil
}
func (*integrationDefinitiveFailureLifecycleClient) SendEvent(context.Context, loops.Event) error {
	return nil
}
func (c *integrationDefinitiveFailureLifecycleClient) SendTransactional(_ context.Context, email loops.TransactionalEmail) error {
	c.sends++
	c.emails = append(c.emails, email)
	c.cancel()
	return fmt.Errorf("loops: 503: temporarily unavailable")
}

type failOnceEmailAuditFinalizationStore struct {
	delegate loops.EmailAuditStore
	err      error
	failed   bool
}

func (s *failOnceEmailAuditFinalizationStore) CreateEmailSendAttempt(ctx context.Context, attempt loops.EmailSendAttempt) (loops.EmailSendAttemptRecord, error) {
	return s.delegate.CreateEmailSendAttempt(ctx, attempt)
}
func (s *failOnceEmailAuditFinalizationStore) CreateSkippedEmailSendAttempt(ctx context.Context, attempt loops.EmailSendAttempt, reason string) (loops.EmailSendAttemptRecord, error) {
	return s.delegate.CreateSkippedEmailSendAttempt(ctx, attempt, reason)
}
func (s *failOnceEmailAuditFinalizationStore) MarkEmailSendAttemptSent(ctx context.Context, id string) error {
	return s.delegate.MarkEmailSendAttemptSent(ctx, id)
}
func (s *failOnceEmailAuditFinalizationStore) MarkEmailSendAttemptFailed(ctx context.Context, id, reason string) error {
	if !s.failed {
		s.failed = true
		return s.err
	}
	return s.delegate.MarkEmailSendAttemptFailed(ctx, id, reason)
}

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

	terminalReason := "provider request outcome unknown after attempting send; manual review required"
	if err := store.MarkPublishingRestrictionEmailRecipientTerminalFailed(ctx, first.RecipientID, terminalReason); err != nil {
		t.Fatal(err)
	}
	lostClaimErr := store.MarkPublishingRestrictionEmailRecipientTerminalFailed(ctx, first.RecipientID, "must not overwrite terminal row")
	if lostClaimErr == nil {
		t.Fatal("terminal transition for a non-sending recipient returned nil, want lost-claim error")
	}
	if !strings.Contains(lostClaimErr.Error(), "lost active send claim") {
		t.Fatalf("terminal zero-row error = %q, want clear lost active send claim", lostClaimErr)
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
	worker := NewPublishingRestrictionEmailWorker(store, sender, "restriction-template", "recovery-template", publishingrestrictions.CampaignDeliveryReadiness{
		PreviewSecret: true, AuditedSender: true, RestrictionTemplate: true, RecoveryTemplate: true,
	})
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

func TestPublishingRestrictionManualRetryRejectsExpiredProviderIdempotencyWindow(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_expired_retry", "worker_user_1", "owner@example.com", time.Now().Add(-25*time.Hour))
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET status='failed', retryable=FALSE, last_error='manual review required'
		WHERE id='recipient_expired_retry';
		UPDATE platform_publishing_restriction_email_campaigns
		SET status='completed_with_failures', created_at=NOW()-INTERVAL '25 hours', completed_at=NOW()
		WHERE id='worker_campaign';
	`)
	if err != nil {
		t.Fatal(err)
	}

	_, err = publishingrestrictions.NewPostgresStore(pool).RetryFailedCampaign(ctx, "tiktok", "worker_campaign")
	if !errors.Is(err, publishingrestrictions.ErrCampaignRetryExpired) {
		t.Fatalf("retry error = %v, want ErrCampaignRetryExpired", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM platform_publishing_restriction_email_recipients WHERE id='recipient_expired_retry'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatalf("recipient status = %q, want failed", status)
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

func TestPublishingRestrictionRecipientSentTransitionAggregatesCampaignAtomically(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_sent", "worker_user_1", "sent@example.com", time.Now())
	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 {
		t.Fatalf("claim = %+v, want one recipient", work)
	}

	if err := store.MarkPublishingRestrictionEmailRecipientSent(ctx, work[0].RecipientID); err != nil {
		t.Fatal(err)
	}

	var recipientStatus, campaignStatus string
	var sentCount, pendingCount int
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, campaign.status, campaign.sent_count, campaign.pending_count
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
		WHERE recipient.id='recipient_sent'
	`).Scan(&recipientStatus, &campaignStatus, &sentCount, &pendingCount); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "sent" || campaignStatus != "completed" || sentCount != 1 || pendingCount != 0 {
		t.Fatalf(
			"recipient=%q campaign=%q sent=%d pending=%d, want sent/completed/1/0",
			recipientStatus,
			campaignStatus,
			sentCount,
			pendingCount,
		)
	}
}

func TestPublishingRestrictionStaleSendingReconcilesSentAuditWithoutReclaiming(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_reconcile_sent", "worker_user_1", "sent@example.com", time.Now())
	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 {
		t.Fatalf("initial claim = %+v, want one recipient", work)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO email_send_attempts (
			id, event_key, recipient_email, provider, idempotency_key,
			delivery_class, status, sent_at
		) VALUES (
			'attempt_reconcile_sent', 'email.publishing_restriction.restriction_notice.v1',
			'sent@example.com', 'loops', 'attempt-reconcile-sent',
			'service_alert', 'sent', NOW()
		)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkPublishingRestrictionEmailAttempt(
		ctx,
		work[0].RecipientID,
		"attempt_reconcile_sent",
		work[0].AttemptCount,
		work[0].AttemptGeneration,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET claimed_at=NOW()-INTERVAL '20 minutes'
		WHERE id='recipient_reconcile_sent'
	`); err != nil {
		t.Fatal(err)
	}

	work, err = store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("reconciliation reclaimed work for a second provider send: %+v", work)
	}
	var recipientStatus, campaignStatus string
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, campaign.status
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
		WHERE recipient.id='recipient_reconcile_sent'
	`).Scan(&recipientStatus, &campaignStatus); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "sent" || campaignStatus != "completed" {
		t.Fatalf("recipient=%q campaign=%q, want sent/completed", recipientStatus, campaignStatus)
	}
}

func TestPublishingRestrictionStaleSendingRecoversDefinitiveFailureAsRetryable(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO email_send_attempts (
			id, event_key, recipient_email, provider, idempotency_key,
			delivery_class, status, last_error
		) VALUES (
			'attempt_definitive_failure', 'email.publishing_restriction.restriction_notice.v1',
			'definitive@example.com', 'loops', 'attempt-definitive-failure',
			'service_alert', 'failed', '{"outcome_class":"provider_definitive_failure","error":"loops: 503: temporarily unavailable"}'
		);
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			status, attempt_count, next_attempt_at, claimed_at, idempotency_key,
			email_send_attempt_id, retryable, created_at, updated_at
		) VALUES (
			'recipient_definitive_failure', 'worker_campaign', 'worker_user_1',
			'definitive@example.com', 'definitive@example.com', 'sending', 1,
			NOW() - INTERVAL '20 minutes', NOW() - INTERVAL '20 minutes',
			'worker_cycle:restriction_notice:worker_user_1', 'attempt_definitive_failure',
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
		t.Fatalf("definitive failure retried before backoff: %+v", work)
	}
	var status, lastError string
	var retryable bool
	if err := pool.QueryRow(ctx, `
		SELECT status, retryable, last_error
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_definitive_failure'
	`).Scan(&status, &retryable, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !retryable || !strings.Contains(lastError, "503") || strings.Contains(lastError, "outcome unknown") {
		t.Fatalf("recipient status=%q retryable=%v error=%q, want retryable definitive failure", status, retryable, lastError)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET next_attempt_at=NOW()
		WHERE id='recipient_definitive_failure'
	`); err != nil {
		t.Fatal(err)
	}
	work, err = store.ClaimPublishingRestrictionEmailRecipients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 || work[0].RecipientID != "recipient_definitive_failure" {
		t.Fatalf("retry claim = %+v, want definitive failure recipient", work)
	}
	if work[0].IdempotencyKey != "worker_cycle:restriction_notice:worker_user_1" || work[0].AttemptCount != 2 {
		t.Fatalf("retry identity key=%q attempt=%d", work[0].IdempotencyKey, work[0].AttemptCount)
	}
}

func TestPublishingRestrictionDefinitiveFailureRejectsLostSendClaim(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	store := NewPostgresPublishingRestrictionEmailStore(pool)

	err := store.MarkPublishingRestrictionEmailRecipientFailed(
		context.Background(),
		"missing_recipient",
		"loops: 503: temporarily unavailable",
	)
	if err == nil || !strings.Contains(err.Error(), "lost active send claim") {
		t.Fatalf("definitive failure transition error = %v, want lost active send claim", err)
	}
}

func TestPublishingRestrictionCampaignRefreshRejectsMissingCampaign(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	store := NewPostgresPublishingRestrictionEmailStore(pool)

	err := store.RefreshPublishingRestrictionEmailCampaign(context.Background(), "missing_campaign")
	if err == nil || !strings.Contains(err.Error(), "affected 0 rows") {
		t.Fatalf("campaign refresh error = %v, want affected 0 rows", err)
	}
}

func TestPublishingRestrictionAuditFinalizationFailureReconcilesWithoutDuplicateSend(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_audit_reconcile", "worker_user_1", "owner@example.com", time.Now())
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restrictions
		SET enabled=TRUE, cycle_id='worker_cycle'
		WHERE platform='tiktok';
		UPDATE users SET email='owner@example.com' WHERE id='worker_user_1';
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_workspace_ids=ARRAY['workspace_1']::TEXT[]
		WHERE id='recipient_audit_reconcile';
		INSERT INTO workspaces (id) VALUES ('workspace_1');
		INSERT INTO subscriptions (workspace_id, plan_id) VALUES ('workspace_1', 'free');
		INSERT INTO workspace_members (workspace_id, user_id, role, status)
		VALUES ('workspace_1', 'worker_user_1', 'owner', 'active');
		INSERT INTO social_accounts (id, workspace_id, platform, status)
		VALUES ('social_audit_reconcile', 'workspace_1', 'tiktok', 'active');
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	client := &integrationUnknownLifecycleClient{}
	auditFailure := fmt.Errorf("audit finalization unavailable")
	auditStore := &failOnceEmailAuditFinalizationStore{
		delegate: loops.NewPostgresEmailAuditStore(db.New(pool)),
		err:      auditFailure,
	}
	worker := NewPublishingRestrictionEmailWorker(
		store,
		loops.NewAuditedClient(client, auditStore),
		"restriction-template",
		"recovery-template",
		readyPublishingRestrictionCampaignDelivery(),
	)

	err = worker.ProcessBatch(ctx)
	if err == nil || !strings.Contains(err.Error(), auditFailure.Error()) {
		t.Fatalf("first ProcessBatch error = %v, want audit finalization failure", err)
	}
	if client.sends != 1 {
		t.Fatalf("provider sends after failed finalization = %d, want 1", client.sends)
	}
	var recipientStatus, auditStatus string
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, attempt.status
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN email_send_attempts attempt ON attempt.id=recipient.email_send_attempt_id
		WHERE recipient.id='recipient_audit_reconcile'
	`).Scan(&recipientStatus, &auditStatus); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "sending" || auditStatus != "pending" {
		t.Fatalf("recipient=%q audit=%q, want sending/pending before reconciliation", recipientStatus, auditStatus)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET claimed_at=NOW()-INTERVAL '20 minutes'
		WHERE id='recipient_audit_reconcile'
	`); err != nil {
		t.Fatal(err)
	}

	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("reconciliation reclaimed unknown provider outcome: %+v", work)
	}
	var retryable bool
	var recipientError, auditError string
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, recipient.retryable, recipient.last_error,
		       attempt.status, attempt.last_error
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN email_send_attempts attempt ON attempt.id=recipient.email_send_attempt_id
		WHERE recipient.id='recipient_audit_reconcile'
	`).Scan(&recipientStatus, &retryable, &recipientError, &auditStatus, &auditError); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "failed" || retryable || auditStatus != "failed" ||
		!strings.Contains(recipientError, "outcome unknown") || !strings.Contains(auditError, "outcome unknown") {
		t.Fatalf(
			"reconciled recipient=%q retryable=%v error=%q audit=%q audit_error=%q",
			recipientStatus,
			retryable,
			recipientError,
			auditStatus,
			auditError,
		)
	}
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if client.sends != 1 {
		t.Fatalf("provider sends after reconciliation = %d, want no duplicate", client.sends)
	}
}

func TestPublishingRestrictionCanceledDefinitiveFailureRecoversAfterRecipientWriteFailure(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx, cancel := context.WithCancel(context.Background())
	insertPendingRestrictionRecipient(t, pool, "recipient_definitive_reconcile", "worker_user_1", "owner@example.com", time.Now())
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restrictions
		SET enabled=TRUE, cycle_id='worker_cycle'
		WHERE platform='tiktok';
		UPDATE users SET email='owner@example.com' WHERE id='worker_user_1';
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_workspace_ids=ARRAY['workspace_1']::TEXT[]
		WHERE id='recipient_definitive_reconcile';
		INSERT INTO workspaces (id) VALUES ('workspace_1');
		INSERT INTO subscriptions (workspace_id, plan_id) VALUES ('workspace_1', 'free');
		INSERT INTO workspace_members (workspace_id, user_id, role, status)
		VALUES ('workspace_1', 'worker_user_1', 'owner', 'active');
		INSERT INTO social_accounts (id, workspace_id, platform, status)
		VALUES ('social_definitive_reconcile', 'workspace_1', 'tiktok', 'active');
		CREATE FUNCTION fail_recipient_failure_transition() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			IF NEW.status = 'failed' THEN
				RAISE EXCEPTION 'recipient failure transition unavailable';
			END IF;
			RETURN NEW;
		END
		$$;
		CREATE TRIGGER fail_recipient_failure_transition
		BEFORE UPDATE ON platform_publishing_restriction_email_recipients
		FOR EACH ROW EXECUTE FUNCTION fail_recipient_failure_transition();
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	client := &integrationDefinitiveFailureLifecycleClient{cancel: cancel}
	worker := NewPublishingRestrictionEmailWorker(
		store,
		loops.NewAuditedClient(client, loops.NewPostgresEmailAuditStore(db.New(pool))),
		"restriction-template",
		"recovery-template",
		readyPublishingRestrictionCampaignDelivery(),
	)

	err = worker.ProcessBatch(ctx)
	if err == nil || !strings.Contains(err.Error(), "recipient failure transition unavailable") {
		t.Fatalf("ProcessBatch error = %v, want recipient transition failure", err)
	}
	if ctx.Err() == nil {
		t.Fatal("provider did not cancel parent context")
	}
	if client.sends != 1 || len(client.emails) != 1 {
		t.Fatalf("provider sends=%d emails=%d, want one", client.sends, len(client.emails))
	}
	providerKey := client.emails[0].IdempotencyKey
	if providerKey != "worker_cycle:restriction_notice:worker_user_1" {
		t.Fatalf("provider idempotency key = %q", providerKey)
	}

	recoveryCtx := context.Background()
	var recipientStatus, auditStatus, auditError string
	if err := pool.QueryRow(recoveryCtx, `
		SELECT recipient.status, attempt.status, attempt.last_error
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN email_send_attempts attempt ON attempt.id=recipient.email_send_attempt_id
		WHERE recipient.id='recipient_definitive_reconcile'
	`).Scan(&recipientStatus, &auditStatus, &auditError); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "sending" || auditStatus != "failed" ||
		!strings.Contains(auditError, "503") || strings.Contains(auditError, "outcome unknown") {
		t.Fatalf("recipient=%q audit=%q audit_error=%q, want sending/definitive failed", recipientStatus, auditStatus, auditError)
	}
	_, err = pool.Exec(recoveryCtx, `
		DROP TRIGGER fail_recipient_failure_transition ON platform_publishing_restriction_email_recipients;
		UPDATE platform_publishing_restriction_email_recipients
		SET claimed_at=NOW()-INTERVAL '20 minutes'
		WHERE id='recipient_definitive_reconcile';
	`)
	if err != nil {
		t.Fatal(err)
	}
	work, err := store.ClaimPublishingRestrictionEmailRecipients(recoveryCtx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("definitive failure ignored retry backoff: %+v", work)
	}
	var retryable bool
	var recipientError string
	if err := pool.QueryRow(recoveryCtx, `
		SELECT status, retryable, last_error
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_definitive_reconcile'
	`).Scan(&recipientStatus, &retryable, &recipientError); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "failed" || !retryable || !strings.Contains(recipientError, "503") || strings.Contains(recipientError, "outcome unknown") {
		t.Fatalf("recipient=%q retryable=%v error=%q", recipientStatus, retryable, recipientError)
	}
	if _, err := pool.Exec(recoveryCtx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET next_attempt_at=NOW()
		WHERE id='recipient_definitive_reconcile'
	`); err != nil {
		t.Fatal(err)
	}
	work, err = store.ClaimPublishingRestrictionEmailRecipients(recoveryCtx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 || work[0].IdempotencyKey != providerKey || work[0].AttemptCount != 2 {
		t.Fatalf("retry work = %+v, want same provider key and second bounded attempt", work)
	}
	if client.sends != 1 {
		t.Fatalf("reconciliation itself resent provider request: sends=%d", client.sends)
	}
}

func TestPublishingRestrictionRecipientSentRecoversAfterTransientCampaignRefreshFailure(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_refresh_retry", "worker_user_1", "sent@example.com", time.Now())
	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 {
		t.Fatalf("initial claim = %+v, want one recipient", work)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO email_send_attempts (
			id, event_key, recipient_email, provider, idempotency_key,
			delivery_class, status, sent_at
		) VALUES (
			'attempt_refresh_retry', 'email.publishing_restriction.restriction_notice.v1',
			'sent@example.com', 'loops', 'attempt-refresh-retry',
			'service_alert', 'sent', NOW()
		);
		CREATE FUNCTION fail_campaign_refresh() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN
			RAISE EXCEPTION 'campaign refresh unavailable';
		END
		$$;
		CREATE TRIGGER fail_campaign_refresh
		BEFORE UPDATE ON platform_publishing_restriction_email_campaigns
		FOR EACH ROW EXECUTE FUNCTION fail_campaign_refresh();
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.LinkPublishingRestrictionEmailAttempt(
		ctx,
		work[0].RecipientID,
		"attempt_refresh_retry",
		work[0].AttemptCount,
		work[0].AttemptGeneration,
	); err != nil {
		t.Fatal(err)
	}

	markErr := store.MarkPublishingRestrictionEmailRecipientSent(ctx, work[0].RecipientID)
	if markErr == nil || !strings.Contains(markErr.Error(), "campaign refresh unavailable") {
		t.Fatalf("sent transition error = %v, want campaign refresh failure", markErr)
	}
	var statusAfterRollback string
	if err := pool.QueryRow(ctx, `
		SELECT status FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_refresh_retry'
	`).Scan(&statusAfterRollback); err != nil {
		t.Fatal(err)
	}
	if statusAfterRollback != "sending" {
		t.Fatalf("recipient status after rollback = %q, want sending", statusAfterRollback)
	}
	_, err = pool.Exec(ctx, `
		DROP TRIGGER fail_campaign_refresh ON platform_publishing_restriction_email_campaigns;
		UPDATE platform_publishing_restriction_email_recipients
		SET claimed_at=NOW()-INTERVAL '20 minutes'
		WHERE id='recipient_refresh_retry';
	`)
	if err != nil {
		t.Fatal(err)
	}

	work, err = store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("recovery reclaimed recipient for a duplicate provider send: %+v", work)
	}
	var recipientStatus, campaignStatus string
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, campaign.status
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
		WHERE recipient.id='recipient_refresh_retry'
	`).Scan(&recipientStatus, &campaignStatus); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "sent" || campaignStatus != "completed" {
		t.Fatalf("recipient=%q campaign=%q, want sent/completed", recipientStatus, campaignStatus)
	}
}

func TestPublishingRestrictionClaimReconcilesRunningCampaignAfterPriorRefreshFailure(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_prior_refresh", "worker_user_1", "sent@example.com", time.Now())
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET status='sent', sent_at=NOW(), claimed_at=NULL
		WHERE id='recipient_prior_refresh';
		UPDATE platform_publishing_restriction_email_campaigns
		SET status='running', pending_count=1, sent_count=0
		WHERE id='worker_campaign';
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("claim returned already-sent work: %+v", work)
	}
	var status string
	var sentCount, pendingCount int
	if err := pool.QueryRow(ctx, `
		SELECT status, sent_count, pending_count
		FROM platform_publishing_restriction_email_campaigns
		WHERE id='worker_campaign'
	`).Scan(&status, &sentCount, &pendingCount); err != nil {
		t.Fatal(err)
	}
	if status != "completed" || sentCount != 1 || pendingCount != 0 {
		t.Fatalf("campaign=%q sent=%d pending=%d, want completed/1/0", status, sentCount, pendingCount)
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

func TestPublishingRestrictionStaleLinkedPreSendFailureRetriesWithFreshAuditAttempt(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	insertPendingRestrictionRecipient(t, pool, "recipient_link_lost", "worker_user_1", "owner@example.com", time.Now())
	_, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restrictions
		SET enabled=TRUE, cycle_id='worker_cycle'
		WHERE platform='tiktok';
		UPDATE users SET email='owner@example.com' WHERE id='worker_user_1';
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_workspace_ids=ARRAY['workspace_1']::TEXT[]
		WHERE id='recipient_link_lost';
		INSERT INTO workspaces (id) VALUES ('workspace_1');
		INSERT INTO subscriptions (workspace_id, plan_id) VALUES ('workspace_1', 'free');
		INSERT INTO workspace_members (workspace_id, user_id, role, status)
		VALUES ('workspace_1', 'worker_user_1', 'owner', 'active');
		INSERT INTO social_accounts (id, workspace_id, platform, status)
		VALUES ('social_link_lost', 'workspace_1', 'tiktok', 'active');
	`)
	if err != nil {
		t.Fatal(err)
	}

	store := NewPostgresPublishingRestrictionEmailStore(pool)
	work, err := store.ClaimPublishingRestrictionEmailRecipients(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 1 {
		t.Fatalf("initial claim = %+v, want one recipient", work)
	}
	first := work[0]
	client := &integrationSuccessLifecycleClient{}
	auditedSender := loops.NewAuditedClient(client, loops.NewPostgresEmailAuditStore(db.New(pool)))
	oldAttempt, err := auditedSender.SendTransactionalWithAttempt(ctx, loops.TransactionalEmail{
		TransactionalID: "restriction-template",
		Email:           first.RecipientEmail,
		UserID:          first.CanonicalUserID,
		IdempotencyKey:  first.IdempotencyKey,
		Audit: loops.EmailAudit{
			EventKey: "email.publishing_restriction.restriction_notice.v1", Provider: "loops",
			DeliveryClass: "service_alert", TriggerSource: "admin_confirmed_campaign",
			TriggerReferenceID: first.RecipientID,
			AttemptIdempotencyKey: fmt.Sprintf(
				"%s:g%d:a%d",
				first.IdempotencyKey,
				first.AttemptGeneration,
				first.AttemptCount,
			),
		},
	}, func(linkCtx context.Context, attemptID string) error {
		if linkErr := store.LinkPublishingRestrictionEmailAttempt(
			linkCtx,
			first.RecipientID,
			attemptID,
			first.AttemptCount,
			first.AttemptGeneration,
		); linkErr != nil {
			return linkErr
		}
		return errors.New("link update response lost")
	})
	if err == nil || !strings.Contains(err.Error(), "link update response lost") {
		t.Fatalf("link error = %v, want lost response", err)
	}
	if client.sends != 0 {
		t.Fatalf("provider sends before stale recovery = %d, want 0", client.sends)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET claimed_at=NOW()-INTERVAL '20 minutes'
		WHERE id='recipient_link_lost'
	`); err != nil {
		t.Fatal(err)
	}

	work, err = store.ClaimPublishingRestrictionEmailRecipients(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(work) != 0 {
		t.Fatalf("stale recovery ignored retry backoff: %+v", work)
	}
	var recipientStatus, recipientError, linkedAttemptID, oldAuditStatus, oldAuditError string
	var retryable bool
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, recipient.retryable, recipient.last_error,
		       recipient.email_send_attempt_id, attempt.status, attempt.last_error
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN email_send_attempts attempt ON attempt.id=recipient.email_send_attempt_id
		WHERE recipient.id='recipient_link_lost'
	`).Scan(&recipientStatus, &retryable, &recipientError, &linkedAttemptID, &oldAuditStatus, &oldAuditError); err != nil {
		t.Fatal(err)
	}
	if recipientStatus != "failed" || !retryable || !strings.Contains(recipientError, "not attempted") || strings.Contains(recipientError, "outcome unknown") {
		t.Fatalf("recovered recipient=%q retryable=%v error=%q", recipientStatus, retryable, recipientError)
	}
	if linkedAttemptID != oldAttempt.ID || oldAuditStatus != "failed" || !strings.Contains(oldAuditError, `"outcome_class":"pre_send_failure"`) {
		t.Fatalf("old attempt link=%q status=%q error=%q, want preserved pre-send evidence", linkedAttemptID, oldAuditStatus, oldAuditError)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET next_attempt_at=NOW()
		WHERE id='recipient_link_lost'
	`); err != nil {
		t.Fatal(err)
	}

	worker := NewPublishingRestrictionEmailWorker(
		store,
		auditedSender,
		"restriction-template",
		"recovery-template",
		readyPublishingRestrictionCampaignDelivery(),
	)
	if err := worker.ProcessBatch(ctx); err != nil {
		t.Fatal(err)
	}
	if client.sends != 1 || len(client.emails) != 1 {
		t.Fatalf("provider sends=%d emails=%d, want exactly one retry", client.sends, len(client.emails))
	}
	var finalRecipientStatus, newAttemptID, newAuditStatus, newAuditKey string
	if err := pool.QueryRow(ctx, `
		SELECT recipient.status, attempt.id, attempt.status, attempt.idempotency_key
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN email_send_attempts attempt ON attempt.id=recipient.email_send_attempt_id
		WHERE recipient.id='recipient_link_lost'
	`).Scan(&finalRecipientStatus, &newAttemptID, &newAuditStatus, &newAuditKey); err != nil {
		t.Fatal(err)
	}
	if finalRecipientStatus != "sent" || newAttemptID == oldAttempt.ID || newAuditStatus != "sent" {
		t.Fatalf("recipient=%q old_attempt=%q new_attempt=%q new_audit=%q", finalRecipientStatus, oldAttempt.ID, newAttemptID, newAuditStatus)
	}
	wantNewAuditKey := first.IdempotencyKey + ":g1:a2"
	if newAuditKey != wantNewAuditKey {
		t.Fatalf("new audit key = %q, want %q", newAuditKey, wantNewAuditKey)
	}
	if err := pool.QueryRow(ctx, `
		SELECT status, last_error FROM email_send_attempts WHERE id=$1
	`, oldAttempt.ID).Scan(&oldAuditStatus, &oldAuditError); err != nil {
		t.Fatal(err)
	}
	if oldAuditStatus != "failed" || !strings.Contains(oldAuditError, `"outcome_class":"pre_send_failure"`) {
		t.Fatalf("old audit after retry status=%q error=%q, want preserved failure evidence", oldAuditStatus, oldAuditError)
	}
}

func TestPublishingRestrictionStalePreSendClaimRecoversWithoutUnknownOutcome(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_recipients (
			id,campaign_id,canonical_user_id,recipient_email,normalized_email,status,attempt_count,
			next_attempt_at,claimed_at,idempotency_key,retryable,created_at,updated_at
		) VALUES (
			'recipient_pre_send','worker_campaign','worker_user_1','pre@example.com','pre@example.com','sending',1,
			NOW()-INTERVAL '20 minutes',NOW()-INTERVAL '20 minutes','worker_cycle:restriction_notice:worker_user_1',
			FALSE,NOW()-INTERVAL '20 minutes',NOW()-INTERVAL '20 minutes'
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
		t.Fatalf("pre-send recovery ignored backoff: %+v", work)
	}
	var status, lastError string
	var retryable bool
	if err := pool.QueryRow(ctx, `SELECT status,retryable,last_error FROM platform_publishing_restriction_email_recipients WHERE id='recipient_pre_send'`).Scan(&status, &retryable, &lastError); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || !retryable || !strings.Contains(lastError, "not attempted") || strings.Contains(lastError, "outcome unknown") {
		t.Fatalf("status=%q retryable=%v error=%q", status, retryable, lastError)
	}
}

func TestPublishingRestrictionDelayedAuditSentCannotRacePastStaleReconciliation(t *testing.T) {
	pool := openPublishingRestrictionWorkerIntegrationPool(t)
	setupPublishingRestrictionWorkerSchema(t, pool)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		INSERT INTO email_send_attempts (id,event_key,recipient_email,provider,idempotency_key,delivery_class,status)
		VALUES ('attempt_race','email.publishing_restriction.restriction_notice.v1','race@example.com','loops','attempt-race','service_alert','pending');
		INSERT INTO platform_publishing_restriction_email_recipients (
			id,campaign_id,canonical_user_id,recipient_email,normalized_email,status,attempt_count,
			next_attempt_at,claimed_at,idempotency_key,email_send_attempt_id,retryable,created_at,updated_at
		) VALUES (
			'recipient_race','worker_campaign','worker_user_1','race@example.com','race@example.com','sending',1,
			NOW()-INTERVAL '20 minutes',NOW()-INTERVAL '20 minutes','worker_cycle:restriction_notice:worker_user_1',
			'attempt_race',FALSE,NOW()-INTERVAL '20 minutes',NOW()-INTERVAL '20 minutes'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT id FROM platform_publishing_restriction_email_recipients WHERE id='recipient_race' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT id FROM email_send_attempts WHERE id='attempt_race' FOR UPDATE`); err != nil {
		t.Fatal(err)
	}
	delayedDone := make(chan error, 1)
	go func() {
		delayedDone <- loops.NewPostgresEmailAuditStore(db.New(pool)).MarkEmailSendAttemptSent(ctx, "attempt_race")
	}()
	select {
	case err := <-delayedDone:
		t.Fatalf("delayed MarkSent did not wait for reconciliation lock: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := tx.Exec(ctx, `UPDATE email_send_attempts SET status='failed',last_error='{"outcome_class":"provider_unknown"}' WHERE id='attempt_race' AND status='pending'`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE platform_publishing_restriction_email_recipients SET status='failed',retryable=FALSE,claimed_at=NULL WHERE id='recipient_race' AND status='sending'`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-delayedDone; err != nil {
		t.Fatal(err)
	}
	var auditStatus, recipientStatus string
	if err := pool.QueryRow(ctx, `
		SELECT attempt.status,recipient.status FROM email_send_attempts attempt
		JOIN platform_publishing_restriction_email_recipients recipient ON recipient.email_send_attempt_id=attempt.id
		WHERE attempt.id='attempt_race'
	`).Scan(&auditStatus, &recipientStatus); err != nil {
		t.Fatal(err)
	}
	if auditStatus != "failed" || recipientStatus != "failed" {
		t.Fatalf("audit=%q recipient=%q, want failed/failed", auditStatus, recipientStatus)
	}
}
