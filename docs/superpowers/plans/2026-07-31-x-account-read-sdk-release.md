# X Account Read SDKs and Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship additive X profile, authored-post history, and X Credits support in the JavaScript, Python, Go, and Java SDKs as version `0.7.0`; complete the API Reference and `/docs/guides/x/profile-and-post-history`; and prove that old SDK calls, Feature Flag off/on behavior, deployed documentation, and public registry packages all work.

**Architecture:** Keep the deployed UniPost API as the source of truth and add SDK surface area without changing existing method signatures or return types. X reads return the full `{data, meta, request_id}` envelope, require a caller-supplied idempotency key, validate deterministic request constraints locally, and preserve `error.details`, `error.is_retriable`, and `Retry-After`. X Credits endpoints are new Billing-resource methods; availability and accounting remain server-authoritative through `x_credits_billing_v1`. The profile/history guide is always public, while only Credits-specific prose, links, and examples are conditionally rendered from the existing public feature-flag surface.

**Tech Stack:** TypeScript/Vitest/tsup, Python 3.9+/pytest/mypy, Go 1.21+/`testing`, Java 11+/JUnit/MockWebServer/Gradle, Next.js 16/React 19/Node test/Playwright, GitHub Actions, Railway, Vercel, npm, PyPI, Go modules, Maven Central.

---

## Fixed paths, branches, and invariants

- Main repository worktree: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation`
- Main repository branch: `dev-x-account-profile-history-openapi`
- Conversation-owned SDK root: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0`
- SDK task branch in each SDK repository: `codex/x-account-read-sdk-0.7.0`
- JavaScript repository: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-js`
- Python repository: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-python`
- Go repository: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-go`
- Java repository: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-java`
- Before every write, test, commit, push, merge, tag, or deployment, print and verify the absolute repository path, current branch, and clean/expected status.
- Never use `/Users/xiaoboyu/unipost-dev/*`, `/tmp/unipost-sdk-plan.*`, a shared `dev`/`staging`/`main` checkout, or another task's worktree for implementation or release.
- Do not bump SDK versions in feature commits. Merge compatible feature changes to each SDK `main`, then let the audited release script create the four `0.7.0` version commits and tags.
- Never generate an idempotency key inside an SDK. A retry is safe only when the caller reuses the same key for the exact same request.
- Preserve every pre-`0.7.0` public method and its return shape. New account-read methods return full envelopes; existing methods continue to unwrap data exactly as before.

## Task 1: Create isolated SDK repositories and record baselines

**Files:**

- Create repositories under: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/`
- Record evidence in: `/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation/artifacts/x-account-read-sdk-release/baseline.md`

- [ ] Verify the main worktree before creating the SDK root:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation"
test "$(git branch --show-current)" = "dev-x-account-profile-history-openapi"
git status --short
```

Expected: the exact path and branch match; only intentional plan/spec changes may be present.

- [ ] Fetch each remote without touching any existing local SDK checkout, clone its latest `main` into the fixed SDK root, and create `codex/x-account-read-sdk-0.7.0`.

```bash
mkdir -p /Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0
for repo in sdk-js sdk-python sdk-go sdk-java; do
  git clone "git@github.com:unipost-dev/${repo}.git" \
    "/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/${repo}"
  git -C "/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/${repo}" \
    switch -c codex/x-account-read-sdk-0.7.0 origin/main
done
```

Expected: four independent clean repositories, each on the owned task branch based on current `origin/main`.

- [ ] Record each repository's baseline SHA, current version, latest tag, and public-package version. Verify there is no existing `v0.7.0` tag or published `0.7.0`.

- [ ] Run unchanged baseline suites:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-js
npm ci && npm run typecheck && npm test && npm run build && npm pack --dry-run --json

cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-python
python3 -m venv .venv
.venv/bin/pip install -e ".[dev]" build
.venv/bin/pytest tests/
.venv/bin/mypy unipost/ --ignore-missing-imports

cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-go
go test ./...
go vet ./...

cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0/sdk-java
./gradlew test
```

Expected: all commands pass. Any failure is a hard stop and must be recorded before feature work.

## Task 2: Add JavaScript X account reads and Billing resources

**Files:**

- Modify: `sdk-js/src/http.ts`
- Modify: `sdk-js/src/errors.ts`
- Modify: `sdk-js/src/resources/accounts.ts`
- Create: `sdk-js/src/resources/billing.ts`
- Modify: `sdk-js/src/client.ts`
- Modify: `sdk-js/src/index.ts`
- Modify: `sdk-js/src/types/accounts.ts`
- Create: `sdk-js/src/types/billing.ts`
- Modify: `sdk-js/src/types/index.ts`
- Modify: `sdk-js/tests/accounts.test.ts`
- Create: `sdk-js/tests/billing.test.ts`
- Create: `sdk-js/tests/x-account-read-errors.test.ts`
- Modify: `sdk-js/tests/package-consumer.ts`

- [ ] Write failing request-shape tests for:

```ts
await client.accounts.getProfile("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "profile-user-42",
});

await client.accounts.listPosts("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "posts-user-42-page-1",
  limit: 20,
  cursor: "xc_opaque",
  startTime: "2026-07-01T00:00:00Z",
  endTime: "2026-08-01T00:00:00Z",
  excludeReposts: true,
  excludeRepliesToOthers: true,
});
```

Assert encoded paths, snake_case query keys, exact `Idempotency-Key`, no request body, full envelope decoding, `meta.replayed === undefined` when absent, and no mutation of parameter objects.

- [ ] Run `npm test -- accounts.test.ts` and confirm failure because the new methods and types do not exist.

- [ ] Add complete `XAccount*` types:

  - `XAccountCreditsReceipt`
  - `XAccountProfile`, `XAccountPublicMetrics`
  - `XAccountPost`, media, post metrics, thread
  - `XAccountProfileResponse`
  - `XAccountPostsResponse`
  - `GetXAccountProfileParams`
  - `ListXAccountPostsParams`

All wire-response properties remain snake_case. Input properties use existing JavaScript SDK camelCase conventions and are translated once in the resource.

- [ ] Add shared validation helpers that reject blank account IDs, blank `externalUserId`, blank idempotency keys, `limit` outside `5..100`, invalid RFC 3339 bounds, or `endTime <= startTime` before any HTTP call.

- [ ] Extend the HTTP GET helper with an optional headers argument while preserving every existing two-argument call. Pass response headers into error parsing so terminal errors expose:

```ts
error.details
error.isRetriable
error.retryAfter
```

Use `Retry-After` as the authoritative retry delay when present; do not remove the existing bounded automatic `429` retry behavior.

- [ ] Implement `accounts.getProfile` and `accounts.listPosts`, returning the unmodified response envelope.

- [ ] Add `client.billing` with:

```ts
await client.billing.getXCredits();
await client.billing.listXCreditEvents({
  accountId: "sa_x_123",
  externalUserId: "user_42",
  operation: "post.read",
  status: "succeeded",
  startTime: "2026-07-01T00:00:00Z",
  endTime: "2026-08-01T00:00:00Z",
  cursor: "xc_opaque",
  limit: 50,
});
```

Return full envelopes and model every allowance/event field from the deployed API.

- [ ] Add compatibility assertions to `tests/package-consumer.ts` for old and new calls. Old `accounts.list`, `accounts.get`, `posts.create`, and `usage.get` assignments must still compile unchanged.

- [ ] Run:

```bash
npm run typecheck
npm test
npm run build
npm pack --dry-run --json
```

Expected: all pass; package contents include declarations and built code for the new methods.

- [ ] Commit only JavaScript SDK changes:

```bash
git add src tests
git commit -m "feat: add X account reads and Credits resources"
```

## Task 3: Add Python sync/async X account reads and Billing resources

**Files:**

- Modify: `sdk-python/unipost/http.py`
- Modify: `sdk-python/unipost/errors.py`
- Modify: `sdk-python/unipost/resources/accounts.py`
- Create: `sdk-python/unipost/resources/billing.py`
- Modify: `sdk-python/unipost/client.py`
- Modify: `sdk-python/unipost/async_client.py`
- Modify: `sdk-python/unipost/types/__init__.py`
- Modify: `sdk-python/unipost/__init__.py`
- Create: `sdk-python/tests/test_x_account_reads.py`
- Create: `sdk-python/tests/test_x_account_read_errors.py`
- Create: `sdk-python/tests/test_billing.py`
- Modify: `sdk-python/tests/test_release.py`

- [ ] Write failing sync and async tests for:

```python
profile = client.accounts.get_profile(
    "sa_x_123",
    external_user_id="user_42",
    idempotency_key="profile-user-42",
)
posts = client.accounts.list_posts(
    "sa_x_123",
    external_user_id="user_42",
    idempotency_key="posts-user-42-page-1",
    limit=20,
    cursor="xc_opaque",
    start_time="2026-07-01T00:00:00Z",
    end_time="2026-08-01T00:00:00Z",
    exclude_reposts=True,
    exclude_replies_to_others=True,
)
```

Repeat the same contract through `AsyncUniPost`. Assert exact URL/query/header behavior, typed full envelopes, and absent replay state represented as `None`.

- [ ] Run `.venv/bin/pytest tests/test_x_account_reads.py -q` and confirm the methods are absent.

- [ ] Add dataclasses for the same complete wire contract as JavaScript, using `XAccount` prefixes to avoid the existing `Profile` type. `_from_dict` must preserve optional fields and tolerate forward-compatible unknown fields according to the current SDK pattern.

- [ ] Add local validation in both sync and async resource paths. Share pure helpers so behavior cannot drift.

- [ ] Extend `UniPostError` additively with `details`, `is_retriable`, and `retry_after`; preserve all existing subclasses and constructor call forms. Feed the response `Retry-After` header into both sync and async terminal error parsing.

- [ ] Add sync and async Billing resources:

```python
allowance = client.billing.get_x_credits()
events = client.billing.list_x_credit_events(
    account_id="sa_x_123",
    external_user_id="user_42",
    operation="post.read",
    status="succeeded",
    limit=50,
)
```

- [ ] Add release/import compatibility tests proving all old resources remain present and all existing type imports still resolve.

- [ ] Run:

```bash
.venv/bin/pytest tests/
.venv/bin/mypy unipost/ --ignore-missing-imports
.venv/bin/ruff check unipost tests
.venv/bin/python -m build
```

Expected: all tests and static checks pass; wheel and sdist include `resources/billing.py`.

- [ ] Commit only Python SDK changes:

```bash
git add unipost tests
git commit -m "feat: add X account reads and Credits resources"
```

## Task 4: Add Go X account reads without changing `APIError`

**Files:**

- Modify: `sdk-go/unipost/client.go`
- Modify: `sdk-go/unipost/accounts.go`
- Create: `sdk-go/unipost/x_account_reads.go`
- Create: `sdk-go/unipost/billing.go`
- Modify: `sdk-go/unipost/http.go`
- Create: `sdk-go/unipost/accounts_x_reads_test.go`
- Create: `sdk-go/unipost/billing_test.go`
- Create: `sdk-go/unipost/x_account_read_errors_test.go`
- Modify: existing compatibility tests where needed

- [ ] Write failing tests for:

```go
profile, err := client.Accounts.Profile(ctx, "sa_x_123", &unipost.XAccountProfileParams{
    ExternalUserID: "user_42",
    IdempotencyKey: "profile-user-42",
})

