# Inbox Comments and DMs Tenant-Isolation Hotfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop cross-account Meta Inbox ingestion, harden every shared Inbox object lookup against a forged workspace stamp, and provide a dry-run-first reversible quarantine procedure for already duplicated Instagram comments and DMs.

**Architecture:** Instagram connections retain the current app-scoped `external_account_id` and persist Meta's webhook `user_id` separately in `social_accounts.metadata`. Meta webhooks resolve only exact active account matches and fail closed; shared Inbox SQL requires both the stored workspace stamp and the workspace derived through the social account's profile. Historical suspect rows are preserved in an append-only incident table before a guarded operator script removes them from the live Inbox in one transaction.

**Tech Stack:** Go 1.25, PostgreSQL, sqlc, Goose, pgx v5, `psql` operator scripts, standard Go tests.

---

## File structure

- Create `api/internal/db/migrations/119_inbox_tenant_isolation.sql`: additive quarantine table and active exact-routing indexes; no live Inbox DML.
- Create `api/internal/db/inbox_tenant_isolation_contract_test.go`: non-skipping migration and SQL authorization/routing contracts.
- Modify `api/internal/db/queries/inbox.sql`: exact routing queries and derived-workspace Inbox guards.
- Modify `api/internal/db/queries/social_accounts.sql`: idempotent Instagram webhook-ID metadata update.
- Regenerate `api/internal/db/inbox.sql.go`, `api/internal/db/social_accounts.sql.go`, and `api/internal/db/models.go` with sqlc.
- Modify `api/internal/connect/connect.go`, `api/internal/connect/instagram.go`, and `api/internal/connect/instagram_test.go`: managed-connect webhook identity.
- Modify `api/internal/handler/connect_callback.go` and `api/internal/handler/connect_sessions_test.go`: persist mapping before subscribing.
- Modify `api/internal/platform/instagram.go` and `api/internal/platform/instagram_test.go`: native OAuth webhook identity metadata.
- Modify `api/internal/instagramwebhooks/subscriber.go` and `api/internal/instagramwebhooks/subscriber_test.go`: token-scoped `user_id` resolver.
- Modify `api/internal/worker/inbox_sync.go` and `api/internal/worker/inbox_sync_subscription_test.go`: existing-account backfill and subscription repair.
- Modify `api/internal/handler/meta_webhook.go` and `api/internal/handler/meta_webhook_test.go`: exact Meta routing, content-free logs, test notifier seam.
- Modify `api/internal/handler/inbox.go` and `api/internal/handler/inbox_test.go`: workspace-scoped account loads and X outbound-status fail-closed behavior.
- Create `api/ops/instagram_inbox_quarantine.sql`: count-and-digest-gated dry-run/apply operation.
- Create `api/internal/db/instagram_inbox_quarantine_runbook_test.go`: operator-script safety contract.
- Create `docs/instagram-inbox-quarantine-runbook.md`: staging/production evidence and recovery procedure without message bodies.
- Modify `api/internal/db/migrate_test.go`: expect migration 119 in disposable migration tests.

### Task 1: Add additive security schema and exact/derived SQL contracts

**Files:**
- Create: `api/internal/db/inbox_tenant_isolation_contract_test.go`
- Create: `api/internal/db/migrations/119_inbox_tenant_isolation.sql`
- Modify: `api/internal/db/queries/inbox.sql`
- Modify: `api/internal/db/queries/social_accounts.sql`
- Regenerate: `api/internal/db/inbox.sql.go`
- Regenerate: `api/internal/db/social_accounts.sql.go`
- Regenerate: `api/internal/db/models.go`
- Modify: `api/internal/db/migrate_test.go`

- [ ] **Step 1: Write the failing schema/query contract test**

