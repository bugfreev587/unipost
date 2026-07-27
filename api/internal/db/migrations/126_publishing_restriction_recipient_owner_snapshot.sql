-- +goose Up
-- unipost:safety reversible

ALTER TABLE platform_publishing_restriction_email_recipients
  ADD COLUMN represented_owner_user_ids TEXT[];

-- +goose StatementBegin
CREATE FUNCTION backfill_publishing_restriction_recipient_owner_snapshot()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
  IF NEW.represented_owner_user_ids IS NULL THEN
    SELECT COALESCE(
      ARRAY_AGG(
        CASE
          WHEN resolution.candidate_count = 1 THEN resolution.owner_user_id
          ELSE NEW.canonical_user_id
        END
        ORDER BY resolution.ordinality
      ),
      ARRAY[]::TEXT[]
    )
    INTO NEW.represented_owner_user_ids
    FROM (
      SELECT represented.ordinality,
             COUNT(owner_user.id) AS candidate_count,
             MIN(owner_user.id) AS owner_user_id
      FROM UNNEST(NEW.represented_workspace_ids) WITH ORDINALITY
        AS represented(workspace_id, ordinality)
      LEFT JOIN workspace_members owner_member
        ON owner_member.workspace_id = represented.workspace_id
       AND owner_member.role = 'owner'
       AND owner_member.status = 'active'
      LEFT JOIN users owner_user
        ON owner_user.id = owner_member.user_id
       AND LOWER(TRIM(owner_user.email)) = NEW.normalized_email
      GROUP BY represented.ordinality
    ) resolution;
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER publishing_restriction_owner_snapshot_backfill
-- Recipient rows are immutable audience snapshots. This compatibility trigger
-- supports old-binary INSERTs only and must not imply UPDATE compatibility.
BEFORE INSERT
ON platform_publishing_restriction_email_recipients
FOR EACH ROW
EXECUTE FUNCTION backfill_publishing_restriction_recipient_owner_snapshot();

-- Historical rows cannot safely distinguish an original same-email owner from
-- a later email/ownership takeover. Preserve the canonical-user fallback so
-- ambiguous historical workspaces fail eligibility instead of being reassigned
-- from current mutable membership data.
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

DROP TRIGGER publishing_restriction_owner_snapshot_backfill
  ON platform_publishing_restriction_email_recipients;

DROP FUNCTION backfill_publishing_restriction_recipient_owner_snapshot();

ALTER TABLE platform_publishing_restriction_email_recipients
  DROP CONSTRAINT platform_publishing_restriction_recipient_owner_snapshot_shape_check,
  DROP COLUMN represented_owner_user_ids;
