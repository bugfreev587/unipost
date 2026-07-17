# Team Plan Regression Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close Team Plan regression gaps, fix every defect exposed by the new tests, and promote the verified change from staging to production and back to dev.

**Architecture:** Add focused Go tests around real sqlc query paths using the repository's `db.DBTX` fakes, keep server-side plan and role checks authoritative, and add thin Dashboard gates for user experience. A release acceptance script and explicit environment checklists exercise the same Team contract against staging and production with disposable resources.

**Tech Stack:** Go 1.24, chi, sqlc/pgx, Node.js test runner, Next.js, Playwright, Clerk, Railway Postgres, GitHub Actions, Vercel, Railway.

---

### Task 1: Fail closed for API keys whose creator is no longer active

**Files:**
- Create: `api/internal/auth/dualauth_api_key_test.go`
- Modify: `api/internal/auth/dualauth.go`

- [ ] **Step 1: Extract an API-key authentication store seam without changing behavior**

Define an internal interface containing `GetAPIKeyByHash`, `UpdateAPIKeyLastUsedAt`, and `GetMembership`, then have `authenticateAPIKey` accept that interface. `*db.Queries` already satisfies it.

- [ ] **Step 2: Write failing authentication tests**

Use a fake store and a real HTTP request/recorder. Assert that an active editor receives `RoleEditor`, a demoted member immediately receives the new role, and missing/inactive/error memberships return `401 UNAUTHORIZED`. Assert that a legacy key with an empty `CreatedByUserID` retains Owner compatibility and a revoked key remains unauthorized.

- [ ] **Step 3: Run the focused test and verify RED**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/auth -run 'TestAuthenticateAPIKey' -count=1`

Expected: missing/inactive/error membership cases fail because the current implementation returns an authenticated Owner context.

- [ ] **Step 4: Implement the minimal fail-closed behavior**

For keys with a non-empty creator, return `401 UNAUTHORIZED` unless `GetMembership` succeeds and the membership is active. Do not distinguish missing rows from transient database errors in the response. Keep the legacy creator-less key path unchanged.

- [ ] **Step 5: Verify GREEN and commit**

Run the focused test and `GOCACHE=/tmp/unipost-go-build go test ./internal/auth ./internal/handler`. Commit only the auth change and its tests.

### Task 2: Lock the Team entitlement bundle and unlimited semantics

**Files:**
- Modify: `api/internal/quota/checker_test.go`
- Modify: `api/internal/handler/free_plan_limits_test.go`
- Modify: `api/internal/handler/me_features_test.go`
- Modify: `api/internal/xcredits/catalog_test.go`

- [ ] **Step 1: Add failing/characterization entitlement tests**

Add table tests for Team and finite plans covering profiles, members, API keys, webhooks, managed accounts/users, Inbox, Analytics, X, white label, Hosted Connect branding, attribution removal, and the all-platform credential limit. Model a plan lookup error separately and assert it is not reported as an explicit Team entitlement.

- [ ] **Step 2: Run focused tests**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/quota ./internal/handler ./internal/xcredits -run 'Team|PlanGate|Limits' -count=1`

If a contract assertion fails, record the root cause before changing implementation. Existing correct behavior becomes characterization coverage.

- [ ] **Step 3: Fix only proven entitlement defects**

Preserve the existing free-only hard-coded caps for API keys/webhooks/managed resources. Do not add unused plan columns. Make errors distinguishable only where the public limits response currently conflates a lookup failure with Team unlimited.

- [ ] **Step 4: Verify generated catalog consistency and commit**

Run `node scripts/generate-x-credits-catalog.mjs --check`, then the focused Go tests. Keep explicit 30,000/3,000 assertions because they protect the published Team contract; the generator check protects source/artifact consistency.

### Task 3: Enforce Team-only Audit Log access

