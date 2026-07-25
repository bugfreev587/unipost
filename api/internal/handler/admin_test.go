package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/xiaoboyu/unipost-api/internal/audit"
	"github.com/xiaoboyu/unipost-api/internal/auth"
	"github.com/xiaoboyu/unipost-api/internal/trials"
)

func TestAdminGrantTrialReturnsCreatedAndPassesAuthenticatedActor(t *testing.T) {
	service := &fakeAdminTrialService{grant: trials.Grant{
		ID: "grant_1", WorkspaceID: "ws_1", Kind: trials.KindFreeToPaid,
		PlanID: "growth", DurationDays: 30, Status: trials.StatusPendingActivation,
		GrantedAt: time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
	}}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials", `{"plan_id":"growth","duration_days":30}`, map[string]string{"workspaceID": "ws_1"})
	rec := httptest.NewRecorder()

	h.GrantTrial(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.grantReq.ActorUserID != "admin_1" || service.grantReq.WorkspaceID != "ws_1" {
		t.Fatalf("request = %#v", service.grantReq)
	}
	var response struct {
		Data trials.Grant `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data.ID != "grant_1" || response.Data.Status != trials.StatusPendingActivation {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestAdminBillingTrialSummaryIsSafeAndQueryUsesOneLateralJoin(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(30 * 24 * time.Hour)
	subscriptionID := "sub_trial"
	scheduleID := "sched_trial"
	row := adminBillingRow{Trial: &adminBillingTrialSummary{
		ID: "grant_1", Kind: trials.KindPaidSamePlan, PlanID: "basic", DurationDays: 30,
		Status: trials.StatusFailed, ScheduledStartAt: &start, EndsAt: &end,
		FailureReason: trials.TerminalReasonUnavailable, StripeSubscriptionID: &subscriptionID, StripeScheduleID: &scheduleID,
	}}
	encoded, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	for _, want := range []string{`"trial"`, `"id":"grant_1"`, `"failure_reason":"trial_unavailable"`, `"stripe_subscription_id":"sub_trial"`, `"stripe_schedule_id":"sched_trial"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("summary missing %q: %s", want, body)
		}
	}
	for _, forbidden := range []string{"granted_by_user_id", "stripe_customer_id", "failure_code", "failure_message"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("summary leaked %q: %s", forbidden, body)
		}
	}
	normalized := strings.ToLower(adminBillingSQL)
	if strings.Count(normalized, "lateral") != 1 || !strings.Contains(normalized, "workspace_trial_grants") || !strings.Contains(normalized, "limit 1") {
		t.Fatalf("admin billing query must select trial in one lateral join: %s", adminBillingSQL)
	}
	if strings.Contains(normalized, "status in ('active', 'trialing')") {
		t.Fatal("admin billing query must not count trialing subscriptions as revenue")
	}
	if !strings.Contains(normalized, "trial.status in ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active')\n    or greatest(s.updated_at, coalesce(trial.updated_at, s.updated_at))") {
		t.Fatal("old open grants must remain visible while terminal grants use the freshness cutoff")
	}
}

func TestAdminBillingTrialSummaryOmitsMissingStripeIDs(t *testing.T) {
	encoded, err := json.Marshal(adminBillingRow{Trial: &adminBillingTrialSummary{ID: "grant_1", Kind: trials.KindFreeToPaid, PlanID: "growth", DurationDays: 30, Status: trials.StatusPendingActivation}})
	if err != nil {
		t.Fatal(err)
	}
	body := string(encoded)
	for _, omitted := range []string{"stripe_subscription_id", "stripe_schedule_id"} {
		if strings.Contains(body, omitted) {
			t.Fatalf("nil optional field %q was emitted: %s", omitted, body)
		}
	}
}

