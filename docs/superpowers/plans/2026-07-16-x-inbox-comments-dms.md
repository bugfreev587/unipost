# X Inbox Comments and Legacy DMs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship PRD Phase 2–3 on `dev`: X public replies and legacy DMs in UniPost Inbox, protected by atomic monthly usage and inbound daily caps, with capability-specific reconnect UX and complete bidirectionally linked docs.

**Architecture:** Keep public replies and private DMs on separate X delivery mechanisms: one app-level Filtered Stream consumer for `x_reply`, and X Activity `dm.received` subscriptions plus bounded DM lookup for `x_dm`. Route both through one shared X Inbox ingestion service that deduplicates before atomic inbound admission, persists normalized Inbox items, and publishes existing WebSocket notifications. Native Dashboard OAuth moves to stored S256 PKCE and both OAuth paths request/persist the same scopes; missing DM scopes disable DM capability only and never disable existing publishing.

**Tech Stack:** Go 1.24, chi, pgx/sqlc, PostgreSQL/goose, Next.js 16, React 19, TypeScript, Node source tests, Playwright, Railway dev, X API v2.

---

## Scope and launch gates

- This plan implements approved PRD phases 2 and 3 only.
- It does not implement buckets, a customer ledger, Stripe Top-up, Auto top-up, or XChat.
- Development uses `TWITTER_BEARER_TOKEN` for Filtered Stream/X Activity management, `TWITTER_CONSUMER_SECRET` for webhook CRC/signatures, and the independent stable `X_INBOX_WEBHOOK_ROUTE_SECRET` for the managed webhook URL generation. These are distinct from `TWITTER_CLIENT_ID` and `TWITTER_CLIENT_SECRET`.
- Workspace X Platform Credentials gain optional encrypted app-level Bearer Token and Consumer Secret fields. BYO X publishing can continue with only Client ID/Secret, but BYO Inbox capability stays disabled with explicit missing-credential guidance until all four values are present.
- A real public test reply or DM must use dedicated test accounts and explicit user approval. Automated acceptance otherwise uses read/sync operations and mocked upstream writes.
- Disconnecting an X account or deleting a workspace must idempotently remove its Filtered Stream rule and Activity subscription before applying the existing Inbox data-deletion policy.
- No staging or production promotion is part of this plan.

## Task 1: Align X OAuth scopes, stored S256 PKCE, and granted-scope persistence

**Files:**
- Create: `api/internal/db/migrations/108_x_inbox_oauth_and_delivery.sql`
- Modify: `api/internal/db/queries/oauth_states.sql`
- Regenerate: `api/internal/db/oauth_states.sql.go`, `api/internal/db/models.go`
- Modify: `api/internal/platform/oauth.go`
- Modify: `api/internal/platform/twitter.go`
- Modify: `api/internal/platform/twitter_test.go`
- Modify: `api/internal/connect/twitter.go`
- Modify: `api/internal/connect/twitter_test.go`
- Modify: `api/internal/handler/oauth.go`
- Test: `api/internal/handler/oauth_test.go`

- [ ] **Step 1: Write failing adapter tests**

Add tests proving both X connectors request:

```text
tweet.read tweet.write users.read offline.access media.write dm.read dm.write
```