Add tests that read migration 119 and `queries/inbox.sql`. Require the quarantine evidence columns and uniqueness, active partial indexes, absence of `INSERT INTO inbox_items`, `UPDATE inbox_items`, or `DELETE FROM inbox_items` in migration Up, and a guarded Down block that raises when evidence exists. Require exact Instagram metadata routing, exact multi-account Threads/Facebook routing, removal of `FindAnyActiveAccountByPlatform` and `FindAllActiveAccountsByPlatform`, and `profiles p` plus both `i.workspace_id =` and `p.workspace_id =` guards in list/get/count/mark/thread queries.

```go
func TestInboxTenantIsolationMigrationIsAdditive(t *testing.T) {
	body, err := os.ReadFile("migrations/119_inbox_tenant_isolation.sql")
	if err != nil { t.Fatal(err) }
	parts := strings.Split(strings.ToLower(string(body)), "-- +goose down")
	if len(parts) != 2 { t.Fatal("migration must have one Down section") }
	up, down := parts[0], parts[1]
	for _, want := range []string{
		"create table inbox_item_quarantine", "incident_key text not null",
		"original_inbox_item_id text not null", "original_row jsonb not null",
		"unique (incident_key, original_inbox_item_id)",
		"instagram_webhook_user_id", "platform, external_account_id",
	} {
		if !strings.Contains(up, want) { t.Errorf("migration Up missing %q", want) }
	}
	for _, forbidden := range []string{"insert into inbox_items", "update inbox_items", "delete from inbox_items"} {
		if strings.Contains(up, forbidden) { t.Errorf("migration Up mutates live Inbox with %q", forbidden) }
	}
	if !strings.Contains(down, "refusing") || !strings.Contains(down, "inbox_item_quarantine") {
		t.Fatal("migration Down must refuse to destroy incident evidence")
	}
}
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInboxTenantIsolation' -count=1`

Expected: FAIL because migration 119 and the hardened query contracts do not exist.

- [ ] **Step 3: Add the schema and SQL changes**

Migration 119 uses Goose `NO TRANSACTION`, creates `inbox_item_quarantine` without foreign keys, with `original_row JSONB CHECK (jsonb_typeof(original_row) = 'object')`, and builds non-unique partial indexes concurrently so deployment does not take a long blocking table lock:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS social_accounts_active_instagram_webhook_user_id_idx
  ON social_accounts ((metadata->>'instagram_webhook_user_id'))
  WHERE platform = 'instagram' AND status = 'active' AND disconnected_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS social_accounts_active_platform_external_id_idx
  ON social_accounts (platform, external_account_id)
  WHERE status = 'active' AND disconnected_at IS NULL;
```

Add exact routing and mapping queries:

```sql
-- name: FindAllActiveInstagramAccountsByWebhookUserID :many
SELECT sa.id, sa.external_account_id,
       sa.metadata->>'instagram_webhook_user_id' AS instagram_webhook_user_id,
       p.workspace_id
FROM social_accounts sa
JOIN profiles p ON p.id = sa.profile_id
WHERE sa.platform = 'instagram'
  AND sa.metadata->>'instagram_webhook_user_id' = $1
  AND sa.status = 'active'
  AND sa.disconnected_at IS NULL
ORDER BY sa.connected_at DESC, sa.id;

-- name: SetInstagramWebhookUserID :execrows
UPDATE social_accounts
SET metadata = COALESCE(metadata, '{}'::jsonb)
  || jsonb_build_object('instagram_webhook_user_id', @instagram_webhook_user_id::TEXT)
WHERE id = @id
  AND platform = 'instagram'
  AND status = 'active'
  AND disconnected_at IS NULL;