func TestAdminBillingTrialDaysCutoffKeepsOldOpenAndFiltersOldTerminal(t *testing.T) {
	normalized := strings.ToLower(adminBillingSQL)
	const openClause = "trial.status in ('provisioning', 'pending_activation', 'checkout_pending', 'scheduled', 'active')"
	if !strings.Contains(normalized, "and (\n    "+openClause+"\n    or greatest(s.updated_at, coalesce(trial.updated_at, s.updated_at)) >= now() - ($2::int * interval '1 day')\n  )") {
		t.Fatalf("visibility predicate does not let an old open grant bypass only the days cutoff: %s", adminBillingSQL)
	}
	for _, terminal := range []string{"completed", "canceled", "revoked", "superseded", "failed"} {
		if strings.Contains(openClause, "'"+terminal+"'") {
			t.Fatalf("terminal status %q bypasses the days cutoff", terminal)
		}
	}
}

func TestAdminGrantTrialStatusMapping(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{
		{name: "invalid plan", err: trials.ErrInvalidPlan, want: http.StatusUnprocessableEntity},
		{name: "invalid duration", err: trials.ErrInvalidDuration, want: http.StatusUnprocessableEntity},
		{name: "workspace missing", err: trials.ErrWorkspaceNotFound, want: http.StatusNotFound},
		{name: "open grant", err: trials.ErrOpenGrantExists, want: http.StatusConflict},
		{name: "paid mismatch", err: trials.ErrPaidPlanMismatch, want: http.StatusConflict},
		{name: "ineligible", err: trials.ErrIneligibleSubscription, want: http.StatusConflict},
		{name: "internal", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeAdminTrialService{grantErr: test.err}
			h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
			req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials", `{"plan_id":"growth","duration_days":30}`, map[string]string{"workspaceID": "ws_1"})
			rec := httptest.NewRecorder()
			h.GrantTrial(rec, req)
			if rec.Code != test.want {
				t.Fatalf("status=%d body=%s, want %d", rec.Code, rec.Body.String(), test.want)
			}
		})
	}
}

