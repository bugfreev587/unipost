# PR 270 Third-Round Hardening Design

## Goal

Resolve the Critical and Important findings from the independent full review of PR 270 before any staging promotion. The changes close the legacy-migrator race in the Railway backup gate, make publishing admission and persistence use one policy snapshot, prevent stale workers from overwriting a newer successful delivery, make ambiguous email outcomes terminal, fail closed when campaign delivery is not configured, correct draft quota accounting, and run the publishing-restriction frontend contracts in CI.

This work remains on the conversation-owned branch and worktree. It does not merge PR 270, deploy or promote staging, touch production, run a Railway database migration, enable the TikTok restriction, or send real email.

## Current review verdict

The exact PR 270 head passed its existing local and remote checks, but the independent full-diff review found one Critical migration race and six Important correctness or coverage gaps. Therefore the existing green checks do not authorize a staging merge. The branch remains blocked until the new behavior is implemented, verified on a replacement SHA, independently reviewed, and accepted by every required remote check.

## Options considered

### Option A: unify the migration lock and fix all findings in the stacked hardening branch

Use Goose's existing PostgreSQL session advisory lock as the single serialization boundary. The gated runner acquires that exact lock before preflight and holds it until migrations finish. All remaining findings receive focused fixes and behavior tests in the same stacked hardening branch.

This is the selected option. It closes the old-binary race without requiring an operational maintenance window, and the compatibility behavior can be proven with an isolated PostgreSQL race test.

### Option B: two-phase release

First release only the new pre-deploy architecture and removal of startup migrations, then release migrations 124 and 125 separately. This gives the cleanest operational separation but requires an additional release cycle and does not by itself repair the other application-level findings.

### Option C: quiesce all services and migrate in a manual maintenance window

Stop old API and worker instances before backup and migration. This can avoid the race operationally, but it relies on exact manual sequencing and downtime. It is less enforceable and less testable than a shared database lock, so it is rejected as the default.

## Selected design

### 1. Share Goose's migration lock across old and new binaries

The current backup gate takes a UniPost-specific advisory lock, while legacy startup binaries call `RunMigrations`, which takes Goose's different session lock. Those two leaders do not exclude one another. An old API or worker restart can therefore acquire Goose's lock and apply migration 124 or 125 while the new pre-deploy command is still creating its backup.

`RunMigrationsWithBackupGate` will instead reserve a dedicated SQL connection and acquire the same `lock.NewPostgresSessionLocker()` lock used by legacy `RunMigrations`. It acquires that lock before reading the Goose version or counting affected rows and holds it through backup verification and migration completion.

The migration implementation will be split into a small internal runner that accepts an existing `*sql.DB` and a flag or option controlling whether it acquires its own session lock:

- legacy `RunMigrations` opens the database and calls the runner with Goose locking enabled;
- the backup gate acquires the Goose lock explicitly, then calls the runner with nested locking disabled;
- concurrent new gated runners block on the same lock, recompute preflight after the leader finishes, and do not create duplicate backups;
- concurrent old binaries block on the same lock and cannot migrate before the new leader verifies its backup.

The former UniPost-specific lock is removed rather than retained in a second lock order. This avoids deadlock between old and new entry points and gives one documented migration-serialization primitive.

The gate remains fail closed. A lock acquisition, preflight, Railway identity, backup creation, evidence verification, backup lock, or migration failure returns nonzero and does not authorize deployment.

#### PostgreSQL compatibility race test

An isolated PostgreSQL integration test will start from the historical schema and affected rows, then:

1. start a gated runner and pause it during backup verification;
2. start the legacy `RunMigrations` entry point;
3. prove the legacy call remains blocked and that migrations 124 and 125 have not changed the affected rows;
4. release successful backup verification;
5. prove the gated runner applies the migrations;
6. prove the legacy runner then completes harmlessly against the current schema;
7. assert exactly one backup was created and the final schema/data state is correct.

The test uses only the CI-local PostgreSQL service. Missing isolated database configuration remains a test failure, never a skip or fallback to a shared database.

