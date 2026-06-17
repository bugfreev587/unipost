package errortriage

import (
	"strings"
	"testing"
)

func TestFindPreviousItemSQLUsesTextFallbackForTextIDs(t *testing.T) {
	if strings.Contains(findPreviousItemSQL, "::uuid") {
		t.Fatalf("FindPreviousItem compares TEXT ids with a UUID fallback: %s", findPreviousItemSQL)
	}
	if !strings.Contains(findPreviousItemSQL, "COALESCE((SELECT supersedes_run_id FROM error_triage_runs WHERE id = $2), '')") {
		t.Fatalf("FindPreviousItem should use a TEXT fallback for supersedes_run_id: %s", findPreviousItemSQL)
	}
}
