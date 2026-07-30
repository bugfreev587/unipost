# Request Event Partition Safety Design

**Date:** 2026-07-29
**Status:** Proposed
**Owner area:** API / Admin Observability

## 1. Problem

Migration 130 created aligned weekly partitions for `api_request_events` and
`api_request_error_details` through `2026-08-10T00:00:00Z`, plus default
partitions. The canonical request-event recorder is an immediate dark
dual-write path; `observability_reads_v2` controls reads only and cannot stop
these writes.

Without follow-up automation, events after August 10 continue to succeed but
land in the default partitions. That avoids an outage while weakening cheap
partition retention and making later partition attachment require a scan or
row movement under stronger locks.

The same rollout temporarily increases database storage because legacy
`integration_logs` and `api_metrics` writers remain active during the
comparison window.

## 2. Goals

- Maintain aligned event/detail weekly partitions without a recurring calendar
  deadline.
- Guarantee at deployment time that at least 14 complete days of explicit
  future partition coverage exist.
- Normally maintain the current UTC week plus eight future UTC weeks.
- Refuse to hide default-partition rows by silently moving or deleting them.
- Expose partition coverage, default occupancy, and storage size to Super
  Admin operators.
- Keep maintenance failure isolated from customer request results while still
  failing a deployment that cannot establish the minimum safe horizon.
- Preserve migration 130 unchanged because it has already run in development.

## 3. Non-goals

- Do not detach, drop, delete, backfill, or move request-event rows.
- Do not activate `observability_reads_v2`.
- Do not stop either legacy writer.
- Do not alter publishing, account connection, billing, quota, authentication,
  or other customer business transactions.
- Do not add a PostgreSQL extension or external scheduler dependency.

Destructive detach/drop automation remains a separate workstream. It requires
sealed rollup completeness, per-plan retention evidence, a production-scale
staging rehearsal, and Railway backup authorization before implementation.

## 4. Considered approaches

### A. Add several hard-coded partitions in migration 133

This is the smallest immediate patch, but it only moves the deadline. Every
release cycle would need another migration, and an overlooked calendar
deadline would recreate the same default-partition accumulation risk.

### B. Application-owned pre-deploy ensure plus runtime worker

One partition service owns naming, validation, creation, inspection, and
structured health. The Railway `migrate` command invokes it after Goose and
fails the pre-deploy step unless the minimum horizon is ready. The API process
uses the same service at startup and every six hours to maintain a wider
eight-week horizon.

This is the selected approach. It provides an immediate bridge and the
non-destructive part of the long-term lifecycle without rewriting applied SQL.

### C. PostgreSQL `pg_cron` or an external scheduled job

This would move scheduling outside the API, but it introduces an environment
capability and operational authority not currently present in UniPost. It
would also require separate rollout, credentials, and monitoring for every
environment.

## 5. Architecture

Create a focused `api/internal/requesteventpartitions` package. It has no
dependency on HTTP handlers or customer business packages.

The package contains:

1. A UTC/ISO-week planner that returns deterministic boundaries and safe child
   table names.
2. A PostgreSQL store that serializes maintenance with one transaction-level
   advisory lock.
3. An `Ensure` operation that validates existing manifest rows and creates
   missing aligned event/detail partitions.
4. An `Inspect` operation that reports explicit horizon, default occupancy,
   estimated rows, and total relation bytes.
5. A runtime worker that maintains coverage and stores process-local attempt
   statistics.
6. A Super Admin handler exposing the combined persistent and process-local
   state.

The pre-deploy migration command opens a bounded PostgreSQL connection after
Goose succeeds, runs `Ensure` for the normal eight-week horizon, runs
`Inspect`, and fails unless at least 14 complete future days are explicit and
both default partitions contain zero rows.

## 6. Partition planning

- Weeks start Monday at `00:00:00Z`.
- The planner always includes the current week and eight future weeks.
- Event children use `api_request_events_YYYYwWW`.
- Detail children use `api_request_error_details_YYYYwWW`.
- Names are generated only from an internal UTC time and validated against
  `^[0-9]{4}w[0-9]{2}$` before identifier quoting.
- The planner treats ISO year boundaries correctly; calendar year is not used
  as a substitute for ISO week-year.

The minimum release horizon is independent of the maintenance target:

- maintenance target: current week plus eight future weeks;
- release readiness: at least 14 complete days after the inspection time.

This separation allows one missed worker run without approaching the release
boundary.

## 7. Transaction and concurrency rules

Each `Ensure` call:

1. Begins one bounded transaction.
2. Sets local lock and statement timeouts.
3. Acquires a transaction-level advisory lock in a request-event partition
   namespace.
4. Loads manifest rows for the requested weeks.
5. Rejects any manifest row whose names or boundaries differ from the
   deterministic plan.
6. Checks both default partitions for rows inside every missing week.
7. If either default contains an in-range row, returns a typed
   `DefaultPartitionOccupiedError` without creating, moving, or deleting data.
8. Creates the event child, then the aligned detail child.
9. Inserts the matching manifest row.
10. Commits only after every requested week is valid.

Concurrent deploys and API workers therefore converge through the same
transaction-level lock. A partial event/detail pair or a mismatched manifest
causes the whole operation to fail closed.

## 8. Runtime behavior

Only API process mode starts the partition worker.

- Run once when the worker starts.
- Repeat every six hours.
- Bound each maintenance attempt with a 30-second context.
- Never block API construction or HTTP readiness.
- On failure, retain the previous successful state and emit a structured error
  containing the event name, planned horizon, default occupancy when known,
  and error class.
- On success, emit one structured summary containing horizon, partition count,
  default rows, estimated total rows, and total bytes.

Customer request handling remains independent. A failed maintenance attempt
cannot change a request status, response, publishing decision, or business
transaction.

## 9. Inspection and monitoring

`Inspect` returns:

- inspection timestamp;
- latest explicit partition end;
- explicit future coverage in whole days;
- manifest partition-pair count;
- exact row count in each default partition;
- estimated live rows across all event partitions;
- estimated live rows across all detail partitions;
- total bytes across each partition tree;
- `ready` and a bounded list of readiness reasons.

Readiness is false when:

- explicit coverage is below 14 days;
- either default partition contains any row;
- a manifest pair is inconsistent with PostgreSQL partition metadata;
- inspection cannot prove the state.

Expose this through
`GET /v1/admin/observability/request-event-partitions`, guarded by the existing
Super Admin route group. The response contains aggregate counts and sizes only;
it never exposes event, workspace, request, header, or payload content.

Structured events provide alert hooks:

- `request_event_partition_maintenance_failed`;
- `request_event_partition_default_occupied`;
- `request_event_partition_horizon_low`;
- `request_event_partition_maintenance_succeeded`.

The staging acceptance checklist records database-size growth during the
dual-write window and blocks production promotion if default occupancy is
nonzero, coverage is below 14 days, or recorder write/drop health is
unacceptable.

## 10. Deployment integration

The existing `migrate` command remains the only Railway pre-deploy command.
After successful migrations it invokes the partition service using the same
`DATABASE_URL`.

Behavior:

- Goose failure: deployment fails as today.
- Partition ensure failure: deployment fails before the new application is
  promoted.
- Inspection not ready: deployment fails with coverage/default evidence.
- Successful ensure and inspection: deployment may continue.

Fresh PR databases and persistent environments use the same partition
readiness behavior. This does not weaken the existing Railway backup gate for
irreversible migrations.

Migration 130 is not edited or split. Development has already applied it, and
changing an applied migration would create environment-dependent migration
history.

## 11. Testing

### Unit tests

- ISO-week planning, including the 2026/2027 boundary.
- Deterministic names and invalid-name rejection.
- Minimum-horizon calculation.
- Worker startup, cadence, cancellation, timeout, and retained stats.
- HTTP response and unavailable-store behavior.
- Migration-command failure propagation.

### PostgreSQL integration tests

- Empty defaults create aligned partitions and manifest rows.
- A second ensure is idempotent.
- Concurrent ensure calls converge without duplicates.
- In-range event default rows fail closed.
- In-range detail default rows fail closed.
- Mismatched manifest rows fail closed.
- Transaction rollback leaves no partial event/detail pair.
- Inspection reports horizon, exact default rows, estimated rows, and bytes.

The CI PostgreSQL job must select these integration tests explicitly so a
renamed or skipped test cannot silently remove the gate.

## 12. Rollout and acceptance

1. Deploy through a task PR and complete Preview Acceptance.
2. Merge to dev and verify the partition endpoint against the exact dev SHA.
3. Confirm dev shows at least 14 days of explicit future coverage and zero
   default rows.
4. Promote to staging through a reviewed `dev -> staging` PR.
5. Record staging database size and row growth for at least one representative
   traffic window.
6. Do not open or merge a production promotion while any readiness reason is
   present.

Destructive partition retirement remains blocked until its separate design
proves rollup sealing, retention eligibility, backup evidence, and aligned
detach/drop safety.
