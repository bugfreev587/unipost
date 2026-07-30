package db

import (
	"os"
	"strings"
	"testing"
)

func TestAPIRequestEventsMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/130_api_request_events.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))

	for _, required := range []string{
		"create table api_request_events",
		"partition by range (occurred_at)",
		"primary key (occurred_at, id, workspace_id)",
		"create table api_request_error_details",
		"primary key (occurred_at, event_id, workspace_id)",
		"foreign key (occurred_at, event_id, workspace_id)",
		"on delete cascade",
		"octet_length(request_payload)",
		"octet_length(response_payload)",
		"octet_length(coalesce(request_content_type, ''))",
		"octet_length(coalesce(response_content_type, ''))",
		"octet_length(coalesce(request_sha256, ''))",
		"octet_length(coalesce(response_sha256, ''))",
		"octet_length(coalesce(request_omission_reason, ''))",
		"octet_length(coalesce(response_omission_reason, ''))",
		"create table api_request_metric_rollups_hourly",
		"primary key (hour_bucket, workspace_id, route_pattern, method, status_code)",
		"create table api_request_partition_manifest",
		"observability_reads_v2",
		"false",
		"on conflict (key) do nothing",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 130 missing %q", required)
		}
	}

	for _, requiredIndex := range []string{
		"api_request_events_workspace_time_idx",
		"api_request_events_admin_time_idx",
		"api_request_events_workspace_request_idx",
		"api_request_events_workspace_route_time_idx",
	} {
		if !strings.Contains(sql, requiredIndex) {
			t.Fatalf("migration 130 missing index %q", requiredIndex)
		}
	}

	for _, protected := range []string{
		"alter table social_posts",
		"alter table social_accounts",
		"alter table social_post_results",
		"alter table post_delivery_jobs",
		"alter table webhook_deliveries",
		"update social_posts",
		"update social_accounts",
		"update social_post_results",
		"update post_delivery_jobs",
		"delete from social_posts",
		"delete from social_accounts",
		"delete from social_post_results",
		"delete from post_delivery_jobs",
	} {
		if strings.Contains(sql, protected) {
			t.Fatalf("migration 130 touches protected surface %q", protected)
		}
	}
	if strings.Contains(sql, "delete from feature_flag_changes") {
		t.Fatal("migration must not delete immutable feature-flag audit history")
	}
}

func TestAPIRequestEventAndDetailPartitionsAreAligned(t *testing.T) {
	body, err := os.ReadFile("migrations/130_api_request_events.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))

	for _, bounds := range []string{
		"from ('2026-07-27 00:00:00+00') to ('2026-08-03 00:00:00+00')",
		"from ('2026-08-03 00:00:00+00') to ('2026-08-10 00:00:00+00')",
	} {
		if strings.Count(sql, bounds) != 2 {
			t.Fatalf("partition bounds %q must occur once for event and once for detail", bounds)
		}
	}
	if strings.Count(sql, "default") < 2 {
		t.Fatal("event and detail parents must both have a deployment-safety default partition")
	}
}
