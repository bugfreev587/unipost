-- +goose Up
-- RBAC Phase 4 supporting (May 2026): per-member API key attribution.
--
-- Today every api_keys row implicitly belongs to the workspace owner
-- because pre-RBAC there was only ever one user per workspace. With
-- the invite flow live, multiple members can create keys; we need to
-- track who created which key so:
--
--   1. The dual-auth path can derive the key's role from the
--      creator's current membership (replacing the hard-coded
--      RoleOwner stamp from PR-C Phase 2).
--   2. The /v1/api-keys list can show "created by" alongside each
--      key.
--   3. Removing a member can revoke their keys atomically.
--   4. The audit log can record which user revoked which key.
--
-- Backfill: every existing key gets the workspace owner as its
-- creator. After the column is NOT NULL, every new key creation must
-- set created_by_user_id explicitly (handler change in same commit).

ALTER TABLE api_keys ADD COLUMN created_by_user_id TEXT;

UPDATE api_keys ak SET created_by_user_id = (
    SELECT user_id FROM workspaces w WHERE w.id = ak.workspace_id
)
WHERE created_by_user_id IS NULL;

ALTER TABLE api_keys ALTER COLUMN created_by_user_id SET NOT NULL;

-- Look up keys by creator (used by Members.Remove to revoke before
-- deleting the membership).
CREATE INDEX api_keys_created_by_idx ON api_keys (created_by_user_id);

-- +goose Down
DROP INDEX IF EXISTS api_keys_created_by_idx;
ALTER TABLE api_keys DROP COLUMN IF EXISTS created_by_user_id;