```

For every authenticated Inbox list/get/count/mutation, join `social_accounts` and `profiles`, require `i.workspace_id = @workspace_id` and `p.workspace_id = @workspace_id`, retain active-account filters where they already exist, and preserve `i.*` projections/signatures. Remove the global/arbitrary routing queries. Update the disposable migration version expectation to 119.

- [ ] **Step 4: Regenerate sqlc code**

Run: `cd api && sqlc generate`

Expected: generated Go code contains the new query methods and no generated methods for the removed unsafe queries.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInboxTenantIsolation|TestEmbeddedMigrationVersionsAreUnique' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit the schema/query foundation**

```bash
git add api/internal/db/migrations/119_inbox_tenant_isolation.sql api/internal/db/queries/inbox.sql api/internal/db/queries/social_accounts.sql api/internal/db/inbox.sql.go api/internal/db/social_accounts.sql.go api/internal/db/models.go api/internal/db/inbox_tenant_isolation_contract_test.go api/internal/db/migrate_test.go
git commit -m "fix: add inbox tenant isolation contracts"
```

### Task 2: Persist the trusted Instagram webhook identity on new connections

**Files:**
- Modify: `api/internal/connect/connect.go`
- Modify: `api/internal/connect/instagram.go`
- Modify: `api/internal/connect/instagram_test.go`
- Modify: `api/internal/handler/connect_callback.go`
- Modify: `api/internal/handler/connect_sessions_test.go`
- Modify: `api/internal/platform/instagram.go`
- Modify: `api/internal/platform/instagram_test.go`

- [ ] **Step 1: Write failing managed/native identity tests**

Extend the managed profile test response to use distinct identifiers and assert the request asks for `user_id`:

```go
_, _ = io.WriteString(w, `{"id":"app-scoped-99","user_id":"17841400000000000","username":"shipper"}`)
// assertions
if p.ExternalAccountID != "app-scoped-99" { t.Fatalf("external id = %q", p.ExternalAccountID) }
if p.WebhookAccountID != "17841400000000000" { t.Fatalf("webhook id = %q", p.WebhookAccountID) }
```

Add a missing-`user_id` test that expects an error. Add native adapter coverage asserting `ConnectResult.Metadata["instagram_webhook_user_id"]` equals the professional account ID while `ExternalAccountID` remains app-scoped. Update the connect callback fake to return distinct IDs, capture saved metadata, and assert subscription uses the professional ID after the metadata save.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform ./internal/handler -run 'Instagram.*Webhook|InstagramFetchProfile|InstagramSubscribes' -count=1`

Expected: FAIL because `WebhookAccountID` and persisted metadata do not exist and subscription uses `external_account_id`.

- [ ] **Step 3: Implement the minimum identity mapping**

Add `WebhookAccountID string` to `connect.Profile`. Managed and native Instagram profile requests must ask for `id,user_id,username,profile_picture_url`, decode both IDs as strings, and reject a missing value. Preserve the existing `ig_user_id` metadata for compatibility and add `instagram_webhook_user_id`.

In the connect callback, construct metadata as:

```go
profileMetadata := map[string]any{
	"username": profile.Username,
	"display_name": profile.DisplayName,
}
if platformName == "instagram" {
	if strings.TrimSpace(profile.WebhookAccountID) == "" {
		h.redirectWithStatus(w, r, session.ReturnUrl.String, "error", "profile_fetch_failed", false)
		return
	}
	profileMetadata["instagram_webhook_user_id"] = profile.WebhookAccountID
}
metadata, _ := json.Marshal(profileMetadata)
```

Persist the account first, then subscribe with `profile.WebhookAccountID`; non-Instagram connectors keep the zero-value field.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform ./internal/handler -run 'Instagram.*Webhook|InstagramFetchProfile|InstagramSubscribes' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit connection mapping**

```bash
git add api/internal/connect/connect.go api/internal/connect/instagram.go api/internal/connect/instagram_test.go api/internal/handler/connect_callback.go api/internal/handler/connect_sessions_test.go api/internal/platform/instagram.go api/internal/platform/instagram_test.go
git commit -m "fix: persist instagram webhook account ids"
```

### Task 3: Backfill existing Instagram mappings before subscription repair

**Files:**
- Modify: `api/internal/instagramwebhooks/subscriber.go`
- Modify: `api/internal/instagramwebhooks/subscriber_test.go`
- Modify: `api/internal/worker/inbox_sync.go`
- Modify: `api/internal/worker/inbox_sync_subscription_test.go`

