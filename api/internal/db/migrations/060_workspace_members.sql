-- +goose Up
-- RBAC Phase 1 (May 2026): introduce a membership table that maps
-- Clerk users to workspaces with a role. Today every workspace has
-- exactly one user (the owner stored on workspaces.user_id) — the
-- backfill creates a single 'owner' row per workspace, so behavior is
-- unchanged. Inviting additional members + the role-aware UI / invite
-- flow ship in later phases (PR-C Phase 4+).
--
-- Why a separate table instead of a column on users:
--   - Membership is many-to-many in spirit (one user → multiple
--     workspaces in the future, multiple users → one workspace today).
--     Even though the product currently constrains 1:1, the table
--     shape makes the eventual unbundling free.
--   - Per-membership state (status, invited_by, accepted_at) doesn't
--     fit on a user row.
--
-- Roles:
--   owner  — billing, delete workspace, transfer ownership. Exactly 1.
--   admin  — invite/remove members, configure platforms, all of editor.
--   editor — create/publish posts, manage own API keys.
--
-- A future 'viewer' role can be added without breaking existing rows
-- because the CHECK constraint will be relaxed and RoleLevel will get
-- a new entry.

CREATE TABLE workspace_members (
    workspace_id TEXT        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT        NOT NULL CHECK (role IN ('owner','admin','editor')),
    status       TEXT        NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','pending')),
    invited_by   TEXT,                                          -- user_id of inviter (NULL for owner self-grant)
    invited_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    accepted_at  TIMESTAMPTZ,                                   -- NULL for pending invites
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, user_id)
);

-- Reverse-direction lookup: "what workspaces is this user a member of?"
-- Used by the auth path on every Clerk session login.
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id);

-- Exactly one owner per workspace. Inviting/promoting code paths must
-- demote the current owner before promoting a new one (transfer flow).
CREATE UNIQUE INDEX workspace_members_one_owner_idx
    ON workspace_members (workspace_id) WHERE role = 'owner';

-- Backfill: every existing workspace's user_id becomes the owner.
-- Status 'active' + accepted_at NOW() because these aren't pending
-- invites — they are existing direct ownerships.
INSERT INTO workspace_members (workspace_id, user_id, role, status, accepted_at)
SELECT id, user_id, 'owner', 'active', NOW()
FROM workspaces
ON CONFLICT (workspace_id, user_id) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS workspace_members;
