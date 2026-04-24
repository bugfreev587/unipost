-- +goose Up

-- social_post_results.remotely_deleted_at records when UniPost
-- discovered that the platform-side copy of a previously-published
-- post no longer exists (user deleted it from the platform UI,
-- platform removed it for policy reasons, etc.). Distinct from
-- social_posts.deleted_at — that tracks UniPost-side deletion.
--
-- The inbox sync worker flips this when Graph returns
-- "#100 subcode 33 (object does not exist...)" on a
-- /{post_id}/comments fetch. Once set, the row is excluded from
-- ListPublishedExternalIDsForInboxSync so we stop polling the
-- dead post on every sync tick. The row itself stays in place so
-- analytics/quota counts that filter on status='published' are
-- unaffected.
ALTER TABLE social_post_results
  ADD COLUMN IF NOT EXISTS remotely_deleted_at TIMESTAMPTZ;

-- +goose Down

ALTER TABLE social_post_results DROP COLUMN IF EXISTS remotely_deleted_at;