- [ ] **Step 1: Write failing resolver/backfill tests**

Add subscriber tests for `GET /me?fields=user_id`, nonempty string decoding, non-200 behavior without token leakage, and empty-ID rejection. Extend the worker fake so a missing mapping must call `FetchWebhookUserID` with the decrypted account token, call `SetInstagramWebhookUserID`, then call `Subscribe` with the fetched professional ID. Assert the cache is set only after all three operations succeed; persistence or subscription failure must retry on the next call.

- [ ] **Step 2: Run focused tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/instagramwebhooks ./internal/worker -run 'WebhookUserID|EnsureInstagramWebhookSubscription' -count=1`

Expected: FAIL because the resolver and backfill sequence do not exist.

- [ ] **Step 3: Implement the resolver and safe backfill**

Extend the worker-facing interface:

```go
type inboxInstagramWebhookSubscriber interface {
	FetchWebhookUserID(context.Context, string) (string, error)
	Subscribe(context.Context, string, string) error
}
```

Project `COALESCE(sa.metadata->>'instagram_webhook_user_id', '') AS instagram_webhook_user_id` from `ListAllInboxAccounts`. For Instagram accounts, use the persisted value if present; otherwise resolve it with that account's own decrypted token and persist it with `SetInstagramWebhookUserID`. Subscribe to `/{user_id}/subscribed_apps`, and cache success only after persistence and subscription succeed. Logs contain internal account IDs and error classes, never access tokens or message content.

- [ ] **Step 4: Regenerate sqlc after the account projection change**

Run: `cd api && sqlc generate`

Expected: `ListAllInboxAccountsRow` contains `InstagramWebhookUserID string`.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/instagramwebhooks ./internal/worker -run 'WebhookUserID|EnsureInstagramWebhookSubscription' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit existing-account backfill**

```bash
git add api/internal/instagramwebhooks/subscriber.go api/internal/instagramwebhooks/subscriber_test.go api/internal/worker/inbox_sync.go api/internal/worker/inbox_sync_subscription_test.go api/internal/db/queries/inbox.sql api/internal/db/inbox.sql.go
git commit -m "fix: backfill instagram webhook account ids"
```

### Task 4: Replace Meta fan-out and arbitrary fallback with exact routing

**Files:**
- Modify: `api/internal/handler/meta_webhook.go`
- Modify: `api/internal/handler/meta_webhook_test.go`

- [ ] **Step 1: Write failing exact-routing tests**

Build a `db.DBTX` fake that returns configured rows for `FindAllActiveInstagramAccountsByWebhookUserID` and `FindAllSocialAccountsByPlatformAndExternalID`, records `UpsertInboxItem` parameters, and exposes a notifier callback. Cover:

- an Instagram comment and DM route only to exact `user_id` matches;
- two duplicate connections sharing the same exact webhook identity both receive the event;
- an unmatched Instagram entry performs zero upserts/notifications;
- unmatched Threads and Facebook entries perform zero upserts/notifications;
- matched Threads/Facebook duplicates receive only exact events;
- own Instagram events compare against the mapped webhook ID;
- message body/comment text and raw payload are absent from structured log calls.

- [ ] **Step 2: Run the routing tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestMetaWebhook.*Routing' -count=1`

Expected: FAIL because Instagram still fans out and Threads/Facebook still use an arbitrary fallback.

- [ ] **Step 3: Implement fail-closed exact routing**

Add `WebhookAccountID` to the internal `webhookAccount`. Instagram resolves `entry.ID` only with `FindAllActiveInstagramAccountsByWebhookUserID`; Threads and Facebook resolve only with the exact `:many` external-ID query. A DB error or empty match logs platform, entry ID, match count/error class and returns. Remove `findAnyActiveAccount`, `findAllActiveAccounts`, and the singular routing helper. Preserve duplicate connections only when the exact provider ID is identical.

Add a private notifier function initialized by the constructor to call `ws.Notify`; tests replace it with an in-memory recorder. Remove logs of raw comment JSON, comment text, DM text, and author content.

