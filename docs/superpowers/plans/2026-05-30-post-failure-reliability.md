# Post Failure Reliability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce recurring publish failures from the 2026-05-23 through 2026-05-30 log review by improving classification, adapter diagnostics, worker source attribution, and deterministic YouTube quota handling.

**Architecture:** Keep the first implementation narrow and backend-focused. Reuse existing `post_failures`, `post_delivery_jobs`, `social_post_results`, and `integration_logs` tables; encode new diagnostic evidence in sanitized error strings and existing structured codes instead of adding schema. No feature flags are added because the user explicitly decided this rollout should not be flag-protected.

**Tech Stack:** Go, sqlc-generated data layer, existing platform adapters, existing `postfailures` taxonomy, existing integration logs.

---

## Scope

Implement this PRD slice now:

- Failure taxonomy and retry decisions for known YouTube/TikTok/Instagram/Threads cases.
- Threads `getUserID` non-2xx/decode/empty-body diagnostics.
- Instagram container poll diagnostics, with timeout still retriable and explicit `ERROR` treated as media/terminal.
- Worker source attribution for background delivery log events.
- TikTok photo `invalid_params` fallback/wrapped diagnostics.
- YouTube upload quota circuit breaker with in-process reset at next midnight Pacific.

Defer:

- Polished admin reporting UI.
- New DB columns.
- Redis-backed/shared YouTube breaker state.
- Deep TikTok codec probing.
- Dashboard recovery UI changes.

## Files

- Modify: `api/internal/postfailures/taxonomy.go`
- Create: `api/internal/postfailures/taxonomy_test.go`
- Modify: `api/internal/platform/threads.go`
- Create: `api/internal/platform/threads_test.go`
- Modify: `api/internal/platform/instagram.go`
- Modify: `api/internal/platform/instagram_test.go`
- Modify: `api/internal/platform/tiktok.go`
- Modify: `api/internal/platform/tiktok_test.go`
- Modify: `api/internal/platform/youtube.go`
- Create: `api/internal/platform/youtube_test.go`
- Modify: `api/internal/handler/social_posts.go`
- Modify: `api/internal/handler/social_post_queue.go`
- Modify: `api/internal/handler/social_post_queue_test.go`
- Modify: `docs/prd-post-failure-reliability.md`

---

### Task 1: Taxonomy Coverage

- [ ] **Step 1: Write failing tests**

Create `api/internal/postfailures/taxonomy_test.go` with table cases for:

```go
func TestClassifyKnownPublishFailures(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		code      string
		retriable bool
	}{
		{name: "tiktok file format", raw: "tiktok publish failed: file_format_check_failed", code: "media_error", retriable: false},
		{name: "tiktok invalid params", raw: `tiktok photo init (400): {"error":{"code":"invalid_params"}}`, code: "validation_error", retriable: false},
		{name: "youtube upload quota", raw: `youtube upload init failed (429): {"error":{"status":"RESOURCE_EXHAUSTED","errors":[{"reason":"rateLimitExceeded"}],"message":"The request cannot be completed because you have exceeded your quota. Video Uploads per day"}}`, code: "quota_exceeded", retriable: false},
		{name: "threads invalid token", raw: `threads get user id failed (401): {"error":{"message":"Invalid OAuth access token"}}`, code: "account_reconnect_required", retriable: false},
		{name: "threads missing permission", raw: `threads get user id failed (403): {"error":{"message":"Missing required permission threads_basic"}}`, code: "missing_permission", retriable: false},
		{name: "instagram timeout", raw: "instagram container processing timed out: container_id=123 poll_count=30 elapsed_ms=60000", code: "temporary_platform_error", retriable: true},
		{name: "instagram error", raw: "instagram container processing failed: container_id=123 status_code=ERROR", code: "media_error", retriable: false},
	}
}
```

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/postfailures
```

Expected: failures for TikTok file format, TikTok invalid params, Threads auth/permission, and Instagram `ERROR`.

- [ ] **Step 3: Implement classification rules**

Update `Classify` so provider-specific known strings are checked before broad `timeout`, `token`, `permission`, and `media` branches.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/postfailures
```

Expected: PASS.

### Task 2: Threads User Lookup Diagnostics

- [ ] **Step 1: Write failing tests**

Create `api/internal/platform/threads_test.go` with `httptest.Server` and a temporary `http.Client` rewrite transport. Tests should cover:

- 401 body returns an error containing `threads get user id failed (401)` and provider body.
- 403 body returns an error containing `threads get user id failed (403)`.
- malformed JSON returns `threads get user id decode`.
- 200 without `id` returns `threads get user id: empty id`.

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestThreadsGetUserID'
```

Expected: failures because current code ignores status and decode errors.

- [ ] **Step 3: Implement diagnostics**

Change `getUserID` to read the body, check non-2xx status, unmarshal with error handling, and include sanitized status/body context in returned errors.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestThreadsGetUserID'
```

Expected: PASS.

### Task 3: Instagram Container Diagnostics

- [ ] **Step 1: Write failing tests**

Add tests in `api/internal/platform/instagram_test.go` using a fake transport:

- explicit `ERROR` returns an error with `container_id`, `status_code=ERROR`, `poll_count`, `elapsed_ms`, and response body context.
- repeated `IN_PROGRESS` with a test poll config returns timeout with `container_id`, `poll_count`, and `elapsed_ms`.
- malformed JSON returns a poll diagnostic error.

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestInstagramWaitForContainer'
```

Expected: failures because current messages are generic.

- [ ] **Step 3: Implement poll diagnostics**

Add package-level poll interval/attempt variables or a small helper struct so tests can run without waiting 60 seconds. Include container ID, poll count, elapsed milliseconds, last HTTP status, status code, and truncated body in error messages.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestInstagramWaitForContainer|TestClassifyKnownPublishFailures'
```

Expected: PASS.

### Task 4: TikTok Photo Init Hardening

- [ ] **Step 1: Write failing tests**

Extend `api/internal/platform/tiktok_test.go`:

- `wrapTikTokInitError` message includes sandbox guidance for `invalid_params`.
- photo init 400 `invalid_params` retries once with `SELF_ONLY` when initial privacy is non-`SELF_ONLY`.

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestTikTok.*(Photo|Init|SelfOnly)'
```

Expected: photo fallback test fails because only video init has the fallback.

- [ ] **Step 3: Implement media proxy test seam and photo fallback**

Use a small unexported interface for `UploadFromURL`; keep `SetMediaProxy(*storage.Client)` compatible. Split photo init into `postPhotoWithPrivacy(..., allowRetry bool)` and reuse `shouldRetryTikTokWithSelfOnly` plus `wrapTikTokInitError`.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestTikTok.*(Photo|Init|SelfOnly)'
```

Expected: PASS.

### Task 5: YouTube Upload Quota Breaker

- [ ] **Step 1: Write failing tests**

Create `api/internal/platform/youtube_test.go` with:

- after a qualifying upload-init 429, the adapter opens a breaker until next midnight Pacific.
- a second upload attempt while the breaker is open returns a quota error before downloading the video URL.
- non-quota 429 does not open the breaker.

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestYouTube.*Quota'
```

Expected: failures because no breaker exists.

- [ ] **Step 3: Implement in-process breaker**

Add a goroutine-safe breaker struct in `youtube.go`, classify Google quota bodies with `RESOURCE_EXHAUSTED`, `rateLimitExceeded`, or `Video Uploads per day`, compute next midnight in `America/Los_Angeles`, check breaker before video download, and open breaker after a qualifying 429.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/platform -run 'TestYouTube.*Quota|TestClassifyKnownPublishFailures'
```

Expected: PASS.

### Task 6: Worker Source Attribution

- [ ] **Step 1: Write failing tests**

Add tests in `api/internal/handler/social_post_queue_test.go` for a pure helper:

```go
func TestDeliveryWorkerEventSourceIsWorker(t *testing.T) {
	event := workerPublishingEvent(integrationlogs.Event{Action: integrationlogs.ActionPostPublishPlatformFailed})
	if event.Source != integrationlogs.SourceWorker {
		t.Fatalf("source = %q, want worker", event.Source)
	}
}
```

Also test dashboard/API inference remains unchanged for request-path events through a new helper in `social_posts.go`.

- [ ] **Step 2: Run red test**

Run:

```bash
cd api
go test ./internal/handler -run 'Test.*Source'
```

Expected: failures until helper exists and queue events opt in.

- [ ] **Step 3: Implement source helpers**

Add `workerPublishingEvent` in `social_post_queue.go` and wrap worker calls to `h.logPublishingEvent`. Extract source inference in `social_posts.go` so request-path behavior remains covered.

- [ ] **Step 4: Run green test**

Run:

```bash
cd api
go test ./internal/handler -run 'Test.*Source|TestRetryDeliveryJobNowMarksDeprecated'
```

Expected: PASS.

### Task 7: Docs and Full Verification

- [ ] **Step 1: Update PRD rollout note**

Edit `docs/prd-post-failure-reliability.md` to record the implementation decision: no feature flags for this run; tests cover both new behavior and unchanged request-path source inference.

- [ ] **Step 2: Format Go**

Run:

```bash
gofmt -w api/internal/postfailures/taxonomy.go api/internal/postfailures/taxonomy_test.go api/internal/platform/threads.go api/internal/platform/threads_test.go api/internal/platform/instagram.go api/internal/platform/instagram_test.go api/internal/platform/tiktok.go api/internal/platform/tiktok_test.go api/internal/platform/youtube.go api/internal/platform/youtube_test.go api/internal/handler/social_posts.go api/internal/handler/social_post_queue.go api/internal/handler/social_post_queue_test.go
```

- [ ] **Step 3: Run package tests**

Run:

```bash
cd api
go test ./internal/postfailures ./internal/platform ./internal/handler
```

- [ ] **Step 4: Run full API test suite**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

- [ ] **Step 5: Merge to local dev, push, and monitor**

After tests pass, merge the task branch to local `dev`, rerun affected tests on `dev`, push `dev` to `origin/dev`, and monitor checks/deployments.

- [ ] **Step 6: Validate development pages**

Use development domains only:

- `https://dev-api.unipost.dev`
- `https://dev-app.unipost.dev`

Verify the app loads and a core dashboard page still renders after the deployment.

- [ ] **Step 7: Merge dev to staging and monitor**

After dev validation passes, merge `dev` to `staging`, push `staging`, monitor checks/deployments, and validate staging using staging domains only.

---

## Plan Review

- Spec coverage: covers Phase 1 plus YouTube breaker and TikTok photo init hardening. Defers admin UI, Redis breaker persistence, schema columns, and dashboard recovery UI.
- No feature flags: explicitly matches the user's decision on 2026-05-30.
- Test policy: every behavior change starts with a failing package test and ends with full `go test ./...`.
