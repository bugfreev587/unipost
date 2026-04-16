-- +goose Up

-- Cache for platform media context (post image, caption, permalink).
-- The MediaContext endpoint previously hit graph.instagram.com /
-- graph.threads.net on every dashboard refresh, which caused Railway
-- proxy timeouts (surfacing as CORS errors in the browser) when the
-- upstream API was slow. This cache makes subsequent reads a single
-- DB lookup.
CREATE TABLE IF NOT EXISTS inbox_media_cache (
  social_account_id TEXT NOT NULL REFERENCES social_accounts(id) ON DELETE CASCADE,
  external_id       TEXT NOT NULL,
  media_url         TEXT NOT NULL DEFAULT '',
  caption           TEXT NOT NULL DEFAULT '',
  timestamp         TEXT NOT NULL DEFAULT '',
  media_type        TEXT NOT NULL DEFAULT '',
  permalink         TEXT NOT NULL DEFAULT '',
  fetched_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (social_account_id, external_id)
);

-- +goose Down

DROP TABLE IF EXISTS inbox_media_cache;
