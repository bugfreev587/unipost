-- +goose Up
CREATE TABLE social_posts (
  id            TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id    TEXT NOT NULL REFERENCES projects(id),
  caption       TEXT,
  media_urls    TEXT[],
  status        TEXT NOT NULL DEFAULT 'pending',
  scheduled_at  TIMESTAMPTZ,
  published_at  TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE social_post_results (
  id                TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  post_id           TEXT NOT NULL REFERENCES social_posts(id),
  social_account_id TEXT NOT NULL REFERENCES social_accounts(id),
  status            TEXT NOT NULL,
  external_id       TEXT,
  error_message     TEXT,
  published_at      TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS social_post_results;
DROP TABLE IF EXISTS social_posts;
