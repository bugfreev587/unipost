-- +goose Up
CREATE TABLE media_processing_jobs (
  id                   TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id         TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  kind                 TEXT NOT NULL,
  status               TEXT NOT NULL,
  input_video_media_id TEXT NOT NULL,
  input_audio_media_id TEXT NOT NULL,
  output_media_id      TEXT,
  mode                 TEXT NOT NULL DEFAULT 'mix',
  fit                  TEXT NOT NULL DEFAULT 'trim_to_video',
  video_volume         INTEGER NOT NULL DEFAULT 100,
  audio_volume         INTEGER NOT NULL DEFAULT 100,
  audio_start_ms       INTEGER NOT NULL DEFAULT 0,
  request              JSONB NOT NULL DEFAULT '{}'::jsonb,
  idempotency_key      TEXT,
  request_hash         TEXT,
  error_code           TEXT,
  error_message        TEXT,
  retryable            BOOLEAN NOT NULL DEFAULT false,
  attempts             INTEGER NOT NULL DEFAULT 0,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at           TIMESTAMPTZ,
  completed_at         TIMESTAMPTZ,
  CHECK (kind IN ('audio_overlay')),
  CHECK (status IN ('queued', 'processing', 'succeeded', 'failed', 'cancelled')),
  CHECK (mode IN ('mix', 'replace')),
  CHECK (fit IN ('trim_to_video', 'loop_to_video')),
  CHECK (video_volume >= 0 AND video_volume <= 100),
  CHECK (audio_volume >= 0 AND audio_volume <= 100),
  CHECK (audio_start_ms >= 0)
);

CREATE INDEX media_processing_jobs_workspace_created_idx
  ON media_processing_jobs(workspace_id, created_at DESC);

CREATE INDEX media_processing_jobs_claim_idx
  ON media_processing_jobs(status, created_at, id)
  WHERE status = 'queued';

CREATE INDEX media_processing_jobs_active_idx
  ON media_processing_jobs(status, created_at)
  WHERE status IN ('queued', 'processing');

CREATE UNIQUE INDEX media_processing_jobs_workspace_idempotency_idx
  ON media_processing_jobs(workspace_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS media_processing_jobs;
