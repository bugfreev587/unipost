package publishingrestrictions

import (
	"os"
	"strings"
	"testing"
)

func TestAdminRestrictionQueryIncludesCycleRetentionMetrics(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"failure.restriction_cycle_id",
		"retained_object_count",
		"retained_bytes",
		"projected_60_day_storage_cost_usd",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin retention metrics missing %q", want)
		}
	}
}

func TestCampaignAudienceQueryUsesOnlyReferencedArguments(t *testing.T) {
	restriction := Restriction{Platform: "tiktok", CycleID: "cycle_1"}

	restrictionSQL, restrictionArgs, err := campaignAudienceQuery(restriction, RestrictionNotice)
	if err != nil {
		t.Fatal(err)
	}
	if len(restrictionArgs) != 1 || restrictionArgs[0] != "tiktok" {
		t.Fatalf("restriction args = %#v, want platform only", restrictionArgs)
	}
	if strings.Contains(restrictionSQL, "$2") {
		t.Fatalf("restriction query unexpectedly references $2:\n%s", restrictionSQL)
	}

	recoverySQL, recoveryArgs, err := campaignAudienceQuery(restriction, RecoveryNotice)
	if err != nil {
		t.Fatal(err)
	}
	if len(recoveryArgs) != 2 || recoveryArgs[0] != "tiktok" || recoveryArgs[1] != "cycle_1" {
		t.Fatalf("recovery args = %#v, want platform and cycle", recoveryArgs)
	}
	if !strings.Contains(recoverySQL, "$2") {
		t.Fatalf("recovery query must reference cycle argument:\n%s", recoverySQL)
	}
}

func TestCampaignAudienceQueryAggregatesAllWorkspacesPerNormalizedEmail(t *testing.T) {
	query, _, err := campaignAudienceQuery(Restriction{Platform: "tiktok"}, RestrictionNotice)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"GROUP BY normalized_email",
		"ARRAY_AGG(DISTINCT workspace_id ORDER BY workspace_id)",
		"(ARRAY_AGG(user_id ORDER BY user_id))[1]",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("audience query must contain %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "SELECT DISTINCT ON (normalized_email)") {
		t.Fatalf("audience query must not discard represented workspaces:\n%s", query)
	}
}
