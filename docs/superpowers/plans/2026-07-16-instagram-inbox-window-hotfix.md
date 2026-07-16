# Instagram Inbox Window Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Repair Instagram messaging webhook subscriptions and make inbox reply-window enforcement accurate and actionable.

**Architecture:** A shared backend subscriber owns the Meta `subscribed_apps` request. The Connect callback invokes it for new accounts, while the inbox worker idempotently repairs existing accounts once per process. A small dashboard helper centralizes the 24-hour rule for both Instagram and Facebook DMs.

**Tech Stack:** Go 1.x, `net/http`, PostgreSQL/sqlc models, Next.js 16, TypeScript, Node test runner, Playwright.

---

### Task 1: Shared Instagram webhook subscriber

**Files:**
- Create: `api/internal/instagramwebhooks/subscriber.go`
- Create: `api/internal/instagramwebhooks/subscriber_test.go`

- [ ] **Step 1: Write the failing success-request test**

Create an `httptest.Server`, invoke:

```go
subscriber := NewSubscriber(server.Client(), server.URL)
err := subscriber.Subscribe(context.Background(), "ig_123", "token_123")
```

Assert a `POST` to `/ig_123/subscribed_apps`, form fields containing
`messages,messaging_postbacks,comments`, the access token, and no error for
`{"success":true}`.

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd api
go test ./internal/instagramwebhooks -run TestSubscriberSubscribe -v
```

Expected: FAIL because the package and `NewSubscriber` do not exist.

- [ ] **Step 3: Implement the minimal subscriber**

Add:

```go
type Subscriber struct {
    client    *http.Client
    graphBase string
}

func NewSubscriber(client *http.Client, graphBase string) *Subscriber
func (s *Subscriber) Subscribe(ctx context.Context, accountID, accessToken string) error
```

Encode the fields and token as `application/x-www-form-urlencoded`, require
HTTP 200 and `success: true`, and include Meta's response body in errors.

- [ ] **Step 4: Add and pass the failure-response test**

Assert a non-200 Meta response produces an error containing the status and
response body. Run:

```bash
go test ./internal/instagramwebhooks -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/instagramwebhooks
git commit -m "fix: subscribe instagram accounts to messaging webhooks"
```

### Task 2: Subscribe new Instagram Connect accounts

**Files:**
- Modify: `api/internal/handler/connect_callback.go`
- Modify: `api/internal/handler/connect_sessions_test.go`

- [ ] **Step 1: Write the failing callback success test**

Add a fake subscriber recording `accountID` and token. Run an Instagram
callback with the existing fake connector and assert the subscriber receives
the fetched profile ID and exchanged access token before the session is marked
complete.

- [ ] **Step 2: Verify RED**

Run:

```bash
cd api
go test ./internal/handler -run TestConnectCallback_Instagram -v
```

Expected: FAIL because `ConnectCallbackHandler` does not invoke a subscriber.

- [ ] **Step 3: Implement subscriber injection and invocation**

Add an interface:

```go
type instagramWebhookSubscriber interface {
    Subscribe(context.Context, string, string) error
}
```

Initialize the production subscriber in `NewConnectCallbackHandler`. After the
account row is saved, subscribe Instagram accounts before completing the
Connect session.

- [ ] **Step 4: Add failure behavior test**

Return a fake subscription error and assert:

- the account is marked `reconnect_required`;
- the session is not completed;
- the callback returns a connect error with reason
  `webhook_subscription_failed`.

- [ ] **Step 5: Run handler tests**

```bash
go test ./internal/handler -run 'TestConnectCallback_Instagram|TestConnectCallbackReuses' -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add api/internal/handler/connect_callback.go api/internal/handler/connect_sessions_test.go
git commit -m "fix: require instagram webhook subscription on connect"
```

### Task 3: Repair existing active Instagram accounts

**Files:**
- Modify: `api/internal/worker/inbox_sync.go`
- Create: `api/internal/worker/inbox_sync_subscription_test.go`

- [ ] **Step 1: Write failing worker subscription-state tests**

Test a focused worker method with a fake subscriber:

- first success subscribes and caches the account ID;
- a second call for the same account does not call Meta again;
- a failure is not cached and the next call retries;
- non-Instagram accounts are ignored.

- [ ] **Step 2: Verify RED**

Run:

```bash
cd api
go test ./internal/worker -run TestInboxSyncWorkerEnsureInstagramWebhookSubscription -v
```

Expected: FAIL because the worker has no subscriber or cache.

- [ ] **Step 3: Implement minimal repair logic**

Add the shared subscriber and:

```go
igWebhookSubscriptions map[string]bool
```

Call the focused ensure method before processing each active Instagram
account. Log failures but continue normal polling.

- [ ] **Step 4: Run worker tests**

```bash
go test ./internal/worker -run TestInboxSyncWorkerEnsureInstagramWebhookSubscription -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add api/internal/worker/inbox_sync.go api/internal/worker/inbox_sync_subscription_test.go
git commit -m "fix: repair instagram webhook subscriptions during inbox sync"
```

### Task 4: Enforce and explain the 24-hour DM window

**Files:**
- Modify: `api/internal/handler/inbox.go`
- Modify: `api/internal/handler/inbox_test.go` or create a focused handler test file
- Create: `dashboard/src/app/(dashboard)/projects/[id]/inbox/reply-window.ts`
- Create: `dashboard/tests/inbox-reply-window.test.mjs`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx`

