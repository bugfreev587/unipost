package db

import (
	"os"
	"strings"
	"testing"
)

func TestInboxQueriesCanExcludeXDMSAtTheDatabaseBoundary(t *testing.T) {
	body, err := os.ReadFile("queries/inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries := string(body)
	if got := strings.Count(queries, "exclude_x_dms"); got != 3 {
		t.Fatalf("exclude_x_dms query gates = %d, want 3 for list, unread count, and mark-all-read", got)
	}
}
