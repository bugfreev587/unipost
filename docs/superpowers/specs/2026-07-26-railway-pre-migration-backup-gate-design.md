# Railway Pre-Migration Backup Gate Design

## Goal

Prevent migrations 124 and 125 from modifying existing Railway PostgreSQL data unless the exact target environment has a newly created, successful, locked Railway volume backup. The gate must fail closed before Goose applies any pending migration. It must not connect to, back up, migrate, or deploy staging or production during implementation and testing.

## Current behavior and risk

Every API and worker process calls `db.RunMigrations` during startup. `RunMigrations` immediately asks Goose to apply every pending embedded migration. There is currently no preflight check, backup creation, or backup verification.

The relevant pending data changes are:

- Migration 124 adds `retryable` and `attempt_generation` to publishing-restriction email recipients. It also changes active `media_post_usages` rows from `retention_reason='plan_status'` to `retention_reason='active_post'`. The previous reason cannot be reconstructed reliably after the update.
- Migration 125 changes every existing publishing-restriction email recipient with `status='failed'` to `retryable=FALSE`. The trustworthy pre-125 value cannot be reconstructed.

Migration 124 may already have run in an isolated PR Preview. This design does not rewrite its historical Up section and does not claim to reconstruct values that were already changed. It protects any environment in which an affected data update is still pending.

## Environment isolation

Staging and production are separate backup and migration targets.

- Before staging applies an affected migration, the gate must create and lock a backup of the staging PostgreSQL volume instance.
- Before production applies an affected migration, the gate must independently create and lock a backup of the production PostgreSQL volume instance.
- A staging backup cannot authorize a production migration, and a production backup cannot authorize a staging migration.
- The verified backup record must match the current Railway environment ID and volume instance ID.
- Dev and PR Preview environments never reuse staging or production backup evidence. Any database may proceed without a backup only when preflight proves that zero existing rows would be irreversibly modified. A dev or Preview database with affected existing rows must create and verify its own backup or fail before migration.

## Selected architecture

### 1. Serialize backup and migration

`RunMigrations` will acquire a UniPost-specific PostgreSQL advisory lock on a dedicated database connection before inspecting pending versions. It will hold that lock through backup verification and Goose migration completion.

Every API and worker process follows the same path. The first process performs the preflight and, when necessary, the backup. Later processes wait for the advisory lock, observe that the migrations are already applied, and do not create duplicate backups.

The existing Goose session lock remains in place as defense in depth.

### 2. Detect whether irreversible data changes are pending

The preflight reads the current Goose version and evaluates only migrations that have not yet run.

For migration 124:

- If migration 122 has already run, count existing `media_post_usages` rows where `cleanup_after_at IS NULL` and `retention_reason='plan_status'`.
- If migration 122 has not run but `media_post_usages` already exists, count rows where `cleanup_after_at IS NULL`, because migration 122 will give those existing rows the default `plan_status` value before migration 124 reclassifies them.
- If the table does not exist or the count is zero, migration 124 has no existing data to irreversibly reclassify.

For migration 125:

- If the recipient table exists, count rows where `status='failed'`.
- If the table does not exist or the count is zero, migration 125 has no existing retryability state to overwrite.

The counts are used only to decide whether a backup is mandatory and for audit logging. They do not replace the migration predicates.

### 3. Create and verify the Railway backup

When at least one affected-row count is nonzero, the migration runner requires:

- a dedicated Railway API token supplied as a secret;
- the exact Railway volume instance ID for the current PostgreSQL service;
- Railway-provided project and environment identity for audit and mismatch detection.

Using Railway's fixed public GraphQL endpoint, the runner will:

1. create a manual backup for the configured volume instance;
2. poll the resulting workflow/backup until Railway reports terminal success;
3. resolve the exact new backup record rather than accepting an older scheduled backup;
4. lock that backup so normal retention cannot expire it;
5. re-read the backup and verify its ID, volume instance, successful state, locked state, and creation time;
6. record the backup ID, environment ID, volume instance ID, affected migration versions, affected-row counts, and timestamp in structured logs;
7. only then call Goose.

The Railway token is never logged. The production client cannot override the Railway API hostname. Tests inject a local fake endpoint through an internal dependency seam.

### 4. Fail closed

Goose is not called if any required condition fails, including:

- missing or malformed Railway identity/configuration;
- missing API token or volume instance ID;
- environment or volume mismatch;
- backup creation failure;
- backup polling timeout;
- terminal backup failure;
- inability to resolve the newly created backup;
- inability to lock the backup;
- inability to verify the locked backup record.

The process exits through the existing migration startup failure path. Error messages identify the target environment, pending migration versions, affected-row counts, and failed backup stage without exposing credentials.

The gate never restores a backup automatically. Restore remains an explicit operator action because it replaces or remounts storage and requires separate authorization.

## Configuration and release contract

The migration runner will use narrowly scoped configuration for the Railway API token and PostgreSQL volume instance. The final implementation plan will use repository-consistent names after checking existing Railway variable conventions; no default value may point to staging or production.

Before an affected release reaches staging or production, operators must configure the correct environment-scoped secret and volume instance ID. Missing configuration is an intentional deployment failure, not a bypass.

The first staging deployment creates and locks a staging backup immediately before its affected migrations. A later production promotion repeats the process against the production volume. Both backup IDs must appear in their respective deployment logs and acceptance evidence.

## Testing strategy

Implementation follows TDD.

Unit tests will prove:

- pending-version and affected-row classification for pre-122, 122-to-123, 124, and 125-or-later databases;
- zero affected rows do not call Railway;
- nonzero affected rows cannot reach Goose without successful backup verification;
- every backup failure state blocks migration;
- a successful newly created and locked backup permits migration;
- an old backup, wrong environment, wrong volume, unlocked backup, or ambiguous backup never permits migration;
- credentials are absent from errors and logs.

The existing isolated PostgreSQL CI service will provide transaction-level integration tests that prove:

- the advisory lock allows only one concurrent backup/migration leader;
- a second startup waits and then observes completed migrations without creating another backup;
- failure before backup verification leaves the Goose version and affected rows unchanged;
- successful fake Railway backup verification permits migrations 124 and 125 and produces the expected row states.

The integration suite must fail rather than skip when its isolated PostgreSQL URL is absent from the required CI job. It will not use testcontainers and will never connect to shared dev, staging, or production databases.

Required verification remains the full API suite, the isolated PostgreSQL integration suite, Dashboard build, related frontend contracts, and Dashboard browser regression with zero required skips.

## Operational evidence

For each persistent-environment migration, the completion report must include:

- environment name and ID;
- exact application SHA;
- database volume instance ID;
- backup ID and creation time;
- confirmation that the backup reached success and was locked;
- migrations applied;
- preflight affected-row counts;
- migration and deployment check URLs.

No staging or production migration is considered authorized or complete without this evidence.

## Non-goals

- Rewriting migration 124 Up after it may have executed.
- Fabricating a reversible Down migration for data whose original value is unknown.
- Automatically restoring or deleting Railway backups.
- Sharing backup evidence across environments.
- Deploying, promoting, enabling publishing restrictions, or sending real email as part of this implementation task.
