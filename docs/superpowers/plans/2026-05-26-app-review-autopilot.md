# App Review Autopilot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the TikTok App Review Autopilot MVP behind `app_review.autopilot_v1`, including backend review-domain/kit/job/session APIs, a gated dashboard setup flow, a customer-domain review app shell, and a version-pinned local recording agent contract.

**Architecture:** The backend remains the authority for workspace access, feature flag checks, readiness state, token issuance, review scripts, and event/artifact ingestion. The dashboard only calls `/v1/me/features` and `/v1/review/*`; the public review app uses a short-lived review-session token and never connects to Unleash directly. The local CLI agent consumes a closed JSON script contract, opens a headed browser, injects the review session, records the browser window/capture region, emits elapsed-time events, and uploads evidence.

**Tech Stack:** Go/chi/sqlc/goose/pgx for API and persistence; Next.js/React/Clerk for dashboard and review app; npm package `@unipost/review-agent` using Playwright plus platform capture helpers; Unleash flag `app_review.autopilot_v1`.

---

### Task 1: Feature Flag Baseline

**Files:**
- Modify: `api/internal/featureflags/flags.go`
- Modify: `api/internal/featureflags/flags_test.go`
- Modify: `dashboard/src/lib/feature-flags.ts`
- Modify: `docs/feature-flags-unleash.md`

- [x] **Step 1: Write failing backend feature flag test**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags`
Expected: FAIL with `undefined: AppReviewAutopilotV1`.

- [x] **Step 2: Register flag and dashboard key**

Use flag key `app_review.autopilot_v1`, env fallback `FEATURE_APP_REVIEW_AUTOPILOT_V1`, development default on, production default off.

- [x] **Step 3: Verify backend flag tests pass**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/featureflags`
Expected: PASS.

### Task 2: Review Persistence Model

**Files:**
- Create: `api/internal/db/migrations/076_app_review_autopilot.sql`
- Create: `api/internal/db/queries/review.sql`
- Generate: `api/internal/db/review.sql.go`
- Modify: `api/internal/db/models.go`

- [ ] **Step 1: Add migration for review tables**

Create `review_domains`, `review_kits`, `review_jobs`, `review_job_events`, and `review_sessions`.

Key constraints:
- `review_domains.workspace_id` references `workspaces(id)` with cascade delete.
- `review_domains.domain` is globally unique.
- `review_kits.platform` is constrained to `tiktok` in MVP.
- `review_jobs.status` is constrained to `queued`, `running`, `waiting_for_user`, `completed`, or `failed`.
- `review_sessions.token_hash` is unique.

- [ ] **Step 2: Add sqlc queries**

Queries must cover create/get/list/update readiness, kit create/get/readiness, job create/get/status updates, event append/list, session create/claim/revoke.

- [ ] **Step 3: Generate sqlc code**

Run: `/Users/xiaoboyu/go/bin/sqlc generate`
Expected: generated Go code compiles.

- [ ] **Step 4: Verify DB package compiles**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/db`
Expected: PASS.

### Task 3: Review Backend Service and API

**Files:**
- Create: `api/internal/handler/review.go`
- Create: `api/internal/handler/review_test.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Write failing route/handler tests**

Cover:
- feature flag off returns `403 FEATURE_DISABLED`
- `POST /v1/review/domains` validates root/review domain and returns DNS records
- `POST /v1/review/kits` requires TikTok credentials, required scopes, redirect attestation, and ready domain
- `POST /v1/review/jobs` creates a fresh job, agent token, review session token, and version-pinned command
- `GET /v1/review/jobs/{id}/script` returns only closed-enum actions
- event append rejects unknown event types and records elapsed milliseconds

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestReview'`
Expected: FAIL before implementation.

- [ ] **Step 2: Implement handler and token helpers**

Use opaque random tokens:
- agent token prefix: `revtok_`
- session token prefix: `revsess_`
- hash tokens with SHA-256 before storing.

- [ ] **Step 3: Wire routes behind `RequireFeatureFlag(featureflags.AppReviewAutopilotV1)`**

Mount under the existing workspace-scoped route group:
- `POST /v1/review/domains`
- `GET /v1/review/domains/{id}`
- `POST /v1/review/domains/{id}/verify`
- `POST /v1/review/kits`
- `GET /v1/review/kits/{id}`
- `POST /v1/review/kits/{id}/readiness`
- `POST /v1/review/jobs`
- `GET /v1/review/jobs/{id}`
- `GET /v1/review/jobs/{id}/script`
- `POST /v1/review/jobs/{id}/events`
- `POST /v1/review/jobs/{id}/complete`
- `POST /v1/review/jobs/{id}/fail`

