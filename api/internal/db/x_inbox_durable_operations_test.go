package db

import (
	"os"
	"strings"
	"testing"
)

func TestXInboxDurableOperationsMigrationIsRollingSafeAndExecutable(t *testing.T) {
	source, err := os.ReadFile("migrations/115_x_inbox_durable_operations.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(source)
	for _, want := range []string{
		"ALTER TABLE x_inbox_outbound_requests",
		"ADD COLUMN IF NOT EXISTS encrypted_payload",
		"ADD COLUMN IF NOT EXISTS body_hash",
		"ADD COLUMN IF NOT EXISTS send_started_at",
		"'outcome_unknown'",
		"'pending_recovery'",
		"'remote_succeeded'",
		"'usage_reversal_pending'",
		"'needs_reconciliation'",
		"Legacy X Inbox write claim requires manual reconciliation",
		"CREATE TABLE x_inbox_backfill_confirmation_operations",
		"account_fingerprint",
		"request_snapshot",
		"estimated_x_credits",
		"nonce",
		"execution_owner",
		"execution_lease_expires_at",
		"CREATE TABLE x_inbox_backfill_exposure_reservations",
		"reserved_units",
		"'read_started'",
		"'finalize_pending'",
		"'release_pending'",
		"reconciliation_attempts",
		"next_attempt_at",
		"UNIQUE (workspace_id, idempotency_key)",
		"-- +goose Down",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("migration missing %q", want)
		}
	}
}

func TestXInboxOutboundCompletionUsesConflictLookupAndAtomicSettlement(t *testing.T) {
	source, err := os.ReadFile("../handler/inbox_x_outbound.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"BeginTx",
		"UpsertInboxItem",
		"errors.Is(err, pgx.ErrNoRows)",
		"GetInboxItemByExternalID",
		"finalizeXUsageInTx",
		"status = 'completed'",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("completion state machine missing %q", want)
		}
	}
}

func TestXInboxWebhookHealingLocksCandidateAndCanRecoverManualState(t *testing.T) {
	source, err := os.ReadFile("queries/inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"ListXInboxOutboundWebhookCandidates",
		"FOR UPDATE OF o",
		"RecordXInboxOutboundRemoteSuccessFromWebhook",
		"'needs_reconciliation'",
		"o.body_hash = @body_hash",
		"o.send_started_at IS NOT NULL",
		"@event_at::TIMESTAMPTZ >= o.send_started_at - INTERVAL '5 minutes'",
		"@event_at::TIMESTAMPTZ <= o.reconciliation_deadline",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("webhook healing query missing %q", want)
		}
	}
	if strings.Contains(text, "NOW() - INTERVAL '2 hours'") ||
		strings.Contains(text, "LIMIT 10") {
		t.Fatal("webhook healing candidates are truncated before exact payload matching")
	}
}

func TestXInboxRecoveryIncludesStaleModernPendingClaimsWithoutResend(t *testing.T) {
	source, err := os.ReadFile("queries/inbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"ListRecoverableXInboxOutboundRequests",
		"status = 'pending'",
		"encrypted_payload IS NOT NULL",
		"reconciliation_deadline <= NOW()",
		"DeletePendingXInboxOutboundRequest :execrows",
		"ClaimPendingXInboxOutboundRecovery :execrows",
		"SET status = 'pending_recovery'",
		"DeleteXInboxOutboundAfterPendingRecovery :execrows",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("modern pending recovery query missing %q", want)
		}
	}
}

func TestXInboxConfirmationConsumptionUsesRowLockAndSingleRunningTransition(t *testing.T) {
	source, err := os.ReadFile("../handler/inbox_x_confirmation.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"FOR UPDATE",
		"status = 'running'",
		"WHERE id = $1 AND status = 'pending'",
		"StartedByThisCall",
		`operation.Status == "pending"`,
		"xBackfillExecutionLease",
		"AND execution_owner = $5",
		"RowsAffected()",
		`status := "completed"`,
		"account_fingerprint",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("confirmation state machine missing %q", want)
		}
	}
}

func TestXInboxExposureReservationUsesWorkspaceDailyLockBeforePaidRead(t *testing.T) {
	source, err := os.ReadFile("../xcredits/exposure_postgres.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"pg_advisory_xact_lock",
		"FOR UPDATE",
		"requested_resources",
		"reserved_units",
		"weighted_units_used = weighted_units_used +",
		"FinalizeExposure",
		"ReleaseExposure",
		"MarkExposureReleasePending",
		"MarkExposureReadStarted",
		"MarkExposureFinalizePending",
		"ReconcilePendingExposures",
		"needs_reconciliation",
		`status IN ('reserved', 'read_started', 'finalize_pending', 'release_pending')`,
		`case "reserved"`,
		`case "release_pending"`,
		"claimStaleReservedExposureForRelease",
		"WHERE id = $1 AND status = 'reserved'",
		`case "finalize_pending"`,
		`case "read_started"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("exposure reservation missing %q", want)
		}
	}
}
