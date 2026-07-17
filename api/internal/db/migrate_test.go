package db

import (
	"database/sql"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMigrationsUsesPostgresSessionLocker(t *testing.T) {
	source, err := os.ReadFile("migrate.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"lock.NewPostgresSessionLocker()",
		"goose.NewProvider(",
		"goose.DialectPostgres",
		"goose.WithSessionLocker(sessionLocker)",
		"provider.Up(context.Background())",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("migration runner missing %q", want)
		}
	}
	if strings.Contains(text, "goose.Up(") {
		t.Fatal("migration runner must not use unlocked legacy goose.Up")
	}
}

func TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose(t *testing.T) {
	databaseURL := os.Getenv("GOOSE_MIGRATION_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GOOSE_MIGRATION_TEST_DATABASE_URL is not configured")
	}

	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var existingTables int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_type = 'BASE TABLE'
	`).Scan(&existingTables); err != nil {
		t.Fatalf("inspect disposable migration database: %v", err)
	}
	if existingTables != 0 {
		t.Fatalf(
			"GOOSE_MIGRATION_TEST_DATABASE_URL must point to an empty disposable database; found %d public tables",
			existingTables,
		)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			errs <- RunMigrations(databaseURL)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent RunMigrations: %v", err)
		}
	}

	var version int64
	if err := database.QueryRow(`
		SELECT version_id
		FROM goose_db_version
		WHERE is_applied
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&version); err != nil {
		t.Fatalf("read final Goose version: %v", err)
	}
	if version != 117 {
		t.Fatalf("final Goose version = %d, want 117", version)
	}
}

func TestEmbeddedMigrationVersionsAreUnique(t *testing.T) {
	seen := map[string]string{}

	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}
		name := filepath.Base(path)
		version, _, ok := strings.Cut(name, "_")
		if !ok {
			t.Fatalf("migration %s does not start with a numeric version prefix", name)
		}
		if previous, exists := seen[version]; exists {
			t.Fatalf("duplicate migration version %s: %s and %s", version, previous, name)
		}
		seen[version] = name
		return nil
	})
	if err != nil {
		t.Fatalf("walk migrations: %v", err)
	}
}

