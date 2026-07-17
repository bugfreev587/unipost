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
