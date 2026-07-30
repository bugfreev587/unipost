# Request Event Default Partition Recovery

## Authorization boundary

Use the row-movement transaction in this procedure only when the Super Admin
`request-event-partitions` status reports default occupancy and automatic
`Ensure` has failed closed. For a `partition_manifest_mismatch`, perform the
catalog triage below and stop; do not use the row-movement transaction to repair
catalog drift. Do not use this procedure for retention, routine partition
creation, or speculative cleanup. A database operator and the release owner
must approve the exact environment, UTC week, backup, and maintenance window
before any write statement runs.

Never use `CASCADE` in this procedure. Automation must not copy, delete, detach,
or drop default-partition rows.

## Required evidence

- exact environment, Railway project/environment IDs, and database identity;
- exact application SHA;
- Railway backup ID created and locked for this recovery attempt;
- affected UTC week and its Monday-to-Monday boundaries;
- event and detail default counts for that week;
- proposed aligned event/detail child names;
- declared maintenance window, database operator, release owner, and approvals;
- current partition-health response and request-event recorder failure/drop
  counters.

Stop if the backup cannot be proven to belong to the target database and locked
for this attempt.

## Manifest/catalog triage

`Ensure` and the release-readiness inspection both validate manifest metadata,
physical attachment, and exact UTC range bounds from PostgreSQL's catalog. Their
reconciliation policy is intentionally narrow:

| Manifest row | Physical event/detail pair | Automatic result |
| --- | --- | --- |
| present and canonical | both attached with exact bounds | no-op |
| absent | both absent | create both, then insert manifest, only when defaults are empty |
| absent | both attached with exact bounds | insert the missing manifest row |
| present | either child missing, detached, or wrong-bound | fail closed |
| absent | partial, detached, name-occupied, or wrong-bound pair | fail closed |

The readiness inspection also fails closed when it finds an attached non-default
child that no manifest row references. It never drops, detaches, renames, or
recreates an existing relation.

For `partition_manifest_mismatch`, record the following read-only evidence for
the affected child names. Run it once for the event child/parent and once for the
detail child/parent. Set the expected UTC boundaries from the approved ISO week.

```psql
\set child_partition 'api_request_events_2026w33'
\set parent_partition 'api_request_events'
\set week_start '2026-08-10 00:00:00+00'
\set week_end '2026-08-17 00:00:00+00'

BEGIN READ ONLY;
SET LOCAL TIME ZONE 'UTC';

SELECT
  child.oid IS NOT NULL AS relation_exists,
  COALESCE(child.relispartition, false) AS is_partition,
  COALESCE(parent.relname = :'parent_partition', false) AS attached_to_expected_parent,
  pg_get_expr(child.relpartbound, child.oid, true) AS actual_bound,
  format(
    'FOR VALUES FROM (%L) TO (%L)',
    :'week_start'::timestamptz,
    :'week_end'::timestamptz
  ) AS expected_bound
FROM (SELECT 1) AS singleton
LEFT JOIN pg_class AS child
  JOIN pg_namespace AS child_namespace
    ON child_namespace.oid = child.relnamespace
   AND child_namespace.nspname = current_schema()
  ON child.relname = :'child_partition'
LEFT JOIN pg_inherits AS inheritance ON inheritance.inhrelid = child.oid
LEFT JOIN pg_class AS parent ON parent.oid = inheritance.inhparent;

ROLLBACK;
```

If the automatic result in the table is `fail closed`, preserve the output,
locked backup, health response, and application SHA, then request a separately
reviewed database remediation. Re-running deployment cannot repair that state.

## Dry run

Set the four psql variables, then run the count-only checks. The names below are
examples; replace them with the approved week.

```psql
\set ON_ERROR_STOP on
\set week_start '2026-08-10 00:00:00+00'
\set week_end '2026-08-17 00:00:00+00'
\set event_partition 'api_request_events_2026w33'
\set detail_partition 'api_request_error_details_2026w33'

SELECT
  (:'week_start'::timestamptz AT TIME ZONE 'UTC') =
    date_trunc('week', :'week_start'::timestamptz AT TIME ZONE 'UTC')
  AND extract(epoch FROM (
    :'week_end'::timestamptz - :'week_start'::timestamptz
  )) = 604800
    AS boundaries_valid,
  starts_with(:'event_partition', 'api_request_events_')
  AND regexp_replace(:'event_partition', '^api_request_events_', '')
      ~ '^[0-9]{4}w[0-9]{2}$'
  AND starts_with(:'detail_partition', 'api_request_error_details_')
  AND regexp_replace(:'detail_partition', '^api_request_error_details_', '')
      ~ '^[0-9]{4}w[0-9]{2}$'
    AS identifiers_valid
\gset

\if :boundaries_valid
\else
  \echo 'ABORT: boundaries are not an exact UTC ISO week'
  \quit
\endif
\if :identifiers_valid
\else
  \echo 'ABORT: partition suffix must match ^[0-9]{4}w[0-9]{2}$'
  \quit
\endif

SELECT
  (SELECT count(*) FROM api_request_events_default
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) AS event_rows,
  (SELECT count(*) FROM api_request_error_details_default
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) AS detail_rows,
  (SELECT count(*) FROM api_request_events_default
   WHERE occurred_at < :'week_start'::timestamptz
      OR occurred_at >= :'week_end'::timestamptz) AS event_rows_outside_week,
  (SELECT count(*) FROM api_request_error_details_default
   WHERE occurred_at < :'week_start'::timestamptz
      OR occurred_at >= :'week_end'::timestamptz) AS detail_rows_outside_week;
```