func TestTeamPlanUnlimitedPostsMigrationExists(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/088_team_unlimited_posts.sql")
	if err != nil {
		t.Fatalf("read team unlimited posts migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	if !strings.Contains(sql, "update plans") ||
		!strings.Contains(sql, "post_limit = -1") ||
		!strings.Contains(sql, "id = 'team'") {
		t.Fatalf("team unlimited migration should set plans.post_limit to -1 for team, got:\n%s", string(body))
	}
	if !strings.Contains(sql, "post_limit = 25000") {
		t.Fatalf("team unlimited migration should include a down migration restoring 25000, got:\n%s", string(body))
	}
}

func TestMediaProcessingJobsMigrationPreservesMediaCleanup(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/096_media_processing_jobs.sql")
	if err != nil {
		t.Fatalf("read media processing jobs migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	if !strings.Contains(sql, "create table media_processing_jobs") {
		t.Fatalf("media processing jobs migration should create table, got:\n%s", string(body))
	}
	if strings.Contains(sql, "references media(") || strings.Contains(sql, "references media (") {
		t.Fatalf("media processing jobs must not add media(id) foreign keys; media cleanup hard-deletes rows, got:\n%s", string(body))
	}
	if !strings.Contains(sql, "idempotency_key") || !strings.Contains(sql, "request_hash") {
		t.Fatalf("media processing jobs migration should include idempotency fields, got:\n%s", string(body))
	}
}

func TestMediaProcessingLifecycleMigration117Exists(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/117_media_processing_lifecycle.sql")
	if err != nil {
		t.Fatalf("read media processing lifecycle migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"add column input_media_id text",
		"add column next_attempt_at timestamptz not null default now()",
		"add column usage_version bigint not null default 0",
		"status in ('queued', 'retry_wait', 'processing', 'succeeded', 'failed', 'cancelled')",
		"media_processing_jobs_retry_due_idx",
		"alter column input_video_media_id drop not null",
		"alter column input_audio_media_id drop not null",
		"media_processing_jobs_kind_inputs_check",
		"kind = 'audio_overlay'",
		"kind = 'gif_to_mp4'",
		"create table media_processing_usages",
		"role in ('input', 'output')",
		"status in ('active', 'succeeded', 'failed', 'cancelled')",
		"cleanup_after_at timestamptz",
		"unique (job_id, media_id, role)",
		"media_processing_usages_active_media_idx",
		"media_processing_usages_cleanup_due_idx",
		"input_video_media_id",
		"input_audio_media_id",
		"output_media_id",
		"update media",
		"cleanup_after_at",
		"status = 'uploaded'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("media processing lifecycle migration missing %q, got:\n%s", want, string(body))
		}
	}
}

func TestMediaProcessingClaimQueryIsKindAware(t *testing.T) {
	sql := strings.ToLower(claimMediaProcessingJobsByKind)
	for _, want := range []string{
		"candidate.kind = $1",
		"candidate.status = 'queued'",
		"for update skip locked",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("kind-aware media processing claim query missing %q, got:\n%s", want, claimMediaProcessingJobsByKind)
		}
	}
}

func TestMediaPostUsageRetentionMigrationExists(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/097_media_post_usage_retention.sql")
	if err != nil {
		t.Fatalf("read media post usage retention migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"create table media_post_usages",
		"media_id",
		"post_id",
		"post_status",
		"cleanup_after_at",
		"media_post_usages_cleanup_due_idx",
		"jsonb_array_elements_text",
		"sp.status in ('published', 'partial', 'failed', 'cancelled')",
		"on conflict (media_id, post_id) do update",
		"update media set cleanup_after_at = null",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("media post usage migration missing %q, got:\n%s", want, string(body))
		}
	}
	if !strings.Contains(sql, "where cleanup_after_at is not null") ||
		!strings.Contains(sql, "post_status in ('published', 'partial', 'failed', 'cancelled')") {
		t.Fatalf("media post usage cleanup index should include every terminal retention status, got:\n%s", string(body))
	}
}

func TestMediaRetentionReviewFixMigrationBackfillsTerminalUsage(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/098_media_retention_review_fixes.sql")
	if err != nil {
		t.Fatalf("read media retention review fix migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"drop index if exists media_post_usages_cleanup_due_idx",
		"post_status in ('published', 'partial', 'failed', 'cancelled')",
		"jsonb_array_elements_text",
		"sp.status in ('published', 'partial', 'failed', 'cancelled')",
		"on conflict (media_id, post_id) do update",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("media retention review fix migration missing %q, got:\n%s", want, string(body))
		}
	}
}

func TestCreateSocialPostWithActiveScheduledCapIsAtomic(t *testing.T) {
	sql := strings.ToLower(createSocialPostWithActiveScheduledCap)
	for _, want := range []string{
		"pg_advisory_xact_lock",
		"count(*)::integer",
		"status = 'scheduled'",
		"deleted_at is null",
		"insert into social_posts",
		"returning",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("active scheduled cap create query missing %q, got:\n%s", want, createSocialPostWithActiveScheduledCap)
		}
	}
}

func TestCommittedScheduledQuotaQueryCountsPublishingAndQuotaHold(t *testing.T) {
	sql := strings.ToLower(countScheduledQuotaUnitsByWorkspaceAndPeriod)
	for _, want := range []string{
		"status in ('scheduled', 'quota_hold')",
		"status = 'publishing'",
		"scheduled_at is not null",
		"deleted_at is null",
		"disconnected_at is null",
		"admin_post_quota_resets",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("committed scheduled quota query missing %q, got:\n%s", want, countScheduledQuotaUnitsByWorkspaceAndPeriod)
		}
	}
}

func TestQuotaHoldLifecycleQueriesAllowRecoveryWithoutDispatch(t *testing.T) {
	rescheduleSQL := strings.ToLower(rescheduleSocialPost)
	for _, want := range []string{
		"status in ('scheduled', 'quota_hold')",
		"status = 'scheduled'",
		"quota_hold_reason = null",
		"quota_hold_at = null",
	} {
		if !strings.Contains(rescheduleSQL, want) {
			t.Fatalf("reschedule query missing %q, got:\n%s", want, rescheduleSocialPost)
		}
	}
	if !strings.Contains(strings.ToLower(cancelSocialPost), "'quota_hold'") {
		t.Fatalf("cancel query must allow quota_hold, got:\n%s", cancelSocialPost)
	}
	if !strings.Contains(strings.ToLower(claimDraftForPublish), "'quota_hold'") {
		t.Fatalf("publish-now claim must allow quota_hold, got:\n%s", claimDraftForPublish)
	}
	dueSQL := strings.ToLower(getDueScheduledPosts)
	if !strings.Contains(dueSQL, "where status = 'scheduled'") {
		t.Fatalf("due scheduler query must exclude quota_hold, got:\n%s", getDueScheduledPosts)
	}
}

