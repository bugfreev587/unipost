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
		"legacy_unknown",
		"UPDATE x_usage_events\nSET connection_mode = 'legacy_unknown'",
		"WHERE x_credit_billing_mode IS NOT NULL",
		"DELETE FROM oauth_states\nWHERE platform = 'twitter'",
		"UPDATE connect_sessions\nSET status = 'expired'",
		"expires_at = LEAST(expires_at, NOW())",
		"completed_at = COALESCE(completed_at, NOW())",
		"WHERE platform = 'twitter'\n  AND status = 'pending'",
		"UPDATE social_accounts\nSET x_app_mode = 'legacy_unknown'\nWHERE platform = 'twitter'",
		"(platform = 'twitter' AND (x_app_mode IS NULL OR x_app_mode IN",
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
	for _, forbidden := range []string{
		"JOIN platform_credentials pc",
		"WHEN sa.connection_type = 'managed'",
		"UPDATE oauth_states os\nSET x_app_mode",
		"UPDATE connect_sessions cs\nSET x_app_mode",
	} {
		if strings.Contains(schema, forbidden) {
			t.Fatalf("migration 108 must not infer legacy X app identity using %q", forbidden)
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
		"-- name: UpdateXInboxFilteredStreamRule :one",
		"-- name: UpdateXInboxActivityDMSubscription :one",
		"-- name: DeleteXInboxDeliveryResource :exec",
		"WHERE social_account_id = $1",
		"ON CONFLICT (social_account_id)",
	} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("x_inbox.sql missing %q", required)
		}
	}
}
