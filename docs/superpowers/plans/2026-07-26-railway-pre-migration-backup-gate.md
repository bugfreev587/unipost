# Railway Pre-Migration Backup Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move database migration out of normal process startup and require a new, locked, environment-specific Railway volume backup before any pending irreversible migration modifies existing rows.

**Architecture:** The existing API binary gains a `migrate` subcommand used by Railway `preDeployCommand`. A focused Railway GraphQL client supplies environment identity and backup operations to a database migration gate that serializes leaders with a PostgreSQL advisory lock, classifies pending irreversible migrations, verifies uniquely attributable backup evidence, and only then invokes Goose. Normal API/worker startup performs a read-only schema-current check and never migrates.

**Tech Stack:** Go 1.25, `database/sql` with pgx, Goose v3, Railway GraphQL Public API, PostgreSQL 16 integration service, GitHub Actions.

---

## File structure

- Create `api/internal/railwaybackup/client.go`: fixed-host Railway GraphQL client and backup evidence types.
- Create `api/internal/railwaybackup/client_test.go`: HTTP fake tests for identity, list/create/lock, response validation, and secret redaction.
- Create `api/internal/db/migration_gate.go`: irreversible migration registry, advisory-lock orchestration, backup evidence verification, and schema-current check.
- Create `api/internal/db/migration_gate_test.go`: unit tests for classification, fail-closed behavior, fresh-backup attribution, crash replacement, and migration manifest coverage.
- Create `api/internal/db/migration_gate_postgres_integration_test.go`: transaction-level PostgreSQL concurrency and failure tests under the `integration` build tag.
- Create `api/cmd/api/migration_command.go`: `migrate` argument routing and environment configuration.
- Create `api/cmd/api/migration_command_test.go`: command-mode wiring tests proving serve mode never migrates.
- Modify `api/internal/db/migrate.go`: expose provider helpers needed by the gate without removing the Goose session lock.
- Modify `api/cmd/api/main.go`: route `migrate` before application-only configuration and replace startup migration with a read-only schema check.
- Modify `api/railway.toml`: add `preDeployCommand = ["./bin/api migrate"]`.
- Modify `.github/workflows/ci.yml`: run the new required PostgreSQL migration-gate integration tests against the isolated service.
- Modify `docs/superpowers/specs/2026-07-26-railway-pre-migration-backup-gate-design.md`: only if implementation names differ from the approved design, keeping semantics unchanged.

### Task 1: Railway backup client

**Files:**
- Create: `api/internal/railwaybackup/client.go`
- Create: `api/internal/railwaybackup/client_test.go`

- [ ] **Step 1: Write failing tests for public API behavior**

Define an `httptest.Server` GraphQL fake and tests that exercise the real JSON client:

```go
func TestClientUsesProjectTokenAndReturnsEnvironmentIdentity(t *testing.T) {
    server := newGraphQLFake(t, func(r *http.Request, body graphQLRequest) any {
        if got := r.Header.Get("Project-Access-Token"); got != "secret-token" {
            t.Fatalf("Project-Access-Token = %q", got)
        }
        return map[string]any{"data": map[string]any{
            "projectToken": map[string]any{"projectId": "project-1", "environmentId": "env-1"},
        }}
    })
    client := NewTestClient(server.URL, "secret-token", server.Client())
    got, err := client.Identity(context.Background())
    if err != nil { t.Fatal(err) }
    if got.ProjectID != "project-1" || got.EnvironmentID != "env-1" { t.Fatalf("identity = %#v", got) }
}

func TestClientCreateReturnsWorkflowNotBackupID(t *testing.T) { /* assert WorkflowID */ }
func TestClientListPreservesBackupReadinessFields(t *testing.T) { /* assert ID/name/createdAt/externalId/referencedMB */ }
func TestClientLockRequiresTrue(t *testing.T) { /* false is an error */ }
func TestClientRejectsGraphQLErrorsWithoutLeakingToken(t *testing.T) { /* error excludes secret-token */ }
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/railwaybackup -count=1 -v`