**Files:**
- Modify: `api/internal/quota/checker.go`
- Modify: `api/internal/quota/checker_test.go`
- Modify: `api/internal/handler/plan_gate.go`
- Modify: `api/internal/handler/plan_gate_test.go`
- Modify: `api/cmd/api/main.go`
- Modify: `api/internal/handler/me.go`
- Modify: `api/internal/handler/me_features_test.go`
- Modify: `dashboard/src/components/dashboard/plan-gate.tsx`
- Modify: `dashboard/src/app/(dashboard)/settings/audit-log/page.tsx`
- Create: `dashboard/tests/regression/team-plan-gating.spec.ts`

- [ ] **Step 1: Write failing server gate tests**

Add `PlanAllowsAuditLog` expectations: Team and Enterprise true; Free/API/Basic/Growth/unknown/error false. Add middleware tests expecting 402 `PLAN_FEATURE_NOT_AVAILABLE` for disallowed plans, 500 for missing workspace context, and downstream execution for Team.

- [ ] **Step 2: Verify RED**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/quota ./internal/handler -run 'AuditLog|PlanGate' -count=1`

Expected: compilation/test failure because the Audit Log plan decision and middleware do not exist.

- [ ] **Step 3: Add the server-side gate**

Implement fail-closed `PlanAllowsAuditLog`, add `RequirePlanAuditLog`, mount it on `GET /v1/audit-log`, and add `audit_log` to `/v1/me/plan-gates` and `/v1/me/features`.

- [ ] **Step 4: Write and verify Dashboard gating tests**

Add `audit_log` to the PlanGate feature union and copy map, read it from `/v1/limits` or the authenticated plan-gate surface, and wrap the Audit Log page. The regression test must assert the page is wrapped and the API route is server-gated.

- [ ] **Step 5: Run API and Dashboard focused tests and commit**

Run Go focused tests plus `npx playwright test tests/regression/team-plan-gating.spec.ts --config=playwright.regression.config.ts`.

### Task 4: Add API key audit coverage and best-effort guarantees

**Files:**
- Create: `api/internal/audit/audit_test.go`
- Create: `api/internal/handler/api_keys_test.go`
- Modify: `api/internal/handler/api_keys.go`

- [ ] **Step 1: Write failing API key audit tests**

Using a `db.DBTX` fake, create and revoke a key through the real handler. Capture `WriteAuditLog` arguments and assert `API_KEY.CREATED`/`API_KEY.REVOKED`, config category, actor, workspace, key resource ID, and key name. Recursively inspect serialized audit JSON and assert the raw key/hash is absent.

- [ ] **Step 2: Verify RED**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'APIKey.*Audit' -count=1`

Expected: no audit write is observed.

- [ ] **Step 3: Add secret-safe best-effort events**

Call `audit.Log` after successful create/revoke. Include only key ID/name/environment/prefix and actor identity. Do not include plaintext or hash.

- [ ] **Step 4: Prove best-effort behavior**

Make the fake return an error from `WriteAuditLog`; assert Create remains 201 and Revoke remains 204. Add an `internal/audit` test confirming invalid/unserializable optional JSON is omitted rather than exposing data or failing the caller.

- [ ] **Step 5: Verify and commit**

Run `go test ./internal/audit ./internal/handler` with the focused pattern, then the full packages.

### Task 5: Add platform credential audit and secret-redaction coverage

**Files:**
- Modify: `api/internal/handler/platform_credentials.go`
- Modify: `api/internal/handler/platform_credentials_test.go`

- [ ] **Step 1: Write failing create/delete audit tests**

Extend the existing DBTX fake to capture audit writes and deletion errors. Assert `PLATFORM_CREDENTIAL.CREATED` and `PLATFORM_CREDENTIAL.DELETED` contain platform/client ID metadata but no plaintext secret, encrypted secret, OAuth secret, or request body.

