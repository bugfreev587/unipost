package db

import (
	"os"
	"strings"
	"testing"
)

func TestAdminScheduledQuotaResetBaselineSQLContract(t *testing.T) {
	source, err := os.ReadFile("social_posts_ext.go")
	if err != nil {
		t.Fatalf("read social_posts_ext.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"admin_post_quota_resets",
		"quota_kind = 'scheduled'",
		"MAX(reset_at)",
		"sp.created_at > COALESCE(",
		"CountScheduledQuotaUnitsByWorkspaceAndPeriod",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("scheduled quota count should honor admin reset baseline %q:\n%s", want, sql)
		}
	}
}

func TestAdminPostQuotaResetMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/099_admin_post_quota_resets.sql")
	if err != nil {
		t.Fatalf("read admin quota reset migration: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS admin_post_quota_resets",
		"quota_kind TEXT NOT NULL",
		"CHECK (quota_kind IN ('post', 'scheduled'))",
		"UNIQUE (user_id, workspace_id, period, quota_kind)",
		"CREATE INDEX IF NOT EXISTS idx_admin_post_quota_resets_workspace_period",
		"DROP TABLE IF EXISTS admin_post_quota_resets",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post quota reset migration missing %q:\n%s", want, sql)
		}
	}
}
