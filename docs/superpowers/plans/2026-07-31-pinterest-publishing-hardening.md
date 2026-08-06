# Pinterest Publishing Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent unavailable, unwritable, stale, or cross-environment Pinterest boards from reaching `POST /v5/pins`, and return stable actionable failures across API, queue, and Dashboard.

**Architecture:** Pinterest owns provider API semantics, direct board lookup, bounded owned-board proof, and Pinterest error normalization. The generic publishing path gains only a typed failure bridge and an internal dispatch-environment marker. Dashboard keeps board selection live and renders structured outcomes; the delivery worker remains authoritative.

**Tech Stack:** Go 1.24, `net/http`, PostgreSQL-backed post metadata, Next.js/React/TypeScript, Node test runner, Playwright.

---

## Contract and references

- Product contract: `docs/prd-pinterest-publish-preflight-and-actionable-errors.md`, Workstream A.
- Pinterest Create Pin documents that the destination is a board owned by the operation user: <https://developers.pinterest.com/docs/api/v5/pins-create/?query=upload>
- Pinterest pagination allows `page_size=250`: <https://developers.pinterest.com/docs/reference/pagination/>
- Pinterest documents a 2,000-board account ceiling including group boards: <https://developers.pinterest.com/docs/work-with-organic-content-and-users/create-boards-and-pins/>
- A successful direct board read proves visibility, not writability. The implementation accepts a board only when `owner.username` matches the authenticated `/user_account` username. Every fallback-list item must satisfy the same owner match; mere list presence is insufficient.

## Stable implementation contracts

Use these names consistently through the tasks:

```go
// api/internal/platform/provider_error.go
type FailureContract struct {
	ErrorCode   string
	Stage       string
	IsRetriable bool
}

type failureContractCarrier interface {
	FailureContractFields() map[string]any
}

func newProviderFailure(message string, providerFields map[string]any, failure FailureContract) error
```

`providerError` implements `Error()`, `ProviderErrorFields()`, `FailureContractFields()`, and `FailureStage()`. `postfailures` reads the map through its own local interface, so `platform` does not import `postfailures`. The queue reads `FailureStage()` before falling back to legacy string inference.

```go
// api/internal/platform/dispatch_context.go
type DispatchMetadata struct {
	SocialAccountID string
	Environment     string
}

func WithDispatchMetadata(context.Context, DispatchMetadata) context.Context
func DispatchMetadataFromContext(context.Context) (DispatchMetadata, bool)
```

```go
// api/internal/platform/validate.go
type PlatformPostInput struct {
	// existing public request fields remain unchanged
	DispatchEnvironment string `json:"-"`
}
```

The persisted internal metadata key is `dispatch_environment`. The runtime values are exactly `production` and `sandbox` and come only from `platform.PinterestEnvironment()`.

## Task 1: Add typed Pinterest failures and taxonomy coverage

**Files:**

- Modify: `api/internal/platform/provider_error.go`
- Modify: `api/internal/platform/pinterest.go`
- Modify: `api/internal/postfailures/contract.go`
- Modify: `api/internal/postfailures/taxonomy.go`
- Test: `api/internal/postfailures/taxonomy_test.go`
- Test: `api/internal/platform/pinterest_test.go`

- [ ] **Step 1: Add failing taxonomy tests for the stable contract**

Add table cases that construct adapter errors through an exported test helper or an in-package Pinterest response parser and call `BuildParamsFromError`. Assert all public fields, not only `ErrorCode`:

```go
{
	name: "pinterest destination code 29",
	err: pinterestFailureForTest(403, 29, "destination_preflight"),
	wantCode: "target_not_found",
	wantProviderCode: "29",
	wantAction: "select_valid_target",
	wantSource: "platform",
	wantTemporality: "permanent",
	wantRetriable: false,
	wantProvider: ProviderError{Provider: "pinterest", HTTPStatus: 403, Code: "29", Reason: "board_not_accessible"},
},
```

Cover code `2`, code `29`, code `40`, HTTP `429`, `5xx`, timeout, and an unknown Pinterest response. Assert legacy text containing `pinterest ... code=29 provider_reason=board_not_accessible` still classifies correctly.

