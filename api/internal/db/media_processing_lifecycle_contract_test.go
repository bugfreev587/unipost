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
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("mark media uploaded base retention missing %q, got:\n%s", want, string(source))
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