- [ ] **Step 4: Run routing tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestMetaWebhook.*Routing|TestMetaWebhookHandle|TestMetaWebhookVerify' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit exact routing**

```bash
git add api/internal/handler/meta_webhook.go api/internal/handler/meta_webhook_test.go
git commit -m "fix: route meta inbox events by exact account"
```

### Task 5: Enforce derived workspace ownership across shared Inbox paths

**Files:**
- Modify: `api/internal/handler/inbox.go`
- Modify: `api/internal/handler/inbox_test.go`
- Verify: `api/internal/db/queries/inbox.sql`
- Verify: `api/internal/db/inbox.sql.go`

- [ ] **Step 1: Write failing handler regressions**

Using `httptest`, `auth.SetWorkspaceID`, chi route params, and a `db.DBTX` fake, add tests proving:

- MediaContext requests `GetSocialAccountByIDAndWorkspace`, never unscoped `GetSocialAccount`;
- Reply returns the existing 404 when hardened `GetInboxItem` rejects the target and never calls an adapter;
- XOutboundStatus returns 404 when the derived target lookup fails, even if `x_inbox_outbound_requests.workspace_id` equals the caller;
- XOutboundStatus returns 404 when target and outbound social-account IDs differ;
- valid X status still returns safe reconciliation fields and never returns encrypted payload.

- [ ] **Step 2: Run focused handler tests and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestInboxTenantIsolation|TestXOutboundStatusTenantIsolation' -count=1`

Expected: FAIL because MediaContext performs an unscoped account load and X status ignores a failed/mismatched target.

- [ ] **Step 3: Implement handler fail-closed checks**

Use the existing `GetSocialAccountByIDAndWorkspace` query in MediaContext. In XOutboundStatus, require all of:

```go
if err != nil || outbound.WorkspaceID != workspaceID {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "X Inbox outbound operation not found")
	return
}
target, targetErr := h.queries.GetInboxItem(r.Context(), db.GetInboxItemParams{ID: outbound.InboxItemID, WorkspaceID: workspaceID})
if targetErr != nil || target.SocialAccountID != outbound.SocialAccountID {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "X Inbox outbound operation not found")
	return
}
```

Preserve existing X DM availability checks and not-found privacy behavior.

- [ ] **Step 4: Run focused handler and DB contract tests and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db -run 'TestInboxTenantIsolation|TestXOutboundStatusTenantIsolation' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit read/mutation hardening**

```bash
git add api/internal/handler/inbox.go api/internal/handler/inbox_test.go
git commit -m "fix: derive inbox ownership from social accounts"
```

### Task 6: Add the reversible, dry-run-first Instagram quarantine operation

**Files:**
- Create: `api/ops/instagram_inbox_quarantine.sql`
- Create: `api/internal/db/instagram_inbox_quarantine_runbook_test.go`
- Create: `docs/instagram-inbox-quarantine-runbook.md`

- [ ] **Step 1: Write the failing operator-script contract test**

Require `ON_ERROR_STOP`, default `apply=false`, required `incident_key`, apply-only `expected_count` and `expected_digest`, repeatable-read transaction, advisory lock, row locks, candidate selection restricted to Instagram `ig_comment`/`ig_dm`, `COUNT(DISTINCT sa.external_account_id) > 1`, `to_jsonb`, insert-before-delete, exact count/digest checks, zero-remaining check, rollback by default, commit only inside apply, and no output of `body`, `author_name`, `author_id`, or `original_row`.

- [ ] **Step 2: Run the runbook test and verify RED**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInstagramInboxQuarantine' -count=1`

Expected: FAIL because the operator script/runbook do not exist.

- [ ] **Step 3: Implement the guarded operator script**

