package db

import (
	"io/fs"
	"os"
	"strings"
	"testing"
)

func TestNotificationQueriesUseUniPostOwnedTables(t *testing.T) {
	raw, err := fs.ReadFile(migrations, "migrations/093_unipost_notification_tables.sql")
	if err != nil {
		t.Fatalf("read UniPost notification table migration: %v", err)
	}
	migrationSQL := strings.ToLower(string(raw))
	querySQL := strings.ToLower(readQueryFile(t, "queries/notifications.sql"))

	for _, table := range []string{
		"unipost_notification_channels",
		"unipost_notification_subscriptions",
		"unipost_notification_deliveries",
	} {
		if !strings.Contains(migrationSQL, table) {
			t.Fatalf("notification migration should create/use %s:\n%s", table, string(raw))
		}
		if !strings.Contains(querySQL, table) {
			t.Fatalf("notification queries should use %s:\n%s", table, querySQL)
		}
	}

	for _, legacy := range []string{
		" from notification_channels",
		" into notification_channels",
		" update notification_channels",
		" from notification_subscriptions",
		" into notification_subscriptions",
		" update notification_subscriptions",
		" from notification_deliveries",
		" into notification_deliveries",
		" update notification_deliveries",
	} {
		if strings.Contains(querySQL, legacy) {
			t.Fatalf("notification queries should not depend on shared legacy table pattern %q:\n%s", legacy, querySQL)
		}
	}
}

func readQueryFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read query file %s: %v", path, err)
	}
	return string(raw)
}
