package socialconnectioncutover

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

type preflightReaderStub struct {
	counts    Counts
	conflicts []Conflict
	warnings  []AliasWarning
	relations []RelationSize
	err       error
	calls     []string
}

func (s *preflightReaderStub) ReadCounts(context.Context) (Counts, error) {
	s.calls = append(s.calls, "counts")
	return s.counts, s.err
}

func (s *preflightReaderStub) ReadConflicts(context.Context) ([]Conflict, error) {
	s.calls = append(s.calls, "conflicts")
	return s.conflicts, s.err
}

func (s *preflightReaderStub) ReadAliasWarnings(context.Context) ([]AliasWarning, error) {
	s.calls = append(s.calls, "warnings")
	return s.warnings, s.err
}

func (s *preflightReaderStub) ReadRelationSizes(context.Context) ([]RelationSize, error) {
	s.calls = append(s.calls, "relations")
	return s.relations, s.err
}

func TestPreflightBuildsDeterministicCutoverBlockers(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC) }
	reader := &preflightReaderStub{
		counts: Counts{Accounts: 5, PublishableNull: 2, ActiveDeliveryJobs: 3},
		conflicts: []Conflict{{
			WorkspaceID: "ws", Platform: "instagram", Reason: "missing_provider_identity", AccountIDs: []string{"sa-1"},
		}},
		warnings:  []AliasWarning{{WorkspaceID: "ws", Platform: "instagram", ProviderIdentities: []string{"b", "a"}}},
		relations: []RelationSize{{Name: "social_accounts", Bytes: 100, Rows: 5}},
	}

	report, err := NewPreflight(reader, clock).Run(context.Background(), "cutover")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reader.calls, []string{"counts", "conflicts", "warnings", "relations"}) {
		t.Fatalf("reader calls = %v", reader.calls)
	}
	if report.GeneratedAt != clock() || report.Counts.ConflictGroups != 1 || report.Counts.AliasWarnings != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(report.Blockers) != 1 || report.Blockers[0].Code != "active_delivery_jobs" {
		t.Fatalf("blockers = %+v", report.Blockers)
	}
}

func TestPreflightExpandReportsButDoesNotBlockForActiveJobs(t *testing.T) {
	reader := &preflightReaderStub{counts: Counts{ActiveDeliveryJobs: 2}}
	report, err := NewPreflight(reader, time.Now).Run(context.Background(), "expand")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Blockers) != 0 {
		t.Fatalf("Expand blockers = %+v", report.Blockers)
	}
}

func TestPreflightFailsClosedOnInvalidModeOrReadError(t *testing.T) {
	reader := &preflightReaderStub{}
	if _, err := NewPreflight(reader, time.Now).Run(context.Background(), "unknown"); err == nil {
		t.Fatal("invalid preflight mode succeeded")
	}

	wantErr := errors.New("database unavailable")
	reader.err = wantErr
	if _, err := NewPreflight(reader, time.Now).Run(context.Background(), "cutover"); !errors.Is(err, wantErr) {
		t.Fatalf("read error = %v, want %v", err, wantErr)
	}
}

func TestPostgresPreflightQueriesAreReadOnlyAndAliasAware(t *testing.T) {
	mutation := regexp.MustCompile(`(?i)\b(insert|update|delete|truncate|lock)\b`)
	for name, query := range map[string]string{
		"counts":    preflightCountsSQL,
		"conflicts": preflightConflictsSQL,
		"aliases":   preflightAliasWarningsSQL,
		"relations": preflightRelationSizesSQL,
	} {
		if mutation.MatchString(query) {
			t.Fatalf("%s preflight query mutates data: %s", name, query)
		}
	}
	for _, identifier := range []string{
		"instagram_webhook_user_id",
		"instagram_business_account_id",
		"business_account_id",
		"external_account_id",
	} {
		if !strings.Contains(preflightAliasWarningsSQL, identifier) {
			t.Fatalf("alias warning query missing %q", identifier)
		}
	}
}
