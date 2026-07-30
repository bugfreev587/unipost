package db

import (
	"os"
	"strings"
	"testing"
)

func TestAdminLogsGlobalIndexMigrationIsConcurrentAndLogOnly(t *testing.T) {
	body, err := os.ReadFile("migrations/128_admin_logs_global_time_index.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- +goose no transaction",
		"create index concurrently if not exists idx_integration_logs_admin_ts_id",
		"on integration_logs (ts desc, id desc)",
		"drop index concurrently if exists idx_integration_logs_admin_ts_id",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	upEnd := strings.Index(sql, "-- +goose down")
	if upEnd < 0 {
		t.Fatal("migration is missing a down section")
	}
	up := sql[:upEnd]
	dropAt := strings.Index(up, "drop index concurrently if exists idx_integration_logs_admin_ts_id")
	createAt := strings.Index(up, "create index concurrently if not exists idx_integration_logs_admin_ts_id")
	if dropAt < 0 || createAt < 0 || dropAt > createAt {
		t.Fatal("up migration must remove an invalid same-name index before retrying concurrent creation")
	}
	for _, protected := range []string{"social_posts", "social_accounts", "social_post_results", "post_delivery_jobs"} {
		if strings.Contains(sql, protected) {
			t.Fatalf("migration touches protected relation %q", protected)
		}
	}
}