Expected: FAIL because package/functions do not exist.

- [ ] **Step 3: Implement the minimal fixed-host client**

Use these public types and keep endpoint injection test-only:

```go
type Identity struct { ProjectID, EnvironmentID string }
type Backup struct {
    ID, Name, CreatedAt, ExternalID string
    ReferencedMB *int64
}
type CreateResult struct { WorkflowID string }
type Client interface {
    Identity(context.Context) (Identity, error)
    List(context.Context, string) ([]Backup, error)
    Create(context.Context, string, string) (CreateResult, error)
    Lock(context.Context, string, string) error
}

func New(token string) *GraphQLClient {
    return &GraphQLClient{endpoint: "https://backboard.railway.com/graphql/v2", token: token, httpClient: http.DefaultClient}
}
```

Implement one internal `do` helper that rejects non-2xx responses, GraphQL `errors`, missing data, and malformed readiness fields without including the token in returned errors.

- [ ] **Step 4: Run tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/railwaybackup -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Commit the client**

```bash
git add api/internal/railwaybackup/client.go api/internal/railwaybackup/client_test.go
git commit -m "feat: add Railway volume backup client"
```

### Task 2: Irreversible migration classifier and backup gate

**Files:**
- Create: `api/internal/db/migration_gate.go`
- Create: `api/internal/db/migration_gate_test.go`
- Modify: `api/internal/db/migrate.go`

- [ ] **Step 1: Write RED tests for classification and fail-closed orchestration**

Use an injected database/gate seam and a recording Railway fake:

```go
func TestMigrationGateSkipsBackupWhenAffectedRowsAreZero(t *testing.T) {
    runner, backup := newGateHarness(t, gateState{CurrentVersion: 123, FailedRecipients: 0})
    if err := runner.Run(context.Background()); err != nil { t.Fatal(err) }
    if backup.Calls() != nil { t.Fatalf("Railway calls = %v", backup.Calls()) }
    if !runner.GooseCalled() { t.Fatal("Goose was not called") }
}

func TestMigrationGateBlocksGooseUntilFreshLockedBackupIsStable(t *testing.T) {
    runner, backup := newGateHarness(t, gateState{CurrentVersion: 124, FailedRecipients: 2})
    backup.ListResults = [][]railwaybackup.Backup{
        {{ID: "old", Name: "scheduled"}},
        {{ID: "old", Name: "scheduled"}, readyBackup("new", runner.ExpectedName())},
        {{ID: "old", Name: "scheduled"}, readyBackup("new", runner.ExpectedName())},
        {{ID: "old", Name: "scheduled"}, readyBackup("new", runner.ExpectedName())},
    }
    if err := runner.Run(context.Background()); err != nil { t.Fatal(err) }
    if got := backup.LockedID; got != "new" { t.Fatalf("locked ID = %q", got) }
    if !runner.GooseCalled() { t.Fatal("Goose was not called after verified lock") }
}

func TestMigrationGateRejectsWorkflowIDAsBackupID(t *testing.T) { /* no matching list record => fail */ }
func TestMigrationGateRejectsOldDuplicateWrongOrUnstableEvidence(t *testing.T) { /* table cases */ }
func TestMigrationGateNeverReusesOrphanFromPriorAttempt(t *testing.T) { /* new suffix and create call */ }
func TestIrreversibleMigrationRegistryCoversHistoricalDataUpdates(t *testing.T) { /* 124 and 125 */ }
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestMigrationGate|TestIrreversibleMigrationRegistry' -count=1 -v`

Expected: FAIL because the gate is absent.

- [ ] **Step 3: Implement classifier, fixed advisory lock, and evidence verifier**

Implement a registry with exact SQL:

```go
var irreversibleMigrations = []irreversibleMigration{
    {Version: 124, CountAffected: countMigration124Rows},
    {Version: 125, CountAffected: countMigration125Rows},
}

const migrationGateAdvisoryLockKey int64 = 0x554E49504F53544

type MigrationGateConfig struct {
    ProjectID, EnvironmentID, VolumeInstanceID, ApplicationSHA string
    PollInterval, Timeout time.Duration
}
```