and proving native OAuth uses a stored random verifier with `code_challenge_method=S256`, derives `scope` from `OAuthConfig.Scopes`, parses the token response `scope`, and falls back to configured scopes only when X omits it.

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/connect ./internal/handler -run 'Twitter|OAuthPKCE' -count=1
```

Expected: failures for missing DM scopes, `plain` PKCE, and empty persisted scopes.

- [ ] **Step 3: Add stored verifier schema and config field**

Migration 108 adds nullable `pkce_verifier TEXT` to `oauth_states`. Extend `CreateOAuthState` and `OAuthConfig` with the verifier. The handler generates at least 64 random bytes for Twitter, stores the base64url verifier, and passes it to both authorization and exchange.

- [ ] **Step 4: Implement S256 and scope parsing**

Use:

```go
sum := sha256.Sum256([]byte(config.PKCEVerifier))
challenge := base64.RawURLEncoding.EncodeToString(sum[:])
```

Build native authorization scopes with `strings.Join(config.Scopes, " ")`. Decode `scope` from the X token response and set `ConnectResult.Scopes`; preserve the same scope list in Hosted Connect.

- [ ] **Step 5: Generate sqlc and verify GREEN**

Run:

```bash
cd api
sqlc generate
gofmt -w internal/platform/oauth.go internal/platform/twitter.go internal/platform/twitter_test.go internal/connect/twitter.go internal/connect/twitter_test.go internal/handler/oauth.go
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/connect ./internal/handler -run 'Twitter|OAuthPKCE' -count=1
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/migrations/108_x_inbox_oauth_and_delivery.sql api/internal/db/queries/oauth_states.sql api/internal/db/oauth_states.sql.go api/internal/db/models.go api/internal/platform/oauth.go api/internal/platform/twitter.go api/internal/platform/twitter_test.go api/internal/connect/twitter.go api/internal/connect/twitter_test.go api/internal/handler/oauth.go api/internal/handler/oauth_test.go
git commit -m "feat: harden X OAuth for inbox scopes"
```

## Task 2: Add X capability state and delivery-resource persistence

**Files:**
- Modify: `api/internal/db/migrations/108_x_inbox_oauth_and_delivery.sql`
- Create: `api/internal/db/queries/x_inbox.sql`
- Regenerate: `api/internal/db/x_inbox.sql.go`, `api/internal/db/models.go`
- Create: `api/internal/xinbox/capabilities.go`
- Test: `api/internal/xinbox/capabilities_test.go`
- Modify: `api/internal/handler/social_accounts.go`
- Test: `api/internal/handler/social_accounts_test.go`
- Modify: `api/internal/db/queries/platform_credentials.sql`
- Regenerate: `api/internal/db/platform_credentials.sql.go`
- Modify: `api/internal/handler/platform_credentials.go`
- Test: `api/internal/handler/platform_credentials_test.go`

- [ ] **Step 1: Write failing capability tests**

Cover:

```go
publishingEnabled == true
dmEnabled == false
missingScopes == []string{"dm.read", "dm.write"}
requiresReconnect == true
```

for an active X account with publishing scopes only. Cover API-plan workspaces returning no DM reconnect prompt.

- [ ] **Step 2: Run and verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox ./internal/handler -run 'X.*Capability|Reconnect' -count=1
```

- [ ] **Step 3: Add persistence**

Migration 108 creates:

```text
x_inbox_delivery_resources
  social_account_id PK/FK
  filtered_stream_rule_id
  activity_dm_subscription_id
  delivery_status: pending|active|paused_cap|paused_allowance|paused_plan|error
  last_error
  last_synced_at
  updated_at
```

Do not change `social_accounts.status` merely because DM scopes are missing.

Extend encrypted Platform Credentials storage for X with app-level Bearer Token and Consumer Secret. Never return either secret after write. Add an explicit app identity/billing mode resolver:

```text
unipost_managed_app
workspace_x_app
```

Do not infer this from `social_accounts.connection_type`, because Hosted Connect accounts and Dashboard-native accounts can use either shared or workspace credentials.

- [ ] **Step 4: Add capability response**

Extend account/capability JSON additively with:

```json
{
  "x_inbox": {
    "comments_enabled": true,
    "dms_enabled": false,
    "missing_scopes": ["dm.read", "dm.write"],
    "reconnect_required": true,
    "delivery_status": "pending",
    "app_mode": "unipost_managed_app",
    "missing_app_credentials": []
  }
}
```

- [ ] **Step 5: Generate and verify GREEN**

```bash
cd api
sqlc generate
gofmt -w internal/xinbox internal/handler/social_accounts.go
GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox ./internal/handler -run 'X.*Capability|Reconnect|PlatformCredential' -count=1
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/migrations/108_x_inbox_oauth_and_delivery.sql api/internal/db/queries/x_inbox.sql api/internal/db/queries/platform_credentials.sql api/internal/db/x_inbox.sql.go api/internal/db/platform_credentials.sql.go api/internal/db/models.go api/internal/xinbox api/internal/handler/social_accounts.go api/internal/handler/social_accounts_test.go api/internal/handler/platform_credentials.go api/internal/handler/platform_credentials_test.go
git commit -m "feat: add capability-specific X inbox state"
```

