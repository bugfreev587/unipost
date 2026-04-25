-- +goose Up
--
-- post_delivery_jobs.dismissed_at lets users archive dead delivery
-- jobs from the Queue page without losing the audit trail. The row
-- itself stays — analytics still reads social_post_results.failed —
-- but the queue list and summary skip dismissed rows so an old
-- non-retriable failure doesn't keep the dead-count tile scary.
--
-- A daily auto-dismiss tick fills this column for any dead job
-- older than 30 days, so a workspace whose user never clicks
-- still doesn't accumulate forever-stale rows.
--
-- The partial index targets the only place this column is read in
-- a hot path: the ListPostDeliveryJobsByWorkspace filter that has
-- to exclude dismissed rows. Tiny because most rows are NULL.

ALTER TABLE post_delivery_jobs
  ADD COLUMN dismissed_at TIMESTAMPTZ;

CREATE INDEX post_delivery_jobs_dismissed_idx
  ON post_delivery_jobs (dismissed_at)
  WHERE dismissed_at IS NOT NULL;

-- +goose Down

DROP INDEX IF EXISTS post_delivery_jobs_dismissed_idx;
ALTER TABLE post_delivery_jobs DROP COLUMN IF EXISTS dismissed_at;