func TestWorkspaceActiveScheduledLimitsMigrationSeedsTemporaryIncidentAllowance(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/102_workspace_active_scheduled_limits.sql")
	if err != nil {
		t.Fatalf("read workspace active scheduled limits migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"create table if not exists workspace_active_scheduled_limits",
		"limit_count integer not null check (limit_count > 0)",
		"expires_at",
		"corcodelgabrielaaa@gmail.com",
		"250",
		"2026-08-01 00:00:00+00",
		"on conflict (workspace_id) do update",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("workspace active scheduled limits migration missing %q, got:\n%s", want, string(body))
		}
	}
}

func TestPaidSoftOverageMigrationExists(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/107_paid_soft_overage.sql")
	if err != nil {
		t.Fatalf("read paid soft overage migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	for _, want := range []string{
		"quota_hold_reason",
		"quota_hold_at",
		"quota_hold_original_scheduled_at",
		"paid_plan_quota_notifications",
		"plan_id",
		"severity",
		"skipped_superseded",
		"retry_wait",
		"paid_quota_follow_ups",
		"unique (workspace_id, period, threshold_percent)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("paid soft overage migration missing %q, got:\n%s", want, string(body))
		}
	}
}

func TestMediaCleanupRunsMigrationAndQueriesExist(t *testing.T) {
	body, err := fs.ReadFile(migrations, "migrations/103_media_cleanup_runs.sql")
	if err != nil {
		t.Fatalf("read media cleanup runs migration: %v", err)
	}

	sql := strings.ToLower(string(body))
	sqlCompact := strings.Join(strings.Fields(sql), " ")
	for _, want := range []string{
		"create table media_cleanup_runs",
		"worker_name",
		"status text not null",
		"status in ('running', 'completed', 'completed_with_errors', 'failed', 'skipped')",
		"next_run_at",
		"scanned_objects integer not null default 0",
		"deleted_objects integer not null default 0",
		"deleted_bytes bigint not null default 0",
		"failed_objects integer not null default 0",
		"failed_bytes bigint not null default 0",
		"media_cleanup_runs_started_at_idx",
		"media_cleanup_runs_finished_at_idx",
		"media_cleanup_runs_status_idx",
		"media_cleanup_runs_one_running_idx",
		"where status = 'running'",
	} {
		if !strings.Contains(sqlCompact, want) {
			t.Fatalf("media cleanup runs migration missing %q, got:\n%s", want, string(body))
		}
	}

	queries, err := os.ReadFile("queries/media_cleanup_runs.sql")
	if err != nil {
		t.Fatalf("read media cleanup run queries: %v", err)
	}
	querySQL := string(queries)
	for _, want := range []string{
		"-- name: CreateMediaCleanupRun :one",
		"-- name: CompleteMediaCleanupRun :one",
		"-- name: MarkStaleMediaCleanupRunsFailed :execrows",
		"-- name: GetAdminObjectStorageCurrent :one",
		"-- name: GetAdminObjectStoragePeriodAdditions :one",
		"-- name: GetAdminObjectStorageDueBacklog :one",
		"-- name: GetAdminObjectStorageReferencedObjects :one",
		"-- name: GetAdminObjectStorageNextCleanupDeadline :one",
		"-- name: GetAdminObjectStoragePeriodCleanupRuns :one",
		"-- name: ListAdminObjectStorageDailyActivity :many",
		"-- name: GetAdminObjectStorageRunningSummary :one",
		"-- name: ListAdminObjectStorageRecentRuns :many",
		"-- name: GetAdminObjectStorageContentTypes :many",
		"-- name: GetAdminObjectStorageStatusBreakdown :many",
	} {
		if !strings.Contains(querySQL, want) {
			t.Fatalf("media cleanup run queries missing %q, got:\n%s", want, querySQL)
		}
	}
}

func TestPostgresDoBlocksAreGooseStatementBlocks(t *testing.T) {
	err := fs.WalkDir(migrations, "migrations", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sql" {
			return nil
		}

		body, err := fs.ReadFile(migrations, path)
		if err != nil {
			t.Fatalf("read migration %s: %v", path, err)
		}
		sql := string(body)
		if strings.Contains(sql, "DO $$") &&
			(!strings.Contains(sql, "-- +goose StatementBegin") ||
				!strings.Contains(sql, "-- +goose StatementEnd")) {
			t.Fatalf("%s contains a PostgreSQL DO block and must wrap it with goose StatementBegin/StatementEnd", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk migrations: %v", err)
	}
}
