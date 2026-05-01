-- name: GetActiveMembership :one
-- Resolve a user's single active membership. PR-C Phase 2 calls this
-- on every Clerk-session auth to stamp role into the request context.
-- Returns ErrNoRows when the user has no active membership in any
-- workspace (fresh signup pre-bootstrap, or removed-from-team).
SELECT * FROM workspace_members
WHERE user_id = $1 AND status = 'active'
ORDER BY created_at ASC
LIMIT 1;

-- name: GetMembership :one
-- Per-(workspace, user) lookup. Used by API-key auth: the key already
-- has a workspace_id; we read the role of the user who created the
-- key. Returns ErrNoRows when the user lost membership while the key
-- is still active — the auth path treats that as 401.
SELECT * FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: ListMembersByWorkspace :many
-- Used by the future Members management UI (PR-C Phase 5). Order by
-- role first (owner → admin → editor) then by created_at so the
-- owner always sits at the top.
SELECT * FROM workspace_members
WHERE workspace_id = $1
ORDER BY
  CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
  created_at ASC;

-- name: CountActiveMembersByWorkspace :one
-- Used by /v1/limits to render "3 of unlimited members" alongside
-- profile counts. Suspended / pending members don't count toward the
-- per-plan member cap.
SELECT COUNT(*)::INTEGER AS count
FROM workspace_members
WHERE workspace_id = $1 AND status = 'active';
