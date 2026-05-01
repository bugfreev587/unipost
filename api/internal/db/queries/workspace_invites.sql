-- name: CreateInvite :one
-- Creates a fresh pending invite. expires_at is supplied by the
-- handler (default 7 days) so test code can shorten the window.
INSERT INTO workspace_invites (workspace_id, email, role, token, invited_by, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetInviteByToken :one
-- Fetch by URL token (the public-facing identifier). Used by both
-- the unauthenticated preview endpoint and the authenticated accept
-- endpoint. Returns ErrNoRows on unknown / typo'd tokens.
SELECT * FROM workspace_invites WHERE token = $1;

-- name: ListPendingInvitesByWorkspace :many
-- Used by the members management UI to show outstanding invites
-- alongside the active member list.
SELECT * FROM workspace_invites
WHERE workspace_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: ListPendingInvitesByEmail :many
-- Used by /v1/me-style endpoints to surface pending invites a freshly
-- signed-up user might want to accept. Cross-workspace, so the dashboard
-- can show "you have an invite to Acme Inc." even before any membership
-- exists.
SELECT * FROM workspace_invites
WHERE email = $1 AND accepted_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: MarkInviteAccepted :exec
-- Idempotent: re-accepting an already-accepted invite is a no-op
-- (accepted_at stays the original timestamp).
UPDATE workspace_invites
SET accepted_at = NOW()
WHERE id = $1 AND accepted_at IS NULL;

-- name: RevokeInvite :exec
-- Admins revoke a pending invite. Already-accepted invites are not
-- revocable here — remove the resulting member instead.
UPDATE workspace_invites
SET revoked_at = NOW()
WHERE id = $1 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: GetPendingInviteByWorkspaceAndEmail :one
-- Duplicate-invite guard: when an admin invites the same email twice,
-- we want the second call to fail / refresh the existing row instead
-- of creating a parallel pending invite.
SELECT * FROM workspace_invites
WHERE workspace_id = $1 AND email = $2
  AND accepted_at IS NULL AND revoked_at IS NULL
ORDER BY created_at DESC
LIMIT 1;