- [ ] **Step 2: Verify RED**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'PlatformCredentials_.*Audit' -count=1`

- [ ] **Step 3: Add events and correct delete error handling**

Emit best-effort audit events only after successful mutations. If the existing Delete handler ignores a database error, keep the failing test and return a stable 500 instead of incorrectly reporting 204.

- [ ] **Step 4: Verify best-effort and commit**

Assert audit failure does not change 201/204 primary results; run the full platform credential test file.

### Task 6: Cover member lifecycle, role boundaries, and audit events

**Files:**
- Create: `api/internal/handler/members_test.go`
- Modify: `api/internal/handler/members.go` only for defects proven by tests

- [ ] **Step 1: Build a deterministic members DBTX fake**

Support membership/invite sqlc queries used by Invite, AcceptInvite, ChangeRole, Remove, RevokeInvite, and TransferOwnership. Capture mutation and audit calls and maintain in-memory membership state.

- [ ] **Step 2: Add lifecycle tests**

Cover Team invitations beyond Growth's three-user limit, duplicate and expired invites, admin/editor boundaries, self-escalation, cross-workspace targets, last-owner protection, removal, and ownership transfer. Assert stable HTTP/error-code contracts and exact member audit actions.

- [ ] **Step 3: Run tests and diagnose every failure**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/handler -run 'Members|Invite|Ownership' -count=1`

For each failure, trace the handler and SQL path, retain the failing case, and apply one minimal fix at a time.

- [ ] **Step 4: Verify best-effort audit and commit**

Inject audit failures into representative Invite and Transfer operations and prove the primary mutation succeeds. Run the entire handler package.

### Task 7: Extend media cleanup lifecycle regression tests

**Files:**
- Modify: `api/internal/worker/media_cleanup_test.go`
- Modify: `api/internal/worker/media_cleanup.go` only for defects proven by tests
- Modify: `api/internal/handler/social_posts_media_retention_test.go`

- [ ] **Step 1: Add boundary and transition tests**

Keep existing 30/60 policy tests. Add tests for exact cleanup eligibility, terminal state transitions writing cleanup times, active states retaining media, repeated cleanup, failed object deletion remaining retryable, and rows from different posts/workspaces remaining isolated.

- [ ] **Step 2: Run focused tests and retain RED cases**

Run: `GOCACHE=/tmp/unipost-go-build go test ./internal/worker ./internal/handler -run 'Media.*(Cleanup|Retention)' -count=1`

- [ ] **Step 3: Fix only lifecycle defects**

Do not use migration 052 comments as the contract. Keep cleanup idempotent and preserve rows when object deletion fails.

- [ ] **Step 4: Verify and commit**

Run both full packages after focused tests pass.

### Task 8: Resolve pricing support copy and add Team UI regressions

**Files:**
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Create: `dashboard/tests/team-plan-contract-source.test.mjs`
- Modify: `dashboard/tests/regression/team-plan-gating.spec.ts`

- [ ] **Step 1: Write failing source tests**

Assert the Team card retains `Priority support`; the Enterprise FAQ and section use `Dedicated support`, SLA, security review, procurement, and capacity planning without saying priority support requires Enterprise; Audit Log remains Team-only in the comparison matrix.

- [ ] **Step 2: Verify RED**

Run: `node --test tests/team-plan-contract-source.test.mjs`

Expected: current Enterprise FAQ/description conflict with the Team card.

- [ ] **Step 3: Apply the minimal copy correction**

Replace Enterprise's generic `Priority support` wording with `Dedicated support` and keep all existing Enterprise differentiators.

- [ ] **Step 4: Add source assertions for Team routes and controls**

Cover Members, API Keys, Audit Log, Credentials, Analytics, and Inbox routes; assert no indefinite-loading path remains when requests return a handled error or empty data.

- [ ] **Step 5: Verify and commit**

Run the new Node test and the focused Playwright file.

### Task 9: Add deployed Team acceptance runner

**Files:**
- Create: `dashboard/scripts/team-plan-acceptance.mjs`
- Modify: `dashboard/package.json`
- Create: `docs/team-plan-release-acceptance.md`

- [ ] **Step 1: Write runner contract tests before implementation**

Create a Node test that imports environment validation and cleanup-ledger helpers. Assert production rejects staging domains, staging rejects production domains, release mode rejects missing owner/admin/editor identities, cleanup runs after a thrown assertion, and a non-empty cleanup ledger fails the run.