- [ ] **Step 2: Run the focused tests and prove RED**

Run from `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/postfailures ./internal/platform -run 'Pinterest|pinterest' -count=1
```

Expected: failures show code `29` falling through to `platform_error`, no Pinterest `provider_error`, and no typed failure stage.

- [ ] **Step 3: Implement the typed failure bridge**

Extend `providerError` without exporting provider-specific types:

```go
type providerError struct {
	message  string
	fields   map[string]any
	failure  FailureContract
}

func (e providerError) FailureContractFields() map[string]any {
	return map[string]any{
		"error_code": e.failure.ErrorCode,
		"failure_stage": e.failure.Stage,
		"is_retriable": e.failure.IsRetriable,
	}
}

func (e providerError) FailureStage() string { return e.failure.Stage }
```

In `postfailures.classifyError`, apply typed `error_code` and `is_retriable` before `enrichClassification`. Add `extractPinterestProviderError` as the legacy fallback. Include `pinterest` in `hasProviderSignal`.

- [ ] **Step 4: Parse Pinterest JSON once at the adapter boundary**

Add a narrow response type and constructor in `pinterest.go`:

```go
type pinterestAPIErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func pinterestProviderFailure(operation string, status int, body []byte, stage string) error
```

The returned public message is fixed copy selected by status/code/context; it never includes the raw body or URL. Provider fields are `provider`, `http_status`, `code`, `reason`, and `is_transient`. Use these mappings:

- `401` or code `2`: `auth_token_invalid`, `token_invalid`, permanent.
- destination/create-Pin `403` code `29`: `target_not_found`, `board_not_accessible`, permanent.
- destination/create-Pin `404` code `40`: `target_not_found`, `board_not_found`, permanent.
- `429`: `rate_limit`, `rate_limited`, retriable.
- `5xx` or timeout: `temporary_platform_error`, `provider_temporary_failure`, retriable.
- unknown: `platform_error`, `unknown`, non-retriable.

- [ ] **Step 5: Run focused and package tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/postfailures ./internal/platform -count=1
```

Expected: `ok` for both packages.

- [ ] **Step 6: Commit the error-normalization slice**

```bash
git add api/internal/platform/provider_error.go api/internal/platform/pinterest.go api/internal/platform/pinterest_test.go api/internal/postfailures/contract.go api/internal/postfailures/taxonomy.go api/internal/postfailures/taxonomy_test.go
git commit -m "fix: normalize Pinterest publishing failures"
```

## Task 2: Carry an internal Pinterest environment marker through scheduling

**Files:**

- Add: `api/internal/platform/dispatch_context.go`
- Add: `api/internal/platform/dispatch_context_test.go`
- Modify: `api/internal/platform/validate.go`
- Modify: `api/internal/platform/post_metadata.go`
- Modify: `api/internal/platform/post_metadata_test.go`
- Modify: `api/internal/platform/pinterest.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_posts_drafts.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Test: `api/internal/handler/social_posts_test.go`
- Test: `api/internal/handler/social_posts_drafts_test.go`

- [ ] **Step 1: Add failing metadata round-trip and dispatch-context tests**

Test all three cases:

```go
input := PlatformPostInput{AccountID: "sa_pin", DispatchEnvironment: "sandbox"}
encoded, err := EncodePlatformPostsMetadata([]PlatformPostInput{input})
decoded, err := DecodePlatformPostsMetadata(encoded)
require.Equal(t, "sandbox", decoded[0].DispatchEnvironment)
```

Also prove an empty legacy marker decodes as empty, and `WithDispatchMetadata` does not mutate unrelated context values.

