package socialconnectioncutover

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestReportJSONIsDeterministicAndSecretFree(t *testing.T) {
	generatedAt := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	report := Report{
		GeneratedAt: generatedAt,
		Mode:        "cutover",
		Counts: Counts{
			Accounts:           4,
			PublishableNull:    1,
			MissingIGIdentity:  1,
			ActiveDeliveryJobs: 2,
		},
		Conflicts: []Conflict{
			{WorkspaceID: "ws-b", Platform: "twitter", ProviderIdentity: "provider-b", Reason: "cross_managed_user", AccountIDs: []string{"sa-2", "sa-1"}},
			{WorkspaceID: "ws-a", Platform: "instagram", Reason: "missing_provider_identity", AccountIDs: []string{"sa-3"}},
		},
		AliasWarnings: []AliasWarning{
			{WorkspaceID: "ws-b", Platform: "instagram", ProviderIdentities: []string{"ig-b", "ig-a"}, SharedIdentifiers: []string{"business-2", "business-1"}},
		},
		Relations: []RelationSize{{Name: "social_accounts", Bytes: 10}, {Name: "inbox_items", Bytes: 20}},
		Blockers:  []Blocker{{Code: "publishable_null_bindings", Count: 1, Message: "one binding has no authority"}},
	}

	first, err := report.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	second, err := report.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("report JSON is not deterministic:\n%s\n%s", first, second)
	}
	for _, secret := range [][]byte{[]byte("access-token"), []byte("refresh-token"), []byte("access_token\"")} {
		if bytes.Contains(first, secret) {
			t.Fatalf("report contains credential material %q: %s", secret, first)
		}
	}
	if !report.HasBlockers() {
		t.Fatal("report with blockers must fail closed")
	}

	var decoded Report
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Conflicts[0].WorkspaceID != "ws-a" {
		t.Fatalf("conflicts are not stably sorted: %+v", decoded.Conflicts)
	}
	if got := decoded.AliasWarnings[0].ProviderIdentities; len(got) != 2 || got[0] != "ig-a" || got[1] != "ig-b" {
		t.Fatalf("alias identifiers are not sorted: %v", got)
	}
}

func TestReportWithoutBlockersCanProceed(t *testing.T) {
	if (Report{}).HasBlockers() {
		t.Fatal("empty report unexpectedly blocks cutover")
	}
}