## Task 3: Implement atomic inbound admission, deduplication, cap configuration, and metrics

**Files:**
- Create: `api/internal/db/migrations/109_x_inbound_usage_controls.sql`
- Modify: `api/internal/db/queries/x_usage.sql`
- Regenerate: `api/internal/db/x_usage.sql.go`, `api/internal/db/models.go`
- Modify: `api/internal/xcredits/service.go`
- Modify: `api/internal/xcredits/postgres.go`
- Test: `api/internal/xcredits/inbound_service_test.go`
- Modify: `api/internal/handler/billing.go`
- Test: `api/internal/handler/x_credits_test.go`
- Modify: `api/internal/events/bus.go`
- Modify: `api/internal/handler/notifications.go`
- Modify: `api/internal/worker/notification_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing concurrency/idempotency tests**

Test that one inbound event:

1. charges once by `(workspace, account, upstream type, upstream id, UTC date)`;
2. atomically checks daily cap and monthly allowance;
3. increments accepted or suppressed exactly once;
4. never stores a private DM body in usage tables;
5. returns distinct `x_inbound_daily_cap_exceeded` and `x_monthly_usage_limit_exceeded` results.

- [ ] **Step 2: Verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits ./internal/handler -run 'Inbound|DailyCap' -count=1
```

- [ ] **Step 3: Add schema and transaction**

Migration 109 reuses `x_inbound_daily_usage` for per-workspace UTC-day totals and adds:

```text
x_inbound_event_receipts
  workspace_id
  social_account_id
  upstream_resource_type
  upstream_resource_id
  utc_date
  decision: accepted|suppressed_daily_cap|suppressed_monthly_allowance
  weighted_units
  created_at
  UNIQUE(workspace_id, social_account_id, upstream_resource_type, upstream_resource_id, utc_date)

x_inbound_cap_settings
  workspace_id PK/FK
  inbound_daily_limit
  updated_by
  acknowledged_exposure
  updated_at

x_inbound_cap_notifications
  workspace_id
  utc_date
  threshold: 80|100
  claimed_at
  UNIQUE(workspace_id, utc_date, threshold)
```

Implement one PostgreSQL transaction that creates the body-free receipt, locks/creates the daily and monthly usage rows, and returns the final admission decision. Duplicate receipts return the original decision without incrementing usage or sending another notification.

- [ ] **Step 4: Add Billing API**

Extend `GET /v1/billing/x-credits` with accepted/suppressed counts, daily reset, percent, and pausing state. Add owner/admin-only:

```text
PATCH /v1/billing/x-credits/inbound-cap
{"inbound_daily_limit": 300}
```

Validate `0 <= requested <= monthly_remaining` for self-serve plans.

At 80%, claim and send one workspace notification. Define the source-pausing safety buffer as:

```text
max(20 Credits, 10% of inbound_daily_limit)
```

When remaining daily allowance reaches this buffer, return `pause_paid_sources=true` so the reconciler disables the workspace's Filtered Stream rule and removes/pauses the Activity subscription before the nominal cap. At 100%, suppress optional paid work, claim one cap-reached notification, and expose the next UTC reset. Raising the cap or crossing into a new UTC day makes the workspace eligible for idempotent source restoration.

Add curated notification events `billing.x_inbound_80pct` and `billing.x_inbound_cap_reached` to the existing event bus and user-notification registry. Publish only after the corresponding threshold claim succeeds, and include counts/reset/cap-management URLs but no upstream content or DM body.

- [ ] **Step 5: Generate and verify GREEN**

```bash
cd api
sqlc generate
gofmt -w internal/xcredits internal/handler/billing.go cmd/api/main.go
GOCACHE=/tmp/unipost-go-build go test ./internal/xcredits ./internal/handler -run 'Inbound|DailyCap|XCredits' -count=1
```

- [ ] **Step 6: Commit**

```bash
git add api/internal/db/migrations/109_x_inbound_usage_controls.sql api/internal/db/queries/x_usage.sql api/internal/db/x_usage.sql.go api/internal/db/models.go api/internal/xcredits api/internal/handler/billing.go api/internal/handler/x_credits_test.go api/internal/events/bus.go api/internal/handler/notifications.go api/internal/worker/notification_test.go api/cmd/api/main.go
git commit -m "feat: enforce atomic X inbound usage caps"
```