- [ ] **Step 2: Run focused tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'DispatchEnvironment|DispatchMetadata|PinterestEnvironment' -count=1
```

Expected: compile failures for the absent field and helpers.

- [ ] **Step 3: Persist the marker without changing the public request**

Add `DispatchEnvironment` with `json:"-"` to `PlatformPostInput` and `DispatchEnvironment string json:"dispatch_environment,omitempty"` to `postMetadataPlatformPostV2`. Update encode/decode in both directions. Keep schema version `2` because this field is additive and optional.

Add:

```go
func PinterestEnvironment() string {
	if pinterestUseSandbox() { return "sandbox" }
	return "production"
}
```

Add handler helper:

```go
func stampPinterestDispatchEnvironments(posts []platform.PlatformPostInput, accounts map[string]platform.ValidateAccount) {
	for i := range posts {
		if accounts[posts[i].AccountID].Platform == "pinterest" {
			posts[i].DispatchEnvironment = platform.PinterestEnvironment()
		}
	}
}
```

Call it after account resolution and before every create, draft, update, schedule, and immediate encode path. Never accept a client-supplied marker.

- [ ] **Step 4: Attach metadata at dispatch and reject cross-environment writes**

Immediately before `adapter.Post`, wrap the dispatch context with `SocialAccountID` and the stored environment. In Pinterest `Post`:

```go
metadata, _ := DispatchMetadataFromContext(ctx)
if metadata.Environment != "" && metadata.Environment != PinterestEnvironment() {
	return nil, newProviderFailure(
		"The selected Pinterest board belongs to a different Pinterest environment.",
		map[string]any{"provider":"pinterest", "reason":"board_environment_mismatch"},
		FailureContract{ErrorCode:"target_not_found", Stage:"destination_preflight"},
	)
}
```

An empty marker is the legacy path and must continue to live board preflight.

- [ ] **Step 5: Run focused tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'DispatchEnvironment|DispatchMetadata|PinterestEnvironment' -count=1
```

Expected: `ok`.

- [ ] **Step 6: Commit environment isolation**

```bash
git add api/internal/platform/dispatch_context.go api/internal/platform/dispatch_context_test.go api/internal/platform/validate.go api/internal/platform/post_metadata.go api/internal/platform/post_metadata_test.go api/internal/platform/pinterest.go api/internal/handler/social_posts.go api/internal/handler/social_posts_drafts.go api/internal/handler/social_post_queue.go api/internal/handler/social_posts_test.go api/internal/handler/social_posts_drafts_test.go
git commit -m "fix: isolate Pinterest board environments"
```

## Task 3: Implement authoritative, cached, bounded board preflight

**Files:**

- Add: `api/internal/platform/pinterest_boards.go`
- Add: `api/internal/platform/pinterest_boards_test.go`
- Modify: `api/internal/platform/pinterest.go`
- Modify: `api/internal/platform/pinterest_test.go`

- [ ] **Step 1: Add failing HTTP-sequence tests**

Use an `httptest.Server` and assert exact request order/count. Cover:

1. `GET /user_account`, then `GET /boards/{id}`, then `POST /pins` for matching `owner.username`.
2. direct `404/code 40` stops before create Pin.
3. direct `403/code 29` stops before create Pin and does not request reconnect.
4. direct visible shared board falls back to `GET /boards?page_size=250`; it proceeds only if a returned item has both exact ID and matching owner username.
5. pagination finds a valid board on page 2 and short-circuits.
6. repeated bookmark, duplicate-page loop, page 9, or board 2,001 returns retriable temporary failure.
7. cache is isolated by account ID, environment, and token fingerprint; a changed token misses cache.
8. cache expires at 60 seconds through an injected clock.
9. an invalid board never calls `POST /pins`.

- [ ] **Step 2: Run adapter tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/platform -run 'PinterestBoardPreflight|PinterestBoardCache' -count=1
```

Expected: tests observe `POST /pins` without board reads.

- [ ] **Step 3: Add the board cache and proof model**

Define in `pinterest_boards.go`:

```go
const (
	pinterestBoardCacheTTL = 60 * time.Second
	pinterestBoardPageSize = 250
	pinterestBoardMaxPages = 8
	pinterestBoardMaxCount = 2000
)

type pinterestBoardOwner struct { Username string `json:"username"` }
type PinterestBoard struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Owner pinterestBoardOwner `json:"owner"`
}

