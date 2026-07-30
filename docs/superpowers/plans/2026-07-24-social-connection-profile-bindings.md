# Social Connection and Profile Bindings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow one verified physical social account to have stable, Profile-scoped account bindings in multiple Profiles without duplicating credentials, weakening managed-user Inbox isolation, or allowing duplicate physical publishing in one logical Post.

**Architecture:** Introduce `social_connections` as the Workspace and managed-user credential authority while retaining `social_accounts` as stable public bindings. Roll out with nullable `connection_id` and explicit legacy-column fallback, migrate only unambiguous identity groups, and keep unresolved groups on the legacy path. Route new and safely migrated connections through a transaction service, then enforce connection-aware publishing, queue serialization, Inbox scope, and Dashboard binding operations.

**Tech Stack:** PostgreSQL/Goose, sqlc/pgx v5, Go HTTP handlers and workers, Next.js/TypeScript, Go tests, Playwright regression.

---

## Delivery boundaries

This is one product feature but five dependent surfaces. Implement in the task order below; each task ends in a focused commit and leaves the branch testable. Do not remove legacy credential or ownership columns in this plan. Their removal requires a later migration after deployed dual-read evidence shows zero null bindings and zero legacy readers/writers.

The four accepted implementation notes are fixed as follows:

- OAuth may create a target Profile binding after verified callback; direct connect preserves its duplicate 409 and uses the explicit bind endpoint.
- Any read with `social_accounts.connection_id IS NULL` falls back to the existing `social_accounts` credential/owner columns.
- Bluesky uses its DID in `external_account_id` as canonical `provider_identity`.
- Binding version is checked transactionally at admission and again before dispatch; the remaining interval between the final database check and provider I/O is accepted and logged.

### Task 1: Add the compatible connection schema and classified backfill

**Files:**
- Create: `api/internal/db/migrations/135_social_connections_and_profile_bindings.sql`
- Create: `api/internal/db/social_connections_migration_contract_test.go`
- Modify: `api/internal/db/x_inbox_migration_fixture_test.go`

- [ ] **Step 1: Write the migration contract tests**

Create tests that load migration 121 and assert these exact contracts:

```go
func TestSocialConnectionsMigrationContract(t *testing.T) {
	body, err := os.ReadFile("migrations/135_social_connections_and_profile_bindings.sql")
	if err != nil { t.Fatal(err) }
	sql := compactSocialConnectionSQL(string(body))

	for _, want := range []string{
		"create table social_connections",
		"external_user_id text",
		"provider_identity text",
		"status text not null",
		"create table social_connection_migration_conflicts",
		"add column connection_id text",
		"add column binding_version bigint not null default 1",
		"metadata->>'instagram_webhook_user_id'",
		"platform <> 'instagram'",
		"external_account_id",
		"status <> 'migration_conflict'",
	} {
		if !strings.Contains(sql, want) { t.Errorf("migration missing %q", want) }
	}
}

func TestSocialConnectionsMigrationNeverMergesManagedOwners(t *testing.T) {
	body, _ := os.ReadFile("migrations/135_social_connections_and_profile_bindings.sql")
	sql := compactSocialConnectionSQL(string(body))
	if !strings.Contains(sql, "count(distinct external_user_id) filter") {
		t.Fatal("backfill must classify cross-managed-owner groups")
	}
	if !strings.Contains(sql, "social_connection_migration_conflicts") {
		t.Fatal("unsafe groups must be recorded, not merged")
	}
}
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestSocialConnectionsMigration'`

Expected: FAIL because migration 121 does not exist.

- [ ] **Step 3: Implement migration 121**

Create:

```sql
CREATE TABLE social_connections (
  id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
  workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
  platform TEXT NOT NULL,
  provider_identity TEXT,
  access_token TEXT NOT NULL,
  refresh_token TEXT,
  token_expires_at TIMESTAMPTZ,
  account_name TEXT,
  account_avatar_url TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  scope TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  status TEXT NOT NULL CHECK (status IN ('active','reconnect_required','disconnected','migration_conflict')),
  connection_type TEXT NOT NULL CHECK (connection_type IN ('byo','managed')),
  external_user_id TEXT,
  external_user_email TEXT,
  last_refreshed_at TIMESTAMPTZ,
  x_app_mode TEXT,
  connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  disconnected_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((connection_type = 'managed' AND external_user_id IS NOT NULL) OR
         (connection_type = 'byo' AND external_user_id IS NULL)),
  CHECK (status = 'migration_conflict' OR provider_identity IS NOT NULL)
);

CREATE UNIQUE INDEX social_connections_canonical_identity_unique_idx
ON social_connections (workspace_id, platform, provider_identity)
WHERE provider_identity IS NOT NULL AND status <> 'migration_conflict';

ALTER TABLE social_accounts
  ADD COLUMN connection_id TEXT REFERENCES social_connections(id),
  ADD COLUMN binding_version BIGINT NOT NULL DEFAULT 1,
  ADD COLUMN binding_status TEXT NOT NULL DEFAULT 'active'
    CHECK (binding_status IN ('active','unbound'));

CREATE UNIQUE INDEX social_accounts_profile_connection_unique_idx
ON social_accounts (profile_id, connection_id)
WHERE connection_id IS NOT NULL;
```

The same migration must build a temporary classified source CTE using Instagram webhook identity and other-platform `external_account_id`, insert every unsafe group into `social_connection_migration_conflicts`, insert one connection for safe groups, and update only their source account rows. The safe-group predicate requires one ownership class, one managed owner, compatible `x_app_mode`, and one canonical identity. Credential precedence is active, latest `last_refreshed_at`, latest `connected_at`, then account ID. Migration Down must refuse to drop evidence while the conflict table contains rows.

- [ ] **Step 4: Extend the disposable migration fixture through version 121**

Add a fixture test that applies migrations 1–121, seeds same-owner duplicates and cross-owner duplicates before 121, and asserts the first group shares one connection while the second group has conflict evidence and null `connection_id` values.

- [ ] **Step 5: Run migration tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db`

Expected: PASS.

Commit: `feat(db): add social connection migration foundation`

### Task 2: Generate connection queries and a dual-read account projection

**Files:**
- Create: `api/internal/db/queries/social_connections.sql`
- Modify: `api/internal/db/queries/social_accounts.sql`
- Modify generated: `api/internal/db/models.go`
- Create generated: `api/internal/db/social_connections.sql.go`
- Modify generated: `api/internal/db/social_accounts.sql.go`
- Create: `api/internal/db/social_connections_query_contract_test.go`

- [ ] **Step 1: Write failing query contract tests**

Assert that connection queries include `CreateSocialConnection`, `GetSocialConnectionForUpdate`, `FindCanonicalSocialConnectionForUpdate`, `RefreshSocialConnection`, `DisconnectSocialConnection`, `CreateOrReactivateSocialAccountBinding`, and `GetResolvedSocialAccountByIDAndWorkspace`.

The resolved query must project:

```sql
sa.connection_id,
sa.binding_version,
sa.binding_status,
COALESCE(sc.access_token, sa.access_token) AS resolved_access_token,
COALESCE(sc.refresh_token, sa.refresh_token) AS resolved_refresh_token,
COALESCE(sc.external_user_id, sa.external_user_id) AS resolved_external_user_id
```

- [ ] **Step 2: Run and observe the missing-query failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestSocialConnectionQueries'`

Expected: FAIL with missing query names.

- [ ] **Step 3: Add the SQL queries**

Use `LEFT JOIN social_connections sc ON sc.id = sa.connection_id` for dual-read projection. `FindCanonicalSocialConnectionForUpdate` must match Workspace, platform, canonical identity across active/reconnect/disconnected states and end with `FOR UPDATE OF sc`. `CreateOrReactivateSocialAccountBinding` must use:

```sql
INSERT INTO social_accounts (
  profile_id, platform, access_token, refresh_token, token_expires_at,
  external_account_id, account_name, account_avatar_url, metadata, scope,
  status, connection_type, connect_session_id, external_user_id,
  external_user_email, last_refreshed_at, x_app_mode, connection_id,
  binding_version, binding_status
)
VALUES (
  @profile_id, @platform, @legacy_access_token, @legacy_refresh_token,
  @token_expires_at, @external_account_id, @account_name,
  @account_avatar_url, @metadata, @scope, 'active', @connection_type,
  @connect_session_id, @external_user_id, @external_user_email,
  @last_refreshed_at, @x_app_mode, @connection_id, 1, 'active'
)
ON CONFLICT (profile_id, connection_id) WHERE connection_id IS NOT NULL
DO UPDATE SET
  binding_status = 'active',
  binding_version = social_accounts.binding_version + 1,
  disconnected_at = NULL,
  status = 'active'
RETURNING *;
```

During dual-write, populate the legacy credential and ownership columns from the connection so unchanged read paths remain compatible.

- [ ] **Step 4: Generate sqlc output and rerun tests**

Run: `cd api && sqlc generate`

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db`

Expected: PASS and generated `SocialConnection` plus resolved row types.

- [ ] **Step 5: Commit**

Commit: `feat(db): add connection and binding queries`

### Task 3: Implement canonical identity and transactional connection reuse

**Files:**
- Create: `api/internal/socialconnections/identity.go`
- Create: `api/internal/socialconnections/identity_test.go`
- Create: `api/internal/socialconnections/store.go`
- Create: `api/internal/socialconnections/store_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write canonical identity tests**

```go
func TestProviderIdentity(t *testing.T) {
	tests := []struct{ platform, external, want string; metadata map[string]any }{
		{platform: "instagram", external: "app-scoped", metadata: map[string]any{"instagram_webhook_user_id": "professional"}, want: "professional"},
		{platform: "bluesky", external: "did:plc:abc", want: "did:plc:abc"},
		{platform: "twitter", external: "42", want: "42"},
	}
	for _, tt := range tests {
		got, err := ProviderIdentity(tt.platform, tt.external, tt.metadata)
		if err != nil || got != tt.want { t.Fatalf("got (%q,%v), want %q", got, err, tt.want) }
	}
}
```

Also test that Instagram without webhook identity fails closed.

- [ ] **Step 2: Run identity tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnections`

Expected: FAIL because the package is absent.

- [ ] **Step 3: Implement the identity resolver and store API**

Define:

```go
type Ownership struct {
	ConnectionType    string
	ExternalUserID    string
	ExternalUserEmail string
}

type CredentialInput struct {
	WorkspaceID, ProfileID, Platform, ProviderIdentity string
	AccessToken, RefreshToken, AccountName, AvatarURL   string
	Scopes []string
	Metadata []byte
	TokenExpiresAt time.Time
	XAppMode string
	Ownership Ownership
}

type SaveMode int
const (
	SaveDirectCreate SaveMode = iota
	SaveOAuthReuse
	SaveManagedReuse
)

type Store interface {
	SaveVerified(ctx context.Context, mode SaveMode, input CredentialInput) (db.SocialAccount, error)
	BindExisting(ctx context.Context, workspaceID, sourceAccountID, targetProfileID, selectedExternalUserID string) (db.SocialAccount, error)
	Unbind(ctx context.Context, workspaceID, accountID string) error
	Disconnect(ctx context.Context, workspaceID, accountID string) ([]db.SocialAccount, error)
}
```

The Postgres implementation begins a transaction, acquires the current length-prefixed provider-identity advisory lock, loads the canonical connection including disconnected rows, enforces managed ownership, refreshes the connection in place, and creates/reactivates the target binding. `SaveDirectCreate` returns `ErrAlreadyConnected` when a canonical connection exists. `BindExisting` resolves the source binding inside the authenticated Workspace and never accepts `connection_id` from the caller.

- [ ] **Step 4: Add transaction tests for same-owner reuse and cross-owner rejection**

Cover new connection, disconnected reconnect with stable ID, same managed owner in another Profile, different managed owner, owner/BYO mismatch, concurrent bind idempotency, and null-connection legacy fallback.

- [ ] **Step 5: Run tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/socialconnections`

