-- +goose Up
--
-- Sprint 3 PR1: UniPost Connect — multi-tenant hosted OAuth.
--
-- This migration introduces the Connect surface: a Stripe-Connect-style
-- hosted flow where UniPost customers onboard *their* end users into
-- managed social_accounts rows that the API key can post against
-- without the customer ever touching OAuth credentials.
--
-- Two things land here:
--
--   1. social_accounts gains the columns needed to distinguish a
--      managed (Connect-flow) row from a legacy BYO row, and to look
--      up rows by the customer's own end-user identifier.
--
--   2. A new connect_sessions table tracks pending hosted-flow
--      sessions during the 30-minute window between session creation
--      and OAuth callback completion.
--
-- Per Sprint 3 decision #3, refresh failures reuse the existing
-- account state by flipping a new `status` column to
-- 'reconnect_required' rather than introducing a third enum value.
-- The dashboard already had a TODO to wire this — Sprint 3 wires it.
--
-- Per Sprint 3 decision #1, re-connecting the same end user reuses
-- the existing social_accounts row (preserving foreign keys from
-- historical posts/results). The partial unique index below is the
-- ON CONFLICT target for that upsert. Bluesky is excluded from this
-- index because one external_user_id may legitimately own multiple
-- handles — Bluesky's upsert detection lives in app code keyed on
-- (project_id, platform, external_account_id) instead.

ALTER TABLE social_accounts
  ADD COLUMN status              TEXT NOT NULL DEFAULT 'active',
  ADD COLUMN connection_type     TEXT NOT NULL DEFAULT 'byo',
  ADD COLUMN connect_session_id  TEXT,
  ADD COLUMN external_user_id    TEXT,
  ADD COLUMN external_user_email TEXT,
  ADD COLUMN last_refreshed_at   TIMESTAMPTZ;

ALTER TABLE social_accounts
  ADD CONSTRAINT social_accounts_status_check
    CHECK (status IN ('active', 'reconnect_required'));

ALTER TABLE social_accounts
  ADD CONSTRAINT social_accounts_connection_type_check
    CHECK (connection_type IN ('byo', 'managed'));

-- Re-connect upsert target for OAuth platforms (Twitter, LinkedIn).
-- Excludes bluesky because the same external_user_id may legitimately
-- map to multiple handles. The Bluesky upsert lookup happens in app
-- code via (project_id, platform, external_account_id).
CREATE UNIQUE INDEX social_accounts_managed_unique_idx
  ON social_accounts (project_id, platform, external_user_id)
  WHERE external_user_id IS NOT NULL AND platform <> 'bluesky';

-- Lookup index for `GET /v1/social-accounts?external_user_id=...`.
-- Sprint 3 exit gate step 4 depends on this filter being indexed.
CREATE INDEX social_accounts_ext_user_idx
  ON social_accounts (project_id, external_user_id)
  WHERE external_user_id IS NOT NULL;

-- Refresh-due scan index for the token refresh worker (PR7).
-- Partial keeps it tiny — most rows aren't managed, and most managed
-- rows aren't due for refresh on any given tick.
CREATE INDEX social_accounts_refresh_due_idx
  ON social_accounts (token_expires_at)
  WHERE connection_type = 'managed' AND status = 'active';

-- connect_sessions tracks one in-flight Connect handshake.
--
-- Lifecycle:
--   pending   → created via POST /v1/connect/sessions
--   completed → OAuth callback (or Bluesky form) succeeded; the
--               resulting social_accounts row id is stored in
--               completed_social_account_id
--   expired   → expires_at < NOW() and never completed (lazily
--               flipped on read; no background sweeper required)
--   cancelled → user denied on the platform's authorize page
--
-- oauth_state is a 32-byte base64url random token used as the bearer
-- for the public dashboard endpoint that the hosted /connect page
-- calls. It's UNIQUE so an unguessable state collision-free.
--
-- pkce_verifier is populated only for Twitter (OAuth 2.0 PKCE);
-- LinkedIn doesn't use PKCE and Bluesky doesn't use OAuth at all.
CREATE TABLE connect_sessions (
  id                          TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id                  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  platform                    TEXT NOT NULL,
  external_user_id            TEXT NOT NULL,
  external_user_email         TEXT,
  return_url                  TEXT,
  status                      TEXT NOT NULL DEFAULT 'pending',
  completed_social_account_id TEXT REFERENCES social_accounts(id),
  oauth_state                 TEXT NOT NULL UNIQUE,
  pkce_verifier               TEXT,
  expires_at                  TIMESTAMPTZ NOT NULL,
  created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  completed_at                TIMESTAMPTZ,
  CONSTRAINT connect_sessions_status_check
    CHECK (status IN ('pending', 'completed', 'expired', 'cancelled')),
  CONSTRAINT connect_sessions_platform_check
    CHECK (platform IN ('twitter', 'linkedin', 'bluesky'))
);

-- Lookup by project (admin / customer-facing list, if added later).
CREATE INDEX connect_sessions_project_idx
  ON connect_sessions (project_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS connect_sessions_project_idx;
DROP TABLE IF EXISTS connect_sessions;
DROP INDEX IF EXISTS social_accounts_refresh_due_idx;
DROP INDEX IF EXISTS social_accounts_ext_user_idx;
DROP INDEX IF EXISTS social_accounts_managed_unique_idx;
ALTER TABLE social_accounts
  DROP CONSTRAINT IF EXISTS social_accounts_connection_type_check,
  DROP CONSTRAINT IF EXISTS social_accounts_status_check,
  DROP COLUMN IF EXISTS last_refreshed_at,
  DROP COLUMN IF EXISTS external_user_email,
  DROP COLUMN IF EXISTS external_user_id,
  DROP COLUMN IF EXISTS connect_session_id,
  DROP COLUMN IF EXISTS connection_type,
  DROP COLUMN IF EXISTS status;
