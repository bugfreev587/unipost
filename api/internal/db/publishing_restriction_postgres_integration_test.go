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

	"github.com/xiaoboyu/unipost-api/internal/testdbguard"
)

const publishingRestrictionIntegrationDatabaseEnv = "PUBLISHING_RESTRICTION_TEST_DATABASE_URL"

func openPublishingRestrictionIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv(publishingRestrictionIntegrationDatabaseEnv))
	if databaseURL == "" {
		t.Fatalf("%s is required and must point to an isolated PostgreSQL test service", publishingRestrictionIntegrationDatabaseEnv)
	}

	ctx := context.Background()
	admin, config, err := testdbguard.OpenValidated(
		publishingRestrictionIntegrationDatabaseEnv,
		databaseURL,
		func(validatedURL string) (*pgxpool.Pool, error) {
			return pgxpool.New(ctx, validatedURL)
		},
	)
	if err != nil {
		t.Fatalf("connect PostgreSQL integration service: %v", err)
	}
	schema := fmt.Sprintf("pr270_%d", time.Now().UnixNano())
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

func createMigration126CrossSchemaDecoy(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	schema := fmt.Sprintf("pr126_decoy_%d", time.Now().UnixNano())
	qualifiedSchema := pgx.Identifier{schema}.Sanitize()
	qualifiedTable := pgx.Identifier{schema, "platform_publishing_restriction_email_recipients"}.Sanitize()
	qualifiedFunction := pgx.Identifier{schema, "backfill_publishing_restriction_recipient_owner_snapshot"}.Sanitize()

	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+qualifiedSchema); err != nil {
		t.Fatalf("create migration 126 cross-schema decoy: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+qualifiedSchema+" CASCADE"); err != nil {
			t.Errorf("drop migration 126 cross-schema decoy: %v", err)
		}
	})
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %s (represented_owner_user_ids TEXT[]);
		CREATE FUNCTION %s()
		RETURNS TRIGGER
		LANGUAGE plpgsql
		AS $$ BEGIN RETURN NEW; END; $$;
		CREATE TRIGGER publishing_restriction_owner_snapshot_backfill
		BEFORE INSERT ON %s
		FOR EACH ROW
		EXECUTE FUNCTION %s();
	`, qualifiedTable, qualifiedFunction, qualifiedTable, qualifiedFunction)); err != nil {
		t.Fatalf("populate migration 126 cross-schema decoy: %v", err)
	}
}

func TestPublishingRestrictionRecipientOwnerSnapshotUpgradeAndDown(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	createMigration126CrossSchemaDecoy(t, pool)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT);
		CREATE TABLE email_send_attempts (id TEXT PRIMARY KEY);
		CREATE TABLE media_post_usages (cleanup_after_at TIMESTAMPTZ);
		CREATE TABLE workspace_members (
			workspace_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			role TEXT NOT NULL,
			status TEXT NOT NULL,
			PRIMARY KEY (workspace_id, user_id)
		);
		INSERT INTO users (id) VALUES
			('canonical_user'), ('empty_user'), ('explicit_writer_user'), ('legacy_fallback_user');
		INSERT INTO users (id, email) VALUES
			('legacy_writer_user', 'legacy@example.com'),
			('legacy_other_owner', 'legacy@example.com'),
			('ambiguous_owner_1', 'fallback@example.com'),
			('ambiguous_owner_2', 'fallback@example.com');
		INSERT INTO workspace_members (workspace_id, user_id, role, status) VALUES
			('legacy_workspace_1', 'legacy_writer_user', 'owner', 'active'),
			('legacy_workspace_2', 'legacy_other_owner', 'owner', 'active'),
			('unresolved_workspace', 'ambiguous_owner_1', 'owner', 'active'),
			('unresolved_workspace', 'ambiguous_owner_2', 'owner', 'active');
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
	if !reflect.DeepEqual(ownerIDs, []string{"legacy_writer_user", "legacy_other_owner"}) {
		t.Fatalf("legacy writer owner IDs = %#v, want resolved same-email owners by workspace ordinality", ownerIDs)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO platform_publishing_restriction_email_recipients (
			id, campaign_id, canonical_user_id, recipient_email, normalized_email,
			represented_workspace_ids, idempotency_key
		) VALUES (
			'recipient_legacy_fallback', 'campaign_owner_snapshot', 'legacy_fallback_user',
			'fallback@example.com', 'fallback@example.com', ARRAY['unresolved_workspace']::TEXT[],
			'owner-snapshot-legacy-fallback'
		);
	`)
	if err != nil {
		t.Fatalf("migration 125 writer fallback insert after migration 126: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT represented_owner_user_ids
		FROM platform_publishing_restriction_email_recipients
		WHERE id='recipient_legacy_fallback'
	`).Scan(&ownerIDs); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ownerIDs, []string{"legacy_fallback_user"}) {
		t.Fatalf("ambiguous legacy writer owner IDs = %#v, want conservative canonical fallback", ownerIDs)
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
			WHERE table_schema=current_schema()
			  AND table_name='platform_publishing_restriction_email_recipients'
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
			SELECT 1
			FROM pg_trigger trigger_definition
			JOIN pg_class table_definition ON table_definition.oid=trigger_definition.tgrelid
			JOIN pg_namespace namespace ON namespace.oid=table_definition.relnamespace
			WHERE namespace.nspname=current_schema()
			  AND trigger_definition.tgname='publishing_restriction_owner_snapshot_backfill'
			  AND NOT trigger_definition.tgisinternal
		)
	`).Scan(&triggerExists); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_proc function_definition
			JOIN pg_namespace namespace ON namespace.oid=function_definition.pronamespace
			WHERE namespace.nspname=current_schema()
			  AND function_definition.proname='backfill_publishing_restriction_recipient_owner_snapshot'
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
	if recipientCount != 5 {
		t.Fatalf("migration 126 Down changed recipient rows: got %d, want 5", recipientCount)
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

func integrationSkippedEmailAttempt(key, eventKey, email, subject, reason string, variables []byte) CreateSkippedEmailSendAttemptAuditParams {
	return CreateSkippedEmailSendAttemptAuditParams{
		EventKey:              eventKey,
		RecipientUserID:       "",
		RecipientEmail:        email,
		WorkspaceID:           "",
		Provider:              "loops",
		ProviderTemplateID:    "template_skipped",
		IdempotencyKey:        key,
		DeliveryClass:         "service_alert",
		SubjectSnapshot:       subject,
		DataVariablesSnapshot: variables,
		TriggerSource:         "worker",
		TriggerReferenceID:    "reference_skipped",
		LastError:             pgtype.Text{String: reason, Valid: true},
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

func TestCreateSkippedEmailSendAttemptAuditPreservesTerminalSentRecord(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	setupEmailAuditIntegrationSchema(t, pool)
	ctx := context.Background()
	queries := New(pool)

	created, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
		"provider-key-sent-skipped", "event.original", "original@example.com", "Original subject", []byte(`{"body":"original"}`),
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

	replayed, err := queries.CreateSkippedEmailSendAttemptAudit(ctx, integrationSkippedEmailAttempt(
		"provider-key-sent-skipped", "event.replayed", "changed@example.com", "Changed subject", "restriction enabled", []byte(`{"body":"changed"}`),
	))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(replayed, before) {
		t.Fatalf("sent audit changed on skipped provider-key replay:\n before=%+v\n replay=%+v", before, replayed)
	}
}

func TestCreateSkippedEmailSendAttemptAuditUpdatesNonSentRecords(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	setupEmailAuditIntegrationSchema(t, pool)
	ctx := context.Background()
	queries := New(pool)

	tests := []struct {
		name  string
		key   string
		setup func(t *testing.T) EmailSendAttempt
	}{
		{
			name: "pending",
			key:  "provider-key-pending-skipped",
			setup: func(t *testing.T) EmailSendAttempt {
				t.Helper()
				row, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
					"provider-key-pending-skipped", "event.pending", "pending@example.com", "Pending subject", []byte(`{"body":"pending"}`),
				))
				if err != nil {
					t.Fatal(err)
				}
				return row
			},
		},
		{
			name: "failed",
			key:  "provider-key-failed-skipped",
			setup: func(t *testing.T) EmailSendAttempt {
				t.Helper()
				row, err := queries.CreateEmailSendAttemptAudit(ctx, integrationEmailAttempt(
					"provider-key-failed-skipped", "event.failed", "failed@example.com", "Failed subject", []byte(`{"body":"failed"}`),
				))
				if err != nil {
					t.Fatal(err)
				}
				if err := queries.MarkEmailSendAttemptAuditFailed(ctx, MarkEmailSendAttemptAuditFailedParams{
					ID:        row.ID,
					LastError: pgtype.Text{String: "temporary failure", Valid: true},
				}); err != nil {
					t.Fatal(err)
				}
				row.Status = "failed"
				return row
			},
		},
		{
			name: "skipped",
			key:  "provider-key-skipped-skipped",
			setup: func(t *testing.T) EmailSendAttempt {
				t.Helper()
				row, err := queries.CreateSkippedEmailSendAttemptAudit(ctx, integrationSkippedEmailAttempt(
					"provider-key-skipped-skipped", "event.skipped", "skipped@example.com", "Skipped subject", "first skip", []byte(`{"body":"skipped"}`),
				))
				if err != nil {
					t.Fatal(err)
				}
				return row
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := tt.setup(t)
			replayed, err := queries.CreateSkippedEmailSendAttemptAudit(ctx, integrationSkippedEmailAttempt(
				tt.key, "event.replayed."+tt.name, tt.name+"-changed@example.com", "Changed "+tt.name, "restriction enabled", []byte(`{"body":"changed"}`),
			))
			if err != nil {
				t.Fatal(err)
			}
			if replayed.ID != before.ID || replayed.Status != "skipped" || replayed.AttemptCount != before.AttemptCount+1 {
				t.Fatalf("%s skipped audit replay = %+v, want same id, skipped, attempt_count=%d", tt.name, replayed, before.AttemptCount+1)
			}
			if !replayed.LastError.Valid || replayed.LastError.String != "restriction enabled" || replayed.SentAt.Valid {
				t.Fatalf("%s skipped audit terminal fields = last_error=%+v sent_at=%+v", tt.name, replayed.LastError, replayed.SentAt)
			}
		})
	}
}

func TestMigration127UpgradeRollingCompatibilityAndDown(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			status TEXT NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE webhooks (id TEXT PRIMARY KEY);
		CREATE TABLE webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL REFERENCES webhooks(id),
			event TEXT NOT NULL,
			payload JSONB NOT NULL
		);
		INSERT INTO workspaces(id) VALUES ('ws_127');
		INSERT INTO social_posts(id,workspace_id,status) VALUES ('post_127','ws_127','failed');
		INSERT INTO webhooks(id) VALUES ('webhook_127');
		INSERT INTO webhook_deliveries(id,webhook_id,event,payload)
		VALUES ('legacy_before','webhook_127','post.failed','{}');
	`); err != nil {
		t.Fatal(err)
	}

	applyMigrationUpForIntegration(t, pool, "127_terminal_post_event_outbox.sql")
	// Simulate an old binary after the additive migration: its INSERT omits the
	// new nullable event_id column and must continue to work unchanged.
	if _, err := pool.Exec(ctx, `INSERT INTO webhook_deliveries(id,webhook_id,event,payload)
		VALUES ('legacy_after','webhook_127','post.failed','{}')`); err != nil {
		t.Fatalf("old binary webhook insert after migration 127: %v", err)
	}
	var legacyID, legacyWebhookID, legacyEvent string
	var legacyPayload []byte
	if err := pool.QueryRow(ctx, `SELECT * FROM webhook_deliveries WHERE id='legacy_before'`).Scan(
		&legacyID, &legacyWebhookID, &legacyEvent, &legacyPayload,
	); err != nil {
		t.Fatalf("old binary SELECT * scan after migration 127: %v", err)
	}
	queries := New(pool)
	if err := RecordPostStatusTransition(ctx, queries, "post_127", "ws_127", "failed", "version_127", "post.failed", map[string]any{"id": "post_127", "status": "failed"}); err != nil {
		t.Fatal(err)
	}
	var events, deliveries int
	if err := pool.QueryRow(ctx, `SELECT
		(SELECT COUNT(*) FROM terminal_post_event_outbox),
		(SELECT COUNT(*) FROM terminal_post_event_deliveries)`).Scan(&events, &deliveries); err != nil {
		t.Fatal(err)
	}
	if events != 1 || deliveries != 3 {
		t.Fatalf("migration 127 outbox events/deliveries = %d/%d", events, deliveries)
	}

	applyMigrationDownForIntegration(t, pool, "127_terminal_post_event_outbox.sql")
	var legacyRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM webhook_deliveries`).Scan(&legacyRows); err != nil {
		t.Fatal(err)
	}
	if legacyRows != 2 {
		t.Fatalf("legacy webhook rows after Down = %d", legacyRows)
	}
	var outboxExists, webhookMappingExists, firstPostElectionExists bool
	if err := pool.QueryRow(ctx, `SELECT
		to_regclass('terminal_post_event_outbox') IS NOT NULL,
		to_regclass('terminal_post_event_webhook_deliveries') IS NOT NULL,
		to_regclass('terminal_post_first_publish_elections') IS NOT NULL`).Scan(&outboxExists, &webhookMappingExists, &firstPostElectionExists); err != nil {
		t.Fatal(err)
	}
	if outboxExists || webhookMappingExists || firstPostElectionExists {
		t.Fatalf("migration 127 Down left outbox/webhook mapping/election: %v/%v/%v", outboxExists, webhookMappingExists, firstPostElectionExists)
	}
}