- [ ] **Step 1: Write the failing dashboard helper tests**

Test:

```ts
isDMReplyWindowClosed("ig_dm", inbound23HoursAgo, now) === false
isDMReplyWindowClosed("ig_dm", inbound25HoursAgo, now) === true
isDMReplyWindowClosed("fb_dm", inbound25HoursAgo, now) === true
isDMReplyWindowClosed("ig_comment", inbound25HoursAgo, now) === false
```

- [ ] **Step 2: Verify dashboard RED**

Run:

```bash
cd dashboard
node --test tests/inbox-reply-window.test.mjs
```

Expected: FAIL because the helper does not exist.

- [ ] **Step 3: Implement and wire the helper**

Export a pure helper using a 24-hour constant and call it from the inbox
composer. Replace Facebook-specific copy with:

`Reply window closed — this person last messaged over 24 hours ago. They need to message this account again before you can reply.`

- [ ] **Step 4: Write the failing backend window/error tests**

Add tests for:

- selected `ig_dm` inbound older than 24 hours returns `PLATFORM_ERROR`
  without invoking Meta;
- an error containing `2534022` maps to the actionable Instagram recovery
  message and does not mark the account `reconnect_required`.

- [ ] **Step 5: Verify backend RED**

Run:

```bash
cd api
go test ./internal/handler -run 'TestInboxReply.*Window|TestInboxReply.*2534022' -v
```

Expected: FAIL because Instagram window handling is absent.

- [ ] **Step 6: Implement minimal backend handling**

Before dispatching a DM send, reject selected inbound items older than 24
hours. Map `2534022` to:

`Instagram DM reply failed because Meta considers this conversation outside the 24-hour reply window. Ask the Instagram user to send a new message, then retry.`

- [ ] **Step 7: Run focused tests**

```bash
cd api
go test ./internal/handler -run 'TestInboxReply.*Window|TestInboxReply.*2534022' -v
cd ../dashboard
node --test tests/inbox-reply-window.test.mjs
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add api/internal/handler/inbox.go api/internal/handler/*inbox*test.go \
  dashboard/src/app/'(dashboard)'/projects/'[id]'/inbox/reply-window.ts \
  dashboard/src/app/'(dashboard)'/projects/'[id]'/inbox/page.tsx \
  dashboard/tests/inbox-reply-window.test.mjs
git commit -m "fix: enforce instagram inbox reply windows"
```

### Task 5: Full local verification

**Files:**
- Verify all changed files.

- [ ] **Step 1: Run full backend validation**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run dashboard unit and build validation**

```bash
cd dashboard
node --test tests/inbox-reply-window.test.mjs
npm run build
```

Expected: PASS.

- [ ] **Step 3: Run dashboard regression validation**

```bash
npm run test:regression:dashboard
```

Expected: PASS when Playwright browsers are installed.

- [ ] **Step 4: Review diff and commit any test-only corrections**

```bash
git status --short
git diff --check
git log --oneline origin/staging..HEAD
```

Expected: only focused hotfix changes.

### Task 6: Staging promotion and verification

- [ ] **Step 1: Merge hotfix into local staging**

Fetch origin, update local `staging` from `origin/staging`, merge
`hotfix-instagram-inbox-window`, and rerun Task 5 validation.

- [ ] **Step 2: Push staging**

```bash
git push origin staging
```

- [ ] **Step 3: Monitor all triggered checks and deployments**

Wait for GitHub Actions, Railway staging, and Vercel staging to finish
successfully.

- [ ] **Step 4: Verify real staging**

Open `https://staging-app.unipost.dev`, verify the inbox composer behavior, and
confirm a staging Instagram account's `subscribed_apps` edge contains the
messaging fields.

### Task 7: Production promotion and verification

- [ ] **Step 1: Create and merge the production PR**

Create a PR from `staging` to `main`, wait for checks, and merge it.

- [ ] **Step 2: Monitor production deployments**

Wait for GitHub Actions, Railway production, and Vercel production to finish.

- [ ] **Step 3: Verify production**

Confirm:

- `https://api.unipost.dev` is healthy;
- `https://app.unipost.dev` loads the affected inbox route;
- the `robynsocial` account's `subscribed_apps` edge contains the UniPost app
  and messaging fields;
- expired Instagram threads are blocked with the new copy;
- raw `2534022` responses are replaced with the actionable message.

Do not send an external Instagram message without explicit message-content
authorization. If no inbound message arrived after subscription repair, ask
the user to have the correspondent send a new DM for the final successful-send
acceptance check.

### Task 8: Sync the hotfix back to development

- [ ] **Step 1: Apply the production hotfix to local dev**

Update local `dev` from `origin/dev`, merge or cherry-pick the hotfix commits,
resolve only cleanly applicable conflicts, and rerun Task 5 validation.

- [ ] **Step 2: Push and monitor development**

Push `dev` to `origin/dev`, wait for Railway/Vercel development deployments,
and verify `https://dev-app.unipost.dev` plus the relevant API behavior.

