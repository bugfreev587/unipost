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
		"-- name: FindXInboxAccountsForProviderUserApp :many",
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

func TestXInboxIngestContractLegacyProviderUserLookupIsExactAndMany(t *testing.T) {
	inboxSQL, err := os.ReadFile("queries/inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(inboxSQL)
	const start = "-- name: FindXInboxAccountsForProviderUserApp :many"
	startIndex := strings.Index(text, start)
	if startIndex < 0 {
		t.Fatalf("inbox.sql missing %q", start)
	}
	query := text[startIndex:]
	if endIndex := strings.Index(query[len(start):], "-- name:"); endIndex >= 0 {
		query = query[:len(start)+endIndex]
	}
	if !strings.Contains(query, "sa.external_account_id = sqlc.arg(provider_user_id)::TEXT") {
		t.Fatal("legacy X DM lookup must compare provider_user_id only to external_account_id")
	}
	if strings.Contains(query, "sa.external_user_id =") {
		t.Fatal("legacy X DM lookup must never compare provider_user_id to external_user_id")
	}
	if strings.Contains(strings.ToUpper(query), "LIMIT 1") {
		t.Fatal("legacy X DM lookup must return all candidates for exact-one cardinality enforcement")
	}
	if !strings.Contains(query, "ORDER BY sa.connected_at DESC, sa.id") {
		t.Fatal("legacy X DM lookup must preserve deterministic ordering")
	}
}
