-- +goose Up

-- social_post_results.fb_media_type records whether a Facebook video
-- post was submitted as mediaType=feed or mediaType=reel at publish
-- time. The status worker uses it to decide whether a /reel/
-- permalink on an in-flight row is the expected Reel outcome (don't
-- fast-fail) or an accidental reclassification of a Feed video
-- (fast-fail after 10 min). NULL preserves legacy behavior for rows
-- created before this column existed and for non-Facebook posts.
ALTER TABLE social_post_results
  ADD COLUMN IF NOT EXISTS fb_media_type TEXT;

-- +goose Down

ALTER TABLE social_post_results DROP COLUMN IF EXISTS fb_media_type;
