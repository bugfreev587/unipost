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

-- name: CreateMembership :one
-- Creates a new active membership. Used by the invite-accept handler
-- (PR-E Phase 4). The unique-owner partial index prevents two owners
-- coexisting; non-owner roles are unconstrained beyond the PK.
INSERT INTO workspace_members (workspace_id, user_id, role, status, invited_by, accepted_at)
VALUES ($1, $2, $3, 'active', $4, NOW())
RETURNING *;

-- name: UpdateMemberRole :one
-- Used by the members management UI (PR-E Phase 5). Owner role is
-- protected by the workspace_members_one_owner_idx unique index —
-- promoting a second owner fails at the DB level. Demote the current
-- owner first via TransferOwnership.
UPDATE workspace_members
SET role = $3, updated_at = NOW()
WHERE workspace_id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMembership :exec
-- Removes a member from a workspace. The CASCADE on workspaces.id
-- means the underlying row goes away automatically when the entire
-- workspace is deleted. This query is used for explicit per-member
-- removal from the management UI.
DELETE FROM workspace_members
WHERE workspace_id = $1 AND user_id = $2;

-- name: DemoteCurrentOwner :exec
-- Step 1 of TransferOwnership (caller wraps these two queries in a
-- single tx so the unique-owner index never sees a transient
-- two-owner state). Demotes the current owner to admin.
UPDATE workspace_members
SET role = 'admin', updated_at = NOW()
WHERE workspace_id = $1 AND role = 'owner';

-- name: PromoteToOwner :exec
-- Step 2 of TransferOwnership. MUST run inside the same transaction
-- as DemoteCurrentOwner.
UPDATE workspace_members
SET role = 'owner', updated_at = NOW()
WHERE workspace_id = $1 AND user_id = $2;
