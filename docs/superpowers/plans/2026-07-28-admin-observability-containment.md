# Admin Observability Performance Containment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Admin Errors and Admin Logs fast and bounded without changing publishing, account connection, or any other customer business behavior.

**Architecture:** Keep existing business and log schemas intact. Admin list queries project metadata only, dedicated detail endpoints load large diagnostic fields on demand, outbound failure capture observes streamed request bodies through a bounded wrapper, and a concurrent log-owned index serves global Admin Logs ordering. All changed files and relations are restricted to the explicit observability allowlist below.

**Tech Stack:** Go 1.25, Chi, pgx/PostgreSQL, Goose, Next.js 16, React 19, TypeScript, Node test runner.

---

## Scope and safety allowlist

Only these files may change in this workstream:

- `api/internal/debugrt/debugrt.go`
- `api/internal/debugrt/debugrt_test.go`
- `api/internal/handler/admin.go`
- `api/internal/handler/admin_observability_test.go`
- `api/cmd/api/main.go`
- `api/cmd/api/admin_observability_routes_test.go`
- `api/internal/db/migrations/128_admin_logs_global_time_index.sql`
- `api/internal/db/admin_logs_index_migration_test.go`
- `api/internal/db/migrate_test.go`
- `api/internal/db/migration_gate_postgres_integration_test.go`
- `dashboard/src/lib/api.ts`
- `dashboard/src/app/admin/errors/page.tsx`
- `dashboard/tests/admin-observability-source.test.mjs`
- this plan and the approved PRD

The only database relation changed is a new index on `integration_logs`. No migration or write may target `social_posts`, `social_accounts`, `social_post_results`, `post_delivery_jobs`, outbox, billing, authentication, quota, receipt, or idempotency tables. Existing `social_post_results.debug_curl` rows remain untouched.

## File responsibility map

- `debugrt.go`: bounded, observational-only capture of failed outbound HTTP requests.
- `admin.go`: metadata-only list projections and one on-demand debug-detail query.
- `main.go`: Super Admin routing for the Admin Errors list and detail endpoint.
- migration 128: global `(ts DESC, id DESC)` index for `integration_logs` using non-blocking concurrent creation.
- `api.ts`: typed client for the dedicated failure debug endpoint.
- Admin Errors page: load debug text only after an operator opens one failure.
- contract tests: prove the SQL, routing, UI request sequence, and protected-table boundary.

### Task 1: Add the non-blocking Admin Logs index

**Files:**
- Create: `api/internal/db/migrations/128_admin_logs_global_time_index.sql`
- Create: `api/internal/db/admin_logs_index_migration_test.go`

- [x] **Step 1: Write the failing migration contract test**

```go
package db

import (
	"os"
	"strings"
	"testing"
)

func TestAdminLogsGlobalIndexMigrationIsConcurrentAndLogOnly(t *testing.T) {
	body, err := os.ReadFile("migrations/128_admin_logs_global_time_index.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := strings.ToLower(string(body))
	for _, required := range []string{
		"-- +goose no transaction",
		"create index concurrently if not exists idx_integration_logs_admin_ts_id",
		"on integration_logs (ts desc, id desc)",
		"drop index concurrently if exists idx_integration_logs_admin_ts_id",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, protected := range []string{"social_posts", "social_accounts", "social_post_results", "post_delivery_jobs"} {
		if strings.Contains(sql, protected) {
			t.Fatalf("migration touches protected relation %q", protected)
		}
	}
}
```

- [x] **Step 2: Run the test and verify the missing file failure**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/db -run TestAdminLogsGlobalIndexMigrationIsConcurrentAndLogOnly -count=1`

Expected: FAIL because migration 128 does not exist.

- [x] **Step 3: Add the concurrent migration**

```sql
-- +goose NO TRANSACTION
-- +goose Up

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_integration_logs_admin_ts_id
    ON integration_logs (ts DESC, id DESC);

-- +goose Down

DROP INDEX CONCURRENTLY IF EXISTS idx_integration_logs_admin_ts_id;
```

- [x] **Step 4: Run the focused and migration test suites**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/db -count=1`

Expected: PASS.

- [x] **Step 5: Commit the log-owned migration**

```bash
git add api/internal/db/migrations/128_admin_logs_global_time_index.sql api/internal/db/admin_logs_index_migration_test.go
git commit -m "perf: add global admin logs index"
```

### Task 2: Make Admin Logs list SQL metadata-only

**Files:**
- Modify: `api/internal/handler/admin.go`
- Create: `api/internal/handler/admin_observability_test.go`

