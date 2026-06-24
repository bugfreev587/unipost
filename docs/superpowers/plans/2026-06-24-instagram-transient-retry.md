# Instagram Transient Retry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Treat Meta/Instagram transient final publish failures as retriable so a one-off Meta 500 does not immediately dead-end a post.

**Architecture:** Keep the change in the publish failure taxonomy because delivery retry behavior already depends on `postfailures.Classify(...).IsRetriable`. The worker should continue to create retry jobs through the existing `handleJobDispatchFailure` path when classification returns `temporary_platform_error` and `IsRetriable=true`.

**Tech Stack:** Go backend, `testing` package, PostgreSQL/sqlc persistence model, existing UniPost standard release flow.

---

## File Structure

- Modify: `api/internal/postfailures/taxonomy_test.go`
  - Add regression cases for the exact Instagram `media_publish` failure shape seen in production: `OAuthException`, `code:2`, `is_transient:true`, and "Please retry your request later."
- Modify: `api/internal/postfailures/taxonomy.go`
  - Add narrow Meta transient detection before generic platform errors.
  - Preserve existing reconnect, permission, media, quota, and validation classification behavior.
- No frontend changes
  - The Users drawer is correctly displaying persisted `error_message`; this fix changes future backend classification/retry behavior, not the UI copy.

---

### Task 1: Create Regression Tests For Meta Transient Publish Failures

**Files:**
- Modify: `api/internal/postfailures/taxonomy_test.go`

- [ ] **Step 1: Add failing test cases**

In `TestClassifyKnownPublishFailures`, add these table entries before the `"instagram timeout"` case:

```go
{
	name: "instagram transient media publish oauth code 2",
	raw:  `publish failed (500): {"error":{"message":"An unexpected error has occurred. Please retry your request later.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"AJ4uhascsOC2cf1lq0bwhgJ"}}`,
	code: "temporary_platform_error",
	retriable: true,
},
{
	name: "instagram transient flag without retry wording",
	raw:  `publish failed (500): {"error":{"message":"An unexpected error has occurred.","type":"OAuthException","is_transient":true,"code":2,"fbtrace_id":"TRACE"}}`,
	code: "temporary_platform_error",
	retriable: true,
},
{
	name: "meta retry later wording",
	raw:  `publish failed (500): {"error":{"message":"Please retry your request later.","type":"OAuthException","code":2}}`,
	code: "temporary_platform_error",
	retriable: true,
},
```

- [ ] **Step 2: Run the focused test and confirm it fails**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./internal/postfailures -run TestClassifyKnownPublishFailures -count=1
```

Expected before implementation:

```text
FAIL: ErrorCode = "platform_error", want "temporary_platform_error"
```

---

### Task 2: Implement Narrow Meta Transient Classification

**Files:**
- Modify: `api/internal/postfailures/taxonomy.go`

- [ ] **Step 1: Add helper functions near `isMetaOAuthReconnectError`**

Add these helpers after `isMetaOAuthReconnectError`:

```go
func isMetaTransientError(s string) bool {
	if strings.Contains(s, `"is_transient":true`) || strings.Contains(s, `"is_transient": true`) {
		return true
	}
	if strings.Contains(s, "please retry your request later") {
		return true
	}
	return strings.Contains(s, "oauthexception") &&
		(strings.Contains(s, `"code":2`) || strings.Contains(s, `"code": 2`)) &&
		(strings.Contains(s, "unexpected error") || strings.Contains(s, "retry"))
}
```

- [ ] **Step 2: Use the helper in `Classify`**

Insert this case immediately after `case isMetaOAuthReconnectError(s):`:

```go
case isMetaTransientError(s):
	c.ErrorCode = "temporary_platform_error"
	c.IsRetriable = true
```

The ordering matters: OAuth code 190 reconnect errors must still win over generic Meta transient handling.

- [ ] **Step 3: Run the focused taxonomy test**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./internal/postfailures -run TestClassifyKnownPublishFailures -count=1
```

Expected after implementation:

```text
ok  	github.com/xiaoboyu/unipost-api/internal/postfailures
```

---

### Task 3: Verify Retry Behavior Through Existing Worker Logic

**Files:**
- Read-only verification: `api/internal/handler/social_post_queue.go`

- [ ] **Step 1: Confirm retry branch still keys off taxonomy**

Inspect `handleJobDispatchFailure` and confirm these existing lines remain true:

```go
failure := postfailures.BuildParams(...)
anotherAttempt := failure.IsRetriable && (job.Kind == "dispatch" || job.Attempts < job.MaxAttempts)
```

Expected: No handler code change is needed because `BuildParams` already calls `Classify`, and retriable dispatch failures already create a `kind='retry'` pending delivery job.

- [ ] **Step 2: Run handler regression tests that cover adjacent publish failure helpers**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Test(PostFailureShouldMarkReconnectRequired|InlineRefreshFailureShouldAbortPublish|WorkerPublishingEventSourceIsWorker)' -count=1
```

Expected:

```text
ok  	github.com/xiaoboyu/unipost-api/internal/handler
```

---

### Task 4: Full Backend Validation On Task Branch

**Files:**
- No source edits

- [ ] **Step 1: Run backend test suite**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected:

```text
ok/fail package output with final exit status 0
```

- [ ] **Step 2: Review local diff**

Run:

```bash
cd /Users/xiaoboyu/unipost
git diff -- api/internal/postfailures/taxonomy.go api/internal/postfailures/taxonomy_test.go
git status --short --branch
```

Expected:

```text
Only taxonomy.go, taxonomy_test.go, and this plan file are modified/added by this task.
Pre-existing untracked files remain unstaged.
```

---

### Task 5: Merge To Local `dev` And Re-Validate

**Files:**
- Git branch state only

- [ ] **Step 1: Commit focused task branch changes**

Run:

```bash
cd /Users/xiaoboyu/unipost
git add api/internal/postfailures/taxonomy.go api/internal/postfailures/taxonomy_test.go docs/superpowers/plans/2026-06-24-instagram-transient-retry.md
git commit -m "fix: retry transient instagram publish failures"
```

Expected:

```text
[dev-instagram-transient-retry ...] fix: retry transient instagram publish failures
```

- [ ] **Step 2: Update local `dev` from `origin/dev`**

Before switching, run:

```bash
cd /Users/xiaoboyu/unipost
git status --short --branch
```

Expected: no unstaged tracked task changes.

Then run:

```bash
git switch dev
git pull --ff-only origin dev
```

Expected: local `dev` is exactly up to date with `origin/dev`.

- [ ] **Step 3: Merge task branch into local `dev`**

Run:

```bash
git merge --no-ff dev-instagram-transient-retry
```

Expected:

```text
Merge made by the 'ort' strategy.
```

- [ ] **Step 4: Re-run backend validation on local `dev`**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: exit status 0.

---

### Task 6: Push `dev` And Verify Development Deployment

**Files:**
- Remote branch/deployment state only

- [ ] **Step 1: Push local `dev` to `origin/dev`**

Run:

```bash
cd /Users/xiaoboyu/unipost
git push origin dev
```

Expected: push succeeds and triggers development checks/deployments.

- [ ] **Step 2: Monitor triggered checks/deployments**

Use the available repo/deployment tooling in this order:

```bash
gh run list --branch dev --limit 10
gh run watch <run-id> --exit-status
```

Then inspect Vercel/Railway deployment status through the configured connectors or CLIs. Expected: all triggered checks and development deployments finish successfully.

- [ ] **Step 3: Self-acceptance in real dev environment**

Use development domains only:

```text
https://dev-api.unipost.dev
https://dev.unipost.dev
https://dev-app.unipost.dev
```

Acceptance criteria:

```text
Backend code deployed to dev includes the new taxonomy behavior.
A Meta publish error containing "is_transient":true or "Please retry your request later" is now classified as temporary_platform_error, is_retriable=true, next_action=retry_later.
The admin Users drawer still displays the raw stored failure message for historical failed posts.
```

Practical verification command if no safe live publish replay is available:

```bash
curl -fsS https://dev-api.unipost.dev/health
```

Then confirm the deployed commit SHA from the deployment provider matches the pushed `origin/dev` commit. If a safe API/debug path exists in the deployed environment, verify the exact classification there; otherwise document that local backend tests are the behavioral proof and dev health/deployed SHA are the environment proof.

---

### Task 7: Promote `dev` To `staging`

**Files:**
- Remote PR/deployment state only

- [ ] **Step 1: Run local CI-equivalent checks before promotion PR**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: exit status 0.

- [ ] **Step 2: Create PR from `dev` to `staging`**

Run:

```bash
cd /Users/xiaoboyu/unipost
gh pr create --base staging --head dev --title "Promote dev to staging: Instagram transient retry" --body "Promotes the Instagram transient publish retry classification fix from dev to staging."
```

Expected: PR URL returned.

- [ ] **Step 3: Wait for PR checks, merge, and monitor staging deployment**

Run:

```bash
gh pr checks <pr-number> --watch
gh pr merge <pr-number> --merge --delete-branch=false
```

Then monitor staging Vercel/Railway deployments until complete.

- [ ] **Step 4: Self-acceptance in staging**

Use staging domains only:

```text
https://staging-api.unipost.dev
https://staging.unipost.dev
https://staging-app.unipost.dev
```

Acceptance criteria:

```text
Staging deployment commit includes the merged staging commit.
API health is good.
The changed backend classification is present in the staging-deployed code path, or local test evidence is tied to the deployed commit SHA when no safe live replay endpoint exists.
```

---

### Task 8: Promote `staging` To Production `main`

**Files:**
- Remote PR/deployment state only

- [ ] **Step 1: Run local CI-equivalent checks before production PR**

Run:

```bash
cd /Users/xiaoboyu/unipost/api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: exit status 0.

- [ ] **Step 2: Create PR from `staging` to `main`**

Run:

```bash
cd /Users/xiaoboyu/unipost
gh pr create --base main --head staging --title "Promote staging to production: Instagram transient retry" --body "Promotes the Instagram transient publish retry classification fix from staging to production."
```

Expected: PR URL returned.

- [ ] **Step 3: Wait for PR checks, merge, and monitor production deployment**

Run:

```bash
gh pr checks <pr-number> --watch
gh pr merge <pr-number> --merge --delete-branch=false
```

Then monitor production Vercel/Railway deployments until complete.

- [ ] **Step 4: Production health and critical-flow verification**

Use production domains only:

```text
https://api.unipost.dev
https://unipost.dev
https://app.unipost.dev
```

Acceptance criteria:

```text
Production API health is good.
Production deployment commit includes the merged main commit.
Existing admin failure drawer behavior remains readable for historical failed posts.
Future Meta transient final publish failures will be retried by the backend because the deployed taxonomy returns temporary_platform_error and is_retriable=true.
```

---

## Self-Review

- Spec coverage: The plan fixes the investigated root cause, validates backend behavior, preserves UI behavior, and follows UniPost's standard release flow through dev, staging, and production.
- Placeholder scan: No task uses TBD/TODO/fill-in language. PR numbers and deployment IDs are runtime values discovered during execution.
- Type consistency: The plan uses existing `Classification.ErrorCode`, `Classification.IsRetriable`, `BuildParams`, and `handleJobDispatchFailure` semantics without adding new public types.
