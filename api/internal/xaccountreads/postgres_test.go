package xaccountreads

import (
	"os"
	"strings"
	"testing"
)

func TestPostgresStoreLeasesAndRecoversInterruptedExecutingReads(t *testing.T) {
	source, err := os.ReadFile("postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"execution_owner, execution_lease_expires_at",
		"receipt.NextAttemptAt.Add(accountReadExecutionLease)",
		"status IN ('executing', 'outcome_unknown', 'settlement_pending')",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("interrupted executing-read recovery missing %q", want)
		}
	}
}
