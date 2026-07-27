package db

import (
	"os"
	"strings"
	"testing"
)

func TestTerminalPostEventOutboxMigrationIsReversibleSchemaOnly(t *testing.T) {
	body, err := os.ReadFile("migrations/127_terminal_post_event_outbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- +goose up",
		"-- +goose down",
		"event_type      text not null default ''",
		"create table terminal_post_first_publish_elections",
		"workspace_id text primary key references workspaces(id) on delete cascade",
		"unique (post_id, parent_version)",
		"primary key (event_id, sink)",
		"create table terminal_post_event_webhook_deliveries",
		"primary key (event_id, webhook_id)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration 127 missing %q", required)
		}
	}
	up := strings.Split(sql, "-- +goose down")[0]
	for _, forbidden := range []string{"\nupdate ", "\ndelete ", "\ninsert into "} {
		if strings.Contains(up, forbidden) {
			t.Fatalf("migration 127 Up contains data mutation %q", forbidden)
		}
	}
}

func TestTerminalPostEventOutboxClaimContract(t *testing.T) {
	body, err := os.ReadFile("queries/terminal_post_event_outbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"for update of delivery skip locked",
		"earlier_event.sequence_number < event.sequence_number",
		"earlier_delivery.status <> 'enqueued'",
		"lease_expires_at <= now()",
		"on conflict (post_id, parent_version) do nothing",
		"on conflict (workspace_id) do nothing",
		"status = 'processing'",
		"attempts = sqlc.arg(expected_attempts)",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("terminal event query missing %q", required)
		}
	}
}
