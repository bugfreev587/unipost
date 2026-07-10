package db

import (
	"os"
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

func TestPostDeliveryJobPhaseTimestampMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/104_post_delivery_job_phase_timestamps.sql")
	if err != nil {
		t.Fatalf("read phase timestamp migration: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ADD COLUMN first_claimed_at TIMESTAMPTZ",
		"ADD COLUMN platform_started_at TIMESTAMPTZ",
		"post_delivery_jobs_reserved_idx",
		"DROP COLUMN IF EXISTS platform_started_at",
		"DROP COLUMN IF EXISTS first_claimed_at",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("phase timestamp migration missing %q:\n%s", want, sql)
		}
	}
}

func TestPostDeliveryJobPhaseTimestampQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"first_claimed_at = COALESCE(j.first_claimed_at, NOW())",
		"platform_started_at = NULL",
		"platform_started_at = COALESCE(platform_started_at, NOW())",
		"MarkPostDeliveryJobPlatformStarted",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("phase timestamp query contract missing %q", want)
		}
	}
}

func TestPostDeliveryJobFairClaimQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"ROW_NUMBER() OVER (PARTITION BY j.workspace_id",
		"ORDER BY rn ASC, created_at ASC, id ASC",
		"ORDER BY rn ASC, sort_key ASC, id ASC",
		"active_cnt + rn <= $",
		"locked_jobs AS",
		"FOR UPDATE OF j SKIP LOCKED",
		"JOIN (SELECT DISTINCT social_account_id FROM locked_jobs)",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("fair claim query contract missing %q", want)
		}
	}
}

func TestPostDeliveryJobLeaseOwnershipQueryContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post_delivery_jobs queries: %v", err)
	}
	sql := string(source)

	for _, want := range []string{
		"RenewPostDeliveryJobLease",
		"MarkPostDeliveryJobPlatformStarted",
		"MarkPostDeliveryJobSucceeded",
		"MarkPostDeliveryJobFailed",
		"lease_owner IS NOT DISTINCT FROM $",
		"last_attempt_at IS NOT DISTINCT FROM $",
		"finished_at = CASE",
		"WHEN $1 IN ('pending', 'failed', 'dead', 'cancelled') THEN NOW()",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("lease ownership query contract missing %q", want)
		}
	}
}
