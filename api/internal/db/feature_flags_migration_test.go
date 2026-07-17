package db

import (
	"os"
	"strings"
	"testing"
)

func TestFeatureFlagsMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/118_feature_flags.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(body)
	for _, required := range []string{
		"CREATE TABLE feature_flags",
		"CREATE TABLE feature_flag_changes",
		"x_dms_v1",
		"x_credits_billing_v1",
		"DEFAULT FALSE",
		"previous_enabled",
		"changed_by",
		"changed_at",
		"ON CONFLICT (key) DO NOTHING",
		"accounting_enabled BOOLEAN NOT NULL DEFAULT TRUE",
	} {
		if !strings.Contains(schema, required) {
			t.Fatalf("migration 118 missing %q", required)
		}
	}
}