## Task 4: Add the X API client, Filtered Stream rules/consumer, and Activity subscriptions

**Files:**
- Create: `api/internal/xinbox/client.go`
- Create: `api/internal/xinbox/client_test.go`
- Create: `api/internal/xinbox/stream.go`
- Create: `api/internal/xinbox/stream_test.go`
- Create: `api/internal/xinbox/subscriptions.go`
- Create: `api/internal/xinbox/subscriptions_test.go`
- Create: `api/internal/worker/x_inbox_delivery.go`
- Test: `api/internal/worker/x_inbox_delivery_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/cmd/worker/main.go`

- [ ] **Step 1: Write failing HTTP-contract tests**

Use `httptest` to assert:

- rules use `(@handle OR to:handle) -is:retweet` and a stable account tag;
- one bearer-authenticated persistent `/2/tweets/search/stream` request requests `conversation_id`, `referenced_tweets`, author expansions, and handles blank 20-second keep-alives;
- `dm.received` subscriptions use `/2/activity/subscriptions`;
- deletes/pauses are idempotent;
- disconnect and workspace deletion remove the exact stored rule/subscription ids idempotently;
- reconnect uses bounded exponential backoff and does not open two app-level stream connections.
- a PostgreSQL advisory lock elects exactly one stream leader per X app identity across worker replicas.

- [ ] **Step 2: Verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox ./internal/worker -run 'Stream|Subscription|XClient' -count=1
```

- [ ] **Step 3: Implement client and worker**

Read only:

```text
TWITTER_BEARER_TOKEN
```

from process configuration for the UniPost-managed app, and decrypt workspace app-level credentials for BYO apps. Never log bearer/user tokens, consumer secrets, or raw DM payloads. The worker reconciles desired rules/subscriptions from eligible accounts and stores returned IDs/status. Each app identity gets its own advisory-lock key and at most one persistent stream connection.

The reconciler treats account disconnect, workspace deletion, plan loss, monthly exhaustion, daily safety-buffer entry, and daily-cap exhaustion as explicit removal/pause intents. It retries cleanup using the stored upstream resource ids and clears those ids only after X confirms deletion or an already-missing response.

- [ ] **Step 4: Verify GREEN**

```bash
cd api
gofmt -w internal/xinbox internal/worker/x_inbox_delivery.go cmd/api/main.go cmd/worker/main.go
GOCACHE=/tmp/unipost-go-build go test ./internal/xinbox ./internal/worker -run 'Stream|Subscription|XClient' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add api/internal/xinbox api/internal/worker/x_inbox_delivery.go api/internal/worker/x_inbox_delivery_test.go api/cmd/api/main.go api/cmd/worker/main.go
git commit -m "feat: add X inbox stream and activity delivery"
```

## Task 5: Add X webhook CRC/signature handling and normalized ingestion

**Files:**
- Create: `api/internal/handler/x_webhook.go`
- Create: `api/internal/handler/x_webhook_test.go`
- Create: `api/internal/xinbox/ingest.go`
- Create: `api/internal/xinbox/ingest_test.go`
- Modify: `api/internal/db/queries/inbox.sql`
- Regenerate: `api/internal/db/inbox.sql.go`
- Modify: `api/internal/worker/inbox_sync.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing security and ingestion tests**

Cover exact HMAC-SHA256 CRC response and constant-time signature comparison using `TWITTER_CONSUMER_SECRET`. Reject missing/invalid/stale delivery before parsing. Cover duplicate reply/DM events, cap suppression, no private body logging, canonical `conversation_id` thread keys, and WebSocket notification after successful insert only.