posts, err := client.Accounts.ListPosts(ctx, "sa_x_123", &unipost.XAccountPostsParams{
    ExternalUserID: "user_42",
    IdempotencyKey: "posts-user-42-page-1",
    Limit: 20,
    Cursor: "xc_opaque",
    StartTime: "2026-07-01T00:00:00Z",
    EndTime: "2026-08-01T00:00:00Z",
    ExcludeReposts: true,
    ExcludeRepliesToOthers: true,
})
```

Assert exact request details, typed full envelopes, optional pointer fields, and local validation.

- [ ] Run `go test ./unipost -run 'XAccount|Billing' -count=1` and confirm compilation fails for missing APIs.

- [ ] Add complete `XAccount*` wire structs. Use `json` tags exactly matching the API. Use `*bool`, `*string`, and `*time.Time` where absence is semantically different from a zero value.

- [ ] Do **not** add fields to `APIError`. Existing users may construct it with unkeyed literals. Add:

```go
type XAccountReadError struct {
    APIError     *APIError
    Details      JSONMap
    IsRetriable  *bool
    RetryAfter   int
}

func (e *XAccountReadError) Error() string { return e.APIError.Error() }
func (e *XAccountReadError) Unwrap() error { return e.APIError }
```

The decoder must preserve `errors.As(err, *APIError)` compatibility through `Unwrap()` and populate `RetryAfter` from the HTTP header first, then the response body.

- [ ] Reuse `doResponseOnce` for account-read requests so headers are retained and automatic SDK retries never duplicate a financially meaningful read. Decode successful responses locally; map non-2xx responses into `XAccountReadError`.

- [ ] Add `Billing *BillingService` to `Client` and implement `GetXCredits` plus `ListXCreditEvents` with complete typed envelopes and event filters.

- [ ] Add compile-time compatibility tests for old calls and an unkeyed `APIError` literal so a future accidental field addition fails the suite.

- [ ] Run:

```bash
gofmt -w unipost
go test ./...
go vet ./...
```

Expected: all pass on Go 1.21-compatible code.

- [ ] Commit only Go SDK changes:

```bash
git add unipost
git commit -m "feat: add X account reads and Credits resources"
```

## Task 5: Add Java X account reads and Billing resources

**Files:**

- Modify: `sdk-java/src/main/java/dev/unipost/UniPost.java`
- Modify: `sdk-java/src/main/java/dev/unipost/ApiHttpClient.java`
- Modify: `sdk-java/src/main/java/dev/unipost/APIError.java`
- Modify: `sdk-java/src/test/java/dev/unipost/ResourceRequestTest.java`
- Modify: `sdk-java/src/test/java/dev/unipost/UniPostTest.java`
- Create: `sdk-java/src/test/java/dev/unipost/XAccountReadErrorTest.java`

- [ ] Write failing MockWebServer tests for:

```java
JsonNode profile = client.accounts().profile(
    "sa_x_123",
    Map.of(
        "external_user_id", "user_42",
        "idempotency_key", "profile-user-42"
    )
);