func TestPostPublishedOutboxSnapshotsFirstPostBeforeDelayedDispatch(t *testing.T) {
	pool := openPublishingRestrictionIntegrationPool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		CREATE TABLE workspaces (id TEXT PRIMARY KEY);
		CREATE TABLE social_posts (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id),
			status TEXT NOT NULL,
			deleted_at TIMESTAMPTZ
		);
		CREATE TABLE webhooks (id TEXT PRIMARY KEY);
		CREATE TABLE webhook_deliveries (
			id TEXT PRIMARY KEY,
			webhook_id TEXT NOT NULL REFERENCES webhooks(id),
			event TEXT NOT NULL,
			payload JSONB NOT NULL
		);
		INSERT INTO workspaces(id) VALUES ('ws_first_snapshot');
		INSERT INTO social_posts(id,workspace_id,status) VALUES
			('post_first_snapshot','ws_first_snapshot','publishing'),
			('post_second_snapshot','ws_first_snapshot','publishing');
	`); err != nil {
		t.Fatal(err)
	}
	applyMigrationUpForIntegration(t, pool, "127_terminal_post_event_outbox.sql")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_first_snapshot'`); err != nil {
		t.Fatal(err)
	}
	var version string
	if err := tx.QueryRow(ctx, `SELECT xmin::text FROM social_posts WHERE id='post_first_snapshot'`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if err := RecordPostStatusTransition(ctx, New(tx), "post_first_snapshot", "ws_first_snapshot", "published", version, "post.published", map[string]any{
		"id": "post_first_snapshot", "status": "published",
	}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// A second publication commits before the delayed outbox dispatch. The
	// first-post decision must remain the transaction-time snapshot.
	if _, err := pool.Exec(ctx, `UPDATE social_posts SET status='published' WHERE id='post_second_snapshot'`); err != nil {
		t.Fatal(err)
	}
	var firstSnapshot bool
	var publishedNow int
	if err := pool.QueryRow(ctx, `SELECT
		(e.payload->>'first_post_published')::boolean,
		(SELECT COUNT(*)::int FROM social_posts WHERE workspace_id='ws_first_snapshot' AND status='published')
		FROM terminal_post_event_outbox e WHERE e.post_id='post_first_snapshot'`).Scan(&firstSnapshot, &publishedNow); err != nil {
		t.Fatal(err)
	}
	if !firstSnapshot || publishedNow != 2 {
		t.Fatalf("first-post snapshot/current count = %v/%d, want true/2", firstSnapshot, publishedNow)
	}
}
