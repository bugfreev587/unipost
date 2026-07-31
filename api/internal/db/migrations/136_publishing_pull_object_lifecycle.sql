-- +goose Up
-- unipost:safety reversible
--
-- Provider-neutral lifecycle ledger for content-addressed objects staged under
-- pull/. A single object can be reused by multiple posts and workspaces. The
-- object becomes deletable only after every post usage reaches its existing
-- publishing-media retention deadline.

CREATE TABLE publishing_pull_objects (
  object_key     TEXT PRIMARY KEY,
  content_type   TEXT NOT NULL,
  size_bytes     BIGINT NOT NULL CHECK (size_bytes > 0),
  cleanup_state  TEXT NOT NULL DEFAULT 'active'
    CHECK (cleanup_state IN ('active', 'deleting')),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (object_key LIKE 'pull/%')
);

CREATE TABLE publishing_pull_object_usages (
  id               TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  object_key       TEXT NOT NULL REFERENCES publishing_pull_objects(object_key) ON DELETE CASCADE,
  workspace_id     TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  post_id          TEXT NOT NULL REFERENCES social_posts(id) ON DELETE CASCADE,
  post_status      TEXT NOT NULL,
  cleanup_after_at TIMESTAMPTZ,
  retention_reason TEXT NOT NULL DEFAULT 'active_post',
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (object_key, post_id)
);

CREATE INDEX publishing_pull_object_usages_post_idx
  ON publishing_pull_object_usages (post_id);

CREATE INDEX publishing_pull_object_usages_cleanup_due_idx
  ON publishing_pull_object_usages (cleanup_after_at)
  WHERE cleanup_after_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS publishing_pull_object_usages_cleanup_due_idx;
DROP INDEX IF EXISTS publishing_pull_object_usages_post_idx;
DROP TABLE IF EXISTS publishing_pull_object_usages;
DROP TABLE IF EXISTS publishing_pull_objects;
