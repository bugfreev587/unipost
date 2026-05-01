-- +goose Up
-- RBAC Phase 6 (May 2026): audit_log records security-relevant
-- mutations across the workspace.
--
-- Schema follows the burnrate-ai shape (DOMAIN.VERB action codes,
-- separate before_json / after_json, polymorphic resource pointer).
-- We store actor identity as either user_id (Clerk session calls) or
-- api_key_id (programmatic calls); exactly one will be populated for
-- a given row.
--
-- Retention: 90 days for non-Team plans, 1 year for Team / Enterprise.
-- Enforcement is a fast-follow worker; v1 keeps everything indefinitely
-- and the size is bounded by mutation rate (small for early customers).

CREATE TABLE audit_log (
    id               BIGSERIAL PRIMARY KEY,
    workspace_id     TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_user_id    TEXT,                     -- NULL when acting via API key
    actor_api_key_id TEXT,                     -- NULL when acting via Clerk session
    action           TEXT        NOT NULL,     -- DOMAIN.VERB e.g. "MEMBER.INVITED"
    resource_type    TEXT        NOT NULL,     -- "membership" | "invite" | "api_key" | "platform_credential" | "subscription" | "workspace"
    resource_id      TEXT,                     -- the affected row's PK; NULL for cross-resource events
    category         TEXT        NOT NULL,     -- "membership" | "billing" | "config" | "publishing" | "auth"
    ip_address       TEXT,
    user_agent       TEXT,
    before_json      JSONB,                    -- pre-mutation snapshot (NULL for create)
    after_json       JSONB,                    -- post-mutation snapshot (NULL for delete)
    metadata         JSONB,                    -- arbitrary extra context
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Hot path: the dashboard's audit log view filters by workspace_id
-- and orders by created_at DESC.
CREATE INDEX audit_log_workspace_created_idx ON audit_log (workspace_id, created_at DESC);

-- Search by actor — "show me everything user X did".
CREATE INDEX audit_log_actor_user_idx ON audit_log (actor_user_id, created_at DESC) WHERE actor_user_id IS NOT NULL;

-- Search by resource — "show me everything that happened to this
-- specific membership / api key".
CREATE INDEX audit_log_resource_idx ON audit_log (resource_type, resource_id) WHERE resource_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS audit_log;
