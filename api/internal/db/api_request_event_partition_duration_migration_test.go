package db

import (
	"os"
	"strings"
	"testing"
)

func TestAPIRequestEventPartitionDurationMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/134_api_request_partition_manifest_duration.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	parts := strings.Split(sql, "-- +goose down")
	if len(parts) != 2 {
		t.Fatal("migration 134 must have exactly one Down section")
	}
	up, down := parts[0], parts[1]
	for _, required := range []string{
		"-- unipost:safety reversible",
		"drop constraint api_request_partition_manifest_check",
		"add constraint api_request_partition_manifest_week_duration_check",
		"extract(epoch from (week_end - week_start)) = 604800",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("migration 134 Up missing %q", required)
		}
	}
	if strings.Contains(up, "week_start + interval '7 days'") {
		t.Fatal("migration 134 Up must not use calendar-day TIMESTAMPTZ arithmetic")
	}
	for _, required := range []string{
		"drop constraint api_request_partition_manifest_week_duration_check",
		"add constraint api_request_partition_manifest_check",
		"week_end = week_start + interval '7 days'",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("migration 134 Down missing %q", required)
		}
	}
}