- [x] **Step 1: Write failing projection tests**

```go
package handler

import (
	"strings"
	"testing"
)

func TestAdminLogsListSelectOmitsPayloadColumns(t *testing.T) {
	query := strings.ToLower(adminLogsSelect(false))
	if strings.Contains(query, "request_payload") || strings.Contains(query, "response_payload") {
		t.Fatalf("list query selects payload columns: %s", query)
	}
}

func TestAdminLogsDetailSelectIncludesPayloadColumns(t *testing.T) {
	query := strings.ToLower(adminLogsSelect(true))
	for _, column := range []string{"l.request_payload", "l.response_payload"} {
		if !strings.Contains(query, column) {
			t.Fatalf("detail query missing %s", column)
		}
	}
}
```

- [x] **Step 2: Run the tests and verify `adminLogsSelect` is undefined**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/handler -run 'TestAdminLogs(List|Detail)Select' -count=1`

Expected: FAIL to compile because `adminLogsSelect` does not exist.

- [x] **Step 3: Split the projection and scan targets**

Implement `adminLogsSelect(includePayloads bool)` so the shared metadata projection ends at `l.metadata`; append `l.request_payload` and `l.response_payload` only when `includePayloads` is true. Build the `row.Scan` destination slice the same way:

```go
func adminLogsSelect(includePayloads bool) string {
	query := adminLogsMetadataSelect
	if includePayloads {
		query += ",\n  l.request_payload,\n  l.response_payload"
	}
	return query + adminLogsFrom
}

destinations := []any{
	&out.ID, &out.WorkspaceID, &out.WorkspaceName, &out.OwnerEmail, &out.PlanID,
	&out.TS, &out.Level, &out.Status, &out.Category, &out.Action, &out.Source,
	&out.Message, &requestID, &traceID, &actorUserID, &actorAPIKeyID,
	&profileID, &socialAccountID, &postID, &platformPostID, &platform,
	&endpoint, &method, &httpStatusCode, &remoteStatusCode, &durationMs,
	&errorCode, &out.Metadata,
}
if includePayloads {
	destinations = append(destinations, &requestPayload, &responsePayload)
}
err := row.Scan(destinations...)
```

Use `adminLogsSelect(false)` in `ListLogs` and `adminLogsSelect(true)` in `GetLog`.

- [x] **Step 4: Run handler tests**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/handler -count=1`

Expected: PASS.

- [x] **Step 5: Commit the query containment**

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_observability_test.go
git commit -m "perf: keep admin log payloads out of list queries"
```

### Task 3: Move publishing debug text behind a Super Admin detail endpoint

**Files:**
- Modify: `api/internal/handler/admin.go`
- Modify: `api/internal/handler/admin_observability_test.go`
- Modify: `api/cmd/api/main.go`
- Create: `api/cmd/api/admin_observability_routes_test.go`

- [x] **Step 1: Add failing SQL and route contract tests**

Add a test asserting `adminPostFailuresListSQL()` contains `NULL::TEXT AS debug_curl` and contains no `spr.debug_curl`. Add a route-source test asserting both `/v1/admin/post-failures` and `/v1/admin/post-failures/{id}/debug` use `auth.RequireSuperAdmin`.

```go
func TestAdminPostFailuresListNeverReadsDebugCurl(t *testing.T) {
	query := strings.ToLower(adminPostFailuresListSQL())
	if strings.Contains(query, "spr.debug_curl") {
		t.Fatalf("list query reads debug TOAST: %s", query)
	}
	if strings.Count(query, "null::text as debug_curl") != 3 {
		t.Fatalf("all three list branches must project a null debug value")
	}
}
```

- [x] **Step 2: Run the focused tests and verify they fail**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/handler ./cmd/api -run 'Admin(PostFailures|Observability)' -count=1`

Expected: FAIL because the list SQL helper and detail route do not exist.

- [x] **Step 3: Extract list SQL and remove every debug column read**

Move the current post-failure query text into `adminPostFailuresListSQL()` and replace both `NULLIF(spr.debug_curl, '') AS debug_curl` projections with `NULL::TEXT AS debug_curl`. Keep the existing scan shape and every filter/order behavior unchanged.

- [x] **Step 4: Add the on-demand debug handler**

```go
type adminPostFailureDebugResponse struct {
	DebugCurl *string `json:"debug_curl"`
}

func (h *AdminHandler) GetPostFailureDebug(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid failure id")
		return
	}
	var out adminPostFailureDebugResponse
	err := h.pool.QueryRow(r.Context(), adminPostFailureDebugSQL, id).Scan(&out.DebugCurl)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Failure not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to load failure debug detail")
		return
	}
	writeSuccess(w, out)
}
```

