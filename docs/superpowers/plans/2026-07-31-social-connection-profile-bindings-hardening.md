# Social Connection Profile Bindings Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make PR #299 rolling-deploy safe and database fail-closed while preserving same-binding threads, Workspace isolation, and recoverable public account IDs.

**Architecture:** Goose migrations 136–139 become Expand-only and install a durable rollout state machine. A persistent Post physical-target reservation enforces one selected public binding per `(post, physical connection)` while allowing multiple jobs for that binding. A backup-gated CLI orchestrates database-level delivery drain, Railway deployment verification, preflight, and forward-only cutover reconciliation; verified reconnect can recover quarantined rows in place.

**Tech Stack:** Go 1.24, PostgreSQL 16, Goose, sqlc, pgx v5, Railway GraphQL API, Next.js 16, TypeScript, Playwright.

---

## File structure

- `api/internal/db/migrations/136_social_connections_and_profile_bindings.sql`: Expand schema, rollout state, compatibility/strict authority trigger, no historical classification.
- `api/internal/db/migrations/137_delivery_job_connection_snapshot.sql`: delivery snapshots, physical-target table/trigger, and cross-version database drain guard.
- `api/internal/db/migrations/138_inbox_connection_deduplication.sql`: connection-aware Inbox schema without historical cutover backfill.
- `api/internal/db/migrations/139_physical_daily_publish_reservations.sql`: legacy-compatible daily ledger schema and compatibility capture only.
- `api/internal/db/queries/social_connection_rollout.sql`: rollout state, preflight read queries, target reservation, and recovery queries.
- `api/internal/db/queries/post_delivery_jobs.sql`: claim gates and snapshot/target checks.
- `api/internal/db/queries/social_accounts.sql`: fail-closed Workspace resolution and quarantined binding recovery.
- `api/internal/socialconnectioncutover/report.go`: stable JSON/human preflight report types.
- `api/internal/socialconnectioncutover/preflight.go`: read-only classifier and alias-warning aggregation.
- `api/internal/socialconnectioncutover/orchestrator.go`: drain, deployment proof, backup, reconciliation, and resume state machine.
- `api/internal/socialconnectioncutover/reconcile.go`: transactional account/job/target/Inbox/quota reconciliation.
- `api/internal/socialconnectioncutover/sql/cutover.sql`: locked forward-only data reconciliation SQL.
- `api/internal/railwaybackup/client.go`: exact-environment active-deployment inventory query.
- `api/cmd/api/social_connection_cutover_command.go`: `preflight` and `cutover` CLI parsing/configuration.
- `api/cmd/api/main.go`: early CLI routing and versioned PostgreSQL application name.
- `api/internal/socialconnections/store.go`: stable-ID targeted recovery.
- `api/internal/handler/oauth.go`, `oauth_facebook.go`, `connect_callback.go`, `connect_bluesky.go`, `social_accounts.go`: authenticated reconnect target propagation and Workspace hardening.
- `dashboard/src/lib/api.ts`, `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`: targeted reconnect request and CTA.
- Contract/unit/PostgreSQL integration tests live next to each modified package.

## Task 1: Convert migrations 136–139 to Expand-only

**Files:**
- Modify: `api/internal/db/migrations/136_social_connections_and_profile_bindings.sql`
- Modify: `api/internal/db/migrations/137_delivery_job_connection_snapshot.sql`
- Modify: `api/internal/db/migrations/138_inbox_connection_deduplication.sql`
- Modify: `api/internal/db/migrations/139_physical_daily_publish_reservations.sql`
- Modify: `api/internal/db/social_connections_migration_contract_test.go`
- Modify: `api/internal/db/migrate_test.go`
- Test: `api/internal/db/social_connections_expand_postgres_integration_test.go`

- [ ] **Step 1: Write failing migration contract tests**

Add assertions that migration 136 creates rollout state in `expand`, permits a null-connection insert, does not contain the historical `UPDATE social_accounts ... reconnect_required`, and does not populate audit/conflict rows. Assert 137–139 contain schema and compatibility triggers but no historical connection-aware backfill.

```go
func TestSocialConnectionMigrationsAreExpandOnly(t *testing.T) {
    migration136 := readMigration(t, "136_social_connections_and_profile_bindings.sql")
    for _, forbidden := range []string{
        "SET status = 'reconnect_required'",
        "INSERT INTO social_connection_migration_audit",
        "SET connection_id = sc.id",
    } {
        if strings.Contains(migration136, forbidden) {
            t.Fatalf("migration 136 contains cutover mutation %q", forbidden)
        }
    }
    for _, required := range []string{
        "social_connection_rollout_state",
        "'global', 'expand'",
        "enforce_social_connection_authority_write_gate",
    } {
        if !strings.Contains(migration136, required) {
            t.Fatalf("migration 136 missing %q", required)
        }
    }
}
```