Expected: PASS.

Commit: `feat(api): add social connection transaction store`

### Task 4: Wire direct, OAuth, managed Connect, and explicit binding

**Files:**
- Modify: `api/internal/handler/social_accounts.go`
- Modify: `api/internal/handler/oauth.go`
- Modify: `api/internal/handler/oauth_facebook.go`
- Modify: `api/internal/handler/connect_callback.go`
- Modify: `api/internal/handler/connect_bluesky.go`
- Modify: `api/internal/connectownership/store.go`
- Modify: `api/internal/connectownership/store_test.go`
- Modify: `api/cmd/api/main.go`
- Create: `api/internal/handler/social_account_bindings_test.go`
- Modify: `api/internal/handler/oauth_test.go`
- Modify: `api/internal/handler/connect_sessions_test.go`

- [ ] **Step 1: Write handler tests for the accepted flow asymmetry**

Test these outcomes:

```text
direct duplicate        -> 409 ACCOUNT_ALREADY_CONNECTED, no new binding
OAuth same Profile      -> refresh connection, reactivate stable binding
OAuth another Profile   -> refresh connection, create target binding
managed same owner      -> create/reactivate target binding
managed different owner -> existing 409 HTML ownership error
explicit bind           -> 201/200 with stable public account ID
```

- [ ] **Step 2: Run the focused handler tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test(SocialAccountBinding|OAuth.*Binding|Connect.*Binding)'`

Expected: FAIL because handlers still write directly to `social_accounts`.

- [ ] **Step 3: Inject the connection store and replace save decisions**

Add `connections socialconnections.Store` to the account, OAuth, and managed Connect handlers. Replace direct `CreateSocialAccount`/`ReactivateSocialAccount` calls at successful verified-identity boundaries with `SaveVerified`. Keep free-plan and provider verification checks before the store call.

Update managed ownership decision so `profile_mismatch` plus the same nonempty `external_user_id` returns a reuse decision; preserve internal conflict classes and the external 409 HTML response for all other conflicts.

- [ ] **Step 4: Register the explicit binding route**

Add:

```go
r.Post("/v1/accounts/{id}/bindings", socialAccountHandler.Bind)
r.Post("/v1/profiles/{profileID}/accounts/{accountID}/bindings", socialAccountHandler.Bind)
```

The body is `{ "profile_id": "pr_target" }` for the Workspace route; the nested route derives the target from the URL. Return the existing SocialAccount response plus `shared_connection` and caller-filtered `bound_profile_ids`. Do not expose `connection_id`.

- [ ] **Step 5: Run focused and package tests, then commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/connectownership ./internal/handler`

Expected: PASS.

Commit: `feat(api): support profile bindings for shared connections`

### Task 5: Separate unbind, physical disconnect, and reconnect lifecycle

**Files:**
- Modify: `api/internal/handler/social_accounts.go`
- Modify: `api/internal/db/queries/social_connections.sql`
- Modify: `api/internal/db/queries/social_accounts.sql`
- Modify generated sqlc files
- Create: `api/internal/handler/social_account_lifecycle_test.go`
- Modify: `api/internal/db/x_inbox_delivery_cleanup_migration_test.go`

- [ ] **Step 1: Write lifecycle tests**

Assert that unbinding increments `binding_version` and leaves the connection active, physical disconnect marks the connection and every binding unavailable, and verified reconnect reuses the same connection and public binding IDs. Add a contract assertion that the X Inbox `BEFORE DELETE ON social_accounts` cleanup route remains installed and credential material is captured before orphan cleanup.

- [ ] **Step 2: Run tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db -run 'Test(SocialAccountLifecycle|.*Cleanup.*Binding)'`

