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

func TestAdminPostsSQLIncludesAllPublishedDurationSeconds(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"DurationSeconds      *int64",
		"`json:\"duration_seconds,omitempty\"`",
		"COUNT(spr.id) > 0",
		"COUNT(*) FILTER (WHERE spr.status = 'published' AND spr.published_at IS NOT NULL) = COUNT(spr.id)",
		"MAX(spr.published_at) >= COALESCE(sp.scheduled_at, sp.created_at)",
		"EXTRACT(EPOCH FROM (MAX(spr.published_at) - COALESCE(sp.scheduled_at, sp.created_at)))",
		"AS duration_seconds",
		"&durationSeconds",
		"item.DurationSeconds = durationSeconds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin post duration contract missing %q", want)
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

func TestAdminPostFailuresSQLSupportsExactUserAndThisMonthFilters(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"Period   string",
		`strings.TrimSpace(q.Get("user_id"))`,
		`normalizeAdminPostFailurePeriod(q.Get("period"))`,
		"period == \"this_month\"",
		"sp.created_at >= date_trunc('month', NOW())",
		"pf.created_at >= date_trunc('month', NOW())",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post failures should support exact user/month filter %q", want)
		}
	}
}

func TestAdminPostFailuresThisMonthFilterKeepsDaysParameterTyped(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, forbidden := range []string{
		`postDateFilterSQL = "sp.created_at >= date_trunc('month', NOW())"`,
		`failureEventDateFilterSQL = "pf.created_at >= date_trunc('month', NOW())"`,
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("admin post failures this_month filter must not drop the typed $2 days parameter: %q", forbidden)
		}
	}

	for _, want := range []string{
		"$8::TEXT = 'this_month'",
		"$8::TEXT <> 'this_month'",
		"NOW() - ($2::INT * INTERVAL '1 day')",
		"opts.Excluded, strings.TrimSpace(opts.Period))",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin post failures this_month filter should keep period and days parameters typed %q", want)
		}
	}
}

func TestAdminUsersListSQLIncludesScheduledPosts(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ScheduledPosts",
		"`json:\"scheduled_posts\"`",
		"AS scheduled_posts",
		"sp.status = 'scheduled'",
		"JOIN workspaces w ON w.id = sp.workspace_id",
		"&u.ScheduledPosts",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin users list should include scheduled posts %q", want)
		}
	}
}

func TestAdminUserScheduledPostsEndpointContract(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"type adminUserScheduledPost struct",
		"`json:\"post_id\"`",
		"`json:\"title\"`",
		"`json:\"created_at\"`",
		"`json:\"scheduled_at\"`",
		"`json:\"platforms\"`",
		"func (h *AdminHandler) ListUserScheduledPosts",
		"sp.status = 'scheduled'",
		"sp.deleted_at IS NULL",
		"w.user_id = $1",
		"ORDER BY sp.scheduled_at ASC NULLS LAST, sp.created_at DESC",
		"adminScheduledPostTitle",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin user scheduled posts endpoint missing %q", want)
		}
	}
}

func TestAdminUserScheduledPostsRouteIsRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(source), `r.Get("/v1/admin/users/{id}/scheduled-posts", adminHandler.ListUserScheduledPosts)`) {
		t.Fatalf("admin user scheduled posts route is not registered")
	}
}

func TestAdminUserQuotaResetEndpointContract(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"type adminUserQuotaResetResponse struct",
		"`json:\"quota_kind\"`",
		"`json:\"affected_workspaces\"`",
		"`json:\"previous_usage\"`",
		"`json:\"reset_at\"`",
		"func (h *AdminHandler) ResetUserPostQuota",
		"func (h *AdminHandler) ResetUserScheduledQuota",
		"quota_kind = 'scheduled'",
		"admin_post_quota_resets",
		"UPDATE usage",
		"post_count = 0",
		"WHERE w.user_id = $1",
		"to_char(NOW(), 'YYYY-MM')",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin user quota reset endpoint contract missing %q", want)
		}
	}
}

