# Admin Quota Email Page Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an admin Email page below Posts that lists free-plan quota reminder email notifications, including trigger event, recipient email, status, threshold, usage context, and send timing.

**Architecture:** Expose a read-only admin API endpoint backed by `free_plan_quota_email_reminders`, then render it in the existing dashboard admin shell. Keep the page operational and scan-friendly: filters at the top, summary cards, then a dense table.

**Tech Stack:** Go `AdminHandler` + chi route, PostgreSQL via existing pgxpool admin query pattern, Next.js App Router client page, existing `AdminShell`, `StatCard`, `SearchHistoryInput`, and `lucide-react`.

---

### Task 1: Backend Contract

**Files:**
- Modify: `api/internal/handler/admin_test.go`
- Modify: `api/internal/handler/admin.go`
- Modify: `api/cmd/api/main.go`

- [ ] Write a failing test that expects admin email notification SQL to read `free_plan_quota_email_reminders`, join `workspaces` and `users`, expose `trigger_event`, filter by status/threshold/search, and order by `attempted_at DESC`.
- [ ] Add response/query types and helper SQL for `GET /v1/admin/email-notifications`.
- [ ] Register the route in the existing admin route group.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestAdminEmail|TestAdminSearchHistory' -count=1`.

### Task 2: Dashboard API And Regression

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Modify: `dashboard/tests/regression/dashboard.spec.ts`

- [ ] Write a failing regression test that requires `/admin/email`, the Email nav item, `listAdminEmailNotifications`, and `/v1/admin/email-notifications`.
- [ ] Add `AdminEmailNotificationRow`, `AdminEmailNotificationListParams`, and `listAdminEmailNotifications`.
- [ ] Add `admin.email.search` to the search history allowlist.
- [ ] Run `npx playwright test --config=playwright.regression.config.ts dashboard.spec.ts -g "admin email notifications"`.

### Task 3: Admin Email Page

**Files:**
- Modify: `dashboard/src/app/admin/_components/admin-ui.tsx`
- Create: `dashboard/src/app/admin/email/page.tsx`

- [ ] Add Email to the admin sidebar directly after Posts.
- [ ] Implement `/admin/email` as a client page using Clerk token, `AdminShell`, filters for status/threshold/period/limit, summary cards, and a table.
- [ ] Include loading, empty, and error states in the table surface.
- [ ] Run `npm run build` from `dashboard/`.

### Task 4: Release Flow

**Files:**
- Validate changed backend and dashboard surfaces.

- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./...` from `api/`.
- [ ] Run `npm run build` from `dashboard/`.
- [ ] Commit on `dev-admin-quota-email-page`.
- [ ] Merge into local `dev`, rerun validation, push `origin/dev`, monitor GitHub/Vercel/Railway, and verify dev admin API/page.
- [ ] Promote `dev` to `staging`, rerun validation, push `origin/staging`, monitor deployments, and verify staging admin API/page.
- [ ] Promote `staging` to `main`, rerun validation, push `origin/main`, monitor deployments, and verify production `https://app.unipost.dev/admin/email`.