Expected: FAIL because only physical account deletion semantics exist.

- [ ] **Step 3: Implement lifecycle routes and state transitions**

Keep existing `DELETE /v1/accounts/{id}` as physical disconnect for compatibility. Add `DELETE /v1/accounts/{id}/binding`, implemented as a conditional update:

```sql
UPDATE social_accounts
SET binding_status = 'unbound',
    binding_version = binding_version + 1,
    status = 'disconnected',
    disconnected_at = NOW()
WHERE id = @id AND connection_id = @connection_id
RETURNING *;
```

Physical disconnect locks the connection, revokes once, updates the connection in place, marks all bindings unavailable, and emits one event listing affected public account/Profile IDs. Reconnect updates the same connection and reactivates only the requested binding unless the UI explicitly chooses more.

- [ ] **Step 4: Generate queries, run tests, and commit**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler`

Expected: PASS.

Commit: `feat(api): separate binding and connection lifecycle`

### Task 6: Add atomic duplicate-connection Post validation

**Files:**
- Modify: `api/internal/platform/validate.go`
- Modify: `api/internal/platform/validate_test.go`
- Modify: `api/internal/handler/social_posts_validate.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_bulk.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/response.go`
- Modify: `api/internal/handler/social_posts_validate_test.go`
- Modify: `api/internal/handler/social_posts_bulk_test.go`
- Modify: `api/internal/handler/social_posts_drafts_test.go`

- [ ] **Step 1: Write pure validator tests**

Extend `ValidateAccount`:

```go
type ValidateAccount struct {
	Platform       string
	Disconnected   bool
	ConnectionType string
	AppMode        string
	ConnectionID   string
	ProfileID      string
}
```

Test one binding, repeated same binding thread, two sibling bindings with equal payloads, different captions, three siblings, deterministic ordering, and legacy accounts with empty `ConnectionID` (group by account ID so unrelated legacy rows are not collapsed).

- [ ] **Step 2: Run validator tests and observe failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/platform -run 'TestValidate.*DuplicateSocialConnection'`

Expected: FAIL because the code and fields are absent.

- [ ] **Step 3: Implement the pure grouping rule**

Add `CodeDuplicateSocialConnection = "duplicate_social_connection"`. Group distinct account IDs by nonempty connection ID, preserve repeated same-account thread entries, and emit one issue per conflicting input index. Return conflicts sorted by platform, sorted account IDs, and input index.

- [ ] **Step 4: Add the dedicated HTTP 422 mapping before all side effects**

Load `connection_id` and Profile ID into the account map. Add `DUPLICATE_SOCIAL_CONNECTION` to `normalizedErrorCodeMap`. Before draft persistence, generic fatal filtering, quota calculation, result insertion, or bulk execution, detect the first conflict group and return:

```go
ErrorResponse{Error: ErrorBody{
	Code: "DUPLICATE_SOCIAL_CONNECTION",
	NormalizedCode: "duplicate_social_connection",
	Message: "The same physical social connection is selected through multiple profiles. Choose one account binding.",
	Issues: conflict.Issues,
	Details: conflict.PublicDetails(),
}}
```

Bulk rejects only the conflicting logical item and leaves independent items eligible. Draft creation must reject this new issue even though unrelated validation remains advisory.

