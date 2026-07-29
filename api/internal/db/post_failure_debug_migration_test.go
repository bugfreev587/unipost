package db

import (
	"os"
	"strings"
	"testing"
)

func TestPostFailureDebugMigrationIsBoundedAndObservabilityOwned(t *testing.T) {
	body, err := os.ReadFile("migrations/129_post_failure_debug_details.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"create table post_failure_debug_details",
		"social_post_result_id text primary key",
		"workspace_id text not null",
		"debug_text text not null",
		"octet_length(debug_text) <= 65536",
		"original_bytes bigint not null",
		"stored_bytes integer not null",
		"redaction_version integer not null",
		"create index post_failure_debug_details_workspace_captured_idx",
		"drop table if exists post_failure_debug_details",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"foreign key",
		"references social_post_results",
		"alter table social_posts",
		"alter table social_accounts",
		"alter table social_post_results",
		"alter table post_delivery_jobs",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration contains protected-table operation %q", forbidden)
		}
	}
}
