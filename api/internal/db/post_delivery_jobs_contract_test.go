package db

import (
	"strings"
	"testing"
)

func TestPostDeliveryJobTerminalUpdatesOnlyAffectInFlightJobs(t *testing.T) {
	for name, query := range map[string]string{
		"success": markPostDeliveryJobSucceeded,
		"failure": markPostDeliveryJobFailed,
	} {
		if !strings.Contains(query, "state IN ('running', 'retrying')") {
			t.Fatalf("%s terminal update must not overwrite already-terminal jobs:\n%s", name, query)
		}
	}
}