type pinterestBoardCacheKey struct { AccountID, Environment string }
type pinterestBoardCacheEntry struct {
	TokenFingerprint string
	Username string
	OwnedIDs map[string]struct{}
	Complete bool
	ExpiresAt time.Time
}
```

Hash tokens with SHA-256 and retain only the digest. Never log it. Add mutex-protected cache methods and `InvalidateBoardCache(accountID string)`.

- [ ] **Step 4: Implement the proof algorithm**

`preflightBoard(ctx, accessToken, boardID, metadata)` performs:

1. Read authenticated account username via `/user_account`, using the cache when present.
2. Direct `GET /boards/{id}`.
3. Accept immediately only when exact ID and case-insensitive owner username match.
4. On ownership ambiguity, traverse `/boards?page_size=250` with bookmark validation.
5. Accept only a matching ID whose item owner also matches the operation username.
6. Detect repeated bookmarks, pages whose full ID set repeats, and count/page caps.
7. Return `temporary_platform_error` when proof cannot finish safely.

Call `preflightBoard` after local board syntax validation and before any media staging. Preserve the final create-Pin code `29`/`40` mapping for the race between read and write.

- [ ] **Step 5: Run race-enabled platform tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test -race ./internal/platform -run 'Pinterest' -count=1
```

Expected: `ok`, no race report.

- [ ] **Step 6: Commit destination preflight**

```bash
git add api/internal/platform/pinterest.go api/internal/platform/pinterest_test.go api/internal/platform/pinterest_boards.go api/internal/platform/pinterest_boards_test.go
git commit -m "fix: preflight Pinterest destination boards"
```

## Task 4: Correct board-handler reconnect semantics and cache invalidation

**Files:**

- Modify: `api/internal/handler/pinterest_boards.go`
- Add: `api/internal/handler/pinterest_boards_test.go`
- Modify: `api/internal/platform/registry.go`
- Add: `api/internal/platform/registry_test.go`
- Modify: `api/internal/handler/oauth.go`
- Modify: `api/internal/handler/oauth_test.go`
- Modify: `api/internal/handler/social_accounts.go`
- Add: `api/internal/handler/social_accounts_test.go`

- [ ] **Step 1: Add failing pure mapping tests**

Extract a pure helper `classifyPinterestBoardEndpointError(err error) pinterestBoardEndpointError`. Assert:

- `401/code 2` returns reconnect guidance.
- account-level missing-scope evidence returns permission-update guidance.
- resource `403/code 29` returns board-selection guidance and `reconnect=false`.
- `429` and `5xx` remain temporary.

- [ ] **Step 2: Run focused tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PinterestBoardEndpoint|PinterestBoardCacheInvalidation' -count=1
```

Expected: current blanket `(403)` logic marks code `29` reconnect-required.

- [ ] **Step 3: Replace blanket 403 detection and wire invalidation**

Use structured provider fields when available and sanitized legacy parsing otherwise. On successful board creation call `pinterestAdapter.InvalidateBoardCache(accountID)`.

Add a small lifecycle interface in `platform/registry.go`:

```go
type AccountStateInvalidator interface { InvalidateAccountState(string) }

func InvalidateAccountState(platformName, accountID string) {
	adapter, err := Get(platformName)
	if err != nil { return }
	if invalidator, ok := adapter.(AccountStateInvalidator); ok {
		invalidator.InvalidateAccountState(accountID)
	}
}
```

`PinterestAdapter.InvalidateAccountState` delegates to its board cache. Call the registry helper after successful OAuth reactivation/save and after successful disconnect. This keeps generic handlers free of Pinterest branches. Token fingerprints also prevent stale use if a lifecycle call is missed, but explicit invalidation remains required and tested.

- [ ] **Step 4: Run handler tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Pinterest' -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit handler semantics**

```bash
git add api/internal/handler/pinterest_boards.go api/internal/handler/pinterest_boards_test.go api/internal/platform/registry.go api/internal/platform/registry_test.go api/internal/handler/oauth.go api/internal/handler/oauth_test.go api/internal/handler/social_accounts.go api/internal/handler/social_accounts_test.go
git commit -m "fix: distinguish Pinterest board and token failures"
```

## Task 5: Preserve failure stage, retry decision, and safe observability

**Files:**

- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `api/internal/platform/dispatch_context.go`
- Modify: `api/internal/platform/dispatch_context_test.go`
- Modify: `api/internal/integrationlogs/normalize.go`
- Modify: `api/internal/integrationlogs/normalize_test.go`

- [ ] **Step 1: Add failing queue tests**

Construct typed failures for destination permanent and temporary cases. Assert:

- typed `FailureStage()` wins over legacy substring inference;
- permanent board failure records `destination_preflight`, one attempt, terminal dead state, and no retry schedule;
- rate limit/timeout records `destination_preflight` and follows existing retry scheduling;
- public message and structured provider JSON contain no access token or board API URL.

- [ ] **Step 2: Run focused tests and prove RED**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'DispatchFailureStage|Pinterest.*Retry|Pinterest.*Observability' -count=1
```

