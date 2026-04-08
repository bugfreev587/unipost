-- +goose Up
--
-- Sprint 2 PR1 + PR7: media library + list_posts cursor index.
--
-- The media table backs the new POST /v1/media flow. Each row
-- represents one user-uploaded asset stored in R2 under the
-- "media/<id>.<ext>" key. Lifecycle:
--
--   pending  → row created, presigned PUT URL returned, client
--              hasn't uploaded yet (or upload still in flight).
--   uploaded → first reference from a publish path triggered a HEAD
--              against R2 that confirmed the object exists. size_bytes
--              and content_type populated from the HEAD response.
--   attached → at least one social_post references this row. Once
--              attached the row is exempt from the abandoned-upload
--              sweeper.
--   deleted  → soft-deleted via DELETE /v1/media/{id}. Hard delete
--              by sweeper after the next tick.
--
-- IDs are TEXT not UUID to match the rest of the schema (projects.id,
-- social_posts.id, etc.). Width / height / duration columns are
-- intentionally omitted in Sprint 2 — none of the validation rules
-- depend on dimensions yet, and extracting them server-side requires
-- decoding image headers, which is hidden complexity we don't need.
-- Sprint 3 can add them when there's a real validation rule that uses
-- them.

CREATE TABLE media (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  storage_key   TEXT NOT NULL UNIQUE,
  content_type  TEXT NOT NULL,
  size_bytes    BIGINT NOT NULL DEFAULT 0,
  status        TEXT NOT NULL DEFAULT 'pending',
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  uploaded_at   TIMESTAMPTZ
);

-- Lookup by project (lists, sweeper).
CREATE INDEX media_project_status_idx ON media (project_id, status);

-- Sweeper target: pending uploads older than 7 days.
-- Partial index keeps it tiny (most rows aren't pending for long).
CREATE INDEX media_pending_sweeper_idx ON media (created_at)
  WHERE status = 'pending';

-- list_posts cursor index for PR7 — covers the (project_id, status?,
-- created_at, id) keyset query. Without it, large projects do a
-- full-table scan + sort for every page. Includes id as the second
-- ORDER BY column so the cursor tuple `(created_at, id)` produces
-- a stable seek even for posts with identical created_at timestamps.
CREATE INDEX social_posts_project_created_idx
  ON social_posts (project_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS social_posts_project_created_idx;
DROP INDEX IF EXISTS media_pending_sweeper_idx;
DROP INDEX IF EXISTS media_project_status_idx;
DROP TABLE IF EXISTS media;
