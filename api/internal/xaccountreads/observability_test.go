package xaccountreads

import (
	"os"
	"strings"
	"testing"
)

func TestAccountReadObservabilityUsesContentFreeOperationalFields(t *testing.T) {
	serviceSource, err := os.ReadFile("service.go")
	if err != nil {
		t.Fatal(err)
	}
	service := string(serviceSource)
	for _, want := range []string{
		`slog.Info("X account read completed"`,
		`"operation_id"`,
		`"endpoint"`,
		`"scanned_count"`,
		`"returned_count"`,
		`"accounting_enabled"`,
		`slog.Warn("X account read idempotency conflict"`,
		`slog.Warn("X account read rate limited"`,
	} {
		if !strings.Contains(service, want) {
			t.Fatalf("account-read observability missing %q", want)
		}
	}

	providerSource, err := os.ReadFile("../platform/twitter.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(providerSource), `slog.Warn("X authored-post response exceeded requested limit"`) {
		t.Fatal("provider over-limit response must emit content-free telemetry")
	}
}