- [ ] **Step 2: Run the contract test and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestSocialConnectionMigrationsAreExpandOnly -count=1`

Expected: FAIL because migration 136 still performs classification/backfill and installs an immediately strict write gate.

- [ ] **Step 3: Rewrite migration 136 as additive Expand schema**

Keep the table/column definitions and add the rollout row:

```sql
CREATE TABLE social_connection_rollout_state (
  id TEXT PRIMARY KEY CHECK (id = 'global'),
  phase TEXT NOT NULL CHECK (phase IN ('expand','draining','cutting_over','cutover')),
  cutover_application_sha TEXT,
  cutover_environment_id TEXT,
  cutover_completed_at TIMESTAMPTZ,
  last_legacy_write_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO social_connection_rollout_state (id, phase) VALUES ('global', 'expand');
```

Remove historical classification, account status mutation, connection creation, audit insertion, and binding attachment from automatic Up. Preserve the classifier SQL for Task 6 instead of deleting its logic.

- [ ] **Step 4: Make the authority trigger rollout-aware and Workspace-safe**

The trigger must always validate a non-null connection against its Profile Workspace/platform. In `expand`, null inserts remain valid and non-null legacy updates mirror the connection authority. In `cutover`, null inserts fail and only a constructively valid null-to-non-null recovery is allowed.

```sql
SELECT state.phase, profile.workspace_id, connection.*
INTO rollout_phase, profile_workspace_id, authority
FROM social_connection_rollout_state state
JOIN profiles profile ON profile.id = NEW.profile_id
LEFT JOIN social_connections connection ON connection.id = NEW.connection_id
WHERE state.id = 'global';

IF NEW.connection_id IS NOT NULL AND (
  authority.id IS NULL OR authority.workspace_id <> profile_workspace_id
  OR authority.platform <> NEW.platform
) THEN
  RAISE EXCEPTION 'social account connection authority scope mismatch'
    USING ERRCODE = '23514';
END IF;

IF NEW.connection_id IS NULL AND rollout_phase = 'cutover' THEN
  RAISE EXCEPTION 'cutover social account bindings require a connection'
    USING ERRCODE = '23514';
END IF;
```

For an Expand update with an unchanged non-null `connection_id` and active binding, update the matching connection from changed legacy fields. When `binding_status` changes to `unbound`, do not mirror binding-local status/disconnected fields as a physical disconnect.

- [ ] **Step 5: Remove automatic backfills from migrations 137–139**

Migration 137 adds nullable snapshots and indexes only. Migration 138 creates Inbox columns, supersession table, indexes, and delete rehome trigger but does not rank/update historical rows. Migration 139 creates/captures the daily ledger using legacy keys but does not consolidate by future connection.

- [ ] **Step 6: Add PostgreSQL Expand integration coverage**

Test old insert, old refresh mirror, old physical disconnect mirror, unbind non-mirror, and cross-Workspace rejection in a disposable transaction.

```go
_, err := tx.Exec(ctx, `INSERT INTO social_accounts (...) VALUES (...)`) // connection_id omitted
if err != nil { t.Fatalf("expand rejected legacy insert: %v", err) }

_, err = tx.Exec(ctx, `UPDATE social_accounts SET access_token=$2 WHERE id=$1`, bindingID, refreshedToken)
if err != nil { t.Fatalf("expand mirror refresh: %v", err) }
assertConnectionToken(t, tx, connectionID, refreshedToken)
```

- [ ] **Step 7: Regenerate sqlc models and run migration tests**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'SocialConnection|Migration' -count=1`

Expected: PASS.

- [ ] **Step 8: Commit Expand migrations**

```bash
git add api/internal/db/migrations/136_social_connections_and_profile_bindings.sql \
  api/internal/db/migrations/137_delivery_job_connection_snapshot.sql \
  api/internal/db/migrations/138_inbox_connection_deduplication.sql \
  api/internal/db/migrations/139_physical_daily_publish_reservations.sql \
  api/internal/db/*social_connection* api/internal/db/migrate_test.go
git commit -m "refactor(db): make social connection rollout expand-only"
```

## Task 2: Enforce one selected binding per Post physical target

**Files:**
- Modify: `api/internal/db/migrations/137_delivery_job_connection_snapshot.sql`
- Create: `api/internal/db/queries/social_connection_rollout.sql`
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify generated: `api/internal/db/models.go`, `api/internal/db/social_connection_rollout.sql.go`, `api/internal/db/post_delivery_jobs.sql.go`
- Test: `api/internal/db/social_post_physical_targets_postgres_integration_test.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Test: `api/internal/handler/social_post_queue_postgres_integration_test.go`

- [ ] **Step 1: Write failing physical-target integration tests**

Cover same-binding thread, sibling rejection, terminal-then-sibling rejection, retry of same binding, conflict target rejection, and two concurrent sibling inserts.

```go
func TestPhysicalTargetAllowsThreadAndRejectsSibling(t *testing.T) {
    insertJob(t, pool, jobInput{PostID: "post-1", AccountID: "sa-a", ConnectionID: "sc-1"})
    insertJob(t, pool, jobInput{PostID: "post-1", AccountID: "sa-a", ConnectionID: "sc-1"})
    err := insertJobErr(pool, jobInput{PostID: "post-1", AccountID: "sa-b", ConnectionID: "sc-1"})
    assertSQLState(t, err, "23514")
    assertTarget(t, pool, "post-1", "connection:sc-1", "sa-a", "active")
}
```

- [ ] **Step 2: Run the test and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run PhysicalTarget -count=1`

Expected: FAIL because the reservation table and insert trigger do not exist.

- [ ] **Step 3: Add target schema and constraint trigger**

Add `social_post_physical_targets` with namespaced keys, status checks, and primary key. Add a `BEFORE INSERT` trigger on `post_delivery_jobs`:

```sql
target_key := CASE
  WHEN NEW.connection_id IS NULL THEN 'legacy-account:' || NEW.social_account_id
  ELSE 'connection:' || NEW.connection_id
END;

INSERT INTO social_post_physical_targets (
  post_id, physical_target_key, connection_id,
  selected_social_account_id, status, conflict_details
) VALUES (
  NEW.post_id, target_key, NEW.connection_id,
  NEW.social_account_id, 'active', '{}'::jsonb
)
ON CONFLICT (post_id, physical_target_key) DO NOTHING;

SELECT * INTO reserved
FROM social_post_physical_targets
WHERE post_id = NEW.post_id AND physical_target_key = target_key
FOR UPDATE;

IF reserved.status <> 'active'
   OR reserved.selected_social_account_id <> NEW.social_account_id THEN
  RAISE EXCEPTION 'post physical target already selected through another binding'
    USING ERRCODE = '23514', CONSTRAINT = 'social_post_physical_target_binding_check';
END IF;
```

- [ ] **Step 4: Reserve a complete target batch transactionally**

Add a query accepting account/connection arrays and reject any existing sibling before results/jobs are inserted:

```sql
-- name: ReserveSocialPostPhysicalTargets :exec
WITH requested AS (
  SELECT
    @post_id::text AS post_id,
    account_id,
    connection_id,
    CASE WHEN connection_id IS NULL
      THEN 'legacy-account:' || account_id
      ELSE 'connection:' || connection_id
    END AS target_key
  FROM UNNEST(@account_ids::text[], @connection_ids::text[]) AS input(account_id, connection_id)
), conflicting AS (
  SELECT 1
  FROM requested request
  JOIN social_post_physical_targets target
    ON target.post_id = request.post_id
   AND target.physical_target_key = request.target_key
  WHERE target.status <> 'active'
     OR target.selected_social_account_id <> request.account_id
)
INSERT INTO social_post_physical_targets (...)
SELECT ... FROM requested
WHERE NOT EXISTS (SELECT 1 FROM conflicting)
ON CONFLICT (post_id, physical_target_key) DO NOTHING;
```

Update every first-party enqueue entry point to begin one transaction, reserve all targets, create results/jobs, update Post status, and commit. A reservation failure rolls back all mutations and maps to `DUPLICATE_SOCIAL_CONNECTION`.

- [ ] **Step 5: Run physical-target and queue tests**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'PhysicalTarget|DuplicateSocialConnection|Thread' -count=1`

Expected: PASS.

- [ ] **Step 6: Add contention performance assertion**

Insert at least 100 same-binding thread jobs concurrently and assert one target row, zero errors, and a bounded test deadline. Record `pg_stat_user_tables`/elapsed data in the test failure message; do not hard-code production latency claims.

- [ ] **Step 7: Commit target enforcement**

```bash
git add api/internal/db/migrations/137_delivery_job_connection_snapshot.sql \
  api/internal/db/queries/social_connection_rollout.sql \
  api/internal/db/queries/post_delivery_jobs.sql api/internal/db/*.sql.go \
  api/internal/db/models.go api/internal/db/*physical_targets* \
  api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_postgres_integration_test.go
git commit -m "feat(db): enforce post physical target selection"
```

## Task 3: Add deterministic preflight and alias warnings

**Files:**
- Create: `api/internal/socialconnectioncutover/report.go`
- Create: `api/internal/socialconnectioncutover/preflight.go`
- Create: `api/internal/socialconnectioncutover/preflight_test.go`
- Create: `api/internal/socialconnectioncutover/preflight_postgres_integration_test.go`
- Modify: `api/internal/db/queries/social_connection_rollout.sql`

- [ ] **Step 1: Define failing report tests**

Require stable ordering, secret-free JSON, hard-blocker exit semantics, classifier reason counts, IG missing identity, table sizes, active jobs, target collapses, and warning-only alias candidates.

```go
type Report struct {
    GeneratedAt time.Time      `json:"generated_at"`
    Mode        string         `json:"mode"`
    Counts      Counts         `json:"counts"`
    Conflicts   []Conflict     `json:"conflicts"`
    Warnings    []AliasWarning `json:"alias_warnings"`
    Relations   []RelationSize `json:"relations"`
    Blockers    []Blocker      `json:"blockers"`
}

func (r Report) HasBlockers() bool { return len(r.Blockers) != 0 }
```

- [ ] **Step 2: Run and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnectioncutover -run Preflight -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement read-only classifier queries**

Move the exact canonical grouping expressions from migration 136 into read-only queries. Return non-secret hashes/counts, not tokens. Add `pg_total_relation_size` queries for `social_accounts`, `post_delivery_jobs`, `social_post_results`, `inbox_items`, and physical daily tables.

- [ ] **Step 4: Implement heuristic alias warnings**

Warnings compare different canonical identities inside the same Workspace and platform when stable secondary identifiers overlap. For Instagram include `external_account_id`, `instagram_webhook_user_id`, and other explicitly named stored professional identifiers. Do not compare names/avatars and do not merge.

```go
type AliasWarning struct {
    WorkspaceID       string   `json:"workspace_id"`
    Platform          string   `json:"platform"`
    ConnectionIDs     []string `json:"connection_ids"`
    ProviderIDs       []string `json:"provider_identities"`
    SharedIdentifiers []string `json:"shared_secondary_identifiers"`
}
```

- [ ] **Step 5: Prove preflight is read-only**

Run the preflight inside a transaction, snapshot relevant row counts/checksums before and after, and assert equality. Serialize twice with a fixed clock and assert byte-identical JSON.

- [ ] **Step 6: Run package and PostgreSQL tests**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnectioncutover ./internal/db -run 'Preflight|AliasWarning|Classifier' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit preflight**

```bash
git add api/internal/socialconnectioncutover api/internal/db/queries/social_connection_rollout.sql api/internal/db/*.sql.go
git commit -m "feat(api): add social connection cutover preflight"
```

## Task 4: Add database drain and Railway deployment proof

**Files:**
- Modify: `api/internal/db/migrations/137_delivery_job_connection_snapshot.sql`
- Modify: `api/internal/db/queries/social_connection_rollout.sql`
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify: `api/internal/railwaybackup/client.go`
- Modify: `api/internal/railwaybackup/client_test.go`
- Modify: `api/cmd/api/main.go`
- Test: `api/internal/db/social_connection_drain_postgres_integration_test.go`

- [ ] **Step 1: Write failing drain tests**

Assert that phase `draining` suppresses both old-style and new-style pending-to-running/retrying claims while allowing already-running jobs to finalize. Assert resetting `expand` resumes claims.

- [ ] **Step 2: Add cross-version database claim guard**

Install a `BEFORE UPDATE` trigger on `post_delivery_jobs`. During `draining` or `cutting_over`, return `NULL` only for a new lease acquisition; allow terminal updates from an already leased job.

```sql
IF rollout_phase IN ('draining', 'cutting_over')
   AND NEW.state IN ('running', 'retrying')
   AND (OLD.state = 'pending' OR OLD.lease_owner IS DISTINCT FROM NEW.lease_owner) THEN
  RETURN NULL;
END IF;
RETURN NEW;
```

- [ ] **Step 3: Version every runtime database session**

Before `pgxpool.NewWithConfig`, set:

```go
poolConfig.ConnConfig.RuntimeParams["application_name"] = fmt.Sprintf(
    "unipost:%s:%s:%s:%s",
    processMode,
    strings.TrimSpace(os.Getenv("RAILWAY_SERVICE_ID")),
    strings.TrimSpace(os.Getenv("RAILWAY_DEPLOYMENT_ID")),
    strings.TrimSpace(os.Getenv("RAILWAY_GIT_COMMIT_SHA")),
)
```

Reject cutover configuration with missing or malformed SHA/service/deployment identity outside a fresh disposable Preview.

- [ ] **Step 4: Extend the Railway client with active deployment inventory**

Add:

```go
type Deployment struct {
    ID        string
    ServiceID string
    Status    string
    CommitSHA string
}

type DeploymentInventoryRequest struct {
    ProjectID     string
    EnvironmentID string
}

type Client interface {
    // existing methods...
    ActiveDeployments(context.Context, DeploymentInventoryRequest) ([]Deployment, error)
}
```

Use Railway’s official Public GraphQL deployment listing for the exact project/environment, keep statuses `BUILDING`, `DEPLOYING`, and `SUCCESS`, and extract the source commit hash. Tests use a local HTTP fixture and reject missing/ambiguous service or commit identity. Official reference: `https://docs.railway.com/integrations/api/manage-deployments`.

- [ ] **Step 5: Implement deployment/session proof**

Cutover accepts only when every active API/post-delivery-worker deployment is the requested SHA and no older active deployment exists. Query `pg_stat_activity`, ignore the cutover backend, and reject unrecognized UniPost application sessions or sessions carrying a different SHA.

- [ ] **Step 6: Run drain and Railway tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/railwaybackup ./cmd/api -run 'Drain|Deployment|ApplicationName' -count=1`

Expected: PASS.

- [ ] **Step 7: Commit drain proof**

```bash
git add api/internal/db/migrations/137_delivery_job_connection_snapshot.sql \
  api/internal/db/queries/post_delivery_jobs.sql api/internal/db/queries/social_connection_rollout.sql \
  api/internal/railwaybackup api/cmd/api/main.go api/internal/db/*drain*
git commit -m "feat(api): guard social connection cutover drain"
```

## Task 5: Build the backup-gated cutover CLI and orchestrator

**Files:**
- Create: `api/internal/socialconnectioncutover/orchestrator.go`
- Create: `api/internal/socialconnectioncutover/orchestrator_test.go`
- Create: `api/cmd/api/social_connection_cutover_command.go`
- Create: `api/cmd/api/social_connection_cutover_command_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/db/migration_gate.go`
- Modify: `api/internal/db/migration_gate_test.go`

- [ ] **Step 1: Write failing state-machine tests**

Test the exact call order and compensating phase reset:

```go
want := []string{
    "lock", "phase:draining", "wait-active-jobs", "verify-deployments",
    "verify-sessions", "preflight", "backup", "reconcile", "phase:cutover",
}
```

For failures before reconciliation, require `phase:expand` and no backup/cutover. For reconciliation failure, require transaction rollback and `phase:expand`. A completed cutover returns the stored run without a second backup.

- [ ] **Step 2: Export the existing backup-gate primitive**

Refactor migration backup code without changing migration behavior:

```go
func RunIrreversibleOperationWithBackupGate(
    ctx context.Context,
    config MigrationGateConfig,
    client railwaybackup.Client,
    affected []AffectedMigration,
    operation func(context.Context) error,
) error {
    return runAfterBackupGate(ctx, config, client, affected, operation)
}
```

The cutover affected count is the number of account/job/Inbox/result/daily rows reported by preflight.

- [ ] **Step 3: Implement orchestrator with bounded waits**

`Orchestrator.Run` acquires a global advisory lock, enters drain, polls active provider-I/O leases with a configured timeout, verifies Railway/session identity, reruns preflight, invokes the backup gate, reconciles, and resumes. All clocks/pollers are injectable in tests.

- [ ] **Step 4: Add CLI parsing and environment binding**

Support exactly:

```text
api social-connections-preflight [--mode=expand|cutover] [--json]
api social-connections-cutover [--json] [--drain-timeout=5m]
```

Unknown flags fail. Cutover requires `DATABASE_URL`, Railway project/environment/service/Postgres/volume IDs, migration backup token, exact 40-character lowercase SHA, and deployment ID.

- [ ] **Step 5: Run command and gate tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api ./internal/socialconnectioncutover ./internal/db -run 'CutoverCommand|Orchestrator|IrreversibleOperation' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit orchestration**

```bash
git add api/cmd/api api/internal/socialconnectioncutover api/internal/db/migration_gate.go api/internal/db/migration_gate_test.go
git commit -m "feat(api): orchestrate backup-gated social connection cutover"
```

## Task 6: Implement locked cutover reconciliation

**Files:**
- Create: `api/internal/socialconnectioncutover/reconcile.go`
- Create: `api/internal/socialconnectioncutover/sql/cutover.sql`
- Create: `api/internal/socialconnectioncutover/reconcile_postgres_integration_test.go`
- Modify: `api/internal/db/queries/social_connection_rollout.sql`

- [ ] **Step 1: Write failing reconciliation fixtures**

Create fixtures for every classifier reason, safe sibling bindings, pending and running jobs, same-binding thread jobs, terminal sibling history, Inbox duplicates, daily counters, and alias warnings. Assert the transaction is fully rolled back on any postcondition failure.

- [ ] **Step 2: Move the canonical classifier into locked cutover SQL**

The SQL acquires `SHARE ROW EXCLUSIVE` locks in deterministic order, reruns the exact preflight identity/ownership/credential/scope/routing classifier, inserts conflict/audit evidence, promotes safe groups, and marks conflict rows `reconnect_required`. It never logs raw credentials.

- [ ] **Step 3: Reconcile delivery snapshots and physical targets**

Refuse any running/retrying job. Backfill pending job connection/binding snapshots. Collapse legacy targets to `connection:<id>`; preserve same-binding thread targets. For terminal multi-binding history create a `migration_conflict` target with account/result/job IDs in `conflict_details`. For nonterminal multi-binding work, abort cutover.

- [ ] **Step 4: Reconcile Inbox rows**

Rank by `(connection_id, source, external_id, received_at, created_at, id)`, retain the earliest canonical row, insert supersession evidence, and set `inbox_items.connection_id` only on canonical rows before enabling connection uniqueness.

- [ ] **Step 5: Consolidate physical daily accounting**

Rewrite operation `physical_account_id` from legacy account IDs to connection IDs, aggregate reservation counts by `(workspace, connection, platform, utc_date)`, and delete only replaced legacy rows. Assert total operation/reservation units are conserved.

- [ ] **Step 6: Verify cutover postconditions in the transaction**

Require zero publishable null bindings, zero Workspace/platform mismatch, zero invalid pending snapshot, zero unresolved nonterminal target collapse, and no duplicate canonical Inbox target. Set phase `cutover` and record SHA/environment only after all checks pass.

- [ ] **Step 7: Run reconciliation integration suite**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnectioncutover -run 'Reconcile|CutoverPostgres' -count=1`

Expected: PASS, including rollback fixtures.

- [ ] **Step 8: Commit reconciliation**

```bash
git add api/internal/socialconnectioncutover api/internal/db/queries/social_connection_rollout.sql api/internal/db/*.sql.go
git commit -m "feat(db): reconcile social connection authority at cutover"
```

## Task 7: Recover quarantined bindings without changing public IDs

**Files:**
- Modify: `api/internal/socialconnections/store.go`
- Modify: `api/internal/socialconnections/store_test.go`
- Modify: `api/internal/db/queries/social_accounts.sql`
- Modify: `api/internal/db/queries/social_connections.sql`
- Modify: `api/internal/handler/oauth.go`
- Modify: `api/internal/handler/oauth_facebook.go`
- Modify: `api/internal/handler/connect_callback.go`
- Modify: `api/internal/handler/connect_bluesky.go`
- Modify: `api/internal/handler/social_accounts.go`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Test: `api/internal/handler/social_account_reconnect_test.go`
- Test: `dashboard/tests/social-account-bindings-source.test.mjs`

- [ ] **Step 1: Write failing store recovery tests**

Add `ReconnectAccountID` to `CredentialInput`. Test exact row-ID reuse, managed-owner rejection, cross-Workspace rejection, ambiguous same-Profile binding rejection, missing conflict evidence, and no replacement row on failure.

```go
input.ReconnectAccountID = "sa-quarantined"
account, err := store.SaveVerified(ctx, SaveOAuthReuse, input)
if err != nil { t.Fatal(err) }
if account.ID != "sa-quarantined" { t.Fatalf("recovery changed public id: %s", account.ID) }
```

- [ ] **Step 2: Add a constructively safe recovery query**

Lock the quarantined row and unresolved evidence, verify Workspace/Profile/platform/owner, find/create canonical connection, then update the same row:

```sql
UPDATE social_accounts account
SET connection_id = @connection_id,
    binding_version = binding_version + 1,
    binding_status = 'active',
    status = 'active',
    disconnected_at = NULL,
    access_token = @authoritative_access_token,
    refresh_token = @authoritative_refresh_token
FROM profiles profile, social_connections connection
WHERE account.id = @account_id
  AND account.profile_id = profile.id
  AND profile.workspace_id = @workspace_id
  AND connection.id = @connection_id
  AND connection.workspace_id = profile.workspace_id
  AND connection.platform = account.platform
  AND account.connection_id IS NULL
  AND account.status = 'reconnect_required'
RETURNING account.*;
```

Mark an evidence row resolved only when all source account IDs have either recovered or been explicitly retained as quarantine.

- [ ] **Step 3: Propagate a server-validated reconnect target through provider flows**

OAuth/connect state carries an optional encrypted/signed `reconnect_account_id`. Before redirect/session creation, load it by authenticated Workspace and expected Profile. Provider callbacks pass it only after verified identity is returned. Direct and managed connect bodies accept the field only on authenticated reconnect routes.

- [ ] **Step 4: Update Dashboard reconnect CTA**

For `status === "reconnect_required"`, start the provider reconnect flow with that account ID. Normal Add Account remains untargeted. Display a conflict message when stable-ID recovery is ambiguous instead of silently adding a new account.

- [ ] **Step 5: Run backend and Dashboard source tests**

Run:

```bash
cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnections ./internal/handler -run 'Reconnect|Recovery|Quarantined' -count=1
cd ../dashboard && node --test tests/social-account-bindings-source.test.mjs
```

Expected: PASS.

- [ ] **Step 6: Commit stable-ID recovery**

```bash
git add api/internal/socialconnections api/internal/db/queries/social_accounts.sql \
  api/internal/db/queries/social_connections.sql api/internal/db/*.sql.go \
  api/internal/handler dashboard/src dashboard/tests/social-account-bindings-source.test.mjs
git commit -m "feat(api): recover quarantined account bindings in place"
```

## Task 8: Complete Workspace and binding-status hardening

**Files:**
- Modify: `api/internal/db/queries/social_accounts.sql`
- Modify: `api/internal/handler/social_accounts.go`
- Modify: `api/internal/handler/social_posts_validate.go`
- Modify: `api/internal/handler/social_post_retry.go`
- Modify: `api/internal/db/resolved_social_account.go`
- Test: `api/internal/db/social_connections_query_contract_test.go`
- Test: `api/internal/handler/social_account_bindings_test.go`
- Test: `api/internal/handler/social_post_queue_test.go`

- [ ] **Step 1: Write failing isolation tests**

Require resolved reads to return no row for a corrupted cross-Workspace connection, Bind/Unbind to reject empty auth Workspace without loading an arbitrary Profile, and availability helpers to reject `binding_status != active`.

- [ ] **Step 2: Add fail-closed resolved predicate**

```sql
WHERE sa.id = @id
  AND p.workspace_id = @workspace_id
  AND (sa.connection_id IS NULL OR sc.workspace_id = @workspace_id);
```

- [ ] **Step 3: Delete Workspace fallback**

Bind and Unbind return an authorization/internal-context error when `auth.GetWorkspaceID` is empty. They never call `GetProfile` to manufacture authority.

- [ ] **Step 4: Include binding status in availability**

```go
func socialAccountUnavailableForDelivery(account db.SocialAccount, ok bool) bool {
    if !ok || account.BindingStatus != "active" || account.DisconnectedAt.Valid {
        return true
    }
    status := strings.ToLower(strings.TrimSpace(account.Status))
    return status == "disconnected" || status == "reconnect_required"
}
```

Apply the same rule to publish admission.

- [ ] **Step 5: Run hardening tests and regenerate sqlc**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'Workspace|BindingStatus|Bind|Unbind|Unavailable' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit isolation hardening**

```bash
git add api/internal/db api/internal/handler
git commit -m "fix(api): enforce connection workspace and binding state"
```

## Task 9: Make forward-only safety and operations auditable

**Files:**
- Modify: `api/internal/db/migrations/irreversible_data_migrations.json`
- Modify: `api/internal/db/migration_gate.go`
- Modify: `api/internal/db/migration_gate_test.go`
- Modify: `api/internal/db/social_connections_migration_contract_test.go`
- Create: `docs/operations/social-connection-cutover.md`
- Modify: `docs/superpowers/specs/2026-07-31-social-connection-profile-bindings-hardening-design.md`

- [ ] **Step 1: Write failing safety-manifest tests**

Require Expand migrations to contain no destructive cutover mutation and require a manifest entry for the explicit `social-connections-cutover` irreversible operation with affected-row classifier, backup action, and rollback action.

- [ ] **Step 2: Register the cutover operation**

Add a manifest section:

```json
{
  "irreversible_operations": [
    {
      "key": "social-connections-cutover",
      "description": "promotes connection authority and quarantines ambiguous historical bindings",
      "rollback": "restore the exact pre-cutover Railway backup and deploy the Expand-compatible SHA"
    }
  ]
}
```

Runtime registry and manifest tests must match exactly.

- [ ] **Step 3: Document the fully scripted sequence**

The operations runbook lists prerequisites, command examples, JSON artifact retention, automatic drain/deployment/session guards, backup binding, success checks, failure compensation, and backup restore. It explicitly says not to run `goose down` after cutover.

- [ ] **Step 4: Run safety tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./cmd/api -run 'MigrationSafety|Irreversible|Cutover' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit safety documentation**

```bash
git add api/internal/db docs/operations/social-connection-cutover.md \
  docs/superpowers/specs/2026-07-31-social-connection-profile-bindings-hardening-design.md
git commit -m "docs: define forward-only social connection cutover"
```

## Task 10: Full verification, review, push, and Draft PR

**Files:**
- Verify all changed files against `origin/dev`
- Update PR description/checklists only after evidence exists

- [ ] **Step 1: Run formatting and generated-code checks**

```bash
cd api
gofmt -w cmd/api internal/socialconnectioncutover internal/socialconnections internal/handler internal/railwaybackup
sqlc generate
git diff --check
git status --short
```

Expected: no formatting errors and generated code matches queries/schema.

- [ ] **Step 2: Run backend full suite**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS; skipped PostgreSQL tests are separately run in Step 3.

- [ ] **Step 3: Run disposable PostgreSQL integration suites**

Use the repository test database guard and explicit disposable URLs for migration, physical target, cutover, Inbox, quota, and queue integration tests. Any missing URL, skip, timeout, or cancellation is a failed required check and blocks push/PR.

- [ ] **Step 4: Run Dashboard checks**

```bash
cd dashboard
npm run build
npm run test:regression:dashboard
node --test tests/social-account-bindings-source.test.mjs
```

Expected: PASS.

- [ ] **Step 5: Audit unique commits and files**

```bash
git fetch origin
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
git diff --check origin/dev...HEAD
```

Confirm every unique file belongs to social connection bindings/hardening or required test/CI support. Unrelated files block push.

- [ ] **Step 6: Request code review and resolve findings**

Use `superpowers:requesting-code-review`, inspect every blocking/security/migration finding against source, fix valid issues with focused tests, and rerun the complete affected suites.

- [ ] **Step 7: Push the owned branch**

```bash
git push -u origin dev-social-account-profile-bindings-hardening
```

- [ ] **Step 8: Open a Draft PR to dev**

Create a Draft PR from `dev-social-account-profile-bindings-hardening` to `dev`. State that it supersedes the implementation content of #299 but does not close or merge #299. Include exact local verification evidence and the Expand/Cutover operational boundary.

- [ ] **Step 9: Monitor exact-head remote gates**

Wait for GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance on the exact head SHA. A failure, timeout, skip, cancellation, or mismatched SHA is a hard stop.

- [ ] **Step 10: Perform Preview Acceptance without merging**

Verify account binding, targeted reconnect, same-binding thread, sibling rejection, physical-target DB backstop, account listing, Inbox, quota, and cutover preflight in the isolated Preview. Leave the PR Draft/open for user review and do not merge to `dev`.

## Self-review checklist

- Spec coverage: Tasks 1–9 cover every hardening spec section; Task 10 covers local/remote acceptance and the no-merge constraint.
- No placeholder implementation steps remain.
- `Report`, `AliasWarning`, `Deployment`, rollout phases, CLI names, and physical-target names are consistent across tasks.
- Same-binding threads remain allowed; sibling bindings are rejected at admission and database insert.
- Railway deployment verification is based on the official environment/deployment API and fails closed when unavailable.
- Cutover is explicit and forward-only; automatic Goose migrations remain Expand-compatible.
