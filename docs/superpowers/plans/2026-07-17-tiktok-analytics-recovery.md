# TikTok Analytics Data Integrity and Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve TikTok public video IDs exactly, classify analytics failures without false reconnect messages, render independent Dashboard sections safely, and provide an idempotent historical recovery runbook.

**Architecture:** Keep the existing `AnalyticsAdapter` contract and fix precision at the TikTok provider boundary with `json.Decoder.UseNumber`. Add a typed TikTok analytics error carrying a stable reason and operation, translate it into the existing API error envelope, and use request generations plus independent settled resources in the Dashboard. Historical rows remain in place and are marked due through a checked-in psql runbook that changes only `post_analytics.fetched_at`.

**Tech Stack:** Go 1.x, `net/http`, `encoding/json`, PostgreSQL/psql, React 19, Next.js 16 App Router, TypeScript, Node test runner, Playwright.

---

## File Map

- Create `api/internal/platform/tiktok_analytics_error.go`: typed reasons and provider-response classification.
- Modify `api/internal/platform/tiktok.go`: exact publish-status decoding and unavailable-result behavior.
- Modify `api/internal/platform/tiktok_test.go`: provider-boundary regression tests.
- Create `api/internal/handler/tiktok_analytics_test.go`: API-envelope and account-state tests.
- Modify `api/internal/handler/tiktok_analytics.go`: account-state checks and detailed error responses.
- Modify `api/internal/handler/social_account_metrics.go`: reuse TikTok classified error handling.
- Modify `api/internal/handler/response.go`: explicitly register supported TikTok error codes.
- Create `api/internal/worker/analytics_refresh_test.go`: worker failure-policy tests.
- Modify `api/internal/worker/analytics_refresh.go`: stable TikTok failure reasons while retaining Pinterest-only reconnect marking.
- Modify `dashboard/src/lib/api.ts`: add `details.reason` to the typed error contract.
- Create `dashboard/src/components/analytics/tiktok-analytics-state.ts`: pure error and readiness state mapping.
- Modify `dashboard/src/components/analytics/tiktok-analytics-rows.ts`: represent unavailable metrics with `null`.
- Modify `dashboard/src/components/analytics/tiktok-analytics-view.tsx`: request-generation isolation and independent section settlement.
- Modify `dashboard/tests/tiktok-analytics-rows.test.mjs`: missing versus legitimate-zero metrics.
- Create `dashboard/tests/tiktok-analytics-state.test.mjs`: error/readiness mapping.
- Create `dashboard/tests/tiktok-analytics-view-source.test.mjs`: request-isolation and all-settled source contract.
- Create `api/ops/tiktok_analytics_recovery.sql`: dry-run-by-default, timestamp-guarded recovery scheduler.
- Create `docs/tiktok-analytics-recovery-runbook.md`: operator procedure and validation queries.
- Create `api/internal/db/tiktok_analytics_recovery_runbook_test.go`: SQL safety contract.

---

### Task 1: Exact TikTok Public Video IDs

**Files:**
- Modify: `api/internal/platform/tiktok_test.go`
- Modify: `api/internal/platform/tiktok.go`

- [ ] **Step 1: Add failing exact-ID tests**

Add tests that route both publish-status and video-query requests through an in-memory `RoundTripper`:

```go
func TestTikTokGetAnalyticsPreservesNumericPublicVideoID(t *testing.T) {
	const exactID = "7663542984343883021"
	transport := &tiktokAnalyticsTransport{
		statusBody: `{"data":{"status":"PUBLISH_COMPLETE","publicaly_available_post_id":[7663542984343883021]},"error":{"code":"ok"}}`,
		videoBody:  `{"data":{"videos":[{"id":"7663542984343883021","view_count":283,"like_count":6,"comment_count":2,"share_count":1}]},"error":{"code":"ok"}}`,
	}
	adapter := NewTikTokAdapter()
	adapter.client = &http.Client{Transport: transport}

	got, err := adapter.GetAnalytics(context.Background(), "token", "publish_1")
	if err != nil {
		t.Fatal(err)
	}
	if transport.queriedVideoID != exactID {
		t.Fatalf("video id = %q, want %q", transport.queriedVideoID, exactID)
	}
	if got.VideoViews != 283 || got.Likes != 6 || got.PlatformSpecific["tiktok_video_id"] != exactID {
		t.Fatalf("metrics = %#v", got)
	}
}
```

