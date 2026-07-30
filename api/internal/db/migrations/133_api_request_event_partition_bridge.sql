-- +goose Up
-- unipost:safety reversible

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

-- +goose StatementBegin
DO $bridge$
DECLARE
  item RECORD;
BEGIN
  FOR item IN
    SELECT *
    FROM (VALUES
      ('2026-08-10 00:00:00+00'::TIMESTAMPTZ, '2026-08-17 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w33', 'api_request_error_details_2026w33'),
      ('2026-08-17 00:00:00+00'::TIMESTAMPTZ, '2026-08-24 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w34', 'api_request_error_details_2026w34'),
      ('2026-08-24 00:00:00+00'::TIMESTAMPTZ, '2026-08-31 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w35', 'api_request_error_details_2026w35'),
      ('2026-08-31 00:00:00+00'::TIMESTAMPTZ, '2026-09-07 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w36', 'api_request_error_details_2026w36'),
      ('2026-09-07 00:00:00+00'::TIMESTAMPTZ, '2026-09-14 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w37', 'api_request_error_details_2026w37'),
      ('2026-09-14 00:00:00+00'::TIMESTAMPTZ, '2026-09-21 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w38', 'api_request_error_details_2026w38'),
      ('2026-09-21 00:00:00+00'::TIMESTAMPTZ, '2026-09-28 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w39', 'api_request_error_details_2026w39'),
      ('2026-09-28 00:00:00+00'::TIMESTAMPTZ, '2026-10-05 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w40', 'api_request_error_details_2026w40')
    ) AS weeks(week_start, week_end, event_partition, detail_partition)
  LOOP
    IF EXISTS (
      SELECT 1
      FROM api_request_events_default
      WHERE occurred_at >= item.week_start AND occurred_at < item.week_end
    ) OR EXISTS (
      SELECT 1
      FROM api_request_error_details_default
      WHERE occurred_at >= item.week_start AND occurred_at < item.week_end
    ) THEN
      RAISE EXCEPTION 'bridge partition contains data for [% - %)', item.week_start, item.week_end;
    END IF;

    EXECUTE format(
      'CREATE TABLE %I PARTITION OF api_request_events FOR VALUES FROM (%L) TO (%L)',
      item.event_partition, item.week_start, item.week_end
    );
    EXECUTE format(
      'CREATE TABLE %I PARTITION OF api_request_error_details FOR VALUES FROM (%L) TO (%L)',
      item.detail_partition, item.week_start, item.week_end
    );

    INSERT INTO api_request_partition_manifest (
      week_start, week_end, event_partition, detail_partition
    ) VALUES (
      item.week_start, item.week_end, item.event_partition, item.detail_partition
    );
  END LOOP;
END
$bridge$;
-- +goose StatementEnd

-- +goose Down

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';

-- +goose StatementBegin
DO $bridge$
DECLARE
  item RECORD;
  contains_data BOOLEAN;
  foreign_key_name TEXT;
  foreign_key_count INTEGER;
BEGIN
  FOR item IN
    SELECT *
    FROM (VALUES
      ('2026-08-10 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w33', 'api_request_error_details_2026w33'),
      ('2026-08-17 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w34', 'api_request_error_details_2026w34'),
      ('2026-08-24 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w35', 'api_request_error_details_2026w35'),
      ('2026-08-31 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w36', 'api_request_error_details_2026w36'),
      ('2026-09-07 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w37', 'api_request_error_details_2026w37'),
      ('2026-09-14 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w38', 'api_request_error_details_2026w38'),
      ('2026-09-21 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w39', 'api_request_error_details_2026w39'),
      ('2026-09-28 00:00:00+00'::TIMESTAMPTZ, 'api_request_events_2026w40', 'api_request_error_details_2026w40')
    ) AS weeks(week_start, event_partition, detail_partition)
  LOOP
    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I LIMIT 1)', item.event_partition)
      INTO contains_data;
    IF contains_data THEN
      RAISE EXCEPTION 'bridge partition contains data in %', item.event_partition;
    END IF;

    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I LIMIT 1)', item.detail_partition)
      INTO contains_data;
    IF contains_data THEN
      RAISE EXCEPTION 'bridge partition contains data in %', item.detail_partition;
    END IF;
  END LOOP;

  DELETE FROM api_request_partition_manifest
  WHERE week_start >= '2026-08-10 00:00:00+00'::TIMESTAMPTZ
    AND week_start < '2026-10-05 00:00:00+00'::TIMESTAMPTZ;

  FOR item IN
    SELECT *
    FROM (VALUES
      ('api_request_events_2026w33', 'api_request_error_details_2026w33'),
      ('api_request_events_2026w34', 'api_request_error_details_2026w34'),
      ('api_request_events_2026w35', 'api_request_error_details_2026w35'),
      ('api_request_events_2026w36', 'api_request_error_details_2026w36'),
      ('api_request_events_2026w37', 'api_request_error_details_2026w37'),
      ('api_request_events_2026w38', 'api_request_error_details_2026w38'),
      ('api_request_events_2026w39', 'api_request_error_details_2026w39'),
      ('api_request_events_2026w40', 'api_request_error_details_2026w40')
    ) AS partitions(event_partition, detail_partition)
  LOOP
    EXECUTE format('DROP TABLE %I', item.detail_partition);
  END LOOP;

  SELECT MIN(conname), COUNT(*)
  INTO foreign_key_name, foreign_key_count
  FROM pg_constraint
  WHERE conrelid = 'api_request_error_details'::REGCLASS
    AND confrelid = 'api_request_events'::REGCLASS
    AND contype = 'f';

  IF foreign_key_count <> 1 THEN
    RAISE EXCEPTION 'expected one request detail foreign key, found %', foreign_key_count;
  END IF;

  -- PostgreSQL materializes dependencies from the partitioned foreign key onto
  -- each event child. Remove and restore that parent constraint transactionally
  -- so empty bridge children can be dropped without DROP TABLE CASCADE.
  EXECUTE format(
    'ALTER TABLE api_request_error_details DROP CONSTRAINT %I',
    foreign_key_name
  );

  FOR item IN
    SELECT event_partition
    FROM (VALUES
      ('api_request_events_2026w33'),
      ('api_request_events_2026w34'),
      ('api_request_events_2026w35'),
      ('api_request_events_2026w36'),
      ('api_request_events_2026w37'),
      ('api_request_events_2026w38'),
      ('api_request_events_2026w39'),
      ('api_request_events_2026w40')
    ) AS partitions(event_partition)
  LOOP
    EXECUTE format('DROP TABLE %I', item.event_partition);
  END LOOP;

  EXECUTE format(
    'ALTER TABLE api_request_error_details ADD CONSTRAINT %I '
    || 'FOREIGN KEY (occurred_at, event_id, workspace_id) '
    || 'REFERENCES api_request_events (occurred_at, id, workspace_id) ON DELETE CASCADE',
    foreign_key_name
  );
END
$bridge$;
-- +goose StatementEnd