Expected: stage defaults to `dispatch` and Pinterest-specific telemetry is absent.

- [ ] **Step 3: Prefer the typed stage and add bounded events**

Change `inferDispatchFailureStage` to accept `error`, read the local interface below, then use existing legacy inference:

```go
type failureStageCarrier interface { FailureStage() string }
```

Add a bounded `DispatchEventRecorder` to `DispatchMetadata`. Pinterest appends start/success/failure records without logging; after `adapter.Post` returns, the queue converts the recorder snapshot into existing integration-log events. Cap the recorder at 16 entries and make it mutex-safe.

Add and emit these action constants through the existing integration-log sink:

- `pinterest_destination_preflight_started`
- `pinterest_destination_preflight_succeeded`
- `pinterest_destination_preflight_failed`
- `pinterest_create_pin_failed`

Dimensions are IDs already present in the dispatch context, environment, status/code/reason/stage/retriable, duration, and a SHA-256 board-ID fingerprint. Do not emit token, raw URL, request body, or provider body.

- [ ] **Step 4: Run queue and observability tests and prove GREEN**

```bash
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/platform ./internal/integrationlogs -run 'Pinterest|DispatchFailureStage|DispatchEvent' -count=1
```

Expected: `ok`.

- [ ] **Step 5: Commit retry and telemetry behavior**

```bash
git add api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_test.go api/internal/platform/dispatch_context.go api/internal/platform/dispatch_context_test.go api/internal/integrationlogs/normalize.go api/internal/integrationlogs/normalize_test.go
git commit -m "fix: preserve Pinterest preflight outcomes"
```

## Task 6: Make Dashboard selection live and failure copy structured

**Files:**

- Add: `dashboard/src/lib/pinterest-boards.ts`
- Add: `dashboard/src/lib/pinterest-boards.test.ts`
- Modify: `dashboard/src/components/posts/create-post/platform-fields/pinterest-fields.tsx`
- Modify: `dashboard/src/components/posts/create-post/create-post-drawer.tsx`
- Modify: `dashboard/src/lib/post-result-errors.ts`
- Modify: `dashboard/tests/post-result-errors.test.mts`
- Modify: `dashboard/tests/dashboard-regression.spec.ts`

- [ ] **Step 1: Add failing pure tests for selection reconciliation and freshness**

Define and test:

```ts
export type PinterestBoardSnapshot = {
  accountId: string;
  environment: "production" | "sandbox";
  fetchedAt: number;
  boardIds: readonly string[];
};

export function reconcilePinterestBoardSelection(
  selectedBoardId: string,
  previous: PinterestBoardSnapshot | null,
  current: PinterestBoardSnapshot,
): string;

export function isPinterestBoardSnapshotFresh(snapshot: PinterestBoardSnapshot, now: number): boolean;
```

TTL is 60 seconds. Account/environment change or missing selected ID returns `""`. Add structured result-copy tests for `target_not_found`, `auth_token_invalid`, and later-compatible `media_error/media_preflight`.

- [ ] **Step 2: Run Node tests and prove RED**

Run from `dashboard/`:

```bash
node --test src/lib/pinterest-boards.test.ts tests/post-result-errors.test.mts
```

Expected: missing module/functions and old generic copy.

- [ ] **Step 3: Implement selection and submission behavior**

In `pinterest-fields.tsx`, reconcile every list response against the previous snapshot and clear the field on account/environment change. Keep the selector non-freeform. Preserve the existing Create Board action; after success refresh, select the created ID, and attach the returned environment.