Also cover string IDs, all three field spellings, malformed JSON, non-2xx status, non-`ok` error envelope, and direct `float64` input rejection.

- [ ] **Step 2: Verify the precision test fails for the expected reason**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/platform -run 'TestTikTok(GetAnalyticsPreservesNumericPublicVideoID|ExtractPublicPostID)' -count=1
```

Expected: the numeric case queries `7663542984343883000`, or the wished-for error-returning extraction signature does not compile.

- [ ] **Step 3: Decode publish status with `UseNumber`**

Change `CheckPublishStatus` to read the body only after the request completes, reject non-2xx responses, and decode with:

```go
decoder := json.NewDecoder(bytes.NewReader(respBody))
decoder.UseNumber()
var result map[string]any
if err := decoder.Decode(&result); err != nil {
	return nil, fmt.Errorf("tiktok publish status decode: %w", err)
}
```

Validate the TikTok `error` envelope before returning `result`.

- [ ] **Step 4: Return exact IDs or an extraction error**

Change the extractor to:

```go
func tiktokExtractPublicPostID(data map[string]any) (string, error)
```

Accept non-empty `string` and `json.Number`, validate every character is `0-9`, and return the original decimal text. Return an explicit error for `float64`, signed values, decimals, empty arrays, and missing fields. Update URL helpers to ignore the extraction error and return an empty URL.

- [ ] **Step 5: Verify Task 1 is green**

Run:

```bash
cd api
gofmt -w internal/platform/tiktok.go internal/platform/tiktok_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/platform -count=1
```

Expected: all platform tests pass.

---

### Task 2: Typed Analytics Availability and Authorization Errors

**Files:**
- Create: `api/internal/platform/tiktok_analytics_error.go`
- Modify: `api/internal/platform/tiktok_test.go`
- Modify: `api/internal/platform/tiktok.go`

- [ ] **Step 1: Add failing classification tests**

Define the wished-for public contract in tests:

```go
func TestTikTokAnalyticsErrorReasonSurvivesWrapping(t *testing.T) {
	err := fmt.Errorf("outer: %w", NewTikTokAnalyticsError(
		TikTokAnalyticsScopeRequired,
		"video.query",
		http.StatusForbidden,
		"scope_not_authorized",
		errors.New("denied"),
	))
	if got, ok := TikTokAnalyticsErrorReasonOf(err); !ok || got != TikTokAnalyticsScopeRequired {
		t.Fatalf("reason = %q, ok=%v", got, ok)
	}
}
```

Add provider-response cases for `access_token_invalid`, `scope_not_authorized`, HTTP 429, timeout/5xx, incomplete publish status, and empty video lists.

- [ ] **Step 2: Verify classification tests fail**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/platform -run 'TestTikTokAnalytics(Error|Unavailable|Pending)' -count=1
```

Expected: the new types/functions are undefined or empty video lists still return successful zero metrics.

- [ ] **Step 3: Implement the typed error**

Create stable reason constants:

```go
type TikTokAnalyticsReason string

const (
	TikTokAccountTokenInvalid    TikTokAnalyticsReason = "account_token_invalid"
	TikTokAnalyticsScopeRequired TikTokAnalyticsReason = "analytics_scope_required"
	TikTokProviderRateLimited    TikTokAnalyticsReason = "provider_rate_limited"
	TikTokProviderTemporaryError TikTokAnalyticsReason = "provider_temporary_error"
	TikTokVideoNotFound          TikTokAnalyticsReason = "video_not_found"
	TikTokVideoNotReady          TikTokAnalyticsReason = "video_not_ready"
)
```

