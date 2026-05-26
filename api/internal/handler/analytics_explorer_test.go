package handler

import (
	"reflect"
	"testing"
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
