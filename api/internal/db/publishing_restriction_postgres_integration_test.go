//go:build integration

package db

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const publishingRestrictionIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openPublishingRestrictionIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionIntegrationDatabaseEnv)
	}

	ctx := context.Background()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL integration service: %v", err)
	}
	schema := fmt.Sprintf("pr270_%d", time.Now().UnixNano())
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

func applyMigrationUpForIntegration(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()
	raw, err := os.ReadFile("migrations/" + filename)
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

func applyMigrationDownForIntegration(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()
	raw, err := os.ReadFile("migrations/" + filename)
	if err != nil {
		t.Fatalf("read migration %s: %v", filename, err)
	}
	parts := strings.Split(string(raw), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration %s must have exactly one Down section", filename)
	}
	if _, err := pool.Exec(context.Background(), parts[1]); err != nil {
		t.Fatalf("apply migration %s Down: %v", filename, err)
	}
}

func TestPublishingRestrictionRecipientOwnerSnapshotUpgradeAndDown(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE email_send_attempts (id TEXT PRIMARY KEY);
		CREATE TABLE media_post_usages (cleanup_after_at TIMESTAMPTZ);
		INSERT INTO users (id) VALUES
			('canonical_user'), ('empty_user'), ('legacy_writer_user'), ('explicit_writer_user');
	`)
	if err != nil {
		t.Fatal(err)
	}
	applyMigrationUpForIntegration(t, pool, "122_platform_publishing_restrictions.sql")
	_, err = pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_campaigns (
			id, restriction_id, cycle_id, campaign_type, subject_snapshot,
			body_snapshot, restriction_version
		)
		SELECT 'campaign_owner_snapshot', id, 'cycle_owner_snapshot',
		       'restriction_notice', 'subject', 'body', version
		FROM platform_publishing_restrictions WHERE platform='tiktok';
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			represented_workspace_ids, idempotency_key
		) VALUES
			('recipient_with_workspaces', 'campaign_owner_snapshot', 'canonical_user',
			 'owner@example.com', 'owner@example.com', ARRAY['workspace_1','workspace_2']::TEXT[],
			 'owner-snapshot-with-workspaces'),
			('recipient_without_workspaces', 'campaign_owner_snapshot', 'empty_user',
			 'empty@example.com', 'empty@example.com', ARRAY[]::TEXT[],
			 'owner-snapshot-without-workspaces');
	`)
	if err != nil {
		t.Fatal(err)
	}
	applyMigrationUpForIntegration(t, pool, "124_publishing_restriction_email_send_gate.sql")
	applyMigrationUpForIntegration(t, pool, "125_publishing_restriction_failed_recipient_retryability.sql")
	applyMigrationUpForIntegration(t, pool, "126_publishing_restriction_recipient_owner_snapshot.sql")

	var ownerIDs []string
	if err := pool.QueryRow(ctx, `
		SELECT represented_owner_user_ids
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_with_workspaces'
	`).Scan(&ownerIDs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerIDs, []string{"canonical_user", "canonical_user"}) {
		t.Fatalf("backfilled owner IDs = %#v, want canonical repeated for each workspace", ownerIDs)
	}
	if err := pool.QueryRow(ctx, `
		SELECT represented_owner_user_ids
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_without_workspaces'
	`).Scan(&ownerIDs); err != nil {
		t.Fatal(err)
	}
	if len(ownerIDs) != 0 {
		t.Fatalf("empty workspace owner IDs = %#v, want empty array", ownerIDs)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			represented_workspace_ids, idempotency_key
		) VALUES (
			'recipient_legacy_writer', 'campaign_owner_snapshot', 'legacy_writer_user',
			'legacy@example.com', 'legacy@example.com', ARRAY['legacy_workspace_1','legacy_workspace_2']::TEXT[],
			'owner-snapshot-legacy-writer'
		);
	`)
	if err != nil {
		t.Fatalf("migration 125 writer insert after migration 126: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT represented_owner_user_ids
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_legacy_writer'
	`).Scan(&ownerIDs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerIDs, []string{"legacy_writer_user", "legacy_writer_user"}) {
		t.Fatalf("legacy writer owner IDs = %#v, want canonical repeated for each workspace", ownerIDs)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			represented_workspace_ids, represented_owner_user_ids, idempotency_key
		) VALUES (
			'recipient_explicit_writer', 'campaign_owner_snapshot', 'explicit_writer_user',
			'explicit@example.com', 'explicit@example.com', ARRAY['explicit_workspace']::TEXT[],
			ARRAY['preserved_owner']::TEXT[], 'owner-snapshot-explicit-writer'
		);
	`)
	if err != nil {
		t.Fatalf("migration 126 writer insert: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT represented_owner_user_ids
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_explicit_writer'
	`).Scan(&ownerIDs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerIDs, []string{"preserved_owner"}) {
		t.Fatalf("explicit writer owner IDs = %#v, want explicit snapshot preserved", ownerIDs)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_owner_user_ids=ARRAY['canonical_user']::TEXT[]
		WHERE id='recipient_with_workspaces'
	`); err == nil {
		t.Fatal("owner snapshot accepted a cardinality mismatch")
	}
	if _, err := pool.Exec(ctx, `
		UPDATE platform_publishing_restriction_email_recipients
		SET represented_owner_user_ids=ARRAY['canonical_user',NULL]::TEXT[]
		WHERE id='recipient_with_workspaces'
	`); err == nil {
		t.Fatal("owner snapshot accepted a NULL owner element")
	}

	applyMigrationDownForIntegration(t, pool, "126_publishing_restriction_recipient_owner_snapshot.sql")
	var columnExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name='platform_publishing_restriction_email_recipients'
			  AND column_name='represented_owner_user_ids'
		)
	`).Scan(&columnExists); err != nil {
		t.Fatal(err)
	}
	if columnExists {
		t.Fatal("migration 126 Down left represented_owner_user_ids behind")
	}
	var triggerExists, functionExists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_trigger
			WHERE tgname='publishing_restriction_owner_snapshot_backfill'
			  AND NOT tgisinternal
		)
	`).Scan(&triggerExists); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_proc
			WHERE proname='backfill_publishing_restriction_recipient_owner_snapshot'
		)
	`).Scan(&functionExists); err != nil {
		t.Fatal(err)
	}
	if triggerExists || functionExists {
		t.Fatalf("migration 126 Down cleanup trigger=%v function=%v, want both false", triggerExists, functionExists)
	}
	var recipientCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM platform_publishing_restriction_email_recipients`).Scan(&recipientCount); err != nil {
		t.Fatal(err)
	}
	if recipientCount != 4 {
		t.Fatalf("migration 126 Down changed recipient rows: got %d, want 4", recipientCount)
	}
}

func TestPublishingRestrictionFailedRecipientUpgradeConvergesAfterExecuted124(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE email_send_attempts (id TEXT PRIMARY KEY);
		CREATE TABLE media_post_usages (cleanup_after_at TIMESTAMPTZ);
		INSERT INTO users (id) VALUES ('user_failed');
	`)
	if err != nil {
		t.Fatal(err)
	}
	applyMigrationUpForIntegration(t, pool, "122_platform_publishing_restrictions.sql")

	_, err = pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_campaigns (
			id, restriction_id, cycle_id, campaign_type, subject_snapshot,
			body_snapshot, restriction_version
		)
		SELECT 'campaign_failed', id, 'cycle_failed', 'restriction_notice',
		       'subject', 'body', version
		FROM platform_publishing_restrictions
		WHERE platform = 'tiktok';

		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email,
			normalized_email, status, next_attempt_at, idempotency_key
		) VALUES (
			'recipient_failed', 'campaign_failed', 'user_failed',
			'failed@example.com', 'failed@example.com', 'failed',
			NOW() - INTERVAL '1 hour', 'cycle_failed:restriction_notice:user_failed'
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	applyMigrationUpForIntegration(t, pool, "124_publishing_restriction_email_send_gate.sql")
	var retryableAfter124 bool
	if err := pool.QueryRow(ctx, `SELECT retryable FROM platform_publishing_restriction_email_recipients WHERE id='recipient_failed'`).Scan(&retryableAfter124); err != nil {
		t.Fatal(err)
	}
	if !retryableAfter124 {
		t.Fatal("fixture must reproduce migration 124 marking an existing failed recipient retryable")
	}

	applyMigrationUpForIntegration(t, pool, "125_publishing_restriction_failed_recipient_retryability.sql")
	var retryableAfter125 bool
	if err := pool.QueryRow(ctx, `SELECT retryable FROM platform_publishing_restriction_email_recipients WHERE id='recipient_failed'`).Scan(&retryableAfter125); err != nil {
		t.Fatal(err)
	}
	if retryableAfter125 {
		t.Fatal("migration 125 must terminalize every existing failed recipient")
	}

	var candidateCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM platform_publishing_restriction_email_recipients recipient
		JOIN platform_publishing_restriction_email_campaigns campaign ON campaign.id=recipient.campaign_id
		WHERE campaign.status IN ('queued','running')
		  AND recipient.status='failed'
		  AND recipient.retryable=TRUE
		  AND recipient.attempt_count < 3
		  AND recipient.next_attempt_at <= NOW()
	`).Scan(&candidateCount); err != nil {
		t.Fatal(err)
	}
	if candidateCount != 0 {
		t.Fatalf("failed recipient candidates after migration 125 = %d, want 0", candidateCount)
	}
}

func setupEmailAuditIntegrationSchema(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE users (id TEXT PRIMARY KEY);
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
	`)
	if err != nil {
		t.Fatal(err)
	}
	applyMigrationUpForIntegration(t, pool, "092_email_send_attempts.sql")
}