func TestAdminUserQuotaResetRoutesAreRegistered(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		`r.Post("/v1/admin/users/{id}/quota/post/reset", adminHandler.ResetUserPostQuota)`,
		`r.Post("/v1/admin/users/{id}/quota/scheduled/reset", adminHandler.ResetUserScheduledQuota)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin user quota reset route missing %q", want)
		}
	}
}

func TestAdminScheduledPostTitleDerivesFromCaption(t *testing.T) {
	longTitle := strings.Repeat("a", 90)
	for _, tt := range []struct {
		name    string
		caption *string
		want    string
	}{
		{name: "nil caption", caption: nil, want: "Untitled scheduled post"},
		{name: "blank caption", caption: ptrString("  \n\t"), want: "Untitled scheduled post"},
		{name: "first non-empty line", caption: ptrString("\n  Launch notes\nSecond line"), want: "Launch notes"},
		{name: "truncates long first line", caption: ptrString(longTitle), want: strings.Repeat("a", 80)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := adminScheduledPostTitle(tt.caption); got != tt.want {
				t.Fatalf("adminScheduledPostTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func ptrString(s string) *string {
	return &s
}

func TestAdminUsersListSQLIncludesFailedPostsThisMonth(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"FailedPostsThisMonth int64",
		"`json:\"failed_posts_this_month\"`",
		"AS failed_posts_this_month",
		"sp.created_at >= date_trunc('month', NOW())",
		"spr.status = 'failed'",
		"COUNT(DISTINCT sp.id)::bigint",
		"&u.FailedPostsThisMonth",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin users list should include this-month failed posts %q", want)
		}
	}
}

func TestAdminUserActivityFilterSQL(t *testing.T) {
	if got := adminUserActivityFilterSQL("all"); got != "" {
		t.Fatalf("all activity filter should not constrain users:\n%s", got)
	}
	if got := adminUserActivityFilterSQL("unknown"); got != "" {
		t.Fatalf("unknown activity filter should fall back to all users:\n%s", got)
	}

	filter := adminUserActivityFilterSQL("active")
	for _, want := range []string{
		"EXISTS(",
		"FROM social_posts sp",
		"JOIN workspaces w ON w.id = sp.workspace_id",
		"w.user_id = u.id",
		"sp.deleted_at IS NULL",
		"sp.published_at >= NOW() - INTERVAL '30 days'",
	} {
		if !strings.Contains(filter, want) {
			t.Fatalf("active user filter missing %q:\n%s", want, filter)
		}
	}
}

func TestAdminUsersListAppliesReusableFiltersToRowsAndTotal(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		`activity := q.Get("activity")`,
		`filtersSQL := adminUserFiltersSQL(plan, activity)`,
		`adminUserActivityFilterSQL(activity)`,
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin users list should support filtered active totals %q", want)
		}
	}
	if count := strings.Count(sql, "+ filtersSQL +"); count < 2 {
		t.Fatalf("admin users rows and total should share filtersSQL, found %d uses", count)
	}
}

func TestAdminEmailNotificationsSQLIncludesQuotaReminderFields(t *testing.T) {
	sql := adminEmailNotificationsBaseSelect()

	for _, want := range []string{
		"free_plan_quota_email_reminders",
		"paid_plan_quota_notifications",
		"email_send_attempts",
		"unipost_notification_deliveries",
		"error_triage_email_sends",
		"'email.quota.free_plan_reminder.v1' AS event_key",
		"n.event_key",
		"'usage_' || r.threshold_percent::TEXT || '_percent' AS trigger_event",
		"provider",
		"delivery_class",
		"preference_category",
		"footer_policy",
		"preference_decision",
		"idempotency_key",
		"failure_reason",
		"trigger_source",
		"trigger_reference_id",
		"subject_snapshot",
		"effective_usage",
		"completed_usage",
		"reserved_usage",
		"ORDER BY attempted_at DESC, created_at DESC",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin email notifications SQL missing %q:\n%s", want, sql)
		}
	}
}