The error struct carries `Reason`, `Operation`, `HTTPStatus`, `ProviderCode`, and `Err`, implements `Error()` and `Unwrap()`, and is inspected with `errors.As`.

- [ ] **Step 4: Use typed errors at TikTok analytics boundaries**

Classify HTTP/envelope errors in publish status, user info, video list, and video query. In `GetAnalytics`:

- non-complete publish status returns `TikTokVideoNotReady`;
- empty `videos` returns `TikTokVideoNotFound`;
- a matched row, including an all-zero row, returns success and the exact `tiktok_video_id`.

- [ ] **Step 5: Verify Task 2 is green**

Run:

```bash
cd api
gofmt -w internal/platform/tiktok_analytics_error.go internal/platform/tiktok.go internal/platform/tiktok_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/platform -count=1
```

Expected: all platform tests pass, including exact ID and unavailable-state cases.

---

### Task 3: API Error Envelope and Account State

**Files:**
- Create: `api/internal/handler/tiktok_analytics_test.go`
- Modify: `api/internal/handler/tiktok_analytics.go`
- Modify: `api/internal/handler/social_account_metrics.go`
- Modify: `api/internal/handler/response.go`

- [ ] **Step 1: Add failing response tests**

Use `httptest.NewRecorder` to call `writeTikTokAnalyticsError` and decode `ErrorResponse`. Assert:

```go
if got.Error.Code != "NEEDS_RECONNECT" ||
	got.Error.Details["reason"] != "analytics_scope_required" {
	t.Fatalf("error = %#v", got.Error)
}
```

Cover all PRD mappings: invalid token, missing scope, 429, provider temporary error, video unavailable, legacy fallback, disconnected state, and `reconnect_required` state.

- [ ] **Step 2: Verify handler tests fail**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestTikTokAnalytics' -count=1
```

Expected: responses lack `details.reason`, transient errors use `TIKTOK_ERROR`, or account-state helpers are undefined.

- [ ] **Step 3: Implement account-state and error mapping**

Add a pure helper that maps:

```text
status=disconnected or disconnected_at valid -> ACCOUNT_DISCONNECTED/account_disconnected
status=reconnect_required -> NEEDS_RECONNECT/account_token_invalid
```

Call it before token decryption in `loadTikTokForAnalytics` and in TikTok's account-metrics path. Translate typed platform reasons through `writeErrorWithDetails`, preserving `NEEDS_RECONNECT` for token/scope compatibility.

- [ ] **Step 4: Register normalized codes**

Add explicit entries for:

```text
ACCOUNT_DISCONNECTED
UPSTREAM_RATE_LIMITED
TIKTOK_TEMPORARY_ERROR
TIKTOK_ANALYTICS_UNAVAILABLE
```

Do not change the existing normalized value of `NEEDS_RECONNECT`.

- [ ] **Step 5: Verify Task 3 is green**

Run:

```bash
cd api
gofmt -w internal/handler/tiktok_analytics.go internal/handler/tiktok_analytics_test.go internal/handler/social_account_metrics.go internal/handler/response.go
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -count=1
```

Expected: all handler tests pass.

---

### Task 4: Worker Failure Preservation Policy

**Files:**
- Create: `api/internal/worker/analytics_refresh_test.go`
- Modify: `api/internal/worker/analytics_refresh.go`

- [ ] **Step 1: Add failing pure policy tests**

Extract and test a helper returning the persisted reason plus whether the account should be marked reconnect-required:

```go
reason, mark := analyticsRefreshFailurePolicy(
	"tiktok",
	platform.NewTikTokAnalyticsError(
		platform.TikTokAnalyticsScopeRequired,
		"video.query",
		http.StatusForbidden,
		"scope_not_authorized",
		errors.New("denied"),
	),
)
if mark || reason != "TikTok analytics unavailable: analytics_scope_required (video.query)" {
	t.Fatalf("reason=%q mark=%v", reason, mark)
}
```

Assert Pinterest auth failures still return `mark=true`, while TikTok scope, rate-limit, temporary, not-found, and not-ready errors return `mark=false`.

- [ ] **Step 2: Verify the worker tests fail**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run 'TestAnalyticsRefreshFailurePolicy' -count=1
```

