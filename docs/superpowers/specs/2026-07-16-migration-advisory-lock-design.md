# PostgreSQL Migration Advisory Lock Design

## Problem

Every API instance calls `db.RunMigrations` during startup. During the production
rollout of migration 116, two instances entered Goose at the same time. Both
started the same `DROP INDEX CONCURRENTLY` / `CREATE INDEX CONCURRENTLY`
sequence, PostgreSQL detected a lock cycle, and one instance exited with
`SQLSTATE 40P01`.

The other instance completed migration 116, so production recovered, but future
multi-instance rollouts can hit the same class of startup race.

## Scope

Serialize Goose migration execution across API processes that point at the same
PostgreSQL database. Do not change migration 116, application startup order,
Railway topology, business behavior, or any Team Plan feature.

## Design

Replace the legacy `goose.Up` call with Goose's provider API:

1. Open the existing PostgreSQL `database/sql` pool.
2. Create Goose's official `lock.NewPostgresSessionLocker`.
3. Create a PostgreSQL Goose provider over the embedded migration filesystem,
   configured with `goose.WithSessionLocker`.
4. Call `provider.Up(context.Background())`.
5. Keep the existing `database migrations completed` application log.

The locker uses PostgreSQL's session-level advisory lock. A rollout instance
that acquires the lock runs pending migrations. Other instances wait. After
acquiring the lock, Goose reads the migration table again; if another instance
already completed the migration, the waiting instance applies nothing and
continues startup.

Use Goose's default lock ID and bounded retry behavior. This keeps the change
small and avoids introducing a UniPost-specific locking protocol.

## Error Handling

Startup remains fail-closed:

- Failure to create the embedded migration filesystem returns an error.
- Failure to create the session locker returns an error.
- Failure to create the Goose provider returns an error.
- Lock timeout, migration failure, or unlock failure returns an error.
- `main` continues to log the failure and exit rather than serving against an
  unknown schema.

## Tests

Add a regression contract test that fails unless `RunMigrations` uses the
PostgreSQL session locker and Goose provider API. Extend the disposable
PostgreSQL migration integration test to start two migration callers
concurrently and require both to succeed with schema version 116.

Run:

- Focused migration tests.
- Real disposable-PostgreSQL concurrent migration test.
- `go test ./...`.
- `go vet ./...`.

## Release

Follow the repository hotfix flow:

`hotfix-migration-advisory-lock` → `staging` → `main` → sync to `dev`.

At every environment, monitor GitHub, Railway, and Vercel checks. In production,
confirm both API rollout instances start without `failed to run migrations`,
the Goose version remains 116, all 15 migration-116 indexes remain valid, and
the API health endpoint stays healthy.