func TestAdminEmailNotificationsSQLFiltersRecipientAndAttemptedRange(t *testing.T) {
	sql := adminEmailNotificationsWhereSQL

	for _, want := range []string{
		"LOWER(email) = LOWER($7)",
		"attempted_at >= $8",
		"attempted_at < $9",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin email notifications filter missing %q:\n%s", want, sql)
		}
	}
}

func TestParseAdminEmailNotificationRange(t *testing.T) {
	start, end, err := parseAdminEmailNotificationRange(
		"2026-07-01T07:00:00Z",
		"2026-07-03T07:00:00Z",
	)
	if err != nil || start == nil || end == nil {
		t.Fatalf("valid range = %v/%v/%v", start, end, err)
	}

	start, end, err = parseAdminEmailNotificationRange("", "2026-07-03T07:00:00Z")
	if err != nil || start != nil || end == nil {
		t.Fatalf("end-only range = %v/%v/%v", start, end, err)
	}

	if _, _, err := parseAdminEmailNotificationRange("bad", ""); err == nil {
		t.Fatal("malformed start_at should fail")
	}
	if _, _, err := parseAdminEmailNotificationRange("", "bad"); err == nil {
		t.Fatal("malformed end_at should fail")
	}
	if _, _, err := parseAdminEmailNotificationRange(
		"2026-07-03T07:00:00Z",
		"2026-07-03T07:00:00Z",
	); err == nil {
		t.Fatal("zero-length range should fail")
	}
	if _, _, err := parseAdminEmailNotificationRange(
		"2026-07-04T07:00:00Z",
		"2026-07-03T07:00:00Z",
	); err == nil {
		t.Fatal("reversed range should fail")
	}
}

func TestAdminEmailNotificationsResponseExposesPreferencePolicy(t *testing.T) {
	source, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatalf("read admin.go: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"PreferenceCategory",
		"`json:\"preference_category\"`",
		"FooterPolicy",
		"`json:\"footer_policy\"`",
		"PreferenceDecision",
		"`json:\"preference_decision\"`",
		"&item.PreferenceCategory",
		"&item.FooterPolicy",
		"&item.PreferenceDecision",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("admin email notification response missing policy field %q", want)
		}
	}
}

func TestNormalizeAdminEmailNotificationStatusAllowsSkipped(t *testing.T) {
	got, ok := normalizeAdminEmailNotificationStatus("skipped")
	if !ok || got != "skipped" {
		t.Fatalf("normalize status = %q/%v, want skipped/true", got, ok)
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
	for _, route := range []string{
		`r.Post("/v1/admin/email-notifications/{id}/retry", adminHandler.RetryPaidQuotaEmailNotification)`,
		`r.Get("/v1/admin/paid-quota-follow-ups", adminHandler.ListPaidQuotaFollowUps)`,
		`r.Patch("/v1/admin/paid-quota-follow-ups/{id}", adminHandler.UpdatePaidQuotaFollowUp)`,
	} {
		if !strings.Contains(string(source), route) {
			t.Fatalf("admin paid quota route is not registered: %s", route)
		}
	}
}

func TestPaidQuotaEmailAdminFiltersAcceptDetailedStatusesAndThresholds(t *testing.T) {
	for _, status := range []string{
		"processing",
		"retry_wait",
		"skipped_superseded",
		"skipped_preference_disabled",
		"skipped_missing_recipient",
	} {
		if got, ok := normalizeAdminEmailNotificationStatus(status); !ok || got != status {
			t.Fatalf("normalize status %q = %q/%v", status, got, ok)
		}
	}
	for _, threshold := range []string{"80", "90", "100", "105", "110", "115", "120"} {
		if _, ok := parseAdminEmailNotificationThreshold(threshold); !ok {
			t.Fatalf("threshold %s should be accepted", threshold)
		}
	}
}