### 2. Carry one publishing-policy snapshot through persistence

The admission handler currently evaluates the restriction, creates or claims a parent post, and later lets fan-out logic read the restriction again. A toggle or read failure between those steps can persist a `publishing` parent with no corresponding delivery jobs.

Each publish execution will calculate one `blockedTargets` snapshot and pass it into all persistence helpers that create results and delivery jobs. Immediate publish, bulk publish, draft publish, and scheduled execution will use the snapshot produced by their own admission phase. Persistence helpers will not re-read policy after the parent row has been created or claimed.

Scheduled posts intentionally re-evaluate policy when the scheduler executes because admission and execution are separated in time. That scheduler evaluation becomes the single snapshot for its persistence operation. Delivery workers retain their just-in-time restriction check immediately before a provider call, because it protects against a restriction enabled after enqueue.

Behavior tests will use an evaluator that changes or fails if called a second time. They will prove that each admission/persistence operation evaluates once and that parent status, result rows, and delivery jobs all match the accepted snapshot.

### 3. Make restricted-delivery finalization lease-atomic

The current restricted path updates `social_post_results` before it performs the lease-conditional `post_delivery_jobs` transition. If the lease was lost, the job update returns no row but the stale worker has already overwritten the newer result.

Add one SQL statement whose first CTE conditionally transitions the delivery job using the existing job ID, state, lease owner, and last-attempt timestamp. Result status and structured restriction failure fields are updated only from the successful transition CTE. If the lease transition returns no row, the entire statement changes nothing and the handler treats it as a stale-worker no-op.

External failure recording, logs, parent-status refresh, and retention updates run only after the atomic statement reports success. They never run for a lost lease. No new database migration is required because the fix changes query behavior only.

A behavior test will model a stale restricted worker racing a newer successful worker and prove that the old lease cannot change the successful result or emit finalization side effects.

### 4. Treat ambiguous provider transport outcomes as terminal

An HTTP transport error does not prove that Loops rejected the request. A timeout or connection loss can happen after the provider accepted the email, so automatic retry can duplicate a customer message even when the same provider idempotency key is supplied.

The Loops client will return a typed error classification:

- configuration, serialization, request-construction, audit-link, and explicit non-2xx provider responses are definitive failures and keep the existing bounded retry behavior;
- any error returned by `http.Client.Do` is an ambiguous send outcome and is terminal for automatic recipient retry.

On an ambiguous outcome the audit attempt remains failed with wording that the provider outcome is unknown, while the campaign recipient becomes `status='failed'`, `retryable=FALSE`, with no next automatic attempt. This preserves an auditable manual-review path without inventing a new database state.

The email store interface receives an explicit terminal-failure operation rather than inferring terminality from an error string. A test transport will observe the outgoing request and then return an error, simulating provider acceptance followed by a lost response. The worker test will prove one network attempt and no retryable recipient state.

### 5. Gate campaign APIs and workers on complete delivery readiness

Campaign preview and confirmation currently know only about the preview secret, while the worker may lack the audited sender or transactional templates. That lets an administrator confirm a campaign that cannot be delivered.

Campaign delivery will use one immutable readiness value wired from the same startup configuration that constructs the audited Loops sender and template IDs. Readiness requires all of the following:

- Loops API key and audited sender;
- restriction-notice transactional template ID;
- recovery-notice transactional template ID;
- campaign preview secret;
- campaign store.

The campaign service will expose a stable `ErrCampaignNotConfigured`. Preview, confirmation, and failed-recipient retry check readiness before reading or mutating campaign state. The HTTP handler maps the error to `503 NOT_CONFIGURED` with no provider or credential detail.

The email worker checks the same readiness before claiming recipients. An unready worker returns a configuration error and claims zero rows. This is deliberately all-or-nothing: the administrator cannot confirm one campaign type while the shared worker is incapable of safely processing the complete campaign queue.

