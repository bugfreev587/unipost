package db

import (
	"os"
	"strings"
	"testing"
)

func TestDeliveryJobLeaseMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/100_delivery_job_lease.sql")
	if err != nil {
		t.Fatalf("read delivery job lease migration: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ALTER TABLE post_delivery_jobs",
		"ADD COLUMN lease_expires_at TIMESTAMPTZ",
		"ADD COLUMN lease_owner",
		"post_delivery_jobs_lease_expiry_idx",
		"DROP COLUMN IF EXISTS lease_expires_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("delivery job lease migration missing %q:\n%s", want, sql)
		}
	}
}

// TestDeliveryJobLeaseQueryContract locks in the lease-based semantics so a
// future edit can't silently revert stale recovery to the static
// last_attempt_at cutoff that caused the double-publish incident.
func TestDeliveryJobLeaseQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		// Claim sets a lease.
		"lease_expires_at = NOW() + make_interval(secs => $",
		// Stale recovery reaps by expired lease, with a NULL fallback.
		"lease_expires_at <= NOW()",
		"lease_expires_at IS NULL",
		// Heartbeat renewal exists.
		"RenewPostDeliveryJobLease",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("delivery job lease query contract missing %q", want)
		}
	}
}