- [ ] **Step 2: Verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/xinbox ./internal/worker -run 'XWebhook|XIngest' -count=1
```

- [ ] **Step 3: Implement route and ingestion**

Register:

```text
GET  /v1/webhooks/twitter
POST /v1/webhooks/twitter
```

Normalize public stream events to `x_reply` and DM activity to `x_dm`. Store only accepted event bodies; suppressed DM events retain counts/dedupe identifiers without private message text.

- [ ] **Step 4: Expand account queries and generate**

Add `twitter` to relevant Inbox account queries and return `scope`, `connection_type`, and workspace plan context required for eligibility/admission.

- [ ] **Step 5: Verify GREEN and commit**

```bash
cd api
sqlc generate
gofmt -w internal/handler/x_webhook.go internal/xinbox/ingest.go internal/worker/inbox_sync.go cmd/api/main.go
GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/xinbox ./internal/worker -run 'XWebhook|XIngest' -count=1
git add api/internal/handler/x_webhook.go api/internal/handler/x_webhook_test.go api/internal/xinbox api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go api/internal/worker/inbox_sync.go api/cmd/api/main.go
git commit -m "feat: ingest X replies and direct messages"
```

## Task 6: Add bounded X backfill, public reply, and DM reply APIs

**Files:**
- Modify: `api/internal/platform/twitter.go`
- Modify: `api/internal/platform/twitter_test.go`
- Modify: `api/internal/handler/inbox.go`
- Modify: `api/internal/handler/inbox_helpers.go`
- Test: `api/internal/handler/inbox_test.go`
- Modify: `api/internal/db/queries/inbox.sql`
- Regenerate: `api/internal/db/inbox.sql.go`

- [ ] **Step 1: Write failing API tests**

Cover:

- recent reply/mention recovery and 30-day DM lookup;
- estimate-before-large-backfill with an explicit confirmation token when the estimate exceeds the configured safe threshold;
- `x_reply` sends a post reply and uses URL classification;
- `x_dm` sends by conversation or participant;
- managed connections count/finalize/reverse usage; BYO counts zero;
- retries with the same `Idempotency-Key` do not duplicate sends;
- response includes `x_credits_counted`, operation, catalog version, and billing mode.

- [ ] **Step 2: Verify RED**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'Twitter.*Inbox|Inbox.*X|XReply|XDM' -count=1
```

- [ ] **Step 3: Implement adapter and handler**

Add X methods for recent mentions/conversation recovery, DM lookup, post reply, and DM send. Reuse the shared ingestion service and usage admission. Never apply Meta's 24-hour reply-window rule to `x_dm`.

`POST /v1/inbox/sync` first returns a dry-run estimate for X backfill. If the estimate exceeds the safe threshold, the server returns `confirmation_required=true` and a short-lived, workspace-bound confirmation token; no paid reads run until the caller resubmits the token. Every page rechecks admission before issuing the next paid request, stops at the daily/monthly boundary, and returns accepted/suppressed counts.

- [ ] **Step 4: Verify GREEN and commit**

```bash
cd api
sqlc generate
gofmt -w internal/platform/twitter.go internal/platform/twitter_test.go internal/handler/inbox.go internal/handler/inbox_helpers.go internal/handler/inbox_test.go
GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/handler -run 'Twitter.*Inbox|Inbox.*X|XReply|XDM' -count=1
git add api/internal/platform/twitter.go api/internal/platform/twitter_test.go api/internal/handler/inbox.go api/internal/handler/inbox_helpers.go api/internal/handler/inbox_test.go api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go
git commit -m "feat: support X inbox sync and replies"
```

## Task 7: Add shared Dashboard Inbox model and X UI

**Required skill:** Use `design-taste-frontend` before editing UI.

**Files:**
- Create: `dashboard/src/lib/inbox-model.ts`
- Create: `dashboard/src/lib/x-inbox-eligibility.ts`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/lib/use-inbox-ws.ts`
- Modify: `dashboard/src/components/platform-icons.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/reply-window.ts`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`
- Modify: `dashboard/src/app/globals.css`
- Test: `dashboard/tests/inbox-source-model.test.mjs`
- Test: `dashboard/tests/inbox-grouping.test.mjs`
- Test: `dashboard/tests/x-inbox-eligibility.test.mjs`
- Modify: `dashboard/tests/inbox-reply-window.test.mjs`

- [ ] **Step 1: Write failing source/grouping/eligibility tests**

Assert `x_reply` maps to public comment/X icon, `x_dm` maps to private DM/X icon, canonical conversation grouping is stable, `x_dm` has no Meta reply window, and missing DM scopes show reconnect without hiding publishing.

- [ ] **Step 2: Verify RED**

