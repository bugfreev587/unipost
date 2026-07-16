package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxDeliveryResourceQueryContract(t *testing.T) {
	migration, err := os.ReadFile("migrations/108_x_inbox_oauth_and_delivery.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(migration)
	for _, required := range []string{
		"ALTER TABLE social_accounts",
		"ADD COLUMN x_app_mode TEXT",
		"unipost_managed_app",
		"workspace_x_app",
		"JOIN platform_credentials pc",
		"WHEN sa.connection_type = 'managed' THEN 'unipost_managed_app'",
		"ELSE 'workspace_x_app'",
		"x_inbox_delivery_resources",
		"filtered_stream_rule_id",
		"activity_dm_subscription_id",
		"paused_allowance",
		"app_bearer_token",
		"consumer_secret",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("migration 108 missing %q", required)
		}
	}

	queries, err := os.ReadFile("queries/x_inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	queryText := string(queries)
	for _, required := range []string{
		"-- name: GetXInboxDeliveryResource :one",
		"-- name: UpsertXInboxDeliveryResource :one",
		"-- name: UpdateXInboxDeliveryResource :one",
		"-- name: DeleteXInboxDeliveryResource :exec",
		"WHERE social_account_id = $1",
		"ON CONFLICT (social_account_id)",
	} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("x_inbox.sql missing %q", required)
		}
	}
}
