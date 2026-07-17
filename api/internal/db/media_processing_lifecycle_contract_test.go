package db

import (
	"os"
	"strings"
	"testing"
)

func TestAudioOverlayJobCreationTracksInputsAtomically(t *testing.T) {
	source, err := os.ReadFile("queries/media_processing_jobs.sql")
	if err != nil {
		t.Fatalf("read media processing jobs queries: %v", err)
	}

	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: createaudiooverlaymediaprocessingjob :one",
		"with created_job as",
		"insert into media_processing_jobs",
		"insert into media_processing_usages",
		"'input'",
		"'active'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("atomic audio overlay creation query missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestGIFJobCreationTracksInputAndFairUseCounts(t *testing.T) {
	source, err := os.ReadFile("queries/media_processing_jobs.sql")
	if err != nil {
		t.Fatalf("read media processing jobs queries: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: creategifmediaprocessingjob :one",
		"'gif_to_mp4'",
		"insert into media_processing_usages",
		"created_job.input_media_id",
		"status in ('queued', 'retry_wait', 'processing')",
		"-- name: countgifconversionssince :one",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("GIF admission query contract missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestMediaProcessingRetryQueryUsesBackoffAndClaimDeadline(t *testing.T) {
	source, err := os.ReadFile("queries/media_processing_jobs.sql")
	if err != nil {
		t.Fatalf("read media processing jobs queries: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"candidate.next_attempt_at <= now()",
		"next_attempt_at = now() +",
		"interval '30 seconds'",
		"attempts < 3",
		"status = 'retry_wait'",
		"-- name: promoteduemediaprocessingretriesbykind :execrows",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("media processing retry contract missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestTerminalMediaProcessingQueriesTransitionLifecycleAtomically(t *testing.T) {
	source, err := os.ReadFile("queries/media_processing_usages.sql")
	if err != nil {
		t.Fatalf("read media processing usage queries: %v", err)
	}

	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: completemediaprocessingjobsucceeded :one",
		"-- name: completemediaprocessingjobfailed :one",
		"update media_processing_usages",
		"insert into media_processing_usages",
		"update media_processing_jobs",
		"cleanup_after_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("atomic terminal lifecycle queries missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestMarkMediaUploadedAssignsPlanAwareBaseDeadlineWithoutShortening(t *testing.T) {
	source, err := os.ReadFile("queries/media.sql")
	if err != nil {
		t.Fatalf("read media queries: %v", err)
	}

	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: markmediauploaded :one",
		"cleanup_after_at = greatest",
		"coalesce(m.cleanup_after_at, '-infinity'::timestamptz)",
		"from subscriptions",
		"when 'enterprise' then interval '30 days'",
		"and m.status = 'pending'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("mark media uploaded base retention missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestAbandonedUploadCleanupClaimsBeforeDeleting(t *testing.T) {
	source, err := os.ReadFile("queries/media.sql")
	if err != nil {
		t.Fatalf("read media queries: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: claimabandonedmedia :many",
		"status = 'pending'",
		"for update of candidate skip locked",
		"set status = 'deleted'",
		"returning m.*",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("abandoned media claim missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestPostUsageUpsertLocksAvailableParentForInsertAndReactivation(t *testing.T) {
	source, err := os.ReadFile("queries/media_post_usages.sql")
	if err != nil {
		t.Fatalf("read media post usage queries: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"with locked_media as",
		"update media",
		"usage_version = usage_version + 1",
		"status = 'uploaded'",
		"from locked_media",
		"select true::boolean as applied",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("post usage parent lock missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestUnifiedMediaCleanupUsesIndependentBasePostAndProcessingLedgers(t *testing.T) {
	for _, path := range []string{
		"queries/media_post_usages.sql",
		"queries/media_cleanup_runs.sql",
	} {
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sql := strings.ToLower(string(source))
		for _, want := range []string{
			"m.status = 'deleted'",
			"m.cleanup_after_at <= now()",
			"from media_post_usages post_due",
			"from media_processing_usages processing_due",
			"from media_post_usages post_blocker",
			"from media_processing_usages processing_blocker",
			"m.cleanup_after_at is null or m.cleanup_after_at <= now()",
		} {
			if !strings.Contains(sql, want) {
				t.Fatalf("unified cleanup query %s missing %q, got:\n%s", path, want, string(source))
			}
		}
	}
}

func TestMediaDeleteQueriesRejectBlockersAndScheduleSoftDelete(t *testing.T) {
	source, err := os.ReadFile("queries/media.sql")
	if err != nil {
		t.Fatalf("read media queries: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: hasblockingmediausage :one",
		"-- name: softdeleteunusedmedia :execrows",
		"from media_post_usages",
		"from media_processing_usages",
		"cleanup_after_at is null",
		"cleanup_after_at > now()",
		"set status = 'deleted'",
		"cleanup_after_at = now()",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("media delete query contract missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestMigration117BackfillsActiveSoftDeletedReferences(t *testing.T) {
	source, err := os.ReadFile("migrations/117_media_processing_lifecycle.sql")
	if err != nil {
		t.Fatalf("read lifecycle migration: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"m.status = 'deleted'",
		"sp.status in ('draft', 'scheduled', 'publishing', 'quota_hold')",
		"insert into media_post_usages",
		"null::timestamptz",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("soft-deleted reference backfill missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestMigration117SkipsOrphanedLegacyProcessingMedia(t *testing.T) {
	source, err := os.ReadFile("migrations/117_media_processing_lifecycle.sql")
	if err != nil {
		t.Fatalf("read lifecycle migration: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"join media existing_media",
		"existing_media.id = source.media_id",
		"existing_media.workspace_id = j.workspace_id",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("legacy processing backfill must skip orphaned media; missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestCleanupClaimsRowsBeforeObjectDeletion(t *testing.T) {
	source, err := os.ReadFile("queries/media_post_usages.sql")
	if err != nil {
		t.Fatalf("read media cleanup query: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"-- name: claimmediadueforretentioncleanup :many",
		"snapshot_candidates as materialized",
		"usage_version",
		"for update of m skip locked",
		"update media",
		"set status = 'deleted'",
		"returning m.*",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("atomic cleanup claim missing %q, got:\n%s", want, string(source))
		}
	}
}

func TestMigration117SerializesUsageCreationAndRollingWorkers(t *testing.T) {
	source, err := os.ReadFile("migrations/117_media_processing_lifecycle.sql")
	if err != nil {
		t.Fatalf("read lifecycle migration: %v", err)
	}
	sql := strings.ToLower(string(source))
	for _, want := range []string{
		"create function protect_media_usage_insert",
		"set usage_version = usage_version + 1",
		"create trigger media_post_usages_protect_media",
		"create trigger media_processing_usages_protect_media",
		"create function track_legacy_media_processing_job_inputs",
		"create trigger media_processing_jobs_track_legacy_inputs",
		"create function normalize_legacy_media_processing_retry",
		"create trigger media_processing_jobs_normalize_legacy_retry",
		"create function transition_legacy_media_processing_usages",
		"create trigger media_processing_jobs_transition_legacy_usages",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("rolling lifecycle compatibility missing %q, got:\n%s", want, string(source))
		}
	}
}