The SQL resolves either a `post_failures.id` or `social_post_results.id`, returns at most one nullable `debug_curl`, and never mutates either relation.

- [x] **Step 5: Protect list and detail routes with the existing Super Admin middleware**

Register:

```go
r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Admin errors are restricted to super admins")).
	Get("/v1/admin/post-failures", adminHandler.ListPostFailures)
r.With(auth.RequireSuperAdmin(superAdminChecker, "FORBIDDEN", "Admin errors are restricted to super admins")).
	Get("/v1/admin/post-failures/{id}/debug", adminHandler.GetPostFailureDebug)
```

- [x] **Step 6: Run API tests and commit**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/handler ./cmd/api -count=1`

Expected: PASS.

```bash
git add api/internal/handler/admin.go api/internal/handler/admin_observability_test.go api/cmd/api/main.go api/cmd/api/admin_observability_routes_test.go
git commit -m "perf: load admin failure debug details on demand"
```

### Task 4: Bound debug capture without pre-reading publishing bodies

**Files:**
- Modify: `api/internal/debugrt/debugrt.go`
- Modify: `api/internal/debugrt/debugrt_test.go`

- [x] **Step 1: Write failing streaming and size tests**

Add tests that prove:

- the base transport begins before the first request-body read;
- a 2 MB JSON request reaches the server byte-for-byte but stores at most 32 KB plus a truncation marker;
- a binary or multipart body reaches the server byte-for-byte but contributes zero body bytes to the curl;
- the omission record includes content type, observed byte count, and SHA-256;
- no serialized recorder output exceeds 64 KB;
- at most eight failed requests are retained;
- successful response behavior remains unchanged.

```go
func TestBinaryRequestBodyIsForwardedButOmitted(t *testing.T) {
	payload := bytes.Repeat([]byte{0xab}, 2<<20)
	// Server reads the complete payload and returns 400.
	// Assert received bytes equal payload, serialized output excludes raw bytes,
	// and output contains "body omitted", "video/mp4", byte count, and SHA-256.
}
```

- [x] **Step 2: Run debug tests and verify at least the size test fails**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/debugrt -count=1`

Expected: FAIL because the current transport calls `io.ReadAll(req.Body)` and embeds the whole body.

- [x] **Step 3: Replace pre-read buffering with a bounded observational wrapper**

Implement an `io.ReadCloser` wrapper that forwards every byte unchanged while it:

- counts observed bytes;
- hashes observed bytes with SHA-256;
- stores at most 32 KB only for JSON, text, and form-urlencoded bodies;
- stores no bytes for multipart, image, audio, video, or unknown binary bodies;
- does not create or alter `GetBody`;
- does not read until the underlying transport reads.

Use eight entries and cap the final serialized diagnostic string at 64 KB. Preserve the current 8 KB response cap. Append explicit truncation/omission metadata instead of binary content.

- [x] **Step 4: Run debug and protected-flow regression tests**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/debugrt -count=1
GOCACHE=/tmp/unipost-log-observability-go-build go test ./internal/handler -run 'SocialPost|Connect|OAuth|Billing|Idempotency|Retry' -count=1
```

Expected: PASS. The second command is a scope safety gate, not evidence that business files changed.

- [x] **Step 5: Commit the bounded recorder**

```bash
git add api/internal/debugrt/debugrt.go api/internal/debugrt/debugrt_test.go
git commit -m "fix: bound publishing failure diagnostics"
```

### Task 5: Fetch Admin Errors debug detail only after selection

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/errors/page.tsx`
- Create: `dashboard/tests/admin-observability-source.test.mjs`

- [x] **Step 1: Write the failing source contract test**

```js
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("Admin Errors loads debug only from the selected detail endpoint", () => {
  const api = read("src/lib/api.ts");
  const page = read("src/app/admin/errors/page.tsx");
  assert.match(api, /getAdminPostFailureDebug/);
  assert.match(api, /post-failures\/\$\{encodeURIComponent\(id\)\}\/debug/);
  assert.match(page, /getAdminPostFailureDebug/);
  assert.match(page, /debugLoading/);
  assert.match(page, /requireSuperAdmin/);
  assert.doesNotMatch(page, /selectedFailure\.debug_curl/);
});
```

- [x] **Step 2: Run the source test and verify it fails**

Run: `cd dashboard && node --test tests/admin-observability-source.test.mjs`

Expected: FAIL because the detail client and loading state do not exist.

- [x] **Step 3: Add the typed detail client**

