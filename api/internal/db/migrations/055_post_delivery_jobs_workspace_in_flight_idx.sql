-- +goose Up
-- Tighter partial index supporting the per-workspace concurrent
-- dispatch cap added in the rate-limit Phase-2 work (April 2026).
-- The claim queries do a CTE aggregation:
--
--   SELECT workspace_id, COUNT(*) FROM post_delivery_jobs
--   WHERE state IN ('running', 'retrying')
--   GROUP BY workspace_id;
--
-- Migration 054's partial index (state IN pending/running/retrying)
-- is broader than this aggregation needs — it lets the planner
-- scan a lot of pending rows it then filters away. This index is
-- the in-flight subset the claim path actually queries.
CREATE INDEX IF NOT EXISTS post_delivery_jobs_workspace_in_flight_idx
  ON post_delivery_jobs(workspace_id)
  WHERE state IN ('running', 'retrying');

-- +goose Down
DROP INDEX IF EXISTS post_delivery_jobs_workspace_in_flight_idx;