On one dedicated SQL connection: acquire `pg_advisory_lock`, re-read the current Goose version, count only pending registry entries, and bypass Railway only when all counts are zero. For nonzero counts, verify Project Token identity, snapshot the existing IDs, create one unique backup, poll until one new exact-name record has `createdAt`, `externalId`, and `referencedMB`, require two stable reads, lock the exact backup, reread it, emit structured evidence, then call the existing Goose provider. Always close the connection to release the advisory lock.

- [ ] **Step 4: Run tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestMigrationGate|TestIrreversibleMigrationRegistry|TestRunMigrations' -count=1 -v`

Expected: PASS.

- [ ] **Step 5: Commit the migration gate**

```bash
git add api/internal/db/migration_gate.go api/internal/db/migration_gate_test.go api/internal/db/migrate.go
git commit -m "feat: gate irreversible migrations on Railway backup"
```

### Task 3: Pre-deploy command and serve-mode schema guard

**Files:**
- Create: `api/cmd/api/migration_command.go`
- Create: `api/cmd/api/migration_command_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/railway.toml`

- [ ] **Step 1: Write RED command routing tests**

```go
func TestProcessCommandRoutesMigrateBeforeApplicationConfiguration(t *testing.T) {
    called := false
    exitCode := runProcess([]string{"api", "migrate"}, func(context.Context) error { called = true; return nil }, nil)
    if exitCode != 0 || !called { t.Fatalf("exit=%d called=%v", exitCode, called) }
}

func TestServeModeChecksSchemaAndNeverCallsMigration(t *testing.T) {
    schemaChecked, migrated := false, false
    exitCode := runProcess([]string{"api"}, func(context.Context) error { migrated = true; return nil }, func(context.Context) error { schemaChecked = true; return nil })
    if exitCode != 0 || !schemaChecked || migrated { t.Fatalf("exit=%d checked=%v migrated=%v", exitCode, schemaChecked, migrated) }
}

func TestRailwayConfigRunsMigrateAsPreDeploy(t *testing.T) { /* read ../../railway.toml and assert exact command */ }
```

- [ ] **Step 2: Run tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api -run 'TestProcessCommand|TestServeMode|TestRailwayConfig' -count=1 -v`

Expected: FAIL because routing helpers/config are absent.

- [ ] **Step 3: Implement early command routing and schema check**

Parse `os.Args` immediately after logger initialization. `migrate` requires only `DATABASE_URL` and backup variables, constructs `railwaybackup.New`, calls `db.RunMigrationsWithBackupGate`, logs evidence, and exits. Reject unknown subcommands. In serve mode remove `db.RunMigrations` and call `db.RequireCurrentSchema`, which reads the latest applied Goose version and compares it to the latest embedded version without writes.

Add to `api/railway.toml`:

```toml
[deploy]
preDeployCommand = ["./bin/api migrate"]
startCommand = "./bin/api"
```

- [ ] **Step 4: Run tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api ./internal/db ./internal/railwaybackup -count=1`

Expected: PASS.

- [ ] **Step 5: Commit command/config changes**

```bash
git add api/cmd/api/main.go api/cmd/api/migration_command.go api/cmd/api/migration_command_test.go api/railway.toml
git commit -m "feat: run migrations in Railway pre-deploy"
```

### Task 4: PostgreSQL transaction-level integration tests

**Files:**
- Create: `api/internal/db/migration_gate_postgres_integration_test.go`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Write RED integration tests**

Use `PUBLISHING_RESTRICTION_TEST_DATABASE_URL`, create a fresh schema/database namespace per test, execute real migrations through 123/124 as needed, and use a thread-safe fake Railway client:

```go
func TestMigrationGatePostgresConcurrentPreDeploysCreateOneBackup(t *testing.T) {
    requireIntegrationDatabase(t)
    // Seed failed rows at version 124, start N=4 gated runners, block the leader
    // inside Create, assert followers cannot call Create, release leader, then
    // assert final version 125 and exactly one backup.
}

func TestMigrationGatePostgresFailureBeforeVerificationLeavesDataAndVersion(t *testing.T) {
    // Return ambiguous list evidence; assert version remains 124 and failed row
    // still has the pre-125 retryable value.
}

func TestMigrationGatePostgresReplacementAfterOrphanCreatesFreshBackup(t *testing.T) {
    // First run locks then fails before Goose; second run creates a different
    // name/ID and reaches 125.
}
```

- [ ] **Step 2: Run integration tests and verify RED**

Run with an isolated local/CI PostgreSQL URL:

`cd api && PUBLISHING_RESTRICTION_TEST_DATABASE_URL=<isolated-url> GOCACHE=/tmp/unipost-go-build go test -tags=integration ./internal/db -run 'TestMigrationGatePostgres' -count=1 -v`

Expected: FAIL on the missing/incomplete integration seam. If the URL is absent, the test must fail, not skip.

- [ ] **Step 3: Add the minimal integration seam and CI invocation**

Adjust gate injection only as required for real transaction tests. Add the test names to the existing `api-postgres-integration` command in `.github/workflows/ci.yml`; keep the existing isolated `postgres:16-alpine` service and do not add testcontainers or any shared URL.

- [ ] **Step 4: Run integration tests and verify GREEN**

Run the same command as Step 2 with the isolated URL.

Expected: PASS with all named tests executed and zero skips.

- [ ] **Step 5: Commit integration coverage**

```bash
git add api/internal/db/migration_gate_postgres_integration_test.go .github/workflows/ci.yml
git commit -m "test: cover migration backup gate concurrency"
```

### Task 5: Full verification and delivery

**Files:**
- Modify only files needed to fix failures discovered by the required suites.

- [ ] **Step 1: Run focused Go tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/railwaybackup ./internal/db ./cmd/api -count=1
```

Expected: PASS, zero skips in required focused tests.

- [ ] **Step 2: Run the required isolated PostgreSQL integration suite**

Run the repository's isolated PostgreSQL service and:

```bash
cd api
PUBLISHING_RESTRICTION_TEST_DATABASE_URL=<isolated-url> GOCACHE=/tmp/unipost-go-build \
  go test -tags=integration ./internal/db ./internal/worker -count=1 -v
```

Expected: PASS, zero required skips.

- [ ] **Step 3: Run the complete API suite**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS.

- [ ] **Step 4: Run Dashboard contracts, build, and regression**

Run:

```bash
cd dashboard
npm run test:docs-ai
npm run test:seo
npm run build
npm run test:regression:dashboard
```

Expected: all commands PASS with no required skips.

- [ ] **Step 5: Request independent code review and address findings**

Use `superpowers:requesting-code-review` against the complete branch diff. For every Critical or Important finding, use `superpowers:receiving-code-review`, reproduce it, add a RED test, implement the minimal fix, and rerun the complete affected suite.

- [ ] **Step 6: Re-audit branch contents**

Run:

```bash
git status --short
git log --oneline 3a4c661c3e85c3c6e858cc8676793ca12191bb40..HEAD
git diff --name-status 3a4c661c3e85c3c6e858cc8676793ca12191bb40..HEAD
git diff --check 3a4c661c3e85c3c6e858cc8676793ca12191bb40..HEAD
```

Expected: clean status, only intended commits/files, no whitespace errors.

- [ ] **Step 7: Push only the owned branch and update Draft PR #271**

Push `codex/pr270-review-hardening` without touching its base branch. Confirm PR #271 remains Draft, head is this branch, and base is `codex/staging-tiktok-free-publishing-restriction`.

- [ ] **Step 8: Monitor exact-SHA checks to terminal success**

Monitor every required/triggered GitHub, Railway Preview, and Vercel check for the exact pushed SHA. Any failure, error, timeout, cancellation, skip, or missing result is a hard stop. Do not merge PR #271 or PR #270, deploy/promote staging, enable restrictions, send email, or access staging/production databases.
