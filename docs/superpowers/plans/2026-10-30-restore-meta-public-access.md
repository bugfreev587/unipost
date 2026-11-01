# Restore Meta Public Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore public Facebook Pages, Instagram, and Threads connection and publishing without requesting unapproved Facebook Messenger access.

**Architecture:** Retire the Facebook audit-only gates at the dashboard and API boundaries, keep existing authorization and ownership checks intact, and narrow the public Facebook OAuth scope set. Release through staging before production, then publish the Meta App and verify fresh grants.

**Tech Stack:** Go, Chi, PostgreSQL/sqlc, Next.js/React/TypeScript, Node test runner, Railway, Vercel, Meta for Developers.

---

### Task 1: Specify public Facebook availability

**Files:**
- Create: `dashboard/tests/facebook-public-access-source.test.mjs`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Modify: `dashboard/src/components/dashboard/shell.tsx`
- Modify: `dashboard/src/lib/features-in-dev.ts`

- [ ] **Step 1: Write the failing source-contract test**

```js
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("Facebook Pages is available to regular dashboard users", () => {
  const accounts = read("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
  const shell = read("src/components/dashboard/shell.tsx");
  const features = read("src/lib/features-in-dev.ts");

  assert.match(accounts, /id:\s*"facebook"/);
  assert.doesNotMatch(accounts, /isFacebookEnabledForMe|FACEBOOK_PLATFORM/);
  assert.doesNotMatch(shell, /isFacebookEnabledForMe/);
  assert.doesNotMatch(features, /facebook_pages/);
});
```

- [ ] **Step 2: Run the test and verify the expected failure**

Run: `cd dashboard && node --test tests/facebook-public-access-source.test.mjs`

Expected: FAIL because the page still uses `isFacebookEnabledForMe` and `facebook_pages` remains in `FEATURES_IN_DEV`.

- [ ] **Step 3: Make Facebook an always-available platform**

Place `{ id: "facebook", name: "Facebook Page", type: "oauth" as const }` directly in `BASE_PLATFORMS`, remove `FACEBOOK_PLATFORM`, remove `isFacebookEnabledForMe`, and remove `facebook_pages` from `FEATURES_IN_DEV`. Preserve the unrelated AI-assist feature.

- [ ] **Step 4: Run the focused test**

Run: `cd dashboard && node --test tests/facebook-public-access-source.test.mjs`

Expected: PASS.

### Task 2: Specify approved public Facebook OAuth scopes

**Files:**
- Modify: `api/internal/platform/facebook_test.go`
- Modify: `api/internal/connect/facebook_test.go`
- Modify: `api/internal/platform/facebook.go`
- Modify: `api/internal/connect/facebook.go`

- [ ] **Step 1: Change the OAuth tests first**

Update the adapter expected scope string to omit `pages_messaging`, assert that `FacebookPagesScopes` still contains the future permission, and add a hosted-connector assertion that the generated `scope` omits `pages_messaging`.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/connect`

Expected: FAIL because current OAuth URLs include `pages_messaging`.

- [ ] **Step 3: Split future Messenger permission from public OAuth scopes**

In `api/internal/platform/facebook.go`, move `pages_messaging` into a dedicated future-permission slice included by `FacebookPagesScopes` but not `FacebookOAuthScopes`. Remove `pages_messaging` from `facebookConnectScopes` in `api/internal/connect/facebook.go`.

- [ ] **Step 4: Run focused scope tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/platform ./internal/connect`

Expected: PASS.

### Task 3: Specify regular-user Facebook OAuth and Page access

**Files:**
- Create: `api/cmd/api/facebook_public_access_wiring_test.go`
- Modify: `api/internal/handler/oauth_test.go`
- Modify: `api/internal/handler/oauth.go`
- Modify: `api/internal/handler/oauth_facebook.go`
- Modify: `api/cmd/api/main.go`

- [ ] **Step 1: Add failing source-wiring and handler tests**

Add a source-wiring test that rejects `RequireFacebookSuperAdmin` in Page-insights and pending-connection route registration. Add a handler test proving a Facebook connect request with a nil/non-super-admin checker reaches normal profile resolution instead of returning `FACEBOOK_DISABLED`.

- [ ] **Step 2: Run the focused tests and verify failure**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api ./internal/handler`

Expected: FAIL because current handlers and routes still enforce the audit-only Facebook gate.

- [ ] **Step 3: Remove only Facebook audit gates**

Delete the start and callback gate blocks from `oauth.go`, delete `callerIsFacebookSuperAdmin` from `oauth_facebook.go`, and register Page-insights and pending-connection routes without `RequireFacebookSuperAdmin`. Keep ownership, role, plan, and free-plan-sharing checks unchanged.

- [ ] **Step 4: Run focused API tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./cmd/api ./internal/handler`

Expected: PASS.

### Task 4: Validate the hotfix locally

**Files:**
- Modify only if validation exposes a defect in the approved scope.

- [ ] **Step 1: Run the complete backend suite**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./...`

Expected: PASS with zero failures.

- [ ] **Step 2: Run the dashboard contract test and build**

Run: `cd dashboard && node --test tests/facebook-public-access-source.test.mjs && npm run build`

Expected: PASS and successful Next.js build.

- [ ] **Step 3: Run dashboard regression**

Run: `cd dashboard && npm run test:regression:dashboard`

Expected: PASS. A skip, timeout, cancellation, or missing browser is a hard failure.

- [ ] **Step 4: Audit the exact hotfix diff**

Run: `git diff --check && git diff --name-status origin/staging...HEAD && git log --oneline origin/staging..HEAD`

Expected: only the Meta public-access hotfix, its tests, and these design/plan documents.

### Task 5: Preview, staging, and production release

**Files:**
- No source changes unless a gate fails.

- [ ] **Step 1: Push the hotfix and open a Draft PR to staging**

Push `hotfix-restore-meta-public-access`, open a Draft PR to `staging`, and wait for every CI, Railway PR Environment, Vercel Preview, and deployed regression check on the exact head SHA.

- [ ] **Step 2: Browser-accept the preview**

Verify that a regular user sees Facebook, Instagram, and Threads, Facebook authorization URL creation succeeds, and TikTok remains unavailable according to the incident policy.

- [ ] **Step 3: Merge to staging and verify staging**

After all preview gates pass, merge to staging, wait for deployment, and repeat connection-flow acceptance on `staging-app.unipost.dev` and `staging-api.unipost.dev`.

- [ ] **Step 4: Promote staging to main**

Create the production PR from `staging` to `main`, audit unique commits/files, merge only after checks pass, and wait for Railway/Vercel production deployments.

- [ ] **Step 5: Obtain action-time confirmation and publish the Meta App**

On the Meta Publish page, show the user the final state and obtain confirmation immediately before clicking Publish. Do not proceed without that confirmation.

- [ ] **Step 6: Production acceptance**

Verify fresh Facebook, Instagram, and Threads connections and one controlled test post per platform. Confirm production health and that TikTok remains unchanged.

- [ ] **Step 7: Synchronize the hotfix back to dev**

Update the owned hotfix branch with latest `origin/dev`, resolve only cleanly applicable changes, rerun required validation, open a PR to `dev`, complete Preview Acceptance, merge, and verify the development environment. Stop and ask if the sync conflicts.

