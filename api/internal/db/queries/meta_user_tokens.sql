-- name: UpsertMetaUserToken :one
-- Called whenever we mint a fresh long-lived Meta User Token — on
-- initial connect AND on proactive refresh. Matching (workspace_id,
-- meta_user_id) means a single Meta user re-authorizing the same
-- workspace updates the row rather than growing the table.
INSERT INTO meta_user_tokens (
  workspace_id,
  meta_user_id,
  long_lived_token_encrypted,
  expires_at,
  updated_at
)
VALUES ($1, $2, $3, $4, NOW())
ON CONFLICT (workspace_id, meta_user_id) DO UPDATE
SET
  long_lived_token_encrypted = EXCLUDED.long_lived_token_encrypted,
  expires_at = EXCLUDED.expires_at,
  updated_at = NOW()
RETURNING *;

-- name: GetMetaUserToken :one
-- Returns NULL (ErrNoRows) for workspaces that never connected a
-- Meta user or whose row was deleted via ON DELETE CASCADE. Callers
-- must separately check expires_at before using — we don't filter
-- here so "Add another Page" can still show a helpful "expired, go
-- reconnect" message instead of a silent not-found.
SELECT * FROM meta_user_tokens
WHERE workspace_id = $1 AND meta_user_id = $2;

-- name: DeleteMetaUserToken :exec
DELETE FROM meta_user_tokens
WHERE workspace_id = $1 AND meta_user_id = $2;
