package paidquota

import (
	"os"
	"strings"
	"testing"
)

const scheduledQuotaSnapshotCaseContract = `WHEN jsonb_typeof(sp.metadata->'scheduled_quota_units') = 'number'
      AND (sp.metadata->>'scheduled_quota_units') ~ '^(0|[1-9][0-9]{0,8})$'
      AND jsonb_typeof(sp.metadata->'platform_posts') = 'array'
      AND (sp.metadata->>'scheduled_quota_units')::NUMERIC <= jsonb_array_length(sp.metadata->'platform_posts') THEN
      (sp.metadata->>'scheduled_quota_units')::INTEGER`

func TestScheduledQuotaSnapshotSQLContractMatchesReservationQueries(t *testing.T) {
	contract := strings.Join(strings.Fields(scheduledQuotaSnapshotCaseContract), " ")
	if !strings.Contains(strings.Join(strings.Fields(listScheduledParentsForPeriodSQL), " "), contract) {
		t.Fatal("paid quota parent query must use the shared scheduled quota snapshot validity contract")
	}

	dbSource, err := os.ReadFile("../db/social_posts_ext.go")
	if err != nil {
		t.Fatalf("read reservation query source: %v", err)
	}
	normalizedDBSource := strings.Join(strings.Fields(string(dbSource)), " ")
	if got := strings.Count(normalizedDBSource, contract); got != 3 {
		t.Fatalf("reservation queries using scheduled quota snapshot contract = %d, want 3 (count, hold, atomic admission snapshot)", got)
	}
	if !strings.Contains(normalizedDBSource, "/* monthly_quota_snapshot */ WITH selected_plan") ||
		!strings.Contains(normalizedDBSource, "reservation_snapshot AS") {
		t.Fatal("third reservation contract copy must belong to the one-statement monthly admission snapshot")
	}
	for name, source := range map[string]string{
		"hold parent query": listScheduledParentsForPeriodSQL,
		"quota count query": string(dbSource),
	} {
		if !strings.Contains(source, "scheduled_execution_reservation_period") {
			t.Fatalf("%s must place publishing reservations in the server-selected execution period", name)
		}
	}
}