Expected: the policy helper is undefined.

- [ ] **Step 3: Implement and use the policy helper**

Replace the inline Pinterest branch with the tested helper. Continue calling `TouchPostAnalyticsFetchedAt` on every analytics error so metrics remain untouched; call `MarkSocialAccountReconnectRequired` only when the helper returns `mark=true`.

- [ ] **Step 4: Verify Task 4 is green**

Run:

```bash
cd api
gofmt -w internal/worker/analytics_refresh.go internal/worker/analytics_refresh_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/worker -count=1
```

Expected: all worker tests pass.

---

### Task 5: Dashboard Request Isolation, Partial Success, and N/A

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/components/analytics/tiktok-analytics-state.ts`
- Modify: `dashboard/src/components/analytics/tiktok-analytics-rows.ts`
- Modify: `dashboard/src/components/analytics/tiktok-analytics-view.tsx`
- Modify: `dashboard/tests/tiktok-analytics-rows.test.mjs`
- Create: `dashboard/tests/tiktok-analytics-state.test.mjs`
- Create: `dashboard/tests/tiktok-analytics-view-source.test.mjs`

- [ ] **Step 1: Read the installed Next.js client-component guidance**

Run:

```bash
cd dashboard
find node_modules/next/dist/docs -iname '*client*component*.md' -o -iname '*server*component*.md'
```

Read the matching App Router guide before changing the client component.

- [ ] **Step 2: Add failing pure state tests**

Test that `tiktokAnalyticsIssue(error)` maps `rawCode` plus `details.reason` to the exact PRD messages, that legacy `NEEDS_RECONNECT` falls back to the expired-token message, and that `scopeReadinessState` does not return reconnect-required solely from stored missing scopes.

- [ ] **Step 3: Add failing row-semantic tests**

Change the expected row metric type to `number | null`. Assert:

```js
assert.equal(rowsWithoutAnalytics[0].views, null);
assert.equal(rowsWithMatchedZero[0].views, 0);
```

A row is matched only when the analytics response contains `platform_specific.tiktok_video_id`.

- [ ] **Step 4: Add a failing request-isolation source contract**

Assert the component imports `useRef`, increments a request generation, guards state updates against the current generation and account ID, and uses `Promise.allSettled` for profile, account metrics, public videos, and posts.

- [ ] **Step 5: Verify Dashboard tests fail**

Run:

```bash
cd dashboard
node --test tests/tiktok-analytics-state.test.mjs tests/tiktok-analytics-rows.test.mjs tests/tiktok-analytics-view-source.test.mjs
```

Expected: new modules/exports are missing and null metrics are still converted to zero.

- [ ] **Step 6: Implement the pure state and row helpers**

Extend `ApiError.error.details` with `reason?: string`. Implement the exact PRD messages and readiness states in `tiktok-analytics-state.ts`. Preserve zero only when a matched analytics row contains the exact TikTok video ID.

- [ ] **Step 7: Implement independent resource settlement**

Use `useRef` for a monotonically increasing request generation. Clear prior account errors on selection, settle profile/metrics/videos/posts independently, render successful sections, and show small inline section errors without hiding other data. `ScopeReadiness` receives the runtime reason; stored missing scopes render an informational verification state, not a reconnect claim.

Keep the existing visual tokens, spacing, typography, and Lucide imports; add no new visual dependency or animation.

- [ ] **Step 8: Verify Task 5 is green**

Run:

```bash
cd dashboard
node --test tests/tiktok-analytics-state.test.mjs tests/tiktok-analytics-rows.test.mjs tests/tiktok-analytics-view-source.test.mjs
npm run build
```

Expected: Node tests and Next.js production build pass.

---

### Task 6: Idempotent Historical Recovery Runbook

**Files:**
- Create: `api/internal/db/tiktok_analytics_recovery_runbook_test.go`
- Create: `api/ops/tiktok_analytics_recovery.sql`
- Create: `docs/tiktok-analytics-recovery-runbook.md`

- [ ] **Step 1: Add a failing SQL source-contract test**

Read the SQL file and require:

- `\set ON_ERROR_STOP on`;
- required `deployment_timestamp`;
- dry-run default;
- one transaction;
- the exact TikTok/active/published/non-deleted/90-day filters;
- `ON CONFLICT (social_post_result_id) DO UPDATE`;
- only `fetched_at` in the conflict update;
- the post-deployment idempotency guard;
- no update to `last_refreshed_at`, metrics, `platform_specific`, or failure columns.

- [ ] **Step 2: Verify the runbook test fails**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestTikTokAnalyticsRecoveryRunbook' -count=1
```