```ts
export interface AdminPostFailureDebugDetail {
  debug_curl?: string;
}

export async function getAdminPostFailureDebug(
  token: string,
  id: string,
): Promise<ApiResponse<AdminPostFailureDebugDetail>> {
  return request(`/v1/admin/post-failures/${encodeURIComponent(id)}/debug`, token);
}
```

- [x] **Step 4: Load detail in the drawer effect**

Add `debugCurl`, `debugLoading`, and `debugError` state. When selection changes, resolve `social_post_result_id ?? post_failure_id`; fetch only that ID, cancel stale effects, and clear detail on close. Render a loading state, a retryable error state, the fetched curl, or the existing empty message. Copy/raw JSON combines the selected metadata with the fetched detail without adding debug text to the list state.

Set `requireSuperAdmin` on the page's `AdminShell`.

- [x] **Step 5: Run frontend checks and commit**

Run:

```bash
cd dashboard
node --test tests/admin-observability-source.test.mjs
npm run build
```

Expected: source test PASS and Next.js build PASS.

```bash
git add dashboard/src/lib/api.ts dashboard/src/app/admin/errors/page.tsx dashboard/tests/admin-observability-source.test.mjs
git commit -m "perf: lazy-load admin failure diagnostics"
```

### Task 6: Full local safety and scope verification

**Files:**
- Modify only if a failure identifies an in-allowlist defect.

- [x] **Step 1: Audit changed files and relations**

Run:

```bash
git diff --name-only origin/dev...HEAD
git diff origin/dev...HEAD -- api/internal/db/migrations
```

Expected: every changed implementation file appears in the allowlist; the only new database object is `idx_integration_logs_admin_ts_id` on `integration_logs`.

- [x] **Step 2: Run full API tests**

Run: `cd api && GOCACHE=/tmp/unipost-log-observability-go-build go test ./...`

Expected: PASS with zero failed or skipped required packages.

- [x] **Step 3: Run Dashboard source, build, and regression checks**

Run:

```bash
cd dashboard
node --test tests/admin-observability-source.test.mjs
npm run build
npm run test:regression:dashboard
```

Expected: all commands PASS. A missing browser, skipped suite, timeout, or cancellation is a failure and blocks push.

- [x] **Step 4: Verify the exact safety invariants**

Run searches proving no implementation diff changes publishing, account connection, business persistence, or protected migrations:

```bash
git diff --exit-code origin/dev...HEAD -- api/internal/handler/social_posts.go api/internal/handler/social_post_queue.go api/internal/handler/connect_callback.go api/internal/handler/oauth.go api/internal/db/queries api/internal/db/models.go
git diff --check origin/dev...HEAD
```

Expected: both commands exit 0.

- [x] **Step 5: Commit any plan tracking update separately**

```bash
git add docs/superpowers/plans/2026-07-28-admin-observability-containment.md
git commit -m "docs: record admin observability implementation plan"
```

### Task 7: Preview Acceptance and development promotion

**Files:** None locally unless a verified failure requires an in-scope fix.

The first PR head exposed an in-scope migration-fixture defect: the version 123/124 PostgreSQL fixtures did not include `integration_logs`, even though migration 66 creates it, and schema-version assertions still stopped at 127. The replacement SHA may update only that test fixture and this plan before the complete gate is rerun.

- [ ] **Step 1: Push only `dev-log-storage-observability`**

Run: `git push -u origin dev-log-storage-observability`

Expected: push succeeds and reports the exact local head SHA.

- [ ] **Step 2: Open a Draft PR to `dev`**

The PR description must list every unique commit and file, repeat the protected-flow boundary, and state that no business table or customer flow changed.

- [ ] **Step 3: Wait for exact-SHA Preview gates**

Require successful local CI, GitHub checks, Railway PR Environment, Vercel Preview wired to that Railway API, deployed regression, and Codex browser acceptance. Verify:

- Admin Logs list response excludes payloads and cold-load plan uses the new index;
- Admin Log detail still loads payloads on click;
- Admin Errors list remains small and contains no `debug_curl`;
- selecting one error loads only its detail;
- a forced telemetry failure does not change a customer request result;
- publishing and account-connection smoke coverage remains successful.

- [ ] **Step 4: Re-audit and merge to `dev` only after every gate passes**

List exact commits and files again, mark the PR ready, merge, then wait for the persistent dev deployment.

- [ ] **Step 5: Perform real development acceptance**

Verify the exact merged SHA on `https://dev-api.unipost.dev` and `https://dev-app.unipost.dev`. Do not begin workstream 2 until this acceptance is recorded and successful.
