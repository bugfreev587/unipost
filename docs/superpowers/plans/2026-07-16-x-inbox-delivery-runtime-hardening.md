# X Inbox Delivery Runtime Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden X inbox delivery leadership, stream lifecycle, cleanup durability, and HTTP boundaries for safe multi-replica production operation.

**Architecture:** A dedicated max-one PostgreSQL lock pool multiplexes all process-owned session advisory locks and cancels owned streams if its session fails. A desired-state stream supervisor reconciles app identity and bearer fingerprints, while a cross-replica reconciliation lock serializes upstream resource mutation. Cleanup intents use database leases and scheduled retries, and credential deletion atomically transfers exact upstream IDs into cleanup authority before clearing local state.

**Tech Stack:** Go, pgx/pgxpool, PostgreSQL advisory locks and triggers, net/http, sqlc-generated models, Railway PostgreSQL transaction tests.

---

### Task 1: Bound X control HTTP and isolate persistent streams

**Files:**
- Modify: `api/internal/xinbox/client.go`
- Test: `api/internal/xinbox/client_test.go`

- [ ] Add failing tests proving default control requests have a deadline, oversized JSON is rejected, and injected stream transport is distinct from the control transport.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox -run 'Control|ResponseLimit|StreamHTTP' -count=1` and confirm the new tests fail for the missing behavior.
- [ ] Add `StreamHTTPClient`, `ControlRequestTimeout`, and `MaxJSONResponseBytes` configuration. Construct hardened default transports with bounded dial, TLS handshake, and response-header timeouts.
- [ ] Apply a 15-second context timeout only inside control `do` calls. Decode successful JSON through a bounded reader and fail when the body exceeds 1 MiB.
- [ ] Use the separate stream client in `ConsumeFilteredStream` without a total client timeout, preserving the idle watchdog.
- [ ] Run all `internal/xinbox` tests and commit the task.

### Task 2: Add leased cleanup intents and non-starving claims

**Files:**
- Modify: `api/internal/db/migrations/110_x_inbox_delivery_cleanup_intents.sql`
- Modify: `api/internal/db/models.go`
- Modify: `api/internal/worker/x_inbox_delivery.go`
- Test: `api/internal/worker/x_inbox_delivery_test.go`
- Test: `api/internal/db/x_inbox_delivery_cleanup_migration_test.go`

- [ ] Add failing worker tests for mutually exclusive cleanup claims, owner-checked completion, partial-success retry persistence, capped deterministic backoff, and a failed old intent not blocking a newer due intent.
- [ ] Add failing executable PostgreSQL assertions that the cleanup table contains `lease_owner`, `lease_until`, and `next_attempt_at`, and that concurrent `FOR UPDATE SKIP LOCKED` claims return disjoint rows.
- [ ] Extend migration 110 with the three scheduling columns and a due-work index.
- [ ] Replace `ListCleanupIntents`, `SaveCleanupIntent`, and unconditional delete with `ClaimCleanupIntents`, `ReleaseCleanupIntent`, and owner-checked `CompleteCleanupIntent`.
- [ ] Implement atomic claim SQL using a CTE ordered by `next_attempt_at, created_at, id`, `FOR UPDATE SKIP LOCKED`, lease update, attempt increment, and `RETURNING`.
- [ ] Implement deterministic capped exponential retry scheduling and preserve only resource IDs still requiring deletion.
- [ ] Regenerate sqlc models, run focused worker/database tests, and commit the task.

### Task 3: Make workspace credential deletion transfer authority transactionally

**Files:**
- Modify: `api/internal/db/migrations/110_x_inbox_delivery_cleanup_intents.sql`
- Test: `api/internal/db/x_inbox_delivery_cleanup_migration_test.go`

- [ ] Extend the executable PostgreSQL test to delete a Twitter workspace credential directly.
- [ ] Assert the exact rule/subscription IDs and encrypted bearer are present in one cleanup intent after deletion.
- [ ] Assert the corresponding delivery-resource IDs are cleared and status/last error transition atomically.
- [ ] Update the credential `BEFORE DELETE` trigger function to upsert the intent first, then update all affected resource rows to cleared IDs and `error`.
- [ ] Verify workspace cascade, direct credential deletion, and repeated trigger paths remain idempotent, then commit.

### Task 4: Build the dedicated process-level PostgreSQL lock manager

**Files:**
- Create: `api/internal/worker/x_inbox_locks.go`
- Test: `api/internal/worker/x_inbox_locks_test.go`
- Modify: `api/cmd/api/main.go`
- Test: `api/cmd/api/x_inbox_delivery_wiring_test.go`

- [ ] Add failing lock-manager tests with a fake single-session executor proving multiple app locks share one session, operations are serialized, and no per-app API-pool acquisition occurs.
- [ ] Add failure tests proving acquire/unlock/session failure invalidates the session generation and invokes every registered stream cancel function.
- [ ] Add reconnect tests proving a later acquire uses a new session and can regain leadership.
- [ ] Implement `PostgresStreamLockManager` with one isolated max-one pgx pool parsed from `DATABASE_URL`, serialized session operations, owned-lock cancel callbacks, lazy reconnect, and explicit close.
- [ ] Support both persistent app locks and short-lived reconciliation locks through the same manager.
- [ ] Wire the isolated lock manager into the worker constructor and close it during process shutdown. Keep the shared API pool out of stream-lock ownership.
- [ ] Run lock, race, and wiring tests, then commit.

### Task 5: Serialize reconciliation across replicas

**Files:**
- Modify: `api/internal/worker/x_inbox_delivery.go`
- Test: `api/internal/worker/x_inbox_delivery_test.go`

- [ ] Add failing tests with two workers sharing a fake reconciliation lock. Block the first inside ensure and prove the second performs no list/ensure/persist mutations.
- [ ] Add a failure test proving a skipped or failed reconciliation does not replace the last complete desired-stream map.
- [ ] Acquire `x-inbox-reconcile` before cleanup claims and account reconciliation; release it after the full list/ensure/delete/persist cycle.
- [ ] Treat lock-unavailable as a skipped cycle rather than an error.
- [ ] Run focused worker and race tests, then commit.

### Task 6: Reconcile desired stream lifecycle and bearer changes

**Files:**
- Modify: `api/internal/worker/x_inbox_delivery.go`
- Test: `api/internal/worker/x_inbox_delivery_test.go`

- [ ] Add failing tests proving a removed app cancels its stream and releases its lock.
- [ ] Add failing tests proving a changed bearer fingerprint cancels/restarts exactly once, while an unchanged bearer leaves the existing stream running.
- [ ] Replace the `map[string]struct{}` active set with `map[string]managedXInboxStream` containing cancel function and SHA-256 bearer fingerprint.
- [ ] After a complete reconciliation, diff the desired map against active streams: stop removed/changed entries, retain unchanged entries, and start missing entries.
- [ ] Register each stream cancel callback with its persistent advisory lock. Ensure stream exit removes only the matching generation and releases the lock.
- [ ] On worker shutdown, cancel all managed streams and close the lock manager.
- [ ] Run focused lifecycle tests and race tests, then commit.

### Task 7: Final verification and self-review

**Files:**
- Review all files changed by Tasks 1–6.

- [ ] Run the executable Railway PostgreSQL migration tests with `X_INBOX_TEST_DATABASE_URL`, confirm all test data and migration changes roll back, and verify the remote goose version is unchanged.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./... -count=1`.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test -race ./internal/xinbox ./internal/worker -run 'Stream|Subscription|XClient|Webhook|XInboxDelivery|Cleanup|Lock' -count=1`.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go vet ./...`.
- [ ] Run `git diff --check`, inspect the complete diff against the design, and verify the shared API pool has no per-app lock path.
- [ ] Commit any final corrections and report commit SHAs without pushing or performing real X writes.
