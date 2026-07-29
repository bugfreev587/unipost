package db

import (
	"os"
	"strings"
	"testing"
)

func TestWorkspaceLogsSeekIndexMigrationIsConcurrentReversibleAndIsolated(t *testing.T) {
	body, err := os.ReadFile("migrations/131_integration_logs_workspace_seek_index.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- +goose no transaction",
		"-- unipost:safety reversible",
		"drop index concurrently if exists idx_integration_logs_workspace_ts_id",
		"create index concurrently if not exists idx_integration_logs_workspace_ts_id",
		"on integration_logs (workspace_id, ts desc, id desc)",
		"-- +goose down",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 131 missing %q", required)
		}
	}
	upEnd := strings.Index(sql, "-- +goose down")
	if upEnd < 0 {
		t.Fatal("migration 131 is missing Down")
	}
	up := sql[:upEnd]
	if strings.Index(up, "drop index concurrently if exists idx_integration_logs_workspace_ts_id") > strings.Index(up, "create index concurrently if not exists idx_integration_logs_workspace_ts_id") {
		t.Fatal("Up must remove an invalid same-name index before concurrent creation")
	}
	for _, protected := range []string{"social_posts", "social_accounts", "social_post_results", "post_delivery_jobs", "webhook_deliveries", "subscriptions", "usage"} {
		if strings.Contains(sql, protected) {
			t.Fatalf("migration 131 touches protected relation %q", protected)
		}
	}
}
