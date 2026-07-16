package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxOutboundIdempotencyPersistsWorkspaceBoundClaims(t *testing.T) {
	source, err := os.ReadFile("migrations/114_x_inbox_outbound_idempotency.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	for _, want := range []string{
		"CREATE TABLE x_inbox_outbound_requests",
		"workspace_id",
		"inbox_item_id",
		"idempotency_key",
		"payload_hash",
		"status",
		"response_inbox_item_id",
		"UNIQUE (workspace_id, inbox_item_id, idempotency_key)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}
