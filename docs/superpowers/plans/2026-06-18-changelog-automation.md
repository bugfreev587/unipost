# Changelog Automation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the daily AI changelog candidate workflow, Discord approval links, admin confirmation, and guarded GitHub Actions publish dispatch without adding a feature flag.

**Architecture:** GitHub Actions collects commits and drafts candidates, the API stores candidates and owns signed/admin-approved state transitions, and the dashboard provides the Clerk-authenticated confirmation page. Public `/changelog` remains git-backed through `dashboard/src/app/changelog/releases.ts`; automation only inserts a verified release entry through a release workflow.

**Tech Stack:** Go/chi/pgx for API, PostgreSQL migration for candidate state, Next.js App Router for the admin confirmation page, Node.js scripts for collection/validation/release-file edits, GitHub Actions for schedules and release orchestration.

---

### Task 1: Backend Candidate Domain

**Files:**
- Create: `api/internal/changelog/types.go`
- Create: `api/internal/changelog/signer.go`
- Create: `api/internal/changelog/validator.go`
- Create: `api/internal/changelog/store.go`
- Create: `api/internal/changelog/github.go`
- Create: `api/internal/changelog/service.go`
- Create: `api/internal/changelog/*_test.go`
- Create: `api/internal/db/migrations/084_changelog_automation.sql`

- [ ] Write tests for candidate validation, SDK package guardrails, HMAC signature verification, expired links, wrong action links, and atomic status transitions.
- [ ] Add the migration for `changelog_candidates`.
- [ ] Implement focused types and validator logic.
- [ ] Implement HMAC signer/verification.
- [ ] Implement Postgres store with atomic `pending -> publishing|saved|discarded` transitions.
- [ ] Implement GitHub workflow dispatch client with dependency injection for tests.
- [ ] Run `GOCACHE=/tmp/unipost-go-build go test ./internal/changelog`.

### Task 2: API Handlers And Routes

**Files:**
- Create: `api/internal/handler/changelog_automation.go`
- Create: `api/internal/handler/changelog_automation_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] Write handler tests for automation-token candidate creation, unauthorized internal access, super-admin candidate preview, save/discard, duplicate click idempotency, and publish dispatch failure when token/config is absent.
- [ ] Implement `POST /internal/changelog-candidates`.
- [ ] Implement `GET /internal/changelog-candidates/{id}` for release workflows.
- [ ] Implement super-admin `GET /v1/admin/changelog-candidates/{id}`.
- [ ] Implement super-admin `POST /v1/admin/changelog-candidates/{id}/actions`.
- [ ] Wire routes in `api/cmd/api/main.go`.
- [ ] Run targeted Go tests.

### Task 3: Dashboard Confirmation Page

**Files:**
- Modify: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/app/admin/changelog-actions/page.tsx`
- Create/modify: `dashboard/tests/changelog-automation-source.test.mjs`

- [ ] Add source-level tests that ensure the page requires `AdminShell requireSuperAdmin`, reads signed query params, fetches candidate details with a Clerk token, and posts explicit action confirmation.
- [ ] Add typed API client helpers for candidate preview and action confirmation.
- [ ] Build a compact admin confirmation page with loading, error, already-handled, and success states.
- [ ] Run `node --test tests/changelog-automation-source.test.mjs` from `dashboard/`.

### Task 4: Node Automation Scripts

**Files:**
- Create: `scripts/changelog-automation/lib.mjs`
- Create: `scripts/changelog-automation/daily.mjs`
- Create: `scripts/changelog-automation/apply-candidate.mjs`
- Create: `scripts/changelog-automation/validate-candidate.mjs`
- Create: `scripts/changelog-automation/*.test.mjs`

- [ ] Write Node tests for Los Angeles previous-day windows, candidate schema validation, SDK registry URL construction, source hash stability, Discord message rendering, and `releases.ts` insertion.
- [ ] Implement shared validation/window/render helpers.
- [ ] Implement `daily.mjs` to collect git activity, call AI when configured, validate, post candidate to the API, and send Discord markdown links.
- [ ] Implement `apply-candidate.mjs` to fetch an approved candidate and insert it into `releases.ts`.
- [ ] Implement `validate-candidate.mjs` for workflow and local use.
- [ ] Run `node --test scripts/changelog-automation/*.test.mjs`.

### Task 5: GitHub Workflows

**Files:**
- Create: `.github/workflows/changelog-daily.yml`
- Create: `.github/workflows/changelog-publish.yml`

- [ ] Add a daily workflow using `timezone: "America/Los_Angeles"` plus manual dispatch.
- [ ] Add workflow steps that install Node, collect full git history, run `daily.mjs`, and upload the candidate artifact.
- [ ] Add a publish workflow with `workflow_dispatch` inputs from the backend.
- [ ] Add guarded release steps: fetch candidate, edit `releases.ts`, validate changelog tests, build dashboard, create PR into `dev`, then promote `dev -> staging -> main` only when `CHANGELOG_RELEASE_GITHUB_TOKEN` is configured.
- [ ] Add explicit failure messages for missing tokens/config so automation stops safely.

### Task 6: Full Validation And Dev Push

**Files:**
- All files above.

- [ ] Run API tests: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`.
- [ ] Run dashboard build: `cd dashboard && npm run build`.
- [ ] Run changelog source tests.
- [ ] Merge implementation branch into local `dev`.
- [ ] Rerun required checks on local `dev`.
- [ ] Push local `dev` to `origin/dev`.
- [ ] Monitor GitHub/Vercel/Railway checks and verify `https://dev.unipost.dev` and `https://dev-api.unipost.dev/health`.
