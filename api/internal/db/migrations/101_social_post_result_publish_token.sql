-- +goose Up
--
-- Idempotent/resumable publish (IG + TikTok).
--
-- Stores the platform's intermediate publish token — Instagram creation_id
-- (container) or TikTok publish_id — the moment an attempt obtains it. If
-- that attempt crashes before recording the final external_id, the retry
-- resumes from this token (TikTok status check; IG re-publish of the same
-- container) instead of re-uploading the media and creating a duplicate.

ALTER TABLE social_post_results ADD COLUMN publish_token TEXT;

-- +goose Down
ALTER TABLE social_post_results DROP COLUMN IF EXISTS publish_token;
