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
		"current_cycle_retained_object_count",
		"current_cycle_retained_bytes",
		"current_cycle_projected_60_day_storage_cost_usd",
		"LEFT JOIN profiles account_profile ON account_profile.id = sa.profile_id",
		"COUNT(DISTINCT account_profile.workspace_id)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("admin retention metrics missing %q", want)
		}
	}
}

func TestCampaignAudienceQueriesResolveAccountWorkspaceThroughProfile(t *testing.T) {
	for _, campaignType := range []CampaignType{RestrictionNotice, RecoveryNotice} {
		query, _, err := campaignAudienceQuery(Restriction{Platform: "tiktok", CycleID: "cycle_1"}, campaignType)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{
			"JOIN profiles account_profile ON account_profile.id = sa.profile_id",
			"account_profile.workspace_id",
		} {
			if !strings.Contains(query, want) {
				t.Fatalf("%s audience must contain %q:\n%s", campaignType, want, query)
			}
		}
		if strings.Contains(query, "sa.workspace_id") {
			t.Fatalf("%s audience uses nonexistent social_accounts.workspace_id:\n%s", campaignType, query)
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
		"ARRAY_AGG(workspace_id ORDER BY user_id, workspace_id)",
		"ARRAY_AGG(user_id ORDER BY user_id, workspace_id)",
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

func TestRecoveryCampaignAudienceRebuildsIdentityFromCanonicalUser(t *testing.T) {
	query, _, err := campaignAudienceQuery(Restriction{Platform: "tiktok", CycleID: "cycle_1"}, RecoveryNotice)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"JOIN users canonical_user ON canonical_user.id = prior.canonical_user_id",
		"UNNEST(prior.represented_workspace_ids) WITH ORDINALITY",
		"UNNEST(prior.represented_owner_user_ids) WITH ORDINALITY",
		"owner.ordinality = workspace.ordinality",
		"owner_member.user_id = prior.owner_user_id",
		"LOWER(TRIM(canonical_user.email))",
		"canonical_user.name",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("recovery audience must rebuild current canonical identity with %q:\n%s", want, query)
		}
	}
	if strings.Contains(query, "prior.recipient_email AS email") || strings.Contains(query, "prior.normalized_email") {
		t.Fatalf("recovery audience must not copy stale recipient identity:\n%s", query)
	}
}

func TestBulkRetryExcludesUnknownOrAmbiguousProviderOutcomes(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"provider_definitive_failure",
		"pre_send_failure",
		"provider_unknown",
		"email_send_attempts",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bulk retry classification missing %q", want)
		}
	}
}