func integrationEmailAttempt(key, eventKey, email, subject string, variables []byte) CreateEmailSendAttemptAuditParams {
	return CreateEmailSendAttemptAuditParams{
		EventKey:              eventKey,
		RecipientUserID:       "",
		RecipientEmail:        email,
		WorkspaceID:           "",
		Provider:              "loops",
		ProviderTemplateID:    "template_1",
		IdempotencyKey:        key,
		DeliveryClass:         "service_alert",
		SubjectSnapshot:       subject,
		DataVariablesSnapshot: variables,
		TriggerSource:         "worker",
		TriggerReferenceID:    "reference_1",
	}
}

func TestCreateEmailSendAttemptAuditPreservesTerminalSentRecord(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	setupEmailAuditIntegrationSchema(t, pool)
	ctx := context.Background()
	queries := New(pool)

	created, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
		"provider-key-sent", "event.original", "original@example.com", "Original subject", []byte(`{"body":"original"}`),
	))
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.MarkEmailSendAttemptAuditSent(ctx, created.ID); err != nil {
		t.Fatal(err)
	}

	var before EmailSendAttempt
	if err := pool.QueryRow(ctx, `SELECT id, event_key, recipient_user_id, recipient_email, workspace_id, provider, provider_template_id, idempotency_key, delivery_class, status, subject_snapshot, data_variables_snapshot, trigger_source, trigger_reference_id, attempt_count, last_error, attempted_at, sent_at, created_at, updated_at FROM email_send_attempts WHERE id=$1`, created.ID).Scan(
		&before.ID, &before.EventKey, &before.RecipientUserID, &before.RecipientEmail, &before.WorkspaceID,
		&before.Provider, &before.ProviderTemplateID, &before.IdempotencyKey, &before.DeliveryClass,
		&before.Status, &before.SubjectSnapshot, &before.DataVariablesSnapshot, &before.TriggerSource,
		&before.TriggerReferenceID, &before.AttemptCount, &before.LastError, &before.AttemptedAt,
		&before.SentAt, &before.CreatedAt, &before.UpdatedAt,
	); err != nil {
		t.Fatal(err)
	}

	replayed, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
		"provider-key-sent", "event.replayed", "changed@example.com", "Changed subject", []byte(`{"body":"changed"}`),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, before) {
		t.Fatalf("sent audit changed on provider-key replay:\n before=%+v\n replay=%+v", before, replayed)
	}
}

func TestCreateEmailSendAttemptAuditStillRetriesFailedRecord(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	setupEmailAuditIntegrationSchema(t, pool)
	ctx := context.Background()
	queries := New(pool)

	created, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
		"provider-key-failed", "event.original", "failed@example.com", "Original subject", []byte(`{"body":"original"}`),
	))
	if err != nil {
		t.Fatal(err)
	}
	if err := queries.MarkEmailSendAttemptAuditFailed(ctx, MarkEmailSendAttemptAuditFailedParams{
		ID:        created.ID,
		LastError: pgtype.Text{String: "temporary failure", Valid: true},
	}); err != nil {
		t.Fatal(err)
	}

	replayed, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
		"provider-key-failed", "event.replayed", "retry@example.com", "Retry subject", []byte(`{"body":"retry"}`),
	))
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ID != created.ID || replayed.Status != "pending" || replayed.AttemptCount != 2 {
		t.Fatalf("failed audit retry = %+v, want same id, pending, attempt_count=2", replayed)
	}
	if replayed.LastError.Valid || replayed.SentAt.Valid {
		t.Fatalf("failed audit retry retained terminal fields: last_error=%+v sent_at=%+v", replayed.LastError, replayed.SentAt)
	}
}
