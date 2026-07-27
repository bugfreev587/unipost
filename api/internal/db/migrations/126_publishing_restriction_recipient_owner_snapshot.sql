-- +goose Up
-- unipost:safety reversible

ALTER TABLE platform_publishing_restriction_email_recipients
  ADD COLUMN represented_owner_user_ids TEXT[];

UPDATE platform_publishing_restriction_email_recipients
SET represented_owner_user_ids = CASE
  WHEN CARDINALITY(represented_workspace_ids) = 0 THEN ARRAY[]::TEXT[]
  ELSE ARRAY_FILL(canonical_user_id, ARRAY[CARDINALITY(represented_workspace_ids)])
END;

ALTER TABLE platform_publishing_restriction_email_recipients
  ALTER COLUMN represented_owner_user_ids SET NOT NULL,
  ADD CONSTRAINT platform_publishing_restriction_recipient_owner_snapshot_shape_check
  CHECK (
    CARDINALITY(represented_owner_user_ids) = CARDINALITY(represented_workspace_ids)
    AND ARRAY_POSITION(represented_owner_user_ids, NULL) IS NULL
  );

-- +goose Down

ALTER TABLE platform_publishing_restriction_email_recipients
  DROP CONSTRAINT platform_publishing_restriction_recipient_owner_snapshot_shape_check,
  DROP COLUMN represented_owner_user_ids;
