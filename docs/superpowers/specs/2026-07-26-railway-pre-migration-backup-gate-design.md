# Railway Pre-Migration Backup Gate Design

## Goal

Prevent an irreversible data migration from running against a Railway PostgreSQL database unless the exact target environment has a newly created, locked, and independently identifiable Railway volume backup. The gate fails closed before Goose applies any pending affected migration.

This task will not back up, connect to, migrate, deploy, or promote staging or production. Staging and production backups are release-time gates and must be created independently in their own environments.

## Current behavior and risk

Every API and worker process currently calls `db.RunMigrations` during normal startup. That call immediately asks Goose to apply every pending embedded migration. Backup API latency or failure therefore delays the HTTP health endpoint and can make Railway kill an otherwise valid deployment before it becomes healthy.

The relevant data changes are:

- Migration 124 adds `retryable` and `attempt_generation` to publishing-restriction email recipients. It also changes active `media_post_usages` rows from `retention_reason='plan_status'` to `retention_reason='active_post'`. The previous reason cannot be reconstructed reliably after that update.
- Migration 125 changes every existing publishing-restriction email recipient with `status='failed'` to `retryable=FALSE`. The trustworthy pre-125 value cannot be reconstructed.

Migration 124 may already have run in an isolated PR Preview. This design does not rewrite its historical Up section, does not assume that editing an applied migration would rerun it, and does not claim to reconstruct values that were already changed. It protects only environments where an affected data update is still pending.

## Environment isolation

Staging and production are separate backup and migration targets:

- When an affected migration would modify existing staging rows, the gate creates and locks a backup of the staging PostgreSQL volume instance.
- When an affected migration would modify existing production rows, the gate independently creates and locks a backup of the production PostgreSQL volume instance.
- A backup from one environment never authorizes migration in another environment.
- Dev and PR Preview never reuse staging or production evidence. A database may bypass backup only when no registered irreversible migration is pending. A pending irreversible migration requires that environment's own backup even when its affected-row count is zero.

The environment-scoped Railway Project Token must report the configured project and environment IDs. Before any backup list/create call, the gate must also read the configured volume instance and require its returned ID, `volume.projectId`, `environmentId`, and `serviceId` to match the configured volume, project, environment, and explicitly trusted PostgreSQL service ID. It then resolves the configured application service ID in the same project/environment, reads that service's unrendered variables, requires `DATABASE_URL` to be exactly `${{<verified Postgres service name>.DATABASE_URL}}`, and requires Railway's rendered deployment value to equal the migration process's runtime `DATABASE_URL`. Missing, sealed, literal, ambiguous, cross-service, or mismatched values fail closed without printing either URL. Matching only an environment or a human-readable service/volume name is insufficient.

## Selected architecture

### 1. Run migration as a Railway pre-deploy command

Add a narrow migration mode to the existing API artifact and configure Railway with:

```toml
[deploy]
preDeployCommand = ["./bin/api migrate"]
startCommand = "./bin/api"
```

`./bin/api migrate` initializes only logging, migration configuration, the database connection, and the Railway backup client. It performs the gate and migrations, then exits. Railway runs it in a separate pre-deploy container; a nonzero exit blocks the deployment before the new service starts.

Normal API and worker startup no longer runs migrations. It performs a read-only schema-current check before starting application workers/routes and fails clearly if required migrations are absent. This avoids a backup operation competing with Railway's HTTP health deadline and prevents an old worker replica from becoming an accidental migration leader.

The same `api/railway.toml` is reused by API and dedicated worker services, so multiple services may invoke the pre-deploy command for one release. Serialization remains mandatory.

### 2. Serialize all leaders in a fixed order

The migration command acquires one documented, UniPost-specific PostgreSQL advisory-lock key on a dedicated connection before it reads the Goose version or affected-row counts. It holds that lock until Goose completes or the command exits.

Lock order is fixed:

1. acquire the UniPost pre-migration advisory lock;
2. inspect current/pending migrations and affected rows;
3. create and verify any required backup;
4. invoke Goose, which retains its existing session locker as defense in depth;
5. release on connection close.

Concurrent pre-deploy containers wait at step 1. After the leader completes, each follower recomputes current migration state; it sees no affected migration pending and neither creates a backup nor reruns data changes.

Each failed leader attempt uses a new unique backup name. A locked backup left by a crash is not silently reused because its relationship to the failed command cannot be proven. The replacement attempt creates a new backup; orphan review and any later deletion are explicit operator work.

### 3. Classify irreversible migrations explicitly

Use a small registry keyed by migration version. Each entry owns an affected-row query and an explanation of the irreversible field(s). Adding a future irreversible migration without adding it to this registry must fail a migration-manifest test.

For migration 124:

- If migration 122 has already run, count `media_post_usages` rows where `cleanup_after_at IS NULL` and `retention_reason='plan_status'`.
- If migration 122 has not run but `media_post_usages` already exists, count rows where `cleanup_after_at IS NULL`, because migration 122 will give those existing rows the `plan_status` default before migration 124 reclassifies them.
- If the table does not exist or the count is zero, migration 124 has no existing value to overwrite. No historical value needs to be guessed or saved in the pre-122 case.

