package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/xiaoboyu/unipost-api/internal/auth"
)

func TestNormalizeAnalyticsPostsLimitDefaultsAndCaps(t *testing.T) {
	limit, err := normalizeAnalyticsPostsLimit("")
	if err != nil {
		t.Fatalf("default limit returned error: %v", err)
	}
	if limit != 50 {
		t.Fatalf("default limit = %d, want 50", limit)
	}

	limit, err = normalizeAnalyticsPostsLimit("5000")
	if err != nil {
		t.Fatalf("capped limit returned error: %v", err)
	}
	if limit != 100 {
		t.Fatalf("capped limit = %d, want 100", limit)
	}

	if _, err := normalizeAnalyticsPostsLimit("0"); err == nil {
		t.Fatal("zero limit should be rejected")
	}
}

func TestNormalizeAnalyticsCursorOffset(t *testing.T) {
	offset, err := normalizeAnalyticsCursorOffset("")
	if err != nil {
		t.Fatalf("empty cursor returned error: %v", err)
	}
	if offset != 0 {
		t.Fatalf("empty cursor offset = %d, want 0", offset)
	}

	offset, err = normalizeAnalyticsCursorOffset("25")
	if err != nil {
		t.Fatalf("numeric cursor returned error: %v", err)
	}
	if offset != 25 {
		t.Fatalf("cursor offset = %d, want 25", offset)
	}

	if _, err := normalizeAnalyticsCursorOffset("-1"); err == nil {
		t.Fatal("negative cursor should be rejected")
	}
}

func TestAnalyticsPostSortSpecsAreAllowlisted(t *testing.T) {
	spec, err := analyticsPostSortSpec("")
	if err != nil {
		t.Fatalf("default sort returned error: %v", err)
	}
	if spec.APIName != "published_at" || spec.Direction != "DESC" {
		t.Fatalf("default sort = %#v, want published_at DESC", spec)
	}

	spec, err = analyticsPostSortSpec("engagement_rate")
	if err != nil {
		t.Fatalf("engagement_rate sort returned error: %v", err)
	}
	if spec.Expression != "COALESCE(pa.engagement_rate, 0)" {
		t.Fatalf("unexpected engagement_rate expression: %q", spec.Expression)
	}

	if _, err := analyticsPostSortSpec("published_at;DROP TABLE post_analytics"); err == nil {
		t.Fatal("unsafe sort should be rejected")
	}
}

func TestAnalyticsCapabilitiesIncludeCorePlatforms(t *testing.T) {
	caps := analyticsPlatformCapabilities()
	for _, platform := range []string{"instagram", "threads", "pinterest", "tiktok"} {
		capability, ok := caps[platform]
		if !ok {
			t.Fatalf("missing analytics capability for %s", platform)
		}
		if len(capability.Metrics) == 0 {
			t.Fatalf("%s capability has no metrics", platform)
		}
	}
}

func TestAnalyticsMetricsStableForClientDocs(t *testing.T) {
	got := analyticsMetricNames()
	want := []string{
		"impressions",
		"reach",
		"likes",
		"comments",
		"shares",
		"saves",
		"clicks",
		"video_views",
		"engagement_rate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("analytics metrics = %#v, want %#v", got, want)
	}
}

func TestRollupEngagementRateUsesSavesAndClicks(t *testing.T) {
	got := rollupEngagementRate(100, 3, 4, 5, 6, 7)
	if got != 0.25 {
		t.Fatalf("rollup engagement = %.4f, want 0.2500", got)
	}
}

func TestAnalyticsExplorerListPostsRequiresWorkspace(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/analytics/posts", nil)

	h.ListPosts(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAnalyticsExplorerListPostsRejectsUnsafeSortBeforeDB(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := analyticsExplorerRequest(http.MethodGet, "/v1/analytics/posts?sort=published_at%3BDROP%20TABLE%20post_analytics", nil)

	h.ListPosts(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func TestAnalyticsExplorerExportPostsRejectsInvalidCursorBeforeDB(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := analyticsExplorerRequest(http.MethodGet, "/v1/analytics/posts/export?cursor=-1", nil)

	h.ExportPostsCSV(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func TestAnalyticsExplorerListPlatformsRejectsInvalidDateBeforeDB(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := analyticsExplorerRequest(http.MethodGet, "/v1/analytics/platforms?from=05-01-2026", nil)

	h.ListPlatforms(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func TestAnalyticsExplorerGetPlatformRejectsUnknownPlatformBeforeDB(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := analyticsExplorerRequest(http.MethodGet, "/v1/analytics/platforms/mastodon", nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("platform", "mastodon")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))

	h.GetPlatform(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAnalyticsExplorerRequestRefreshRejectsUnknownPlatformBeforeDB(t *testing.T) {
	h := NewAnalyticsExplorerHandler(nil)
	w := httptest.NewRecorder()
	req := analyticsExplorerRequest(http.MethodPost, "/v1/analytics/refresh", strings.NewReader(`{"platform":"mastodon"}`))

	h.RequestRefresh(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnprocessableEntity)
	}
}

func analyticsExplorerRequest(method, target string, body *strings.Reader) *http.Request {
	var reader *strings.Reader
	if body != nil {
		reader = body
	} else {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, reader)
	return req.WithContext(auth.SetWorkspaceID(req.Context(), "ws_test"))
}
