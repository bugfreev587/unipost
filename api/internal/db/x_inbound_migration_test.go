package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboundMigrationBackfillsDefaultNotificationsForExistingEligibleMembers(t *testing.T) {
	source, err := os.ReadFile("migrations/109_x_inbound_usage_controls.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	for _, want := range []string{
		"unipost_notification_channels",
		"unipost_notification_subscriptions",
		"workspace_members",
		"wm.status = 'active'",
		"wm.role IN ('owner', 'admin')",
		"billing.x_inbound_80pct",
		"billing.x_inbound_cap_reached",
		"c.workspace_id IS NULL",
		"c.verified_at IS NOT NULL",
		"s.workspace_id = em.workspace_id OR s.workspace_id IS NULL",
		"NOT EXISTS",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing notification backfill contract %q", want)
		}
	}
	if strings.Contains(sql, "DO UPDATE SET enabled") {
		t.Fatal("notification backfill must not re-enable a pre-existing opt-out")
	}
}

func TestXInboundMigrationReceiptPersistsOriginalAdmissionSnapshot(t *testing.T) {
	source, err := os.ReadFile("migrations/109_x_inbound_usage_controls.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	for _, want := range []string{
		"period_start",
		"period_end",
		"monthly_used_after",
		"monthly_remaining_after",
		"inbound_daily_used_after",
		"inbound_daily_limit",
		"events_accepted_after",
		"events_suppressed_after",
		"pause_paid_sources",
		"pause_reason",
		"reset_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("receipt schema missing %q", want)
		}
	}
}
