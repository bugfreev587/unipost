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
		"pc.client_id = sqlc.arg(app_client_id)::TEXT",
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
	if !strings.Contains(string(credentialsSQL), "-- name: ListTwitterConsumerSecretsByClientID :many") {
		t.Fatal("platform credential queries must resolve workspace consumer secrets by public app client id")
	}
}
