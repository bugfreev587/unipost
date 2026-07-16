-- +goose NO TRANSACTION
-- +goose Up

-- Current nonterminal write state is reconciled every minute. Keep completed
-- history out of this index while covering the fields used by BYO and stale
-- operation checks.
CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_outbound_reconciliation_current_idx
  ON x_inbox_outbound_requests (status, updated_at)
  INCLUDE (usage_event_id, social_account_id)
  WHERE status NOT IN ('completed', 'succeeded');

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_outbound_reconciliation_day_idx
  ON x_inbox_outbound_requests (created_at, status)
  INCLUDE (updated_at, workspace_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_confirmation_created_day_idx
  ON x_inbox_backfill_confirmation_operations (created_at, status)
  INCLUDE (estimated_x_credits, workspace_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_confirmation_completed_day_idx
  ON x_inbox_backfill_confirmation_operations (completed_at)
  INCLUDE (started_at, estimated_x_credits, workspace_id)
  WHERE status = 'completed';

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_confirmation_running_lease_idx
  ON x_inbox_backfill_confirmation_operations (execution_lease_expires_at)
  WHERE status = 'running';

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_confirmation_pending_expiry_idx
  ON x_inbox_backfill_confirmation_operations (expires_at)
  WHERE status = 'pending';

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_exposure_reconciliation_current_idx
  ON x_inbox_backfill_exposure_reservations (status, updated_at)
  INCLUDE (reconciliation_deadline)
  WHERE status NOT IN ('finalized', 'released');

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_exposure_reconciliation_deadline_idx
  ON x_inbox_backfill_exposure_reservations (reconciliation_deadline)
  WHERE status NOT IN ('finalized', 'released')
    AND reconciliation_deadline IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_items_x_latency_day_idx
  ON inbox_items (created_at, ((metadata->>'backfill')))
  INCLUDE (received_at, workspace_id)
  WHERE source IN ('x_reply', 'x_dm');

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_usage_events_settled_day_idx
  ON x_usage_events (updated_at, status, operation_key, catalog_version)
  INCLUDE (weighted_units)
  WHERE status IN ('finalized', 'reversed');

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbound_receipts_evidence_day_idx
  ON x_inbound_event_receipts (created_at, decision)
  INCLUDE (workspace_id);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbound_notifications_reconciliation_idx
  ON x_inbound_cap_notifications (utc_date, threshold, status)
  INCLUDE (lease_expires_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_delivery_reconciliation_status_idx
  ON x_inbox_delivery_resources (delivery_status, updated_at)
  INCLUDE (last_synced_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_cleanup_reconciliation_idx
  ON x_inbox_delivery_cleanup_intents (next_attempt_at)
  INCLUDE (created_at, lease_until);

CREATE INDEX CONCURRENTLY IF NOT EXISTS x_inbox_cleanup_lease_idx
  ON x_inbox_delivery_cleanup_intents (lease_until)
  WHERE lease_until IS NOT NULL;

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS x_inbox_cleanup_lease_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_cleanup_reconciliation_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_delivery_reconciliation_status_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbound_notifications_reconciliation_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbound_receipts_evidence_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_usage_events_settled_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_items_x_latency_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_exposure_reconciliation_deadline_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_exposure_reconciliation_current_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_confirmation_pending_expiry_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_confirmation_running_lease_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_confirmation_completed_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_confirmation_created_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_outbound_reconciliation_day_idx;
DROP INDEX CONCURRENTLY IF EXISTS x_inbox_outbound_reconciliation_current_idx;
