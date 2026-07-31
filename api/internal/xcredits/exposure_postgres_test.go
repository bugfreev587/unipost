package xcredits

import (
	"os"
	"strings"
	"testing"
)

func TestPostgresExposureRecoveryScopesInboxPurpose(t *testing.T) {
	source, err := os.ReadFile("exposure_postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	reconcileStart := strings.Index(string(source), "func (s *PostgresStore) ReconcilePendingExposures")
	if reconcileStart < 0 {
		t.Fatal("ReconcilePendingExposures not found")
	}
	reconcileSource := string(source)[reconcileStart:]
	if !strings.Contains(reconcileSource, "purpose = 'inbox_backfill'") {
		t.Fatal("Inbox exposure recovery must not claim account-read purposes")
	}
}

func TestFinalizedExposureRunsMatchingMutation(t *testing.T) {
	source, err := os.ReadFile("exposure_postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	settleStart := strings.Index(string(source), "func (s *PostgresStore) settleExposure")
	if settleStart < 0 {
		t.Fatal("settleExposure not found")
	}
	settleSource := string(source)[settleStart:]
	for _, invariant := range []string{
		`status == "finalized" || status == "released"`,
		"actualUnits != storedActualUnits",
		"if mutation != nil",
		"mutation(ctx, tx)",
	} {
		if !strings.Contains(settleSource, invariant) {
			t.Fatalf("terminal exposure mutation invariant missing %q", invariant)
		}
	}
}