- [ ] **Step 2: Verify RED**

Run: `node --test tests/team-plan-acceptance.test.mjs`

- [ ] **Step 3: Implement the acceptance runner**

Use Clerk short-lived sessions and API calls against configured domains. Prefix all artifacts with `codex-team-acceptance-<timestamp>`, maintain an append-only cleanup ledger, verify Team limits and gates, exercise profile/member/key/credential/audit flows, visit Team Dashboard routes with Playwright, and clean resources in `finally`. Never accept an existing customer workspace ID.

- [ ] **Step 4: Document required secrets and emergency cleanup**

Document environment variables, staging/production commands, expected assertions, read/write scope, cleanup queries, and how to revoke every temporary Clerk session or API key after interruption.

- [ ] **Step 5: Run local contract tests and commit**

Do not run production mode locally until the committed runner passes all unit/contract tests.

### Task 10: Full local verification and code review

**Files:**
- Modify only files required by review findings

- [ ] **Step 1: Run formatting and generation checks**

Run `gofmt` on changed Go files, `node scripts/generate-x-credits-catalog.mjs --check`, and `git diff --check`.

- [ ] **Step 2: Run required local gates**

Run `GOCACHE=/tmp/unipost-go-build go test ./...`, `npm run build`, and `npm run test:regression:dashboard`.

- [ ] **Step 3: Request code review**

Review the complete diff from `origin/staging` to HEAD for security, test realism, product-contract coverage, secret handling, and cleanup safety. Fix every critical and important finding with a new failing test where behavior changes.

- [ ] **Step 4: Re-run all gates and inspect scope**

Verify only requested files are changed and no artifacts, Playwright outputs, credentials, or temporary IDs are tracked.

### Task 11: Stage, deploy, and accept staging

**Files:**
- No source changes unless staging reveals a reproducible defect

- [ ] **Step 1: Update local staging and merge the hotfix**

Fetch origin, update the dedicated staging worktree to `origin/staging`, merge `hotfix-team-plan-coverage`, and rerun all local gates on the merged staging result.

- [ ] **Step 2: Push staging and monitor every triggered check**

Push `staging` to `origin/staging`; wait for GitHub Actions, Vercel `unipost-staging`, Railway `staging`, and every visible deployment/check to finish successfully.

- [ ] **Step 3: Run staging acceptance**

Execute the acceptance runner only against `https://staging-api.unipost.dev` and `https://staging-app.unipost.dev`. Confirm cleanup leaves no removable artifacts.

- [ ] **Step 4: Repair failures in the correct source branch**

For any staging failure, reproduce it as a failing local test, fix on `hotfix-team-plan-coverage`, remerge staging, repush, and repeat monitoring/acceptance.

### Task 12: Promote to production, accept, and sync dev

**Files:**
- No source changes unless an environment exposes a reproducible defect

- [ ] **Step 1: Validate the promotion diff**

Fetch origin; inspect `origin/main...origin/staging`, GitHub PR changed files, and a local merge-result simulation. Confirm production-only CiteLoop files remain and unrelated staging drift is absent.

- [ ] **Step 2: Create and merge the `staging` to `main` PR**

Wait for all required PR checks, merge only when green, then monitor Vercel production, Railway production, GitHub Actions, and all triggered checks to completion.

- [ ] **Step 3: Run production acceptance**

Execute the runner only against `https://api.unipost.dev` and `https://app.unipost.dev` using a dedicated disposable Team workspace. Verify all writes, role boundaries, audit entries, secret redaction, UI states, and final cleanup.

- [ ] **Step 4: Sync the exact change back to dev**

Merge or cherry-pick the hotfix commits into local `dev` without overwriting unrelated work, run all required gates, push `origin/dev`, wait for development deployments, and verify `dev-api.unipost.dev` and `dev-app.unipost.dev`.

- [ ] **Step 5: Report evidence**

Report commits, PR, check/deployment results, staging/production/dev acceptance results, defects found and fixed, permanent tests added, and any explicitly untestable operational commitment.