The script materializes candidate rows with `FOR UPDATE`, computes `candidate_count` and a deterministic `md5(string_agg(id, ',' ORDER BY id))` digest, and prints only aggregate counts/digest. Apply is permitted only when supplied values exactly match. It inserts complete evidence with `ON CONFLICT DO NOTHING`, requires inserted count to equal candidate count, deletes only captured IDs, requires deleted count to equal candidate count and remaining count to be zero, then commits. Every other path rolls back or raises an error.

The runbook documents:

- dry-run invocation and expected evidence fields;
- recovery snapshot/PITR confirmation;
- staging execution and verification;
- production execution only after explicit user approval;
- no message bodies in terminals, CI logs, screenshots, or release evidence;
- incident-specific restoration requires independent upstream ownership evidence and cannot blanket-restore rows.

- [ ] **Step 4: Run the safety contract test and verify GREEN**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/db -run 'TestInstagramInboxQuarantine' -count=1`

Expected: PASS.

- [ ] **Step 5: Commit the recoverable operation**

```bash
git add api/ops/instagram_inbox_quarantine.sql api/internal/db/instagram_inbox_quarantine_runbook_test.go docs/instagram-inbox-quarantine-runbook.md
git commit -m "ops: add recoverable instagram inbox quarantine"
```

### Task 7: Full local verification and promotion audit

**Files:**
- Review all files unique to `origin/staging...HEAD`.

- [ ] **Step 1: Format changed Go files**

Run: `cd api && gofmt -w internal/connect/connect.go internal/connect/instagram.go internal/connect/instagram_test.go internal/handler/connect_callback.go internal/handler/connect_sessions_test.go internal/platform/instagram.go internal/platform/instagram_test.go internal/instagramwebhooks/subscriber.go internal/instagramwebhooks/subscriber_test.go internal/worker/inbox_sync.go internal/worker/inbox_sync_subscription_test.go internal/handler/meta_webhook.go internal/handler/meta_webhook_test.go internal/handler/inbox.go internal/handler/inbox_test.go internal/db/inbox_tenant_isolation_contract_test.go internal/db/instagram_inbox_quarantine_runbook_test.go internal/db/migrate_test.go`

Expected: exit 0.

- [ ] **Step 2: Confirm generated SQL is current**

Run: `cd api && sqlc generate && git diff --exit-code -- internal/db/inbox.sql.go internal/db/social_accounts.sql.go internal/db/models.go`

Expected: exit 0 with no generated diff.

- [ ] **Step 3: Run focused security suites**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/connect ./internal/platform ./internal/instagramwebhooks ./internal/worker ./internal/handler ./internal/db -run 'Instagram|MetaWebhook|InboxTenantIsolation|XOutboundStatusTenantIsolation|Quarantine' -count=1`

Expected: PASS with zero failed focused tests.

- [ ] **Step 4: Run the complete backend CI-equivalent suite**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS. Any failure, error, timeout, cancellation, skip of a newly required hotfix test, or wrong-SHA result is a hard stop.

- [ ] **Step 5: Audit branch content**

Run:

```bash
git status --short --branch
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
git diff --check origin/staging...HEAD
```

Expected: clean worktree; only the approved design/plan, routing/mapping, Inbox authorization, migration, generated sqlc, tests, quarantine script, and runbook files.

- [ ] **Step 6: Request final security/code review**

Review the exact `origin/staging...HEAD` diff for spec compliance, tenant-boundary completeness, token/message-content leakage, migration reversibility, and operator-script transaction safety. Resolve every important finding and rerun Steps 2–5.

- [ ] **Step 7: Start the repository hotfix promotion flow**

Push only `hotfix-inbox-comments-idor`, open a Draft PR to `staging`, and monitor every required GitHub/Railway/Vercel check on the exact head SHA. Do not execute the quarantine script against any environment during local implementation.

- [ ] **Step 8: Stop at the production data-mutation gate**

After staging deploy/acceptance, run a content-free staging dry run and apply only with recorded recovery readiness. After the production code deploy, run only the production dry run and present mapping coverage, count, digest, snapshot/PITR evidence, and exact command for explicit user approval. No production Inbox row may be moved before that approval.
