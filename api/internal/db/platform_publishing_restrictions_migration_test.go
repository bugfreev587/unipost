package db

import (
	"os"
	"strings"
	"testing"
)

func TestPlatformPublishingRestrictionsMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/122_platform_publishing_restrictions.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	required := []string{
		"CREATE TABLE platform_publishing_restrictions",
		"CREATE TABLE platform_publishing_restriction_events",
		"CREATE TABLE platform_publishing_restriction_email_campaigns",
		"CREATE TABLE platform_publishing_restriction_email_recipients",
		"ADD COLUMN retention_reason",
		"ARRAY['free']::TEXT[]",
		"'tiktok'",
		"FALSE",
		"UNIQUE (cycle_id, campaign_type)",
		"UNIQUE (campaign_id, canonical_user_id)",
		"UNIQUE (campaign_id, normalized_email)",
		"CHECK (retention_reason IN ('active_post', 'plan_status', 'publishing_restriction'))",
	}
	for _, fragment := range required {
		if !strings.Contains(sql, fragment) {
			t.Errorf("migration missing %q", fragment)
		}
	}
	if strings.Contains(sql, "enabled, restricted_plan_ids") && !strings.Contains(sql, "'tiktok', FALSE, ARRAY['free']::TEXT[]") {
		t.Error("TikTok restriction seed must be disabled for Free")
	}
}

func TestPublishingRestrictionCycleCorrelationUsesFollowUpMigration(t *testing.T) {
	body, err := os.ReadFile("migrations/123_post_failures_restriction_cycle.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, fragment := range []string{"ADD COLUMN restriction_cycle_id", "post_failures_restriction_cycle_idx"} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("follow-up migration missing %q", fragment)
		}
	}
}