In `create-post-drawer.tsx`, refresh a stale snapshot before `createSocialPost`; if the selected ID is absent, clear it and block submission with:

```text
The selected Pinterest board is no longer available for this account. Choose another board and publish again.
```

The zero-board state disables submit for that account and renders:

```text
This Pinterest account has no available boards. Create a board before publishing a Pin.
```

`describePostResultFailure` branches only on structured `errorCode`, `failureStage`, `platform`, and `platformErrorCode`; it does not parse provider prose.

- [ ] **Step 4: Add Playwright regression cases**

Cover board load, zero-board disabled state, Create Board refresh/select, account change, environment change, pre-submit stale refresh, board failure copy, and token reconnect copy. Use intercepted API responses and assert no post request occurs after stale selection rejection.

- [ ] **Step 5: Run unit, build, and focused browser tests**

```bash
node --test src/lib/pinterest-boards.test.ts tests/post-result-errors.test.mts
npm run build
npx playwright test tests/dashboard-regression.spec.ts --grep 'Pinterest'
```

Expected: all commands exit `0`.

- [ ] **Step 6: Commit Dashboard behavior**

```bash
git add dashboard/src/lib/pinterest-boards.ts dashboard/src/lib/pinterest-boards.test.ts dashboard/src/components/posts/create-post/platform-fields/pinterest-fields.tsx dashboard/src/components/posts/create-post/create-post-drawer.tsx dashboard/src/lib/post-result-errors.ts dashboard/tests/post-result-errors.test.mts dashboard/tests/dashboard-regression.spec.ts
git commit -m "fix: keep Pinterest board selection authoritative"
```

## Task 7: Document the API behavior and verify Workstream A locally

**Files:**

- Add: `docs/pinterest-publishing-api.md`
- Modify: `docs/sdk-api-coverage-matrix.md`

- [ ] **Step 1: Add exact invalid-board and inaccessible-board examples**

Document unchanged request shape, board discovery endpoints, `destination_preflight`, code `29`/`40`, actions, retry semantics, and the internal-only environment marker. Do not document Workstream B external-media staging as available.

- [ ] **Step 2: Run repository hygiene and full local gates**

From `api/`:

```bash
GOCACHE=/tmp/unipost-go-build go test ./...
```

From `dashboard/`:

```bash
node --test src/lib/pinterest-boards.test.ts tests/post-result-errors.test.mts
npm run build
npm run test:regression:dashboard
```

From the repository root:

```bash
git diff --check
git status --short
```

Expected: every test/build exits `0`; `git diff --check` is silent; status contains only intended docs before the final docs commit.

- [ ] **Step 3: Commit documentation**

```bash
git add docs dashboard api
git diff --cached --name-only
git commit -m "docs: describe Pinterest destination failures"
```

The staged-name audit must contain only documentation/OpenAPI files not already committed.

## Task 8: Preserve the Workstream A checkpoint before Workstream B

- [ ] **Step 1: Record the exact A checkpoint**

```bash
git rev-parse HEAD
git log --oneline --decorate origin/main..HEAD
git diff --name-only origin/main...HEAD
```

Save the SHA, commit list, changed-file list, and local gate outputs in the Draft PR description under `Workstream A checkpoint`. Do not add generated evidence files to the repository.

- [ ] **Step 2: Push the owned branch and open/update the Draft PR to `dev`**

```bash
git push -u origin dev-pinterest-403-analysis
gh pr create --draft --base dev --head dev-pinterest-403-analysis --title "fix: harden Pinterest publishing" --body-file <reviewed-pr-body-file>
```

The temporary PR body file must be outside the repository. Monitor GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and browser acceptance for the exact A SHA.

- [ ] **Step 3: Enforce the A checkpoint rule**

If any required check fails, times out, skips, cannot start, or validates another SHA, treat it as failed and stop Workstream B until A is corrected and all gates pass on the replacement A SHA. Record run URLs and deployment URLs in the PR description.

- [ ] **Step 4: Continue to Workstream B only after A evidence is complete**

Workstream A may be described as “checkpoint accepted on the task branch.” It must not be described as deployed or complete because the single task PR has not yet merged to `dev`.
