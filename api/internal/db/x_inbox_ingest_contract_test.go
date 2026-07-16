package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxIngestQueriesPreserveAppAndPlanIsolation(t *testing.T) {
	inboxSQL, err := os.ReadFile("queries/inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(inboxSQL)
	for _, required := range []string{
		"sa.platform IN ('instagram', 'threads', 'facebook', 'twitter')",
		"-- name: FindXInboxAccountForApp :one",
		"-- name: FindXInboxAccountsForExternalUserApp :many",
		"sa.x_app_mode = 'unipost_managed_app'",
		"sa.x_app_mode = 'workspace_x_app'",
		"pc.webhook_route_key = sqlc.arg(webhook_route_key)::TEXT",
		"COALESCE(pl.allow_inbox, FALSE) AS plan_allows_inbox",
		"sa.scope",
		"sa.connection_type",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("inbox.sql missing %q", required)
		}
	}

	credentialsSQL, err := os.ReadFile("queries/platform_credentials.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(credentialsSQL), "-- name: ListTwitterConsumerSecretsByWebhookRouteKey :many") {
		t.Fatal("platform credential queries must resolve workspace consumer secrets by unguessable webhook route key")
	}
	for _, required := range []string{
		"FROM x_inbox_delivery_cleanup_intents",
		"webhook_route_key = $1",
		"consumer_secret IS NOT NULL",
	} {
		if !strings.Contains(string(credentialsSQL), required) {
			t.Fatalf("platform credential secret resolver query missing pending cleanup support %q", required)
		}
	}
}