```bash
cd dashboard
node --test tests/inbox-source-model.test.mjs tests/inbox-grouping.test.mjs tests/x-inbox-eligibility.test.mjs tests/inbox-reply-window.test.mjs
```

- [ ] **Step 3: Implement the shared model and UI**

Render X reply trees with X permalinks/context, private DM threads with a dedicated composer, explicit sync/cap/reconnect states, responsive master/detail behavior, accessible tabs and resize controls, and server-returned credit results.

- [ ] **Step 4: Verify GREEN and commit**

```bash
cd dashboard
node --test tests/inbox-source-model.test.mjs tests/inbox-grouping.test.mjs tests/x-inbox-eligibility.test.mjs tests/inbox-reply-window.test.mjs tests/inbox-unread-gate.test.mjs
npm run lint
git add src/lib/inbox-model.ts src/lib/x-inbox-eligibility.ts src/lib/api.ts src/lib/use-inbox-ws.ts src/components/platform-icons.tsx 'src/app/(dashboard)/projects/[id]/inbox/page.tsx' 'src/app/(dashboard)/projects/[id]/inbox/reply-window.ts' 'src/app/(dashboard)/projects/[id]/accounts/page.tsx' 'src/app/(dashboard)/settings/billing/page.tsx' src/app/globals.css tests
git commit -m "feat: add X replies and DMs to Inbox"
```

## Task 8: Add X Inbox Reference, Guidance, navigation, and disclosure tests

**Files:**
- Create: `dashboard/src/app/docs/api/inbox/list/page.tsx`
- Create: `dashboard/src/app/docs/api/inbox/reply/page.tsx`
- Create: `dashboard/src/app/docs/api/inbox/sync/page.tsx`
- Create: `dashboard/src/app/docs/guides/x/comments/page.tsx`
- Create: `dashboard/src/app/docs/guides/x/direct-messages/page.tsx`
- Create: `dashboard/src/app/docs/guides/x/reconnect-permissions/page.tsx`
- Modify: `dashboard/src/app/docs/api/inbox/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/api/page.tsx`
- Modify: `dashboard/src/app/docs/api/_components/doc-components.tsx`
- Modify: `dashboard/src/app/docs/platforms/[platform]/_data.tsx`
- Modify: `dashboard/src/app/docs/platform-credentials/[platform]/_data.tsx`
- Modify: `dashboard/src/app/docs/api/accounts/capabilities/page.tsx`
- Modify: `dashboard/src/app/docs/api/errors/page.tsx`
- Modify: `dashboard/src/app/docs/guides/x/credits/page.tsx`
- Modify: `dashboard/src/app/docs/api/x-credits/page.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Modify: `dashboard/src/app/sitemap.ts`
- Test: `dashboard/tests/x-inbox-docs-source.test.mjs`

- [ ] **Step 1: Write failing docs-source tests**

Assert correct baseline sources (`ig_comment`, `ig_dm`, `threads_reply`, `fb_comment`, `fb_dm`), shipped X sources, every required two-way link, sitemap/search/sidebar registration, and absence of `youtube_comment`, Top-up, Auto top-up, XChat, upstream dollar pricing, or margin claims.

- [ ] **Step 2: Verify RED**

```bash
cd dashboard
node --test tests/x-inbox-docs-source.test.mjs tests/x-credits-foundation-source.test.mjs
```

- [ ] **Step 3: Implement references and procedural guides**

Every guide includes complete cURL examples against the Inbox list/reply/sync endpoints, managed-vs-BYO behavior, plan eligibility, allowance/cap errors, and exact reconnect steps.

- [ ] **Step 4: Verify GREEN and commit**

```bash
cd dashboard
node --test tests/x-inbox-docs-source.test.mjs tests/x-credits-foundation-source.test.mjs
npm run test:docs-ai
npm run build
git add src/app/docs src/lib/docs-ai-search-index.ts src/app/sitemap.ts tests/x-inbox-docs-source.test.mjs
git commit -m "docs: add X inbox reference and guidance"
```

## Task 9: Add operations, reconciliation, and dev runbook

**Files:**
- Create: `docs/x-inbox-operations.md`
- Create: `api/internal/worker/x_inbox_reconciliation.go`
- Test: `api/internal/worker/x_inbox_reconciliation_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/cmd/worker/main.go`

- [ ] **Step 1: Write failing reconciliation tests**

Cover provisional/reversed event counts, stream/activity capacity thresholds 70/85/95, cap suppression metrics, 80/100 notification claims, source-pause/restore latency, stale delivery resources, disconnect/workspace-deletion cleanup, and no raw DM bodies/tokens in logs.

- [ ] **Step 2: Implement metrics/reconciliation**

The runbook records dev app owner, billing owner, secret rotation, required env vars, spending-limit check, rule/subscription capacity, 80/100 notification delivery, manual-backfill confirmations, pause/restore and disconnect/deletion cleanup procedures, and escalation. The job emits structured counts and alerts without customer content.

- [ ] **Step 3: Verify and commit**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./internal/worker -run 'XInbox.*Reconciliation' -count=1
git add internal/worker/x_inbox_reconciliation.go internal/worker/x_inbox_reconciliation_test.go cmd/api/main.go cmd/worker/main.go ../docs/x-inbox-operations.md
git commit -m "feat: add X inbox operations reconciliation"
```