JsonNode posts = client.accounts().listPosts(
    "sa_x_123",
    Map.of(
        "external_user_id", "user_42",
        "idempotency_key", "posts-user-42-page-1",
        "limit", 20
    )
);
```

The resource extracts `idempotency_key` into `Idempotency-Key`, never puts it in the query, and returns the full root `JsonNode`.

- [ ] Run `./gradlew test --tests '*ResourceRequestTest*'` and confirm failure for missing methods.

- [ ] Add overloaded `ApiHttpClient.get(path, query, extraHeaders)` while retaining both old GET overloads.

- [ ] Preserve the existing five-argument `APIError` constructor. Add an overloaded constructor and read-only accessors for `JsonNode details`, `Boolean retriable`, and `Integer retryAfterSeconds`. Parse `Retry-After` from `HttpHeaders`, falling back to any compatible body field.

- [ ] Add account-read validation for required IDs/key, `limit`, and time bounds. Return the complete envelope rather than using `Resource.data`.

- [ ] Add `BillingResource`, `billing()` accessor, `getXCredits`, and `listXCreditEvents`. Return complete `JsonNode` envelopes to match current Java SDK conventions.

- [ ] Add binary/source compatibility assertions: instantiate `new APIError(status, code, message, requestId, body)`, call all existing resource methods, and compile on Java 11.

- [ ] Run:

```bash
./gradlew test
./gradlew build
./gradlew publishToMavenLocal
```

Expected: all pass; the locally published artifact exposes old and new methods.

- [ ] Commit only Java SDK changes:

```bash
git add src
git commit -m "feat: add X account reads and Credits resources"
```

## Task 6: Extend central source-validation and release safety

**Files:**

- Modify: `scripts/sdk-validation/js/index.mjs`
- Modify: `scripts/sdk-validation/python/main.py`
- Modify: `scripts/sdk-validation/go/main.go`
- Modify: `scripts/sdk-validation/java/src/main/java/dev/unipost/validation/Main.java`
- Modify: `scripts/sdk-source-validation/run-suite.sh` only if new non-secret inputs are required
- Modify: `.github/workflows/sdk-source-validation.yml`
- Modify: `scripts/release/create-sdk-release.sh`
- Modify: `scripts/release/bump-sdk-version.sh` if newly introduced version locations require it
- Modify: `docs/sdk-api-coverage-matrix.md`
- Modify: `docs/sdk-release.md`
- Create: `dashboard/tests/x-account-read-sdk-docs-source.test.mjs`

- [ ] Add failing source tests requiring all four source validators to call profile, posts, allowance, and event APIs and to assert:

  - full envelope access;
  - accounting-enabled and bypassed receipts;
  - replay handling;
  - posts cursor continuation;
  - insufficient credits without provider delivery;
  - retriable metadata and `Retry-After`;
  - unchanged representative old calls.

- [ ] Add workflow-dispatch inputs `sdk_js_ref`, `sdk_python_ref`, `sdk_go_ref`, and `sdk_java_ref`, defaulting to `main`. Pass each exact ref into the corresponding checkout. This lets branch CI validate exact PR heads before merge and defaults to release validation against all `main` branches.

- [ ] Extend each language validator using the new public SDK surface. Make X-live checks conditional only when `TEST_X_ACCOUNT_ID` and `TEST_EXTERNAL_USER_ID` are present; lack of required release acceptance secrets is a failed required check, not a silent skip, when `REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true`.

- [ ] Update the release script's allowed/staged paths so version-only release commits may touch every actual version-bearing file while refusing feature-source drift. Add preflight assertions that each repo `main` contains the new account-read and Billing symbols before a `0.7.0` tag can be created.

- [ ] Correct `docs/sdk-release.md`: Go is publicly resolved; list current baselines; explain feature PRs precede version tags; document required secrets and exact acceptance gates.

- [ ] Change the coverage matrix for the four routes from `No` to their exact `0.7.0` method names only after the SDK source commits exist.

- [ ] Run:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation/dashboard
node --test tests/x-account-read-sdk-docs-source.test.mjs tests/x-account-reads-docs-source.test.mjs

cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation
shellcheck scripts/release/create-sdk-release.sh scripts/release/bump-sdk-version.sh scripts/sdk-source-validation/run-suite.sh
```