Record the output in the incident/release evidence. Verify the proposed UTC
boundaries and identifiers independently. Abort if either identifier or boundary
check is false, either intended count differs from the approved evidence, or any
row exists outside the one intended week. A second affected week requires a
separate reviewed recovery attempt.

## Transaction

Run only inside the approved maintenance window. The two-key advisory lock uses
the `RQPT` namespace (`0x52515054` = `1381060692`) and key `1`, matching the
automatic maintainer. Parent/default locks pause competing writes while rows are
moved. Both default row sets are copied before deleting either one.

```psql
\set ON_ERROR_STOP on

BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(1381060692, 1); -- RQPT

LOCK TABLE api_request_events IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE api_request_error_details IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE api_request_events_default IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE api_request_error_details_default IN SHARE ROW EXCLUSIVE MODE;

CREATE TEMP TABLE recovery_event_rows ON COMMIT DROP AS
SELECT *
FROM api_request_events_default
WHERE occurred_at >= :'week_start'::timestamptz
  AND occurred_at < :'week_end'::timestamptz;

CREATE TEMP TABLE recovery_detail_rows ON COMMIT DROP AS
SELECT *
FROM api_request_error_details_default
WHERE occurred_at >= :'week_start'::timestamptz
  AND occurred_at < :'week_end'::timestamptz;

SELECT
  (SELECT count(*) FROM recovery_event_rows) AS source_event_rows,
  (SELECT count(*) FROM recovery_detail_rows) AS source_detail_rows
\gset

-- Delete detail before event so the foreign-key dependency remains valid.
DELETE FROM api_request_error_details_default
WHERE occurred_at >= :'week_start'::timestamptz
  AND occurred_at < :'week_end'::timestamptz;

DELETE FROM api_request_events_default
WHERE occurred_at >= :'week_start'::timestamptz
  AND occurred_at < :'week_end'::timestamptz;

SELECT format(
  'CREATE TABLE %I PARTITION OF api_request_events FOR VALUES FROM (%L) TO (%L)',
  :'event_partition', :'week_start'::timestamptz, :'week_end'::timestamptz
) \gexec

SELECT format(
  'CREATE TABLE %I PARTITION OF api_request_error_details FOR VALUES FROM (%L) TO (%L)',
  :'detail_partition', :'week_start'::timestamptz, :'week_end'::timestamptz
) \gexec

-- Reinsert event before detail so the detail foreign key can resolve.
INSERT INTO api_request_events SELECT * FROM recovery_event_rows;
INSERT INTO api_request_error_details SELECT * FROM recovery_detail_rows;

INSERT INTO api_request_partition_manifest (
  week_start, week_end, event_partition, detail_partition
) VALUES (
  :'week_start'::timestamptz,
  :'week_end'::timestamptz,
  :'event_partition',
  :'detail_partition'
);

SELECT
  (SELECT count(*) FROM api_request_events
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) = :source_event_rows::bigint
  AND
  (SELECT count(*) FROM api_request_error_details
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) = :source_detail_rows::bigint
  AND
  (SELECT count(*) FROM api_request_events_default
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) = 0
  AND
  (SELECT count(*) FROM api_request_error_details_default
   WHERE occurred_at >= :'week_start'::timestamptz
     AND occurred_at < :'week_end'::timestamptz) = 0
  AS counts_match
\gset

\if :counts_match
  COMMIT;
\else
  \echo 'ABORT: recovery count assertion failed; executing ROLLBACK'
  ROLLBACK;
  \quit
\endif
```

If any statement, lock, identifier check, or count assertion fails, execute
`ROLLBACK` and preserve the locked backup. Do not retry with broader locks,
different boundaries, or manual deletes without a new review.

## Acceptance

- both default counts for the affected week are zero;
- both child counts equal their recorded source counts;
- manifest boundaries and child names match the approved ISO week;
- both children are attached to the expected parents;
- the Super Admin partition-health response reports `ready: true` with at least
  14 whole future coverage days;
- request-event recorder drop/write-failure counter deltas during the
  maintenance window are recorded;
- the application SHA, transaction result, locked backup ID, and operator
  acceptance are attached to the release evidence.
