package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxDeliveryCleanupMigrationPreservesUpstreamIDsBeforeCascade(t *testing.T) {
	migration, err := os.ReadFile("migrations/110_x_inbox_delivery_cleanup_intents.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(migration)
	for _, required := range []string{
		"CREATE TABLE x_inbox_delivery_cleanup_intents",
		"filtered_stream_rule_id",
		"activity_dm_subscription_id",
		"app_bearer_token",
		"user_access_token",
		"BEFORE DELETE ON social_accounts",
		"BEFORE DELETE ON platform_credentials",
		"enqueue_x_inbox_delivery_cleanup",
		"enqueue_workspace_x_inbox_delivery_cleanup",
		"ON CONFLICT (social_account_id)",
		"COALESCE(EXCLUDED.app_bearer_token, x_inbox_delivery_cleanup_intents.app_bearer_token)",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}
