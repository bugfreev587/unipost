-- +goose Up
--
-- Persist media cleanup worker telemetry so admin storage dashboards can
-- report actual object deletion history after media rows are hard-deleted.

CREATE TABLE media_cleanup_runs (
  id              TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  worker_name     TEXT NOT NULL DEFAULT 'media_cleanup',
  status          TEXT NOT NULL CHECK (status IN ('running', 'completed', 'completed_with_errors', 'failed', 'skipped')),
  started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at     TIMESTAMPTZ,
  next_run_at     TIMESTAMPTZ,
  scanned_objects INTEGER NOT NULL DEFAULT 0,
  deleted_objects INTEGER NOT NULL DEFAULT 0,
  deleted_bytes   BIGINT NOT NULL DEFAULT 0,
  failed_objects  INTEGER NOT NULL DEFAULT 0,
  failed_bytes    BIGINT NOT NULL DEFAULT 0,
  error_summary   TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX media_cleanup_runs_started_at_idx
  ON media_cleanup_runs (started_at DESC);

CREATE INDEX media_cleanup_runs_finished_at_idx
  ON media_cleanup_runs (finished_at DESC);

CREATE INDEX media_cleanup_runs_status_idx
  ON media_cleanup_runs (status, started_at DESC);

CREATE UNIQUE INDEX media_cleanup_runs_one_running_idx
  ON media_cleanup_runs (worker_name)
  WHERE status = 'running';

-- +goose Down
DROP INDEX IF EXISTS media_cleanup_runs_one_running_idx;
DROP INDEX IF EXISTS media_cleanup_runs_status_idx;
DROP INDEX IF EXISTS media_cleanup_runs_finished_at_idx;
DROP INDEX IF EXISTS media_cleanup_runs_started_at_idx;
DROP TABLE IF EXISTS media_cleanup_runs;