- [ ] **Step 5: Run all publish tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'Test.*(Validate|Bulk|Draft|DuplicateSocialConnection)'`

Expected: PASS with zero-side-effect assertions.

Commit: `feat(api): reject duplicate physical publish targets`

### Task 7: Serialize and verify delivery by physical connection

**Files:**
- Create: `api/internal/db/migrations/136_delivery_job_connection_snapshot.sql`
- Modify: `api/internal/db/queries/post_delivery_jobs.sql`
- Modify generated: `api/internal/db/post_delivery_jobs.sql.go`
- Modify generated: `api/internal/db/models.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/worker/post_delivery.go`
- Modify: `api/internal/db/post_delivery_jobs_contract_test.go`
- Modify: `api/internal/worker/post_delivery_worker_test.go`

- [ ] **Step 1: Write migration/query/worker tests**

Require `connection_id TEXT` and `binding_version BIGINT` snapshots on each job. Claim queries must serialize active work by `COALESCE(connection_id, social_account_id)`, lock the current binding, and return only jobs whose binding remains active with the same connection/version. Worker tests must prove that unbind/rebind after enqueue fails before the adapter call.

- [ ] **Step 2: Run focused tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/worker -run 'Test.*(ConnectionSnapshot|BindingVersion|PhysicalConnection)'`

Expected: FAIL because job snapshots and checks are absent.

- [ ] **Step 3: Implement migration, enqueue snapshot, and claim checks**

Backfill existing jobs from their binding where available; leave null for legacy rows and use `social_account_id` fallback. Add a final `ValidateDeliveryBindingSnapshot` query immediately before provider dispatch and a structured `binding_version_mismatch` failure path.

- [ ] **Step 4: Generate queries, run tests, and commit**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler ./internal/worker`

Expected: PASS.

Commit: `feat(worker): serialize delivery by social connection`

### Task 8: Move Inbox, health, refresh, and account metrics authority to connections

**Files:**
- Modify: `api/internal/db/queries/inbox.sql`
- Modify: `api/internal/db/queries/x_inbox.sql`
- Modify: `api/internal/db/queries/social_accounts.sql`
- Modify generated sqlc files
- Modify: `api/internal/db/inbox_tenant_isolation_contract_test.go`
- Modify: `api/internal/handler/inbox.go`
- Modify: `api/internal/handler/social_account_health.go`
- Modify: `api/internal/handler/social_account_metrics.go`
- Modify: `api/internal/worker/inbox_sync.go`
- Modify: `api/internal/worker/analytics_refresh.go`
- Modify: `api/internal/handler/inbox_test.go`
- Create: `api/internal/handler/social_account_health_test.go`
- Modify: `api/internal/handler/social_account_metrics_test.go`
- Modify: `api/internal/worker/inbox_sync_test.go`
- Modify: `api/internal/worker/analytics_refresh_test.go`

- [ ] **Step 1: Rewrite the tenant-isolation contract tests first**

Managed predicates must resolve ownership as:

```sql
AND (
  sqlc.arg('workspace_scope')::BOOLEAN
  OR (
    COALESCE(sc.connection_type, sa.connection_type) = 'managed'
    AND COALESCE(sc.external_user_id, sa.external_user_id) = sqlc.arg('external_user_id')::TEXT
  )
)
```

Every query still includes both stored and Profile-derived Workspace predicates. Add tests that two bindings to one connection do not duplicate Inbox rows and that `bound_profile_ids` excludes Profiles outside the request's managed scope.

- [ ] **Step 2: Run Inbox contract tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/handler -run 'TestInbox.*(Connection|Tenant|ManagedScope)'`

Expected: FAIL because queries still authorize through binding ownership only.

- [ ] **Step 3: Update connection-scoped reads and workers**

Add `LEFT JOIN social_connections sc ON sc.id = sa.connection_id` to every Inbox ownership query and use explicit `COALESCE` only for null-connection dual-read fallback. Token refresh, health, provider account metrics, webhook/rate-limit identity, Inbox sync enumeration, and analytics refresh must load the resolved connection credential projection. Post analytics remain keyed through `social_post_results.social_account_id`.

