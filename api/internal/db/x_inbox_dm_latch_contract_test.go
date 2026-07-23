package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxDMLatchContract(t *testing.T) {
	migration, err := os.ReadFile("migrations/120_x_inbox_dm_forbidden_latch.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationText := string(migration)
	for _, required := range []string{
		"ALTER TABLE x_inbox_delivery_resources",
		"ADD COLUMN dm_subscription_forbidden_fingerprint TEXT",
	} {
		if !strings.Contains(migrationText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"ADD COLUMN dm_subscription_forbidden_fingerprint TEXT NOT NULL",
		"ADD COLUMN dm_subscription_forbidden_fingerprint TEXT DEFAULT",
		"UPDATE x_inbox_delivery_resources",
		"DELETE FROM x_inbox_delivery_resources",
	} {
		if strings.Contains(migrationText, forbidden) {
			t.Fatalf("migration must add a nullable column without rewriting data; found %q", forbidden)
		}
	}

	queries, err := os.ReadFile("queries/x_inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	queryText := string(queries)
	for _, required := range []string{
		"dm_subscription_forbidden_fingerprint",
		"dm_subscription_forbidden_fingerprint = EXCLUDED.dm_subscription_forbidden_fingerprint",
	} {
		if !strings.Contains(queryText, required) {
			t.Fatalf("x_inbox state queries missing %q", required)
		}
	}
}