Expected: the SQL file does not exist.

- [ ] **Step 3: Implement the psql runbook**

The script requires:

```bash
psql "$DATABASE_URL" \
  -v deployment_timestamp="'2026-07-17T23:00:00Z'" \
  -v execute=false \
  -f api/ops/tiktok_analytics_recovery.sql
```

It builds the eligible set in a temporary table, prints counts by account and age bucket, and rolls back by default. With `execute=true`, it inserts missing analytics rows or sets only `fetched_at` to the epoch when the current value is null or older than the deployment timestamp, returns the scheduled count, and commits.

- [ ] **Step 4: Write the operator runbook**

Document prerequisites, 75-90-day publish-ID retention validation, dry-run reconciliation, execute command, worker monitoring, completion queries, pause conditions for 429/publishing regression, and the rule that this script is not run in production until production release authorization.

- [ ] **Step 5: Verify Task 6 is green**

Run:

```bash
cd api
gofmt -w internal/db/tiktok_analytics_recovery_runbook_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/db -count=1
```

Expected: database package tests pass.

---

### Task 7: Full Validation, Integration, and Dev Acceptance

**Files:**
- Modify only files listed in Tasks 1-6.

- [ ] **Step 1: Run task-branch validation**

Run in parallel:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

```bash
cd dashboard
npm run build
npm run test:regression:dashboard
```

Expected: all commands pass. If Playwright browsers are unavailable, install the configured Chromium browser and rerun.

- [ ] **Step 2: Review scope and secrets**

Run:

```bash
git diff --check
git status --short
git diff --name-only origin/dev...HEAD
rg -n 'access_token|refresh_token|Authorization' api/ops docs/tiktok-analytics-recovery-runbook.md
```

Verify no token values, credentials, unrelated files, or generated artifacts are included.

- [ ] **Step 3: Commit focused implementation checkpoints**

Use focused commits for provider/backend, Dashboard, and recovery runbook. Do not include `artifacts/`.

- [ ] **Step 4: Update local dev and merge**

From the existing `dev` worktree:

```bash
git status --short --branch
git pull --ff-only origin dev
git merge --no-ff dev-tiktok-analytics-recovery
```

Stop if unrelated local changes prevent the merge.

- [ ] **Step 5: Re-run validation on local dev**

Run the full Go test suite, Dashboard build, and Dashboard Playwright regression suite again from local `dev`.

- [ ] **Step 6: Push and monitor development deployment**

Push local `dev` to `origin/dev`. Monitor GitHub checks, Railway development deployment, and Vercel `unipost-dev` deployment until every triggered item is terminal and successful.

- [ ] **Step 7: Perform real development acceptance**

Use only:

- `https://dev-api.unipost.dev`
- `https://dev-app.unipost.dev`

Verify healthy-account sections load without a false reconnect banner, disconnected and missing-scope states show distinct messages, account switching cannot show stale state, missing post analytics show `N/A`, a matched zero shows `0`, and a known published post uses the exact public TikTok ID. Confirm a healthy TikTok publish flow still works when a safe development-owned account is available.
