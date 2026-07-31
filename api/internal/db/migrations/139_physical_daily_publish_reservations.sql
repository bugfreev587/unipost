-- +goose Up
-- unipost:safety reversible

ALTER TABLE social_post_results
  ADD COLUMN daily_reservation_operation_key TEXT,
  ADD COLUMN daily_reservation_release_pending BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE physical_daily_publish_reservations (
  workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  physical_account_id TEXT NOT NULL,
  platform            TEXT NOT NULL,
  utc_date            DATE NOT NULL,
  reserved_count      INTEGER NOT NULL CHECK (reserved_count >= 0),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, physical_account_id, platform, utc_date)
);

CREATE TABLE physical_daily_publish_operations (
  workspace_id        TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  operation_key       TEXT NOT NULL,
  physical_account_id TEXT NOT NULL,
  platform            TEXT NOT NULL,
  utc_date            DATE NOT NULL,
  status              TEXT NOT NULL CHECK (status IN ('reserved', 'finalized')),
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (workspace_id, operation_key, utc_date)
);

CREATE INDEX physical_daily_publish_operations_account_idx
  ON physical_daily_publish_operations (
    workspace_id,
    physical_account_id,
    platform,
    utc_date
  );

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reserve_physical_daily_publish(
  requested_workspace_id TEXT,
  requested_physical_account_id TEXT,
  requested_platform TEXT,
  requested_operation_key TEXT,
  requested_daily_cap INTEGER
)
RETURNS INTEGER
LANGUAGE plpgsql
AS $$
DECLARE
  operation_created INTEGER;
  resulting_count INTEGER;
  today_utc DATE := (NOW() AT TIME ZONE 'UTC')::DATE;
BEGIN
  INSERT INTO physical_daily_publish_operations (
    workspace_id,
    operation_key,
    physical_account_id,
    platform,
    utc_date,
    status
  ) VALUES (
    requested_workspace_id,
    requested_operation_key,
    requested_physical_account_id,
    requested_platform,
    today_utc,
    'reserved'
  )
  ON CONFLICT (workspace_id, operation_key, utc_date) DO NOTHING;

  GET DIAGNOSTICS operation_created = ROW_COUNT;
  IF operation_created = 0 THEN
    RETURN 1;
  END IF;

  INSERT INTO physical_daily_publish_reservations (
    workspace_id,
    physical_account_id,
    platform,
    utc_date,
    reserved_count
  ) VALUES (
    requested_workspace_id,
    requested_physical_account_id,
    requested_platform,
    today_utc,
    1
  )
  ON CONFLICT (workspace_id, physical_account_id, platform, utc_date)
  DO UPDATE SET
    reserved_count = physical_daily_publish_reservations.reserved_count + 1,
    updated_at = NOW()
  WHERE physical_daily_publish_reservations.reserved_count < requested_daily_cap
  RETURNING reserved_count INTO resulting_count;

  IF resulting_count IS NULL THEN
    DELETE FROM physical_daily_publish_operations
    WHERE workspace_id = requested_workspace_id
      AND operation_key = requested_operation_key
      AND utc_date = today_utc;
    RETURN 0;
  END IF;

  RETURN 2;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION release_physical_daily_publish(
  requested_workspace_id TEXT,
  requested_operation_key TEXT
)
RETURNS BOOLEAN
LANGUAGE plpgsql
AS $$
DECLARE
  released physical_daily_publish_operations%ROWTYPE;
  today_utc DATE := (NOW() AT TIME ZONE 'UTC')::DATE;
BEGIN
  DELETE FROM physical_daily_publish_operations
  WHERE workspace_id = requested_workspace_id
    AND operation_key = requested_operation_key
    AND utc_date = today_utc
    AND status = 'reserved'
  RETURNING * INTO released;

  IF released.operation_key IS NULL THEN
    RETURN FALSE;
  END IF;

  UPDATE physical_daily_publish_reservations
  SET reserved_count = GREATEST(reserved_count - 1, 0),
      updated_at = NOW()
  WHERE workspace_id = released.workspace_id
    AND physical_account_id = released.physical_account_id
    AND platform = released.platform
    AND utc_date = released.utc_date;
  RETURN TRUE;
END;
$$;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION finalize_physical_daily_publish(
  requested_workspace_id TEXT,
  requested_operation_key TEXT
)
RETURNS BOOLEAN
LANGUAGE SQL
AS $$
  WITH finalized AS (
    UPDATE physical_daily_publish_operations
    SET status = 'finalized', updated_at = NOW()
    WHERE workspace_id = requested_workspace_id
      AND operation_key = requested_operation_key
      AND utc_date = (NOW() AT TIME ZONE 'UTC')::DATE
    RETURNING 1
  )
  SELECT EXISTS (SELECT 1 FROM finalized)