For migration 125:

- If the recipient table exists, count rows where `status='failed'`.
- If the table does not exist or the count is zero, migration 125 has no existing retryability state to overwrite.

Pending registry membership decides whether backup is mandatory. Counts are audit evidence only and do not replace the SQL migration predicates.

### 4. Create and verify a uniquely attributable Railway backup

When any registry entry is pending, regardless of its affected-row count, the migration command requires:

- a Railway Project Token scoped to the exact target environment;
- exact project and environment IDs;
- the exact PostgreSQL volume instance ID;
- the exact PostgreSQL service ID independently copied from the target Railway Postgres service;
- the exact application service ID from `RAILWAY_SERVICE_ID`;
- the exact application SHA.

The production client uses Railway's fixed public GraphQL hostname; only tests can inject a fake endpoint. The token is sent with `Project-Access-Token` and is never logged.

The command then:

1. reads the volume instance by ID and verifies its exact project, environment, and PostgreSQL service identity;
2. verifies that the application service's unrendered `DATABASE_URL` exactly references the verified PostgreSQL service and that its rendered deployment value exactly equals the runtime `DATABASE_URL`;
3. lists current backups and records all existing backup IDs;
4. creates a backup with a unique name containing environment identity, application SHA, every pending irreversible migration version, and a random attempt suffix;
5. records Railway's server-returned workflow ID for correlation, but does not mistake it for the backup ID;
6. polls the backup list until exactly one record has the unique name, a new ID absent from step 3, a server `createdAt`, a nonempty `externalId`, and non-null `referencedMB`;
7. requires those identifying fields to remain stable across two reads;
8. locks that exact backup and requires the mutation to return `true`;
9. re-lists and requires the same exact ID/name/identity fields to remain present;
10. writes structured audit evidence, then invokes Goose.

The public API exposes no dependable terminal backup-status field to either the account token tested during the spike or the documented backup-list object. `workflowStatus` was introspectable but returned `Not Authorized`; the implementation must not depend on it. Backup-list readiness fields plus successful lock and exact reread are therefore the fail-closed public-API contract. If Railway changes that contract or any field is absent/ambiguous, migration stops.

The first real staging and production release must treat this API contract as an acceptance gate: if its environment-scoped token cannot perform identity, list, create, lock, and reread operations, the pre-deploy command fails and no affected migration runs.

### 5. Fail closed

Goose is not called when any required condition fails, including:

- missing or malformed Railway/database identity;
- missing token, SHA, volume instance ID, application service ID, or trusted PostgreSQL service ID;
- token identity, volume/project/environment/service mismatch, or unverified runtime database binding;
- backup list, create, attribution, readiness, lock, or reread failure;
- timeout, ambiguity, duplicate unique name, or evidence-field regression;
- an unregistered irreversible migration;
- advisory-lock or affected-row query failure.

Errors name the environment, pending migration versions, affected-row counts, and failed backup stage without exposing credentials. The gate never restores, unlocks, or deletes a backup automatically.

## Snapshot and recovery semantics

Railway volume backup is a storage snapshot, not a logical PostgreSQL dump. The disposable spike proved that a committed row present before backup was restored and a row committed after backup was absent. This is evidence of point-in-time volume behavior for the tested PostgreSQL image, not a promise of application-level consistency under every write workload.

The old deployment may remain live while Railway runs a pre-deploy command. Therefore a restore represents the moment of the backup, and writes committed after that moment are not expected in the restored volume. Operational rollback must account for that interval through maintenance/write quiescence when required, PostgreSQL recovery capabilities, or explicit reconciliation. The backup gate guarantees a recovery checkpoint before migration; it does not fabricate continuous point-in-time recovery.

Restore remains an explicit operator action because Railway creates a separate restored volume that must be deliberately mounted. The runbook must preserve the original volume, restore into a separate volume, verify the database offline or through an isolated service, and require separate authorization before any production remount.

## Configuration and release contract

Implementation uses `RAILWAY_MIGRATION_BACKUP_TOKEN`, `RAILWAY_PROJECT_ID`, `RAILWAY_ENVIRONMENT_ID`, `RAILWAY_SERVICE_ID`, `RAILWAY_POSTGRES_VOLUME_INSTANCE_ID`, `RAILWAY_POSTGRES_SERVICE_ID`, and `RAILWAY_GIT_COMMIT_SHA`. No default may point to a real environment. Each persistent environment supplies its own Project Token, project ID, environment ID, application service ID, PostgreSQL volume instance ID, trusted PostgreSQL service ID, and application SHA. `RAILWAY_POSTGRES_SERVICE_ID` is the service that owns the database volume; `RAILWAY_SERVICE_ID` is the API or worker service running the migration command.

Staging and production configure these values independently. A release is intentionally not ready while either environment lacks its own trusted PostgreSQL service ID; operators must copy the service ID from that environment's Postgres service and verify it against the volume attachment before promotion. The migration process never infers this trust anchor from a service name.

Staging and production backups are not created during development of this change. They are created only by their respective release-time pre-deploy commands, immediately before any pending irreversible migration. A zero affected-row count does not bypass the backup. Missing configuration is an intentional deployment failure, never a bypass.

