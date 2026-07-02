# Admin Users Scheduled Posts Drawer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make nonzero Scheduled counts on `/admin/users` open a right-side drawer listing that user's scheduled posts.

**Architecture:** Add a dedicated admin endpoint for user scheduled-post details so drawer rows match the existing scheduled count source. The dashboard API client exposes that endpoint, and `/admin/users` manages a separate drawer state from the existing View detail panel.

**Tech Stack:** Go `net/http` handler with pgx SQL, Next.js client page, existing admin CSS, existing `PlatformIcon`, source tests plus build validation.

---

### Task 1: Backend Scheduled Posts Endpoint

**Files:**
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/cmd/api/main.go`

- [x] **Step 1: Write failing backend tests**

Add source assertions that require:
- `type adminUserScheduledPost`
- JSON fields `post_id`, `title`, `created_at`, `scheduled_at`, and `platforms`
- SQL filters `sp.status = 'scheduled'`, `sp.deleted_at IS NULL`, and `w.user_id = $1`
- `ORDER BY sp.scheduled_at ASC NULLS LAST, sp.created_at DESC`
- route `r.Get("/v1/admin/users/{id}/scheduled-posts", adminHandler.ListUserScheduledPosts)`

- [x] **Step 2: Run focused backend test and confirm failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminUserScheduledPosts'`

Expected: FAIL because the scheduled-post type, handler, SQL, and route do not exist.

- [x] **Step 3: Implement endpoint**

Add an `adminUserScheduledPost` type and `ListUserScheduledPosts` handler. Query scheduled, non-deleted posts for the requested user, derive `title` from `caption` by trimming the first non-empty line to 80 characters, fallback to `Untitled scheduled post`, and use `adminPostPlatformsSQL("sp")` for platform icons.

- [x] **Step 4: Register route**

Add `r.Get("/v1/admin/users/{id}/scheduled-posts", adminHandler.ListUserScheduledPosts)` beside the existing admin user detail routes.

- [x] **Step 5: Run focused backend test and confirm pass**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminUserScheduledPosts'`

Expected: PASS.

### Task 2: Dashboard Client and Users Drawer

**Files:**
- Modify: `dashboard/tests/admin-users-scheduled-source.test.mjs`
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/src/app/admin/users/page.tsx`

- [x] **Step 1: Write failing dashboard source tests**

Add assertions that require:
- `AdminUserScheduledPost` type
- `getAdminUserScheduledPosts` client calling `/v1/admin/users/${id}/scheduled-posts`
- `openScheduledPosts(u)` from the Scheduled column
- nonzero scheduled counts rendered as a button
- drawer states for loading, error, empty, and rows with `PlatformIcon`

- [x] **Step 2: Run focused dashboard source test and confirm failure**

Run: `cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs`

Expected: FAIL because the client function and drawer UI do not exist.

- [x] **Step 3: Implement dashboard API client**

Add `AdminUserScheduledPost` and `getAdminUserScheduledPosts(token, id)` to `dashboard/src/lib/api.ts`.

- [x] **Step 4: Implement users page drawer**

Import the new API type/function. Add drawer state, `openScheduledPosts`, `closeScheduledPosts`, nonzero Scheduled button, and a fixed right-side drawer with loading/error/empty states. Keep the existing `View` detail panel unchanged.

- [x] **Step 5: Run focused dashboard source test and confirm pass**

Run: `cd dashboard && node --test tests/admin-users-scheduled-source.test.mjs`

Expected: PASS.

### Task 3: Validation, Merge, Push, and Dev Verification

**Files:**
- No additional source files unless validation exposes a scoped fix.

- [x] **Step 1: Run task branch validation**

Run:
- `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`
- `cd dashboard && npm run build`

Expected: both pass.

- [ ] **Step 2: Merge to local dev and revalidate**

Inspect `git status`, update local `dev` from `origin/dev`, merge `dev-scheduled-posts-drawer`, then rerun relevant checks on local `dev`.

- [ ] **Step 3: Push `dev` to `origin/dev`**

Push only after validation passes.

- [ ] **Step 4: Monitor checks and deployed dev environment**

Wait for triggered checks/deployments to finish, then verify `https://dev-app.unipost.dev/admin/users` shows nonzero Scheduled counts as drawer triggers and the drawer lists scheduled posts with title, creation time, scheduled publishing time, and platform icons.