Expected: all source contracts and shell checks pass.

- [ ] Commit central validation/release changes separately:

```bash
git add .github/workflows/sdk-source-validation.yml scripts docs/sdk-api-coverage-matrix.md docs/sdk-release.md dashboard/tests/x-account-read-sdk-docs-source.test.mjs
git commit -m "test: validate X account read SDK contracts"
```

## Task 7: Complete API Reference and the guidance page

**Required skill before editing:** `design-taste-frontend`

**Files:**

- Modify: `dashboard/src/app/docs/api/accounts/profile/page.tsx`
- Modify: `dashboard/src/app/docs/api/accounts/posts/page.tsx`
- Modify: `dashboard/src/app/docs/api/accounts/capabilities/page.tsx`
- Create: `dashboard/src/app/docs/guides/x/profile-and-post-history/page.tsx`
- Create supporting client component only if needed under the same guide directory
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/api/page.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Modify: `dashboard/src/app/sitemap.ts`
- Modify: `dashboard/src/lib/docs-feature-flags.ts` only for shared conditional chunks, not to hide the new guide route
- Modify: `dashboard/tests/x-account-reads-docs-source.test.mjs`
- Modify: `dashboard/tests/x-feature-flag-docs-source.test.mjs`
- Modify: `dashboard/tests/docs-ai-search-evals.test.mjs`
- Create: `dashboard/tests/x-profile-post-history-guide-source.test.mjs`

- [ ] Write failing source tests that require:

  - four SDK snippets plus cURL on both endpoint pages;
  - exact method names and full-envelope access;
  - every stable error code and `is_retriable`/`Retry-After`;
  - prerequisites: `users.read`, `tweet.read` for posts, `offline.access`, bound `external_user_id`, persisted X app identity;
  - `start_time` inclusive and `end_time` exclusive;
  - explicit idempotency retry rules;
  - pagination, filtering, Credits estimation, and insufficient-Credits handling;
  - reconnect guide link;
  - guide discovery in navigation, guides landing, search, sitemap, and AI search;
  - no route-level feature requirement for `/docs/guides/x/profile-and-post-history`;
  - all Credits-only guide chunks marked `required_feature: "x_credits_billing_v1"` or conditionally rendered from the same flag.

- [ ] Run the new tests and confirm failure because the guide and SDK examples are absent.

- [ ] Refactor the profile/posts reference pages into client-aware content only where necessary. Always show endpoint availability and bypass behavior. Show charge/allowance/event instructions and `INSUFFICIENT_X_CREDITS` examples only when `x_credits_billing_v1` is enabled.