- [ ] **Step 4: Generate, run security suites, and commit**

Run: `cd api && sqlc generate && GOCACHE=/tmp/unipost-go-build go test ./internal/db ./internal/inboxaccess ./internal/handler ./internal/worker ./internal/xinbox`

Expected: PASS, including all migration 119 tenant-isolation tests.

Commit: `feat(api): enforce connection-scoped shared state`

### Task 9: Add Dashboard binding controls, documentation, and complete acceptance

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Create: `dashboard/src/components/accounts/profile-bindings.tsx`
- Create: `dashboard/tests/social-account-bindings-source.test.mjs`
- Modify: `dashboard/src/app/docs/api/accounts/list/content.tsx`
- Modify: `dashboard/src/app/docs/api/accounts/connect/page.tsx`
- Create: `dashboard/src/app/docs/api/accounts/bind/page.tsx`
- Modify: `dashboard/src/app/docs/api/page.tsx`
- Modify: `dashboard/tests/dashboard-regression.spec.ts`

- [ ] **Step 1: Write component/API regression tests**

Test caller-visible `bound_profile_ids`, bind/unbind actions, shared-state labels, physical-disconnect confirmation listing all affected Profiles, and composer sibling-disable behavior. The UI must not render `connection_id`.

- [ ] **Step 2: Run frontend tests and observe failure**

Run: `cd dashboard && node --test tests/social-account-bindings-source.test.mjs`

Expected: FAIL because the component and API methods are absent. Follow the
existing source-contract test style and assert the binding endpoint, filtered
Profile rendering, distinct unbind/disconnect labels, and absence of rendered
`connection_id`.

- [ ] **Step 3: Implement API types and binding UI**

Extend `SocialAccount` with:

```ts
shared_connection?: boolean;
bound_profile_ids?: string[];
```

Add `bindSocialAccount(token, accountId, profileId)` and `unbindSocialAccount(token, accountId)`. The bindings component uses Profile names from `listProfiles`, disables already-bound targets, and distinguishes “Remove from this Profile” from “Disconnect physical account.”

- [ ] **Step 4: Update public docs and regression coverage**

Document that publish requests still submit only binding `account_id`; duplicate sibling bindings return the exact 422 envelope. Add a deployed regression case that binds one disposable connection to two Profiles, confirms distinct account IDs, verifies separate requests, and verifies atomic duplicate rejection.

- [ ] **Step 5: Run complete local validation**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd dashboard && npm run build
cd dashboard && npm run test:regression:dashboard
```

Expected: every command exits 0. Any skipped, timed-out, cancelled, or no-result suite is a failure.

- [ ] **Step 6: Audit, commit, and publish through Preview Acceptance**

Commit: `feat: share social connections across profile bindings`

Before push, list every commit and changed file unique to `origin/staging`. Push
only `codex/social-account-profile-bindings`, open a new Draft PR to `staging`,
and confirm its exact head SHA. PR #251 remains the accepted PRD-only review and
must not be repurposed or merged. Wait for API tests, Dashboard build, Railway PR
API, Vercel Preview, deployed regression, and Codex browser acceptance. Do not
merge until the user separately authorizes the next workflow step and every
required gate succeeds.

## Plan self-review checklist

- [x] Every PRD blocking item A/B/C/D maps to Tasks 1, 3, 5, and 8.
- [x] Direct/OAuth/managed behavior is explicitly different and tested.
- [x] Instagram, Bluesky, legacy-null, and cross-managed-owner identity cases are explicit.
- [x] Publish validation precedes draft persistence, quota, credits, rows, jobs, and provider I/O.
- [x] Bulk atomicity is per logical item.
- [x] Binding version is defined in schema, job snapshot, claim, and final dispatch check.
- [x] Inbox authorization remains `(workspace_id, external_user_id)` and does not derive visibility from Profile binding membership.
- [x] No destructive legacy-column removal is included in this rollout.