## Task 10: Full local validation, dev integration, deployment, and acceptance

- [ ] **Step 1: Run backend validation**

```bash
cd api
GOCACHE=/tmp/unipost-go-build go test ./...
```

- [ ] **Step 2: Run Dashboard validation**

```bash
cd dashboard
node --test tests/inbox-source-model.test.mjs tests/inbox-grouping.test.mjs tests/x-inbox-eligibility.test.mjs tests/inbox-reply-window.test.mjs tests/inbox-unread-gate.test.mjs tests/x-inbox-docs-source.test.mjs tests/x-credits-foundation-source.test.mjs
npm run test:docs-ai
npm run lint
npm run build
npm run test:regression:dashboard
```

- [ ] **Step 3: Review**

Run spec-compliance review against PRD Sections 8.3, 10.3–10.4, 11–12, 14–17, and 20.1, then code-quality review. Fix every Critical/Important issue and rerun validation.

- [ ] **Step 4: Merge into local dev and validate again**

Update local `dev` from `origin/dev`, merge `dev-x-inbox-comments-dms`, and rerun the backend and Dashboard commands above.

- [ ] **Step 5: Configure only Railway dev**

Set matching API/worker values without printing them:

```text
TWITTER_CLIENT_ID
TWITTER_CLIENT_SECRET
TWITTER_BEARER_TOKEN
TWITTER_CONSUMER_SECRET
X_INBOX_WEBHOOK_ROUTE_SECRET
X_INBOX_WEBHOOK_URL=https://dev-api.unipost.dev/v1/webhooks/twitter
```

`X_INBOX_WEBHOOK_ROUTE_SECRET` is a separate stable, environment-specific
secret used only to derive the managed app's webhook route. Do not reuse
`TWITTER_CONSUMER_SECRET`: rotating X's consumer secret must update signature
validation without changing the registered webhook URL. Rotate the route
secret only as an explicit webhook-generation migration that recreates the
managed Activity subscription.

Register the dev webhook, complete CRC, create only dev rules/subscriptions, and keep X-side dev spend limits active.

- [ ] **Step 6: Push only `origin/dev` and monitor**

Push local `dev` to `origin/dev`. Wait for GitHub checks, Vercel `unipost-dev`, Railway `unipost`, and `post-delivery-worker` to finish successfully.

- [ ] **Step 7: Real dev acceptance**

Using dedicated X test accounts:

1. Reconnect `@unipostdev` and verify granted scopes include DM scopes while publishing remains active.
2. Verify one inbound X mention appears as `x_reply` with correct parent/conversation/permalink and one usage count.
3. Verify a duplicate delivery creates no duplicate Inbox item or usage.
4. Verify one legacy DM appears as `x_dm` without leaking its body to logs.
5. With explicit approval, reply to the test mention and test DM; otherwise validate mocked writes and document that destructive external writes were not performed.
6. Verify cap suppression/restore with a temporary low dev cap.
7. Verify Dashboard desktop/mobile, Accounts/Billing reconnect state, API list/reply/sync contracts, and all new docs links.
8. Verify staging and production refs/deployments were not changed.