- [ ] Build the new guide around this flow:

  1. Confirm account eligibility/capabilities.
  2. Read the profile with a caller-owned idempotency key.
  3. Read an authored-post page with a caller-owned idempotency key.
  4. Follow `next_cursor` with a new logical-page key.
  5. Handle replays, in-progress/settlement-pending responses, reauthorization, upstream retry, and cursor refresh.
  6. When Credits accounting is enabled, preflight allowance/events and handle `402`.
  7. When accounting is disabled or the customer owns the X app, explain `accounting_enabled=false` and `bypass_reason` without linking hidden Credits pages.

- [ ] Include tested examples in JavaScript, Python sync, Go, and Java. Every example must inspect `meta.credits`, preserve `request_id`, and avoid presenting automatic idempotency-key generation.

- [ ] Add the guide to docs navigation, search, sitemap, guides landing, and AI search. Do not add it to `DOCS_PATH_FEATURES`; the route is always public.

- [ ] Add AI-search chunks for general X reads without `required_feature`, and separate Credits chunks with `required_feature: "x_credits_billing_v1"`.

- [ ] Run:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation/dashboard
node --test \
  tests/x-account-reads-docs-source.test.mjs \
  tests/x-feature-flag-docs-source.test.mjs \
  tests/x-account-read-sdk-docs-source.test.mjs \
  tests/x-profile-post-history-guide-source.test.mjs \
  tests/docs-ai-search-evals.test.mjs
npm run build
npm run test:regression:dashboard
```

Expected: source tests, production build, and dashboard regression all pass.

- [ ] Commit documentation separately:

```bash
git add src/app/docs src/lib/docs-ai-search-index.ts src/lib/docs-feature-flags.ts src/app/sitemap.ts tests
git commit -m "docs: add X profile and post history guidance"
```

## Task 8: Validate and merge each SDK feature PR

- [ ] In every SDK repository, verify owned path/branch, inspect the exact unique commits and changed files, and confirm no version bump or unrelated change.

- [ ] Push `codex/x-account-read-sdk-0.7.0` in all four repositories and open Draft PRs to `main`.

- [ ] Monitor every matrix job:

  - JavaScript: Node 18, 20, 22; typecheck; Vitest; build; pack dry-run.
  - Python: 3.9, 3.10, 3.11, 3.12; pytest; mypy.
  - Go: 1.21, 1.22, 1.23; test; vet.
  - Java: 17, 21; Gradle test.

Any failed, skipped, timed-out, cancelled, or missing result is a hard stop.

- [ ] Run central `SDK Source Validation` from the main repository branch with the four exact SDK PR branch refs and `REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true`.

- [ ] Confirm the workflow tests the exact four PR head SHAs and upload/download the validation artifact. Record run URL and artifact URL.

- [ ] Mark each SDK PR ready only after its CI and the exact-head central source validation pass.

- [ ] Re-audit each PR's unique commits/files and merge one SDK PR at a time. After every merge, monitor the SDK `main` workflow and verify the merged SHA.

## Task 9: Validate the UniPost Draft PR and Preview

- [ ] From the main worktree, run the complete changed-surface checks:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation/dashboard
npm ci
node --test tests/x-account-reads-docs-source.test.mjs tests/x-feature-flag-docs-source.test.mjs tests/x-account-read-sdk-docs-source.test.mjs tests/x-profile-post-history-guide-source.test.mjs
npm run build
npm run test:regression:dashboard

cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation/api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: all pass. The API suite is a regression guard even though no API production code should change.

- [ ] Audit unique commits/files relative to the latest `origin/dev`. Rebase only inside the owned branch if needed; rerun the complete suite after any rebase.

- [ ] Push the owned branch and open/update a Draft PR to `dev`.

- [ ] Wait for GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and Preview Acceptance on the exact head SHA.

- [ ] Use browser acceptance on the Preview URL with both public-feature paths:

  - flag off: guide/profile/posts remain available; Credits-only text/links/examples are absent; direct Credits docs remain unavailable;
  - flag on: the same guide/reference pages expose Credits sections and valid links.

- [ ] Verify mobile/desktop navigation, code tabs, anchors, search discovery, AI answer discovery, and no console/network errors.

- [ ] Mark ready, re-audit commits/files, and merge to `dev` only when every required Preview gate succeeds.

## Task 10: Verify official development deployment

- [ ] Monitor all post-merge GitHub, Railway development, and Vercel `unipost-dev` deployments to terminal success.

- [ ] Confirm the deployed commit equals the merged `dev` SHA.

- [ ] On `https://dev.unipost.dev` and the relevant app/docs hosts, repeat the flag-off and flag-on documentation acceptance against the real development environment.