The release workflow must capture the structured backup evidence before accepting the deployment. A failed deployment may leave a locked backup; the report lists it as an orphan candidate, but no automated cleanup occurs.

## Disposable Railway capability spike

The API assumptions were tested on 2026-07-26 in a new disposable project, not in UniPost staging or production:

- project `pr270-backup-spike` (`261b6fe0-bdca-4c39-8e25-e37a640d2182`);
- environment `spike` (`97a9ccb3-85de-47a8-88cc-f0b506485332`);
- source volume instance `dc2dbdc2-d920-4f28-a896-87551de9fef5`;
- backup `751466fe-df7c-4cd0-bb3f-faee1f14622a`, locked after exact reread;
- restored independent volume `d41825e8-286e-40a6-b804-a3a4aae16749`.

Observed behavior:

- backup create returned a workflow ID, while list returned the distinct backup ID and server metadata;
- list, create, exact-name resolution, lock, reread, restore-to-new-volume, and isolated remount succeeded;
- restoring the first backup produced `before_backup` and did not contain the later `after_backup` marker;
- an environment-scoped Project Token identified the exact project/environment and successfully performed backup create/list/lock; capability backup `cb8181be-d7d6-475d-b617-f8e1c13a995a` remains locked;
- `workflowStatus` returned `Not Authorized` and is not part of the design;
- all spike resources remain available for inspection; nothing was deleted.

This spike validates capability and data-point recovery only. It does not authorize or substitute for staging and production's own backups.

## Testing strategy

Implementation follows TDD.

Unit tests prove:

- pending-version and affected-row classification for pre-122, 122-to-123, 124, and 125-or-later databases;
- zero affected rows still require Railway backup evidence when an irreversible migration is pending;
- nonzero affected rows never reach Goose without complete evidence;
- old, ambiguous, duplicate, wrong-project, wrong-environment, wrong-service, wrong-volume, missing-field, unstable, or unlocked backup evidence is rejected;
- workflow ID is never accepted as backup ID;
- every API/timeout/lock failure blocks migration;
- credentials are absent from errors and logs;
- normal serve mode does not call Goose and rejects an outdated schema;
- the irreversible-migration registry remains synchronized with the migration manifest.

The existing isolated PostgreSQL CI service proves:

- N concurrent migration commands elect one leader, create one required backup, and apply migrations once;
- followers wait and then observe completed migrations;
- a leader crash/failure before verified backup leaves Goose version and affected rows unchanged;
- a leader crash after a locked backup but before Goose leaves an auditable orphan, and a replacement attempt creates fresh evidence rather than reusing it;
- failure at every point before Goose leaves affected data unchanged;
- successful fake Railway evidence permits migrations 124 and 125 and yields expected row states.

The integration job fails rather than skips when its isolated PostgreSQL URL is absent. It never uses testcontainers or connects to shared dev, staging, or production databases.

Required verification remains the full API suite, isolated PostgreSQL integration suite, Dashboard build, related frontend contracts, and Dashboard browser regression with zero required skips.

## Operational evidence

For each persistent-environment migration, the completion report includes:

- environment name and ID;
- exact application SHA;
- database volume instance ID;
- database PostgreSQL service ID and the volume instance identity returned by Railway;
- backup workflow ID, backup ID, unique name, server `createdAt`, `externalId`, and size metadata;
- lock mutation result and exact reread confirmation;
- migrations applied and preflight affected-row counts;
- any orphan backup IDs from failed attempts;
- migration and deployment check URLs.

No staging or production migration is considered authorized or complete without its own evidence. Pending irreversible migrations remain fail-closed even when their affected-row counts are zero in every persistent environment.

The sole exception is a newly provisioned, disposable Railway pull-request environment. Its backup may be bypassed only when all of the following are proven while the migration session lock is held:

- the environment name is exactly `unipost-pr-<number>` and Railway's `RAILWAY_PUBLIC_DOMAIN` is exactly `preview-api-unipost-pr-<same-number>.up.railway.app` (Railway provides this value as a hostname without a URL scheme);
- the project ID, environment ID, application service ID, and exact 40-character lowercase application SHA are present;
- the current Goose version is `0` and the current PostgreSQL schema contains zero base tables;
- every pending irreversible migration reports zero affected rows.

The bypass logs all of that evidence before migrations run. A malformed or mismatched Preview identity, an inspection error, an existing table, a nonzero schema version, or any nonzero affected-row count falls back to the normal fail-closed backup gate. This exception never applies to development, staging, or production and does not weaken their token, volume, database-binding, backup-readiness, or lock requirements.

## Non-goals

- Rewriting migration 124 Up after it may have executed.
- Fabricating a reversible Down migration for data whose original value is unknown.
- Automatically restoring, remounting, unlocking, or deleting Railway backups.
- Reusing backup evidence across attempts or environments.
- Claiming volume snapshots replace PostgreSQL point-in-time recovery or write quiescence.
- Deploying, promoting, enabling publishing restrictions, sending real email, or touching PR #270/#271 as part of this design revision.
