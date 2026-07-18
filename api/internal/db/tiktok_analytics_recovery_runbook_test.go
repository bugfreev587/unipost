package db

import (
	"os"
	"strings"
	"testing"
)

func TestTikTokAnalyticsRecoveryRunbook(t *testing.T) {
	sourceBytes, err := os.ReadFile("../../ops/tiktok_analytics_recovery.sql")
	if err != nil {
		t.Fatalf("read recovery SQL: %v", err)
	}
	source := strings.ToLower(string(sourceBytes))

	for _, required := range []string{
		`\set on_error_stop on`,
		`:{?deployment_timestamp}`,
		`\set execute false`,
		`begin;`,
		`create temp table tiktok_analytics_recovery_eligible`,
		`sa.platform = 'tiktok'`,
		`sa.status = 'active'`,
		`sa.disconnected_at is null`,
		`spr.status = 'published'`,
		`spr.external_id is not null`,
		`spr.published_at is not null`,
		`interval '90 days'`,
		`sp.deleted_at is null`,
		`on conflict (social_post_result_id) do update`,
		`set fetched_at = excluded.fetched_at`,
		`post_analytics.fetched_at < :'deployment_timestamp'::timestamptz`,
		`'1970-01-01 00:00:00+00'::timestamptz`,
		`\if :execute`,
		`\if :execute`,
		`rollback;`,
		`commit;`,
	} {
		if !strings.Contains(source, required) {
			t.Errorf("recovery SQL missing %q", required)
		}
	}
	if strings.Count(source, "begin;") != 1 {
		t.Errorf("BEGIN count = %d, want 1", strings.Count(source, "begin;"))
	}
	if strings.Contains(source, "last_refreshed_at") {
		t.Error("recovery SQL must not use social_accounts.last_refreshed_at")
	}

	conflictStart := strings.Index(source, "on conflict (social_post_result_id) do update")
	if conflictStart < 0 {
		t.Fatal("could not find recovery conflict update")
	}
	returningStart := strings.Index(source[conflictStart:], "returning")
	if returningStart < 0 {
		t.Fatal("could not isolate recovery conflict update")
	}
	conflictUpdate := source[conflictStart : conflictStart+returningStart]
	for _, forbidden := range []string{
		"views =",
		"likes =",
		"comments =",
		"shares =",
		"reach =",
		"impressions =",
		"saves =",
		"clicks =",
		"video_views =",
		"platform_specific =",
		"raw_data =",
		"consecutive_failures =",
		"last_failure_reason =",
	} {
		if strings.Contains(conflictUpdate, forbidden) {
			t.Errorf("conflict update must not contain %q", forbidden)
		}
	}
}
