package featureflags

import (
	"os"
	"strings"
	"testing"
)

func TestPostgresFeatureFlagStateChangesRemainTransactionallyAudited(t *testing.T) {
	body, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	start := strings.Index(source, "func (s *PostgresStore) Set")
	if start < 0 {
		t.Fatal("could not find PostgresStore.Set")
	}
	end := strings.Index(source[start:], "func (s *PostgresStore) GlobalEnabled")
	if end < 0 {
		t.Fatal("could not isolate PostgresStore.Set")
	}
	setSource := source[start : start+end]
	for _, required := range []string{
		"actor == \"\"",
		"BeginTx",
		"FOR UPDATE",
		"previous != enabled",
		"INSERT INTO feature_flag_changes",
		"previous_enabled, enabled, changed_by",
		"key, previous, enabled, actor",
		"tx.Commit",
	} {
		if !strings.Contains(setSource, required) {
			t.Fatalf("PostgresStore.Set audit contract missing %q", required)
		}
	}
	if strings.Index(setSource, "INSERT INTO feature_flag_changes") > strings.Index(setSource, "tx.Commit") {
		t.Fatal("feature flag audit insert must happen before the state-change transaction commits")
	}
}

func TestPostgresFeatureFlagAdminResponsesUseDefinitionMetadata(t *testing.T) {
	body, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	listStart := strings.Index(source, "func (s *PostgresStore) List")
	setStart := strings.Index(source, "func (s *PostgresStore) Set")
	globalStart := strings.Index(source, "func (s *PostgresStore) GlobalEnabled")
	if listStart < 0 || setStart < 0 || globalStart < 0 || !(listStart < setStart && setStart < globalStart) {
		t.Fatal("could not isolate PostgresStore List and Set")
	}
	listSource := source[listStart:setStart]
	setSource := source[setStart:globalStart]
	if !strings.Contains(listSource, "flagWithDefinitionMetadata(flag)") {
		t.Fatal("List must replace persisted descriptions with registered definition metadata")
	}
	if !strings.Contains(setSource, "flag.Description = definition.Description") {
		t.Fatal("Set must return the registered definition description after a mutation")
	}
}
