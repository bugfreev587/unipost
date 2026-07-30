package requestevents

import (
	"strings"
	"testing"
)

func TestParitySQLComparesCanonicalEventsOnlyWithLegacyAPIMetrics(t *testing.T) {
	sql := strings.ToLower(compareRequestEventHourSQL)
	for _, required := range []string{
		"from api_metrics",
		"from api_request_events",
		"full outer join",
		"workspace_id",
		"api_key_id",
		"method",
		"route_pattern",
		"status_code",
		"where created_at >= $1 and created_at < $2",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("parity SQL missing %q", required)
		}
	}
	for _, protected := range []string{"social_posts", "social_accounts", "social_post_results", "post_delivery_jobs"} {
		if strings.Contains(sql, protected) {
			t.Fatalf("parity SQL touches protected relation %q", protected)
		}
	}
}

func TestControlledCohortParityUsesNoAmbiguousTimeBoundary(t *testing.T) {
	sql := strings.ToLower(compareControlledCohortSQL)
	for _, required := range []string{
		"from api_metrics",
		"from api_request_events",
		"workspace_id = $1",
		"api_key_id",
		"method = $3",
		"path = $4",
		"route_pattern = $4",
		"status_code = $5",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("controlled cohort SQL missing %q", required)
		}
	}
	if strings.Contains(sql, "created_at") || strings.Contains(sql, "occurred_at") {
		t.Fatal("exact controlled cohort comparison must not depend on an async time boundary")
	}
}
