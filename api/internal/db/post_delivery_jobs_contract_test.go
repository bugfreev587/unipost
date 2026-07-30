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

func TestPostDeliveryJobSuccessUsesCapturedFinishedAt(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post delivery jobs: %v", err)
	}
	body := string(source)

	for _, want := range []string{
		"finished_at = $1",
		"FinishedAt    pgtype.Timestamptz",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("successful completion timestamp contract missing %q", want)
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
		"locked_accounts AS",
		"WHERE EXISTS (",
		"FOR UPDATE OF j SKIP LOCKED",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("fair claim query contract missing %q", want)
		}
	}

	if strings.Contains(sql, "SELECT DISTINCT social_account_id FROM locked_jobs") {
		t.Fatalf("fair claim query must not combine DISTINCT with FOR UPDATE")
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

func TestPostDeliveryJobConnectionSnapshotMigrationContract(t *testing.T) {
	source, err := os.ReadFile("migrations/136_delivery_job_connection_snapshot.sql")
	if err != nil {
		t.Fatalf("read connection snapshot migration: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"ADD COLUMN connection_id TEXT",
		"ADD COLUMN binding_version BIGINT",
		"SET connection_id = sa.connection_id",
		"binding_version = sa.binding_version",
		"COALESCE(connection_id, social_account_id)",
		"DROP COLUMN IF EXISTS binding_version",
		"DROP COLUMN IF EXISTS connection_id",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("connection snapshot migration missing %q:\n%s", want, sql)
		}
	}
}

func TestPostDeliveryJobPhysicalConnectionClaimContract(t *testing.T) {
	source, err := os.ReadFile("post_delivery_jobs.sql.go")
	if err != nil {
		t.Fatalf("read generated post delivery jobs: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"COALESCE(active.connection_id, active.social_account_id) = COALESCE(j.connection_id, j.social_account_id)",
		"sa.connection_id = j.connection_id",
		"sa.binding_version = j.binding_version",
		"sa.binding_status = 'active'",
		"binding_version_mismatch",
		"ValidateDeliveryBindingSnapshot",
		"earlier.kind IN ('dispatch', 'retry')",
		"CASE WHEN earlier.kind = 'retry' THEN COALESCE(earlier.next_run_at, earlier.created_at) ELSE earlier.created_at END",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("physical connection claim contract missing %q", want)
		}
	}
	if count := strings.Count(sql, "earlier.kind IN ('dispatch', 'retry')"); count != 2 {
		t.Fatalf("dispatch and retry claims must both arbitrate across job kinds; found %d shared predicates", count)
	}
}

func TestPlatformStartAtomicallyValidatesBindingSnapshot(t *testing.T) {
	source, err := os.ReadFile("queries/post_delivery_jobs.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	start := strings.Index(sql, "-- name: MarkPostDeliveryJobPlatformStarted")
	end := strings.Index(sql[start:], "-- name: MarkPostDeliveryJobSucceeded")
	if start < 0 || end < 0 {
		t.Fatal("MarkPostDeliveryJobPlatformStarted boundaries not found")
	}
	query := sql[start : start+end]
	for _, want := range []string{
		"EXISTS (",
		"JOIN social_accounts sa ON sa.id = snapshot.social_account_id",
		"sa.binding_status = 'active'",
		"sa.connection_id = snapshot.connection_id",
		"sa.binding_version = snapshot.binding_version",
	} {
		if !strings.Contains(query, want) {
			t.Errorf("atomic platform-start guard missing %q", want)
		}
	}
}

func TestDailyPublishReservationUsesPhysicalConnectionAtomically(t *testing.T) {
	source, err := os.ReadFile("queries/social_post_results.sql")
	if err != nil {
		t.Fatal(err)
	}
	query := string(source)
	for _, want := range []string{
		"-- name: ReservePhysicalDailyPublish :one",
		"reserve_physical_daily_publish(",
		"@operation_key",
		"-- name: ReleasePhysicalDailyPublish :one",
		"-- name: FinalizePhysicalDailyPublish :one",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("atomic physical daily reservation missing %q", want)
		}
	}
	migration, err := os.ReadFile("migrations/138_physical_daily_publish_reservations.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrationSQL := string(migration)
	for _, want := range []string{
		"PRIMARY KEY (workspace_id, physical_account_id, platform, utc_date)",
		"PRIMARY KEY (workspace_id, operation_key, utc_date)",
		"COALESCE(sa.connection_id, sa.id)",
		"COUNT(*)::INTEGER",
		"physical_daily_publish_reservations.reserved_count + 1",
		"physical_daily_publish_reservations.reserved_count < requested_daily_cap",
		"social_post_results_capture_legacy_daily_publish",
		"daily_reservation_release_pending BOOLEAN NOT NULL DEFAULT FALSE",
		"social_post_results_reconcile_daily_release",
		"release_physical_daily_publish(target_workspace_id, NEW.daily_reservation_operation_key)",
	} {
		if !strings.Contains(migrationSQL, want) {
			t.Fatalf("daily reservation migration missing %q", want)
		}
	}
}
