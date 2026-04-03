-- name: GetUsage :one
SELECT * FROM usage WHERE project_id = $1 AND period = $2;

-- name: UpsertUsage :one
INSERT INTO usage (project_id, period, post_count)
VALUES ($1, $2, 0)
ON CONFLICT (project_id, period) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: IncrementUsage :exec
INSERT INTO usage (project_id, period, post_count)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, period)
DO UPDATE SET post_count = usage.post_count + $3, updated_at = NOW();
