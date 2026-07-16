package db

import (
	"strings"
	"testing"
)

func TestScheduledQuotaQueryCountsOnlyOutstandingPublishingResults(t *testing.T) {
	sql := strings.ToLower(countScheduledQuotaUnitsByWorkspaceAndPeriod)
	for _, want := range []string{
		"sp.status = 'publishing'",
		"from social_post_results spr",
		"spr.status not in ('published', 'failed')",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("scheduled quota query missing %q:\n%s", want, countScheduledQuotaUnitsByWorkspaceAndPeriod)
		}
	}
}

func TestPublishingUsageTransitionsAreAtomic(t *testing.T) {
	for name, sql := range map[string]string{
		"result": updateSocialPostResultAfterRetryAndIncrementUsageSQL,
		"parent": updateSocialPostStatusAndIncrementUsageSQL,
	} {
		normalized := strings.ToLower(sql)
		for _, want := range []string{
			"with updated_",
			"insert into usage",
			"cross join usage_increment",
		} {
			if !strings.Contains(normalized, want) {
				t.Fatalf("%s transition query missing %q:\n%s", name, want, sql)
			}
		}
	}
}
