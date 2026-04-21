-- name: CreatePendingConnection :one
-- pages_json is JSONB; caller supplies the already-marshalled blob.
-- Expires is defaulted via the table's DEFAULT clause so callers
-- don't need to compute the timestamp locally.
INSERT INTO pending_connections (
  workspace_id,
  profile_id,
  platform,
  meta_user_id,
  user_token_encrypted,
  user_token_expires_at,
  pages_json
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetPendingConnection :one
-- Workspace-scoped so one workspace can't finalize another's pending
-- connection just by guessing the ID. expires_at check is inline to
-- keep stale rows invisible.
SELECT * FROM pending_connections
WHERE id = $1
  AND workspace_id = $2
  AND expires_at > NOW();

-- name: DeletePendingConnection :exec
-- Called on successful finalize; expired rows get swept by the
-- background cleanup worker via DeleteExpiredPendingConnections.
DELETE FROM pending_connections WHERE id = $1;

-- name: DeleteExpiredPendingConnections :exec
DELETE FROM pending_connections WHERE expires_at <= NOW();