Unit tests cover preview, confirmation, retry, handler mapping, and worker zero-claim behavior. No test sends real email.

### 6. Exclude restricted draft targets from quota units

When publishing a draft, quota accounting currently counts all parsed targets after policy evaluation. It will instead count `allowedPublishingTargets(posts, blockedTargets)`, matching immediate and bulk behavior.

A mixed-platform boundary test will prove that a restricted Free-plan TikTok target contributes zero quota units while an allowed target still contributes one. Fully restricted behavior and existing disconnected-account filtering remain unchanged.

This correction applies to the publish action for a claimed draft. Scheduled-draft editing and reservation calculations remain unchanged unless their own execution path has a current policy snapshot; the persistence invariant is that quota is based on the targets accepted for the specific publish execution.

### 7. Run publishing-restriction frontend contracts in CI

Add a dedicated Dashboard script that executes the existing publishing-restriction customer, admin, shared helper, and post-result error contracts under Node 22. Add a named CI step before the Dashboard build so these files cannot exist without being executed.

The script will include:

- `src/lib/publishing-restrictions.test.ts`;
- `tests/admin-publishing-restrictions-source.test.mjs`;
- `tests/publishing-restrictions-customer-source.test.mjs`;
- `tests/post-result-errors.test.mts`.

If Node's TypeScript execution mode requires an explicit flag, the package script will declare it rather than relying on a developer-machine default. A CI contract test will assert that the dedicated script and workflow step remain present.

## TDD and verification strategy

Every behavior change begins with a focused failing test and recorded RED result, followed by the minimum implementation and a GREEN rerun. Generated sqlc code is regenerated from query sources; generated files are not hand-edited.

Focused verification includes:

- Goose legacy-versus-gated PostgreSQL lock race;
- existing migration 122-to-124-to-125 upgrade and email claim integration tests;
- one-snapshot admission/persistence behavior for immediate, bulk, draft, and scheduled execution surfaces touched by the signature change;
- stale-lease restricted finalization behavior;
- ambiguous email outcome and definitive-failure retry behavior;
- campaign readiness service, handler, and worker behavior;
- mixed draft quota behavior;
- dedicated frontend publishing-restriction contracts.

Completion verification includes, with zero required skips:

- `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`;
- the isolated PostgreSQL integration suite with the integration build tag;
- `go vet ./...` if retained by the repository's CI-equivalent checks;
- the dedicated Dashboard publishing-restriction contract command;
- existing related Dashboard contract suites;
- `npm run build` from `dashboard/`;
- `npm run test:regression:dashboard` with all required tests executed.

After local success, an independent read-only code reviewer will review the complete replacement diff, not only the newest commit. Any Critical or Important finding blocks push or integration until resolved. After a permitted push, every triggered GitHub, Railway Preview, Vercel Preview, and deployed-regression check must reach success on the exact head SHA. Failure, error, timeout, cancellation, skip, no result, or a result for another SHA is a hard stop.

## Database and release safety

This implementation does not execute migrations or create environment backups. Staging and production remain independent release gates:

- the first authorized staging deployment that would affect existing rows must create, verify, and lock a new staging PostgreSQL volume backup before Goose runs;
- a later production deployment must have its own production-scoped token, volume instance, identity verification, and newly created locked production backup;
- staging evidence never authorizes production and production configuration will not be added or tested without separate release authorization;
- a proven zero affected-row preflight can bypass backup only for that exact environment and migration attempt, with the zero counts logged.

No PR 270 merge or staging promotion is allowed until this design is implemented and all replacement-SHA gates pass. Production promotion remains out of scope.

## Non-goals

- Rewriting historical migration 124 or fabricating reversible retention data.
- Running, restoring, deleting, unlocking, or remounting a Railway backup.
- Connecting tests to shared dev, staging, or production databases.
- Adding testcontainers.
- Relying on provider idempotency as the only duplicate-email control.
- Removing the delivery worker's final policy check before provider dispatch.
- Enabling the TikTok restriction, sending real email, or merging PR 270.