$$;
-- +goose StatementEnd

-- A definite dispatch failure normally releases immediately. If every
-- application-side settlement attempt loses contact with Postgres, persist the
-- release intent together with the result row. This BEFORE trigger makes the
-- release and result write one transaction: either the durable counter is
-- reconciled and the marker is cleared, or the result write fails for retry.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reconcile_daily_publish_release_from_result()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  target_workspace_id TEXT;
BEGIN
  IF NOT NEW.daily_reservation_release_pending THEN
    RETURN NEW;
  END IF;
  IF NEW.daily_reservation_operation_key IS NULL THEN
    RAISE EXCEPTION 'daily reservation release requires an operation key';
  END IF;

  SELECT p.workspace_id
  INTO target_workspace_id
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE sa.id = NEW.social_account_id;

  IF target_workspace_id IS NULL THEN
    RAISE EXCEPTION 'daily reservation release cannot resolve workspace for account %', NEW.social_account_id;
  END IF;

  PERFORM release_physical_daily_publish(target_workspace_id, NEW.daily_reservation_operation_key);
  NEW.daily_reservation_release_pending := FALSE;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER social_post_results_reconcile_daily_release
BEFORE INSERT OR UPDATE OF daily_reservation_release_pending, daily_reservation_operation_key
ON social_post_results
FOR EACH ROW
WHEN (NEW.daily_reservation_release_pending)
EXECUTE FUNCTION reconcile_daily_publish_release_from_result();

-- Capture successful writes from rolling old pods, which do not populate the
-- new operation key. New application writes already reserved the slot and are
-- therefore excluded from this compatibility path.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION capture_legacy_physical_daily_publish()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
  target_workspace_id TEXT;
  target_physical_account_id TEXT;
  target_platform TEXT;
BEGIN
  IF NEW.published_at IS NULL
     OR NEW.daily_reservation_operation_key IS NOT NULL
     OR (TG_OP = 'UPDATE' AND OLD.published_at IS NOT NULL) THEN
    RETURN NEW;
  END IF;

  SELECT p.workspace_id, COALESCE(sa.connection_id, sa.id), sa.platform
  INTO target_workspace_id, target_physical_account_id, target_platform
  FROM social_accounts sa
  JOIN profiles p ON p.id = sa.profile_id
  WHERE sa.id = NEW.social_account_id;

  IF target_workspace_id IS NOT NULL THEN
    INSERT INTO physical_daily_publish_reservations (
      workspace_id,
      physical_account_id,
      platform,
      utc_date,
      reserved_count
    ) VALUES (
      target_workspace_id,
      target_physical_account_id,
      target_platform,
      (NEW.published_at AT TIME ZONE 'UTC')::DATE,
      1
    )
    ON CONFLICT (workspace_id, physical_account_id, platform, utc_date)
    DO UPDATE SET
      reserved_count = physical_daily_publish_reservations.reserved_count + 1,
      updated_at = NOW();
  END IF;

  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER social_post_results_capture_legacy_daily_publish
AFTER INSERT OR UPDATE OF published_at ON social_post_results
FOR EACH ROW
EXECUTE FUNCTION capture_legacy_physical_daily_publish();

-- +goose Down

DROP TRIGGER IF EXISTS social_post_results_capture_legacy_daily_publish ON social_post_results;
DROP FUNCTION IF EXISTS capture_legacy_physical_daily_publish();
DROP TRIGGER IF EXISTS social_post_results_reconcile_daily_release ON social_post_results;
DROP FUNCTION IF EXISTS reconcile_daily_publish_release_from_result();
DROP FUNCTION IF EXISTS finalize_physical_daily_publish(TEXT, TEXT);
DROP FUNCTION IF EXISTS release_physical_daily_publish(TEXT, TEXT);
DROP FUNCTION IF EXISTS reserve_physical_daily_publish(TEXT, TEXT, TEXT, TEXT, INTEGER);
DROP INDEX IF EXISTS physical_daily_publish_operations_account_idx;
DROP TABLE IF EXISTS physical_daily_publish_operations;
DROP TABLE IF EXISTS physical_daily_publish_reservations;
ALTER TABLE social_post_results
  DROP COLUMN IF EXISTS daily_reservation_release_pending,
  DROP COLUMN IF EXISTS daily_reservation_operation_key;
