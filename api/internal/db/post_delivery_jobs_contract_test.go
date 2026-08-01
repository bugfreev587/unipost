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
	source, err := os.ReadFile("migrations/138_delivery_job_connection_snapshot.sql")
	if err != nil {
		t.Fatalf("read connection snapshot migration: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"ADD COLUMN connection_id TEXT",
		"ADD COLUMN binding_version BIGINT",
		"COALESCE(connection_id, social_account_id)",
		"DROP COLUMN IF EXISTS binding_version",
		"DROP COLUMN IF EXISTS connection_id",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("connection snapshot migration missing %q:\n%s", want, sql)
		}
	}
	if strings.Contains(sql, "UPDATE post_delivery_jobs") {
		t.Fatal("Expand migration must not backfill historical delivery jobs")
	}
}

func TestPostDeliveryJobPhysicalTargetConstraintContract(t *testing.T) {
	source, err := os.ReadFile("migrations/138_delivery_job_connection_snapshot.sql")
	if err != nil {
		t.Fatalf("read connection snapshot migration: %v", err)
	}
	sql := string(source)
	for _, want := range []string{
		"CREATE TABLE social_post_physical_targets",
		"PRIMARY KEY (post_id, physical_target_key)",
		"selected_social_account_id",
		"migration_conflict",
		"reserve_post_delivery_job_physical_target",
		"BEFORE INSERT ON post_delivery_jobs",
		"social_post_physical_target_binding_check",
		"reserved.selected_social_account_id <> NEW.social_account_id",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("physical target migration missing %q", want)
		}
	}
}

func TestPostPhysicalTargetBatchReservationContract(t *testing.T) {
	migration, err := os.ReadFile("migrations/138_delivery_job_connection_snapshot.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"reserve_social_post_physical_targets",
		"CARDINALITY(requested_account_ids)",
		"UNNEST(requested_account_ids, requested_connection_ids)",
		"ORDER BY target_key, account_id",
		"social_post_physical_target_binding_check",
	} {
		if !strings.Contains(string(migration), want) {
			t.Fatalf("batch reservation migration missing %q", want)
		}
	}

	query, err := os.ReadFile("queries/social_connection_rollout.sql")
	if err != nil {
		t.Fatalf("read rollout queries: %v", err)
	}
	if !strings.Contains(string(query), "-- name: ReserveSocialPostPhysicalTargets :exec") ||
		!strings.Contains(string(query), "reserve_social_post_physical_targets") {
		t.Fatal("rollout queries must expose transactional physical-target reservation")
	}
}

func TestPostDeliveryDrainBlocksOnlyNewClaims(t *testing.T) {
	source, err := os.ReadFile("migrations/138_delivery_job_connection_snapshot.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	for _, want := range []string{
		"guard_post_delivery_job_claim_during_social_connection_cutover",
		"rollout_phase IN ('draining', 'cutting_over')",
		"OLD.state = 'pending'",
		"OLD.lease_owner IS DISTINCT FROM NEW.lease_owner",
		"RETURN NULL",
		"BEFORE UPDATE ON post_delivery_jobs",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("delivery drain migration missing %q", want)
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

	// Claiming must never make a job terminal. Marking a job dead from inside
	// the claim query skips post_failures, the parent post status rollup, and
	// the terminal event outbox, which strands the parent post in a
	// non-terminal state with no notification. Stale bindings and unavailable
	// accounts are both detected after the claim, on the delivery path, which
	// records all three.
	for _, forbidden := range []string{
		"SET state = 'dead'",
		"UPDATE social_post_results r",
	} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("claim queries must not terminate work; found %q", forbidden)
		}
	}
}

// Claim eligibility must not depend on account availability. Filtering
// disconnected or unbound accounts out of the claim leaves their queued
// deliveries pending forever, because the code that records a terminal failure
// only ever runs on a claimed job.
func TestPostDeliveryJobClaimDoesNotFilterUnavailableAccounts(t *testing.T) {
	source, err := os.ReadFile("queries/post_delivery_jobs.sql")
	if err != nil {
		t.Fatalf("read post delivery job queries: %v", err)
	}
	sql := string(source)
	for _, name := range []string{"ClaimPostDispatchJobs", "ClaimPostRetryJobs"} {
		start := strings.Index(sql, "-- name: "+name)
		if start < 0 {
			t.Fatalf("%s not found", name)
		}
		end := strings.Index(sql[start+1:], "-- name: ")
		if end < 0 {
			t.Fatalf("%s end boundary not found", name)
		}
		query := sql[start : start+1+end]
		for _, forbidden := range []string{
			"sa.disconnected_at IS NULL",
			"sa.binding_status = 'active'",
		} {
			if strings.Contains(query, forbidden) {
				t.Fatalf("%s must not gate claims on account availability; found %q", name, forbidden)
			}
		}
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
	migration, err := os.ReadFile("migrations/140_physical_daily_publish_reservations.sql")
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
