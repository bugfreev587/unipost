package db

import "context"

const getActiveScheduledPostLimitOverride = `
-- name: GetActiveScheduledPostLimitOverride :one
SELECT limit_count
FROM workspace_active_scheduled_limits
WHERE workspace_id = $1
  AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY expires_at ASC NULLS LAST
LIMIT 1
`

func (q *Queries) GetActiveScheduledPostLimitOverride(ctx context.Context, workspaceID string) (int32, error) {
	row := q.db.QueryRow(ctx, getActiveScheduledPostLimitOverride, workspaceID)
	var limit int32
	err := row.Scan(&limit)
	return limit, err
}
