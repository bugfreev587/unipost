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
