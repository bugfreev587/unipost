# Social connection authority cutover

This runbook promotes the additive Social Connection schema to connection-level authority. The operation is forward-only and must be run from the exact Expand-compatible application SHA deployed in the target Railway environment.

## Preconditions

- Migrations 138–141 are applied and `social_connection_rollout_state.phase` is `expand`.
- The API and every delivery-worker service are healthy on the same exact 40-character Git SHA.
- `RAILWAY_MIGRATION_BACKUP_TOKEN` can read deployments and create/lock backups only in the target project and environment.
- `UNIPOST_CUTOVER_RUNTIME_SERVICE_IDS` contains every API and delivery-worker Railway service ID, comma separated. `RAILWAY_API_SERVICE_ID` and `RAILWAY_POST_DELIVERY_WORKER_SERVICE_ID` are also accepted.
- The standard Railway identity and backup variables are present: `DATABASE_URL`, `RAILWAY_PROJECT_ID`, `RAILWAY_ENVIRONMENT_ID`, `RAILWAY_SERVICE_ID`, `RAILWAY_DEPLOYMENT_ID`, `RAILWAY_GIT_COMMIT_SHA`, `RAILWAY_POSTGRES_VOLUME_INSTANCE_ID`, and `RAILWAY_POSTGRES_SERVICE_ID`.
- A maintenance window is active. Do not start a release, manual delivery retry, or account maintenance concurrently.

## Preflight

Retain the JSON output with the release evidence:

```bash
api social-connections-preflight --mode=cutover --json > social-connections-preflight.json
```

Review all blockers, conflict groups, Instagram missing-identity counts, alias warnings, active leases, affected-row counts, and relation sizes. Alias warnings are advisory only: never merge two identities from a name, avatar, or heuristic overlap. Resolve blockers and rerun preflight until it is clean.

## Execute

```bash
api social-connections-cutover --json --drain-timeout=5m > social-connections-cutover.json
```

The command performs the sequence; no separate manual “pause worker” step is required:

1. acquires the global cutover lock and records a durable attempt;
2. changes the database phase to `draining`, causing the database claim trigger to refuse new delivery leases from old and new workers;
3. waits for running/retrying provider leases to reach zero;
4. verifies every active required Railway deployment is `SUCCESS` on the exact SHA;
5. verifies every UniPost database session identifies an allowed service, deployment, and SHA;
6. reruns the secret-free preflight;
7. creates and locks a fresh Railway volume backup bound to the exact project, environment, Postgres service, volume, application service, and SHA; and
8. runs the locked reconciliation transaction and commits authority, evidence, and rollout phase together.

Any failure before a committed reconciliation restores `expand`, so database claims resume. A reconciliation failure rolls back its transaction. A completed operation is idempotent and does not create a second backup or repeat data changes.

## Success verification

Keep both JSON files, the Railway backup ID/workflow ID, the exact SHA, and the `social_connection_cutover_runs` row. Confirm:

- rollout phase is `cutover` with the exact application SHA and environment ID;
- there are no publishable active bindings with null `connection_id`;
- no pending delivery job lacks `connection_id` or `binding_version`;
- same-binding thread jobs are accepted and sibling-binding selection returns `DUPLICATE_SOCIAL_CONNECTION`;
- quarantined accounts can reconnect while keeping their original public `account_id`;
- Inbox visibility remains scoped by `(workspace_id, external_user_id)`;
- current-day reservation units and operation rows are conserved; and
- only the verified runtime SHA remains active.

## If the command dies mid-run

The command restores `expand` itself on any failure before a committed
reconciliation. That compensation runs in-process, so it does **not** run if the
process is killed outright — SIGKILL, an OOM kill, or a pod eviction. A phase
left at `draining` or `cutting_over` halts publishing for every workspace.

**This recovers on its own; no operator action is required.** Every API and
worker process runs a recovery check every 30 seconds. It probes the cutover
advisory lock, which Postgres releases automatically when the holding
connection dies, and restores `expand` when all of the following hold:

- the phase is `draining` or `cutting_over`;
- no session holds the cutover advisory lock, so no cutover is running;
- `cutover_completed_at` is NULL, so no reconciliation committed; and
- the phase has been untouched for at least two minutes.

Expect publishing to resume within roughly a minute of the process dying. The
recovery logs at WARN:

```
restored social connection rollout phase to expand after an interrupted cutover
```

A live cutover holds the advisory lock for its entire run, so a slow drain is
never mistaken for an abandoned one. A committed cutover sets phase `cutover`
in the same transaction as the authority changes, so it can never be walked
back.

Queued deliveries are not lost while the phase is stuck; they stay pending and
resume when the phase returns to `expand`.

After a recovery fires, review the interrupted attempt before retrying:

```sql
SELECT phase, cutover_completed_at, cutover_application_sha, cutover_backend_pid
FROM social_connection_rollout_state WHERE id = 'global';

SELECT id, status, phase_before, started_at, completed_at
FROM social_connection_cutover_runs ORDER BY started_at DESC LIMIT 1;
```

If the latest run is `succeeded`, or `cutover_completed_at` is set, a
reconciliation committed — treat it as a completed cutover and use the rollback
section below rather than retrying.

## Failure and rollback

Do not run `goose down` after cutover. Goose Down removes additive schema; it cannot reverse an authority decision, quarantine classification, Inbox supersession, target reservation, or quota consolidation.

If a committed cutover must be rolled back:

1. stop API and worker writes in the affected environment;
2. restore the exact locked pre-cutover Railway volume backup recorded by the command;
3. deploy the Expand-compatible SHA recorded with that backup;
4. verify the restored database phase is `expand`; and
5. rerun the full preflight before scheduling another attempt.

Never restore backup evidence from another Railway environment or SHA.
