-- +goose Up

-- social_post_results.url holds the platform's canonical post URL
-- returned by the adapter at publish time. We fetch it from the
-- platform API (Graph API permalink, Twitter post URL, etc.) so
-- the dashboard "View post" link points at the real post page
-- instead of a constructed URL that may not work (Threads uses
-- shortcodes that aren't derivable from the numeric post ID).
ALTER TABLE social_post_results
  ADD COLUMN IF NOT EXISTS url TEXT;

-- +goose Down

ALTER TABLE social_post_results DROP COLUMN IF EXISTS url;
