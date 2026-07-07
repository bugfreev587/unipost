# Admin Posts Quota Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add admin User detail actions that reset a customer's current-month post usage quota and scheduled reservation quota without deleting scheduled posts.

**Architecture:** Post quota reset directly clears current-month `usage.post_count` for every workspace owned by the selected user. Scheduled quota reset records a reset baseline per user/workspace/period so existing scheduled posts stop reserving quota, while scheduled posts created after the reset still count. Admin endpoints are protected by the existing `/v1/admin` middleware and surfaced through the existing admin Users detail panel.

**Tech Stack:** Go `net/http`/chi handlers, pgx raw SQL, existing sqlc query extensions, Next.js admin client components, source-level Node tests, Go tests.

---

### Task 1: Backend Reset Contracts

**Files:**
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/db/social_posts_ext.go`
- Create: `api/internal/db/migrations/099_admin_post_quota_resets.sql`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing Go source tests**

Add tests that require:

```go
`type adminUserQuotaResetResponse struct`
`func (h *AdminHandler) ResetUserPostQuota`
`func (h *AdminHandler) ResetUserScheduledQuota`
`UPDATE usage`
`post_count = 0`
`admin_post_quota_resets`
`quota_kind = 'scheduled'`
`created_at > COALESCE(`
`r.Post("/v1/admin/users/{id}/quota/post/reset", adminHandler.ResetUserPostQuota)`
`r.Post("/v1/admin/users/{id}/quota/scheduled/reset", adminHandler.ResetUserScheduledQuota)`
```

- [ ] **Step 2: Run backend contract tests to verify RED**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db
```

Expected: FAIL because reset handlers/routes/migration-aware scheduled quota SQL are missing.

- [ ] **Step 3: Implement minimal backend**

Create `admin_post_quota_resets` with one row per `user_id`, `workspace_id`, `period`, and `quota_kind`. Add:

```go
func (h *AdminHandler) ResetUserPostQuota(w http.ResponseWriter, r *http.Request)
func (h *AdminHandler) ResetUserScheduledQuota(w http.ResponseWriter, r *http.Request)
```

Return:

```json
{
  "user_id": "user_x",
  "period": "2026-07",
  "quota_kind": "post",
  "affected_workspaces": 1,
  "previous_usage": 100,
  "reset_at": "2026-07-07T00:00:00Z"
}
```

Update `CountScheduledQuotaUnitsByWorkspaceAndPeriod` so scheduled reservations only count posts with `sp.created_at` after the latest scheduled reset baseline for the workspace/period.

- [ ] **Step 4: Run backend tests to verify GREEN**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler ./internal/db
```

Expected: PASS.

### Task 2: Dashboard Reset Actions

**Files:**
- Modify: `dashboard/tests/admin-users-scheduled-source.test.mjs`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/users/page.tsx`

- [ ] **Step 1: Write failing dashboard source tests**

Add tests requiring:

```js
resetAdminUserPostQuota
resetAdminUserScheduledQuota
/v1/admin/users/${id}/quota/post/reset
/v1/admin/users/${id}/quota/scheduled/reset
Posts quota reset
Reset schedule quota
Reset post quota
quotaResetPending
quotaResetMessage
```

- [ ] **Step 2: Run dashboard tests to verify RED**

Run:

```bash
cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs
```

Expected: FAIL because API helpers and UI actions are missing.

- [ ] **Step 3: Implement minimal dashboard UI**

Add two buttons inside the existing admin User detail panel. Each button calls its API helper, shows pending state, displays success/error inline, and refreshes the user list/detail after success.

- [ ] **Step 4: Run dashboard tests to verify GREEN**

Run:

```bash
cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs
```

Expected: PASS.

### Task 3: Validation and Release

**Files:**
- Validate changed backend and dashboard surfaces.

- [ ] **Step 1: Run local CI-equivalent checks on hotfix branch**

Run:

```bash
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd dashboard && npm run build
```

- [ ] **Step 2: Merge hotfix into local staging and validate**

Run:

```bash
git switch staging
git pull --ff-only origin staging
git merge --no-ff hotfix-admin-posts-quota-reset
cd api && GOCACHE=/tmp/unipost-go-build go test ./...
cd dashboard && npm run build
```

- [ ] **Step 3: Push staging and verify real staging**

Push `staging` to `origin/staging`, monitor triggered checks/deployments, then verify the admin Users page in `https://staging-app.unipost.dev`.

- [ ] **Step 4: Promote staging to production and verify real production**

Create and merge the required PR from `staging` to `main`, monitor triggered checks/deployments, then verify production health and the admin Users quota reset flow in `https://app.unipost.dev`.

- [ ] **Step 5: Sync back to dev and verify real development**

Merge or cherry-pick the same change into local `dev`, validate, push `origin/dev`, monitor triggered checks/deployments, then verify `https://dev-app.unipost.dev`.
