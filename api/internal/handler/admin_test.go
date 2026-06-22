package handler

import (
	"os"
	"strings"
	"testing"
)

func TestAdminPostPlatformsSQLIncludesStoredTargetPlatforms(t *testing.T) {
	sql := adminPostPlatformsSQL("sp")

	for _, want := range []string{
		"social_post_results",
		"jsonb_typeof(sp.metadata->'platform_posts') = 'array'",
		"target_post.value->>'account_id'",
		"jsonb_typeof(sp.metadata->'account_ids') = 'array'",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post platforms SQL missing %q:\n%s", want, sql)
		}
	}
}

func TestAdminPostPlatformFilterSQLUsesStoredTargetPlatforms(t *testing.T) {
	sql := adminPostPlatformFilterSQL("sp", "$7")

	if !strings.Contains(sql, "$7 = ANY(") {
		t.Fatalf("admin post platform filter should compare against the platform array:\n%s", sql)
	}
	if !strings.Contains(sql, "platform_posts") || !strings.Contains(sql, "account_ids") {
		t.Fatalf("admin post platform filter should include stored target metadata:\n%s", sql)
	}
	if strings.Contains(sql, "__POST_ALIAS__") {
		t.Fatalf("admin post platform filter still contains an unsubstituted alias placeholder:\n%s", sql)
	}
}

func TestAdminPostsSQLSupportsFailedResultFiltering(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ResultStatus string",
		`normalizeAdminPostResultStatus(q.Get("result_status"))`,
		"failed_result_count > 0",
		"($10::TEXT = '' OR ($10 = 'failed' AND failed_result_count > 0))",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin posts SQL should support failed result filtering %q", want)
		}
	}
}

func TestAdminPostsPageExposesPartialAndFailedAttemptFilters(t *testing.T) {
	pageSource, err := os.ReadFile("../../../dashboard/src/app/admin/posts/page.tsx")
	if err != nil {
		t.Fatalf("read admin posts page: %v", err)
	}
	apiSource, err := os.ReadFile("../../../dashboard/src/lib/api.ts")
	if err != nil {
		t.Fatalf("read dashboard api: %v", err)
	}
	combined := string(pageSource) + "\n" + string(apiSource)

	for _, want := range []string{
		`"partial"`,
		"RESULT_STATUS_OPTIONS",
		"Has failed attempts",
		"result_status",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("admin posts UI/API should expose partial and failed-attempt filters %q", want)
		}
	}
}

func TestAdminPostFailuresSQLSearchesConcreteFailureIdentifiers(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"pf.id AS post_failure_id",
		"spr.id AS social_post_result_id",
		"OR COALESCE(post_failure_id, '') = $5",
		"OR COALESCE(social_post_result_id, '') = $5",
		"OR post_id = $5",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post failures SQL should support concrete error lookup %q", want)
		}
	}
}

func TestAdminPostFailuresSQLLinksHistoricalFailureEventsByConcreteID(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"linked_failure_events AS",
		"FROM post_failures pf\n  JOIN social_posts sp ON sp.id = pf.post_id",
		"AND $5::TEXT <> ''",
		"AND (pf.id = $5 OR COALESCE(spr.id, '') = $5 OR sp.id = $5)",
		"pf.created_at >= NOW() - ($2::INT * INTERVAL '1 day')",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post failures SQL should link historical post_failures by concrete ID %q", want)
		}
	}
}

func TestAdminEmailNotificationsSQLIncludesQuotaReminderFields(t *testing.T) {
	sql := adminEmailNotificationsBaseSelect()

	for _, want := range []string{
		"free_plan_quota_email_reminders",
		"'free_plan_quota_reminder' AS event_type",
		"'usage_' || r.threshold_percent::TEXT || '_percent' AS trigger_event",
		"LEFT JOIN workspaces w ON w.id = r.workspace_id",
		"LEFT JOIN users u ON u.id = r.user_id",
		"effective_usage",
		"completed_usage",
		"reserved_usage",
		"ORDER BY r.attempted_at DESC, r.created_at DESC",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin email notifications SQL missing %q:\n%s", want, sql)
		}
	}
}

func TestAdminEmailNotificationsRouteIsRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(source), `r.Get("/v1/admin/email-notifications", adminHandler.ListEmailNotifications)`) {
		t.Fatalf("admin email notifications route is not registered")
	}
}
