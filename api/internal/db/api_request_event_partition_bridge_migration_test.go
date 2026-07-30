package db

import (
	"os"
	"strings"
	"testing"
)

func TestAPIRequestEventPartitionBridgeMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/133_api_request_event_partition_bridge.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- unipost:safety reversible",
		"-- +goose statementbegin",
		"-- +goose statementend",
		"set local lock_timeout = '5s'",
		"set local statement_timeout = '30s'",
		"api_request_events_default",
		"api_request_error_details_default",
		"2026-08-10 00:00:00+00",
		"2026-10-05 00:00:00+00",
		"api_request_events_2026w33",
		"api_request_error_details_2026w40",
		"api_request_partition_manifest",
		"bridge partition contains data",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 133 missing %q", required)
		}
	}
	if strings.Count(sql, "-- +goose statementbegin") != 2 ||
		strings.Count(sql, "-- +goose statementend") != 2 {
		t.Fatal("migration 133 must wrap both DO blocks in Goose statement boundaries")
	}
	for _, week := range []string{"33", "34", "35", "36", "37", "38", "39", "40"} {
		if !strings.Contains(sql, "api_request_events_2026w"+week) ||
			!strings.Contains(sql, "api_request_error_details_2026w"+week) {
			t.Fatalf("migration 133 missing aligned week %s", week)
		}
	}
}

func TestAPIRequestEventPartitionBridgeDownIsDataPreserving(t *testing.T) {
	body, err := os.ReadFile("migrations/133_api_request_event_partition_bridge.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(strings.ToLower(string(body)), "-- +goose down")
	if len(parts) != 2 {
		t.Fatal("migration 133 must have one Down section")
	}
	down := parts[1]
	for _, required := range []string{
		"api_request_events_2026w33",
		"api_request_error_details_2026w40",
		"raise exception",
		"delete from api_request_partition_manifest",
		"from pg_constraint",
		"drop constraint",
		"add constraint",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("migration 133 Down missing %q", required)
		}
	}
	if strings.Contains(down, "drop table %i cascade") {
		t.Fatal("migration 133 Down must not bypass dependencies with DROP TABLE CASCADE")
	}
}