func TestAdminGrantTrialOpenConflictIncludesSafeCurrentTrialSummary(t *testing.T) {
	current := trials.Grant{ID: "grant_existing", WorkspaceID: "ws_1", Kind: trials.KindPaidSamePlan, PlanID: "basic", DurationDays: 30, Status: trials.StatusScheduled, ActorUserID: "admin_secret", StripeSubscriptionID: "sub_secret", FailureMessage: "secret failure"}
	service := &fakeAdminTrialService{grantErr: &trials.OpenGrantConflictError{Current: current}}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials", `{"plan_id":"growth","duration_days":30}`, map[string]string{"workspaceID": "ws_1"})
	rec := httptest.NewRecorder()
	h.GrantTrial(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Error struct {
			Code    string                     `json:"code"`
			Details map[string]json.RawMessage `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "TRIAL_GRANT_CONFLICT" || response.Error.Details["current_trial"] == nil {
		t.Fatalf("response=%s", rec.Body.String())
	}
	for _, forbidden := range []string{"admin_secret", "sub_secret", "secret failure", "granted_by_user_id", "stripe_subscription_id", "failure_message"} {
		if strings.Contains(rec.Body.String(), forbidden) {
			t.Fatalf("conflict leaked %q: %s", forbidden, rec.Body.String())
		}
	}
}

func TestAdminGrantTrialUnrelatedScheduleHasSpecificConflict(t *testing.T) {
	service := &fakeAdminTrialService{grantErr: trials.ErrUnrelatedSchedule}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials", `{"plan_id":"basic","duration_days":30}`, map[string]string{"workspaceID": "ws_1"})
	rec := httptest.NewRecorder()
	h.GrantTrial(rec, req)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), `"code":"TRIAL_SCHEDULE_CONFLICT"`) || !strings.Contains(rec.Body.String(), "Subscription already has an unrelated Stripe schedule") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminGrantTrialRejectsMalformedBodyBeforeService(t *testing.T) {
	service := &fakeAdminTrialService{}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials", `{"plan_id":"growth","duration_days":30.5}`, map[string]string{"workspaceID": "ws_1"})
	rec := httptest.NewRecorder()
	h.GrantTrial(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || service.grantCalls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", rec.Code, service.grantCalls, rec.Body.String())
	}
}

func TestAdminRevokeTrialReturnsConflictWhenCheckoutCompletionWins(t *testing.T) {
	service := &fakeAdminTrialService{revokeErr: trials.ErrRevokeConflict}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials/grant_1/revoke", `{}`, map[string]string{"workspaceID": "ws_1", "trialID": "grant_1"})
	rec := httptest.NewRecorder()
	h.RevokeTrial(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if service.revokeReq.ActorUserID != "admin_1" || service.revokeReq.GrantID != "grant_1" {
		t.Fatalf("request=%#v", service.revokeReq)
	}
}

func TestAdminRevokeTrialAmbiguousStripeFailureReturnsInternal(t *testing.T) {
	service := &fakeAdminTrialService{revokeErr: fmt.Errorf("expire Checkout outcome unknown: %w", context.DeadlineExceeded)}
	h := NewAdminHandler(nil, nil, nil).SetTrialService(service)
	req := adminTrialRequest(t, http.MethodPost, "/v1/admin/workspaces/ws_1/trials/grant_1/revoke", `{}`, map[string]string{"workspaceID": "ws_1", "trialID": "grant_1"})
	rec := httptest.NewRecorder()
	h.RevokeTrial(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAdminTrialRoutesAndAuditActionsAreWired(t *testing.T) {
	source, err := os.ReadFile("../../cmd/api/main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`r.Post("/v1/admin/workspaces/{workspaceID}/trials", adminHandler.GrantTrial)`,
		`r.Post("/v1/admin/workspaces/{workspaceID}/trials/{trialID}/revoke", adminHandler.RevokeTrial)`,
	} {
		if !strings.Contains(string(source), want) {
			t.Errorf("main route missing %q", want)
		}
	}
	if audit.ActionTrialGranted != "TRIAL.GRANTED" || audit.ActionTrialRevoked != "TRIAL.REVOKED" {
		t.Fatalf("audit actions = %q, %q", audit.ActionTrialGranted, audit.ActionTrialRevoked)
	}
	adminSource, err := os.ReadFile("admin.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"audit.ActionTrialGranted", "audit.ActionTrialRevoked"} {
		if !strings.Contains(string(adminSource), want) {
			t.Errorf("admin audit missing %q", want)
		}
	}
}

type fakeAdminTrialService struct {
	grant      trials.Grant
	grantErr   error
	revokeErr  error
	grantReq   trials.GrantRequest
	revokeReq  trials.RevokeRequest
	grantCalls int
}

func (s *fakeAdminTrialService) Grant(_ context.Context, req trials.GrantRequest) (trials.Grant, error) {
	s.grantCalls++
	s.grantReq = req
	return s.grant, s.grantErr
}
func (s *fakeAdminTrialService) Revoke(_ context.Context, req trials.RevokeRequest) (trials.Grant, error) {
	s.revokeReq = req
	if s.grant.ID == "" {
		s.grant = trials.Grant{ID: req.GrantID, WorkspaceID: req.WorkspaceID, Status: trials.StatusRevoked}
	}
	return s.grant, s.revokeErr
}

func adminTrialRequest(t *testing.T, method, target, body string, params map[string]string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req = req.WithContext(context.WithValue(req.Context(), auth.UserIDKey, "admin_1"))
	routeContext := chi.NewRouteContext()
	for key, value := range params {
		routeContext.URLParams.Add(key, value)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeContext))
}

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
		"LOWER(BTRIM(email)) = LOWER(BTRIM($7))",
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

func TestAdminEmailNotificationFilterOptionsUseCompleteDistinctRecipientSet(t *testing.T) {
	sql := adminEmailNotificationFilterOptionsSQL()

	for _, want := range []string{
		"email_notifications AS",
		"SELECT DISTINCT ON (LOWER(BTRIM(email))) BTRIM(email) AS email",
		"WHERE BTRIM(email) <> ''",
		"ORDER BY LOWER(email), email",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("admin email filter options SQL missing %q:\n%s", want, sql)
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
		`r.Get("/v1/admin/email-notifications/filter-options", adminHandler.ListEmailNotificationFilterOptions)`,
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
