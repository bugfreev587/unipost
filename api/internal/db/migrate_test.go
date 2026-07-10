package db

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
