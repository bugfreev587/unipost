# PostgreSQL Migration Advisory Lock Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent multiple API instances from running the same PostgreSQL Goose migrations concurrently.

**Architecture:** Replace the legacy global `goose.Up` call with a Goose provider configured with its official PostgreSQL session advisory locker. The lock is held on a dedicated database session while the provider re-reads migration state and applies pending migrations.

**Tech Stack:** Go 1.25, PostgreSQL, `database/sql`, Goose v3 provider API and `goose/lock`.

---

### Task 0: Record the approved design

**Files:**
- Create: `docs/superpowers/specs/2026-07-16-migration-advisory-lock-design.md`
- Create: `docs/superpowers/plans/2026-07-16-migration-advisory-lock.md`

- [ ] **Step 1: Commit the approved design and plan**

```bash
git add \
  docs/superpowers/specs/2026-07-16-migration-advisory-lock-design.md \
  docs/superpowers/plans/2026-07-16-migration-advisory-lock.md
git commit -m "docs: design migration advisory lock"
```

### Task 1: Lock the migration runner

**Files:**
- Modify: `api/internal/db/migrate.go`
- Modify: `api/internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing advisory-lock contract test**

Add a test that reads `migrate.go` and requires the migration runner to use:

```go
func TestRunMigrationsUsesPostgresSessionLocker(t *testing.T) {
	source, err := os.ReadFile("migrate.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, want := range []string{
		"lock.NewPostgresSessionLocker()",
		"goose.NewProvider(",
		"goose.DialectPostgres",
		"goose.WithSessionLocker(sessionLocker)",
		"provider.Up(context.Background())",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("migration runner missing %q", want)
		}
	}
	if strings.Contains(text, "goose.Up(") {
		t.Fatal("migration runner must not use unlocked legacy goose.Up")
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run TestRunMigrationsUsesPostgresSessionLocker -count=1
```

Expected: FAIL because `migrate.go` does not yet call
`lock.NewPostgresSessionLocker`.

- [ ] **Step 3: Implement the minimal Goose provider lock**

Update `RunMigrations` to use the embedded migration sub-filesystem and official
session locker:

```go
func RunMigrations(databaseURL string) error {
	database, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("failed to open database for migrations: %w", err)
	}
	defer database.Close()

	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("failed to open embedded migrations: %w", err)
	}
	sessionLocker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("failed to create migration session locker: %w", err)
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		database,
		migrationFS,
		goose.WithSessionLocker(sessionLocker),
	)
	if err != nil {
		return fmt.Errorf("failed to create migration provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database migrations completed")
	return nil
}
```

Add imports for `context`, `io/fs`, and
`github.com/pressly/goose/v3/lock`. Remove the legacy global Goose filesystem
and dialect setup.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestRunMigrationsUsesPostgresSessionLocker|TestEmbeddedMigrationVersionsAreUnique' -count=1
```

Expected: PASS.

### Task 2: Verify concurrent callers against PostgreSQL

**Files:**
- Modify: `api/internal/db/migrate_test.go`

- [ ] **Step 1: Extend the disposable database test with concurrent callers**

In `TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose`, start two callers
behind the same channel and collect both errors:

```go
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			errs <- RunMigrations(databaseURL)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent RunMigrations: %v", err)
		}
	}
```

Keep the existing final Goose version assertion so the test requires both
callers to succeed and the schema to reach version 116 exactly once.

- [ ] **Step 2: Run the real PostgreSQL test**

Start a disposable PostgreSQL 16 container, wait for it to accept connections,
run the integration test, and remove it:

```bash
docker run --rm -d \
  --name unipost-migration-lock-test \
  -e POSTGRES_PASSWORD=postgres \
  -p 127.0.0.1::5432 \
  postgres:16
port=$(docker port unipost-migration-lock-test 5432/tcp | sed 's/.*://')
until pg_isready -h 127.0.0.1 -p "$port" -U postgres; do sleep 1; done
cd api
GOOSE_MIGRATION_TEST_DATABASE_URL="postgresql://postgres:postgres@127.0.0.1:${port}/postgres?sslmode=disable" \
  GOCACHE=/tmp/unipost-go-build \
  go test ./internal/db -run TestRunMigrationsAppliesAllEmbeddedMigrationsWithGoose -count=1 -v
docker rm -f unipost-migration-lock-test
```

Expected: PASS; both concurrent callers return nil and the final version is
116.

- [ ] **Step 3: Run all backend validation**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
GOCACHE=/tmp/unipost-go-build go vet ./...
```

Expected: both commands pass.

### Task 3: Commit and run the hotfix release flow

**Files:**
- Commit the implementation and tests above.

- [ ] **Step 1: Commit the focused hotfix**

```bash
git add \
  api/internal/db/migrate.go \
  api/internal/db/migrate_test.go
git commit -m "fix: serialize database migrations"
```

- [ ] **Step 2: Merge to staging and verify**

Merge `hotfix-migration-advisory-lock` into local `staging`, rerun backend
validation, push `origin/staging`, and wait for all checks and deployments.
Verify `https://staging-api.unipost.dev/health` and confirm the new API
deployment contains no `failed to run migrations` error.

- [ ] **Step 3: Promote to production and verify**

Create and merge the `staging` → `main` production PR after checks pass. Wait
for GitHub, Vercel, Railway API, worker, and MCP deployments. Verify:

```text
https://api.unipost.dev/health returns status ok
Goose version 116 is applied
15/15 migration-116 indexes are valid and ready
No production API log contains failed to run migrations for the new deployment
```

- [ ] **Step 4: Sync the hotfix to dev**

Merge or cherry-pick the hotfix into local `dev`, rerun backend validation,
push `origin/dev`, wait for the development deployment, and verify
`https://dev-api.unipost.dev/health` plus migration startup logs.
