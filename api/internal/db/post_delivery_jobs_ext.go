package db

import "context"

type CancelActivePostDeliveryJobsByPostParams struct {
	PostID      string `json:"post_id"`
	WorkspaceID string `json:"workspace_id"`
}

const cancelActivePostDeliveryJobsByPost = `-- name: CancelActivePostDeliveryJobsByPost :exec
UPDATE post_delivery_jobs
SET state = 'cancelled',
    failure_stage = 'post_deleted',
    error_code = 'post_deleted',
    platform_error_code = NULL,
    last_error = 'delivery job cancelled because its parent post was deleted',
    next_run_at = NULL,
    updated_at = NOW(),
    finished_at = NOW()
WHERE post_id = $1
  AND workspace_id = $2
  AND state IN ('pending', 'running', 'retrying')
`

func (q *Queries) CancelActivePostDeliveryJobsByPost(ctx context.Context, arg CancelActivePostDeliveryJobsByPostParams) error {
	_, err := q.db.Exec(ctx, cancelActivePostDeliveryJobsByPost, arg.PostID, arg.WorkspaceID)
	return err
}
