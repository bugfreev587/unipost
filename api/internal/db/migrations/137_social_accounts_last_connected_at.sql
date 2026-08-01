-- +goose Up
-- unipost:safety reversible

-- last_connected_at records the latest successful *user-authorized* connect
-- (Hosted Connect OAuth callback, Bluesky submission, or BYO OAuth
-- reconnect). It is distinct from:
--   connected_at       — when the UniPost account row was created; and
--   last_refreshed_at  — which also advances on background token refresh.
-- Background token refresh must never advance last_connected_at, so a
-- customer can verify that a reconnect actually replaced the provider
-- identity behind a stable managed account ID.
ALTER TABLE social_accounts
  ADD COLUMN last_connected_at TIMESTAMPTZ;

-- Backfill managed rows from the latest completed Connect Session that
-- references the account: that completion time is the last proven
-- user-authorized connect.
UPDATE social_accounts sa
SET last_connected_at = latest.completed_at
FROM (
  SELECT completed_social_account_id AS account_id, MAX(completed_at) AS completed_at
  FROM connect_sessions
  WHERE status = 'completed'
    AND completed_social_account_id IS NOT NULL
    AND completed_at IS NOT NULL
  GROUP BY completed_social_account_id
) AS latest
WHERE sa.id = latest.account_id
  AND sa.connection_type = 'managed';

-- Historical fallback: managed rows with no completed Connect Session (for
-- example rows created before session completion tracking) fall back to row
-- creation time. This is documented as a fallback, not a proven connect time.
UPDATE social_accounts
SET last_connected_at = connected_at
WHERE connection_type = 'managed'
  AND last_connected_at IS NULL;

-- BYO rows intentionally stay NULL until their next user-authorized OAuth
-- reconnect sets the value going forward.

-- +goose Down

ALTER TABLE social_accounts
  DROP COLUMN IF EXISTS last_connected_at;
