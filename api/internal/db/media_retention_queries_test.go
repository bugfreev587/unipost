package db

import (
	"os"
	"strings"
	"testing"
)

func TestMediaRetentionCleanupQueryIncludesExactDeadlineAndActiveBlockers(t *testing.T) {
	body, err := os.ReadFile("queries/media_post_usages.sql")
	if err != nil {
		t.Fatalf("read media retention queries: %v", err)
	}
	sql := string(body)
	for _, required := range []string{
		"due.cleanup_after_at <= NOW()",
		"blocker.cleanup_after_at IS NULL",
		"blocker.cleanup_after_at > NOW()",
		"NOT EXISTS",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("media retention cleanup query missing %q", required)
		}
	}
}