- [ ] **Step 4: Verify handler tests pass**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'TestReview'`
Expected: PASS.

### Task 4: Review Script Contract

**Files:**
- Create: `api/internal/reviewscript/script.go`
- Create: `api/internal/reviewscript/script_test.go`

- [ ] **Step 1: Write failing tests for closed action enum**

Test that `goto`, `click`, `fill`, `assert_visible`, `assert_url_contains`, `manual_pause`, `wait_for_navigation`, `wait_for_network_idle`, `screenshot`, and `emit_marker` validate; `eval`, `js`, and unknown actions fail.

- [ ] **Step 2: Implement script builder**

Return TikTok MVP script with start URL on customer review domain and `agent_version: "0.1.0"`.

- [ ] **Step 3: Verify package tests pass**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/reviewscript`
Expected: PASS.

### Task 5: Dashboard Setup Flow

**Files:**
- Modify: `dashboard/src/components/dashboard/shell.tsx`
- Modify: `dashboard/src/lib/api.ts`
- Create: `dashboard/src/app/(dashboard)/projects/[id]/accounts/native/review/page.tsx`
- Create: `dashboard/src/app/(dashboard)/projects/[id]/accounts/native/review/review-autopilot-client.tsx`

- [ ] **Step 1: Add API client types and functions**

Expose review domain/kit/job calls with typed responses.

- [ ] **Step 2: Add gated nav entry**

Under Connections submenu, add `App Review Autopilot` gated by `FEATURE_FLAG_KEYS.appReviewAutopilotV1`.

- [ ] **Step 3: Build readiness page**

The page must show landing/domain/brand/TikTok credentials/redirect attestation/readiness/job states, a copyable callback URI, macOS recording permission warning, and a version-pinned `npx --yes @unipost/review-agent@0.1.0 run --token ...` command after job creation.

- [ ] **Step 4: Verify dashboard build**

Run: `npm run build`
Expected: PASS.

### Task 6: Public Review App Shell

**Files:**
- Modify: `dashboard/src/proxy.ts`
- Create: `dashboard/src/app/review/tiktok/posting/page.tsx`
- Create: `dashboard/src/app/review/tiktok/posting/review-tiktok-posting-client.tsx`
- Modify: `dashboard/src/lib/api.ts`

- [ ] **Step 1: Add safe no-session state**

If no signed review session is present, render "No active review session" with no credentials/account data/publish controls.

- [ ] **Step 2: Add review session bootstrap API calls**

The app must load kit/job state from a backend endpoint using the review-session cookie or header.

- [ ] **Step 3: Render stable selectors**

Include `data-review-step="connect-tiktok"`, `creator-info`, `publish-tiktok`, and `publish-result`.

- [ ] **Step 4: Verify dashboard build**

Run: `npm run build`
Expected: PASS.

### Task 7: Local Review Agent MVP

**Files:**
- Create: `review-agent/package.json`
- Create: `review-agent/src/index.ts`
- Create: `review-agent/src/script-contract.ts`
- Create: `review-agent/src/doctor.ts`
- Create: `review-agent/src/runner.ts`
- Create: `review-agent/src/recorder.ts`
- Create: `review-agent/tests/script-contract.test.ts`

- [ ] **Step 1: Write failing contract tests**

Reject unknown script actions and require version-pinned script compatibility.

- [ ] **Step 2: Implement CLI commands**

Commands:
- `unipost-review-agent run --token revtok_xxx`
- `unipost-review-agent doctor`
- `unipost-review-agent resume --job rev_xxx`

- [ ] **Step 3: Implement mandatory preflight**

Detect macOS and print Screen Recording permission guidance before recording.

- [ ] **Step 4: Implement headed browser runner**

Fetch script, inject review-session cookie, open customer review domain, execute closed action set, emit elapsed-time events, and upload completion/failure state.

- [ ] **Step 5: Implement MVP recorder abstraction**

Provide a macOS-first capture helper interface and ffmpeg/region fallback with clear warnings when per-window capture cannot be guaranteed.

### Task 8: End-to-End Validation

**Files:**
- Potentially modify tests in `dashboard/tests`

- [ ] **Step 1: Run backend tests**

Run: `GOCACHE=/tmp/unipost-go-build go test ./...`
Expected: PASS.

- [ ] **Step 2: Run dashboard build**

Run: `npm run build`
Expected: PASS.

- [ ] **Step 3: Run dashboard regression if browsers are installed**

Run: `npm run test:regression:dashboard`
Expected: PASS or report missing browser dependency.

- [ ] **Step 4: Merge to local dev and re-run changed-surface checks**

Run backend and dashboard validations again on `dev`.

- [ ] **Step 5: Push `dev` and validate dev environment**

Use `https://dev-api.unipost.dev` and `https://dev.unipost.dev`. Confirm `/v1/me/features` exposes `app_review.autopilot_v1` in development and the gated dashboard surface appears in dev.
