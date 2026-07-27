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

func TestPublishingRestrictionSendGateMigrationRepairsActiveRetentionCompatibility(t *testing.T) {
	body, err := os.ReadFile("migrations/124_publishing_restriction_email_send_gate.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(body)
	for _, fragment := range []string{
		"ADD COLUMN retryable",
		"ADD COLUMN attempt_generation",
		"SET retention_reason = 'active_post'",
		"WHERE cleanup_after_at IS NULL",
		"AND retention_reason = 'plan_status'",
	} {
		if !strings.Contains(sql, fragment) {
			t.Errorf("compatibility migration missing %q", fragment)
		}
	}
}

func TestPublishingRestrictionFailedRecipientCorrectionMigrationContract(t *testing.T) {
	historical, err := os.ReadFile("migrations/124_publishing_restriction_email_send_gate.sql")
	if err != nil {
		t.Fatal(err)
	}
	historicalParts := strings.Split(string(historical), "-- +goose Down")
	if len(historicalParts) != 2 {
		t.Fatalf("migration 124 must have one Down section, got %d parts", len(historicalParts))
	}
	if !strings.Contains(strings.ToLower(historicalParts[1]), "retention_reason backfill is irreversible") {
		t.Fatal("migration 124 Down must explicitly document that its retention_reason backfill is irreversible")
	}

	correction, err := os.ReadFile("migrations/125_publishing_restriction_failed_recipient_retryability.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(correction), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration 125 must have one Down section, got %d parts", len(parts))
	}
	up := strings.ToLower(parts[0])
	for _, fragment := range []string{
		"update platform_publishing_restriction_email_recipients",
		"set retryable = false",
		"where status = 'failed'",
	} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("migration 125 Up missing %q", fragment)
		}
	}
	if !strings.Contains(strings.ToLower(parts[1]), "retryability correction is irreversible") {
		t.Fatal("migration 125 Down must refuse to reactivate failed recipients and document irreversibility")
	}
}

func TestPublishingRestrictionRecipientOwnerSnapshotMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/126_publishing_restriction_recipient_owner_snapshot.sql")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(body), "-- +goose Down")
	if len(parts) != 2 {
		t.Fatalf("migration 126 must have one Down section, got %d parts", len(parts))
	}
	up := strings.ToLower(parts[0])
	for _, fragment := range []string{
		"add column represented_owner_user_ids text[]",
		"create function backfill_publishing_restriction_recipient_owner_snapshot()",
		"before insert",
		"new.represented_owner_user_ids is null",
		"unnest(new.represented_workspace_ids) with ordinality",
		"lower(trim(owner_user.email)) = new.normalized_email",
		"count(owner_user.id) as candidate_count",
		"resolution.candidate_count = 1",
		"array_fill(canonical_user_id",
		"array[]::text[]",
		"historical rows cannot safely",
		"set not null",
		"cardinality(represented_owner_user_ids) = cardinality(represented_workspace_ids)",
		"array_position(represented_owner_user_ids, null) is null",
	} {
		if !strings.Contains(up, fragment) {
			t.Fatalf("migration 126 Up missing %q", fragment)
		}
	}
	if strings.Contains(up, "before insert or update") || strings.Contains(up, "update of represented_workspace_ids") {
		t.Fatal("migration 126 trigger must be INSERT-only because recipient snapshots are immutable")
	}
	down := strings.ToLower(parts[1])
	for _, fragment := range []string{
		"drop trigger",
		"drop function backfill_publishing_restriction_recipient_owner_snapshot()",
		"drop constraint platform_publishing_restriction_recipient_owner_snapshot_shape_check",
		"drop column represented_owner_user_ids",
	} {
		if !strings.Contains(down, fragment) {
			t.Fatalf("migration 126 Down missing %q", fragment)
		}
	}
}
