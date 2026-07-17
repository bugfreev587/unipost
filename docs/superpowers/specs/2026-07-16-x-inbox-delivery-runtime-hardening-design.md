# X Inbox Delivery Runtime Hardening Design

## Goal

Make X comments and DM delivery safe under multiple API replicas without consuming
one shared API database connection per X app, leaving stale streams running, racing
upstream resource creation, or allowing failed cleanup work to starve newer intents.

## Architecture

### Dedicated stream lock manager

`PostgresStreamLockManager` owns a PostgreSQL pool configured with exactly one
connection. This pool is constructed from `DATABASE_URL` independently of the API
pool and is closed with the worker. The manager serializes every operation on its
single session and may hold multiple session advisory locks at the same time.

Each acquired app lock is associated with the stream's cancel function. If an
acquire, unlock, or session health operation fails, the manager closes the failed
session generation and cancels every stream whose lock belonged to it. A later
acquire reconnects lazily and re-elects leadership. A locked session is never
returned to the shared API pool.

The manager also acquires the short-lived `x-inbox-reconcile` advisory lock. Only
the replica holding this lock may claim cleanup work, list accounts, ensure or
delete upstream resources, and persist the resulting local state.

### Desired-stream supervisor

The delivery worker maintains:

```text
app identity -> cancel function + SHA-256 bearer fingerprint
```

Every successful reconciliation produces the complete desired app-stream map.
The supervisor:

1. cancels streams missing from the new desired map;
2. cancels and restarts streams whose bearer fingerprint changed;
3. leaves matching streams running;
4. starts newly desired streams.

The stream context owns its advisory lock. Stream exit releases the lock. Lock
manager failure cancels all affected contexts, so streams cannot continue without
known leadership.

### Reconciliation ownership

The reconciliation lock covers the complete list/ensure/persist cycle. If another
replica owns it, the current cycle is skipped without changing the desired map.
Stable upstream tags and exact persisted resource IDs remain the idempotency and
orphan-recovery mechanism after process failure.

### Workspace credential deletion

The `BEFORE DELETE ON platform_credentials` trigger performs two operations in the
same database transaction:

1. upserts cleanup intents containing the exact rule ID, subscription ID, app mode,
   and encrypted workspace bearer;
2. clears those IDs from affected `x_inbox_delivery_resources` rows and marks them
   `error` with a credential-deleted explanation.

The cleanup intent remains authoritative until upstream deletion is confirmed.
The next desired-map reconciliation sees no usable workspace credential and
cancels the active stream.

Workspace and social-account cascade triggers remain idempotent through the unique
`social_account_id` cleanup-intent constraint and conflict updates.

### X HTTP clients

Control-plane X calls and persistent filtered streams use separate HTTP clients.

Control calls use:

- bounded connect, TLS handshake, and response-header timeouts;
- a 15-second per-request context deadline;
- a 1 MiB maximum JSON response body;
- bounded error-body draining.

Filtered streams use the same connection-establishment protections but no total
request deadline. The existing stream idle watchdog remains responsible for
detecting stalled persistent responses.

Injected test clients continue to be supported independently for control and
stream requests.

### Cleanup leasing and scheduling

Cleanup intents gain:

- `lease_owner`;
- `lease_until`;
- `next_attempt_at`.

The store atomically claims due, unleased or expired rows by selecting them in
`next_attempt_at, created_at, id` order with `FOR UPDATE SKIP LOCKED`, updating the
lease owner and expiry, incrementing attempts, and returning the claimed rows.

Success deletes an intent only when its lease owner matches. Failure or partial
success persists remaining exact upstream IDs, clears the lease, and schedules a
deterministic capped exponential retry. Scheduling a failed old intent into the
future allows newer due intents to be claimed and prevents starvation.

## Error handling

- Advisory-lock session failure cancels all streams owned by that session
  generation and causes lazy reconnection.
- A failed reconciliation retains the previous desired-stream set because the
  cycle did not establish a complete new desired state.
- Confirmed upstream cleanup deletes the intent. Any unconfirmed response retains
  the intent and schedules retry.
- Partial cleanup retains only the upstream ID that still needs deletion.
- HTTP responses exceeding the configured JSON limit fail closed.

## Verification

Tests must prove:

- multiple app locks share one dedicated session and never consume the API pool;
- session/unlock failure cancels all owned streams and reconnects safely;
- removed streams stop, bearer changes restart, and unchanged streams remain;
- two replicas cannot run reconciliation mutations concurrently;
- workspace credential deletion captures exact cleanup data and clears local IDs
  in one PostgreSQL transaction;
- control requests enforce deadlines and body limits while streams remain
  persistent;
- cleanup claims are mutually exclusive and failed old work does not starve newer
  intents;
- all existing X inbox tests, full Go tests, race tests, vet, and executable
  PostgreSQL migration topology tests pass.

No test may perform a real X write.
