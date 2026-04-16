-- name: GetUserActivationCounts :one
-- Returns the three activation-guide counters for a user by aggregating
-- across all their workspaces. Step completion is derived from these on
-- the handler side — we never store per-step state because reality is
-- the source of truth (if a user deletes their only account, step 1
-- correctly reverts to incomplete).
SELECT
  (SELECT COUNT(*)::INTEGER
    FROM social_accounts sa
    JOIN profiles p ON p.id = sa.profile_id
    JOIN workspaces w ON w.id = p.workspace_id
    WHERE w.user_id = $1
      AND sa.disconnected_at IS NULL
      AND sa.status = 'active') AS connected_accounts_count,
  (SELECT COUNT(*)::INTEGER
    FROM social_posts sp
    JOIN workspaces w ON w.id = sp.workspace_id
    WHERE w.user_id = $1
      AND sp.status IN ('published', 'scheduled', 'publishing')) AS posts_sent_count,
  (SELECT COUNT(*)::INTEGER
    FROM api_keys ak
    JOIN workspaces w ON w.id = ak.workspace_id
    WHERE w.user_id = $1
      AND ak.revoked_at IS NULL) AS api_keys_count;

-- name: MarkActivationCompleted :exec
UPDATE users
SET activation_completed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND activation_completed_at IS NULL;

-- name: DismissActivationGuide :exec
UPDATE users
SET activation_guide_dismissed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND activation_guide_dismissed_at IS NULL;