- [ ] Run source validation against the four merged SDK `main` branches and `https://dev-api.unipost.dev`, requiring the configured X test account.

- [ ] Verify:

  - profile full envelope;
  - authored posts full envelope and cursor behavior;
  - idempotent replay;
  - Credits allowance and event endpoints when enabled;
  - `accounting_enabled=false`/`bypass_reason` when disabled or customer-app mode applies;
  - representative old SDK calls still work.

Any required check without a result is failure, not a skip.

## Task 11: Create and publish four `0.7.0` releases

- [ ] Create a new conversation-owned release root:

`/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0-release`

Clone each merged SDK `main` fresh. Verify clean status, exact main SHA, no existing `v0.7.0`, and no unmerged feature files.

- [ ] From the main worktree, run the release script without `--push` first:

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-profile-history-evaluation
UNIPOST_DEV_ROOT=/Users/xiaoboyu/.config/superpowers/worktrees/unipost-sdk-x-account-read-0-7-0-release \
UNIPOST_API_KEY="$UNIPOST_API_KEY" \
BASE_URL=https://dev-api.unipost.dev \
TEST_ACCOUNT_ID="$TEST_X_ACCOUNT_ID" \
TEST_EXTERNAL_USER_ID="$TEST_EXTERNAL_USER_ID" \
REQUIRE_X_ACCOUNT_READ_ACCEPTANCE=true \
scripts/release/create-sdk-release.sh 0.7.0
```

Expected: source validation passes, each repository gets one focused version commit and local `v0.7.0` tag, and no tag is pushed yet.

- [ ] Audit every release commit and tag:

  - version fields all equal `0.7.0`;
  - JavaScript dist contains the new methods and `unipost-js/0.7.0`;
  - Python `__version__`, user agent, and package metadata agree;
  - Go `sdkVersion` and user agent agree;
  - Java Gradle/POM/README/SDK version agree;
  - release commits contain only allowed version/generated artifacts.

- [ ] Re-run all four local repository suites on the tag commits.

- [ ] Push each release commit to `main`, verify the push, then push its `v0.7.0` tag. Do not push the next repository's tag until the prior repository publish workflow has reached a terminal success, unless the workflow is demonstrably independent and monitoring remains exact.

- [ ] Monitor npm, PyPI, Go, and Maven workflows and retain run URLs/log artifacts. A workflow that does not start or does not finish successfully is a release failure.

## Task 12: Verify public packages and close the release

- [ ] Poll authoritative registries until `0.7.0` is visible:

  - npm package metadata for `@unipost/sdk`;
  - PyPI JSON for `unipost`;
  - `go list -m github.com/unipost-dev/sdk-go@v0.7.0`;
  - Maven Central repository metadata for `dev.unipost:sdk-java`.

- [ ] In four fresh temporary consumer projects, install only from public registries and execute compile/import smoke tests for:

  - one pre-existing API call;
  - profile read;
  - authored-post list;
  - X Credits allowance;
  - X Credits events;
  - new error metadata access.

- [ ] Run the public packages against the development API test account first. If the production X-read API is already released and the user authorizes live production verification, run a minimal read-only production smoke with bounded limits and unique idempotency keys.

- [ ] Update `docs/sdk-api-coverage-matrix.md` and `docs/sdk-release.md` with actual tag SHAs, registry URLs, workflow URLs, and the public-install verification date if those facts were intentionally left pending before release. Any documentation change follows the same owned-branch PR/Preview/dev acceptance gate.

- [ ] Produce a final promotion-content audit:

  - main repository PR commits/files;
  - four SDK feature PR commits/files;
  - four release commit/tag SHAs;
  - local test commands/results;
  - Preview and development deployment SHAs/URLs;
  - flag-off/on evidence;
  - four public registry versions and clean-install evidence;
  - remaining non-blocking follow-ups, if any.

- [ ] Mark the goal complete only when every required check above is successful and no publication or deployment remains pending.
