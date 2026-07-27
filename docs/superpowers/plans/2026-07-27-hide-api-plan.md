# Hide API Plan from New Users Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hide the API plan from new-user frontend sales surfaces while preserving the current-plan card for existing API subscribers.

**Architecture:** A small shared frontend helper defines API as hidden for new users and allows the current plan through in Dashboard Billing. Pricing filters its existing hardcoded plan data at render time, retaining legacy API data for easy restoration and existing source contracts.

**Tech Stack:** Next.js 16, React 19, TypeScript, Node test runner, Playwright.

---

### Task 1: Lock the visibility contract with a failing test

**Files:**
- Create: `dashboard/tests/api-plan-visibility-source.test.mjs`

- [ ] **Step 1: Install Dashboard dependencies and run the existing focused baseline**

Run:

```bash
cd dashboard
npm install
node --test tests/team-plan-contract-source.test.mjs tests/enterprise-pricing-source.test.mjs
```

Expected: the existing Team and Enterprise source-contract tests pass.

- [ ] **Step 2: Write the failing source-contract test**

Create a test that reads missing files as an empty string so the initial result is an assertion failure rather than a file-read error. Assert all required render paths, copy updates, retained hidden comparison data, and existing-user Billing exception:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  try {
    return await readFile(join(root, path), "utf8");
  } catch {
    return "";
  }
}

test("API plan is hidden from new-user pricing and upgrade surfaces", async () => {
  const visibility = await source("src/lib/plan-visibility.ts");
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");
  const billing = await source("src/app/(dashboard)/settings/billing/page.tsx");

  assert.match(visibility, /planId !== "api"/);
  assert.match(visibility, /isPlanVisibleToNewUsers\(planId\) \|\| planId === currentPlanId/);
  assert.match(pricing, /const PUBLIC_TIERS = TIERS\.filter\(\(tier\) => isPlanVisibleToNewUsers\(tier\.id\)\)/);
  assert.match(pricing, /const PUBLIC_COMPARE_PLAN_IDS = COMPARE_PLAN_IDS\.filter\(isPlanVisibleToNewUsers\)/);
  assert.match(pricing, /const PUBLIC_X_CREDIT_PLANS = X_CREDIT_PLANS\.filter\(\(plan\) => isPlanVisibleToNewUsers\(plan\.id\)\)/);
  assert.match(pricing, /PUBLIC_TIERS\.map/);
  assert.match(pricing, /PUBLIC_COMPARE_PLAN_IDS\.map/);
  assert.equal((pricing.match(/PUBLIC_X_CREDIT_PLANS\.map/g) ?? []).length, 2);
  assert.doesNotMatch(pricing, /repeat\(5,/);
  assert.match(pricing, /api: false/);
  assert.match(billing, /PLANS\.filter\(\(plan\) => isPlanVisibleInBilling\(plan\.id, billing\?\.plan\)\)/);
  assert.doesNotMatch(pricing, /Why Free, API, Basic, Growth, Team\?/);
  assert.doesNotMatch(billing, /Free \/ API \/ Basic \/ Growth \/ Team/);
});
```

- [ ] **Step 3: Run the new test and verify RED**

Run:

```bash
cd dashboard
node --test tests/api-plan-visibility-source.test.mjs
```

Expected: FAIL because the shared visibility helper and filtered render paths do not exist yet.

### Task 2: Add the shared visibility rule and filter Pricing

**Files:**
- Create: `dashboard/src/lib/plan-visibility.ts`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`

- [ ] **Step 1: Add the minimal shared helper**

```ts
export function isPlanVisibleToNewUsers(planId: string): boolean {
  return planId !== "api";
}

export function isPlanVisibleInBilling(planId: string, currentPlanId?: string | null): boolean {
  return isPlanVisibleToNewUsers(planId) || planId === currentPlanId;
}
```

- [ ] **Step 2: Derive Pricing's visible plan collections**

Import `isPlanVisibleToNewUsers`. Keep `TIERS` and every `COMPARE_ROWS.api` value intact. Add:

```ts
const COMPARE_PLAN_IDS = ["free", "api", "basic", "growth", "team"] as const;
const PUBLIC_TIERS = TIERS.filter((tier) => isPlanVisibleToNewUsers(tier.id));
const PUBLIC_COMPARE_PLAN_IDS = COMPARE_PLAN_IDS.filter(isPlanVisibleToNewUsers);
const PUBLIC_X_CREDIT_PLANS = X_CREDIT_PLANS.filter((plan) => isPlanVisibleToNewUsers(plan.id));
```

Render cards from `PUBLIC_TIERS`, generate comparison headers from `PUBLIC_TIERS`, render comparison values from `PUBLIC_COMPARE_PLAN_IDS`, and render both X Credits layouts from `PUBLIC_X_CREDIT_PLANS`. Because the helper hides only `api`, the generated Enterprise X Credits row remains.

- [ ] **Step 3: Adjust Pricing grids from five plan columns to four**

Change every Pricing comparison `repeat(5, ...)` declaration to `repeat(4, ...)`, including base and responsive rules. Change the base pricing-card grid to four columns; keep the existing three-, two-, and one-column responsive card rules.

- [ ] **Step 4: Update Pricing sales copy**

Remove API from the visible plan ladder, quota/retention explanations, paid-plan starting price, API-versus-Basic FAQ, competitor price comparison, and quota behavior panel. Preserve generic product API wording and retain the hidden API data objects.

- [ ] **Step 5: Run the focused test**

Run:

```bash
cd dashboard
node --test tests/api-plan-visibility-source.test.mjs
```

Expected: still FAIL only on the Dashboard Billing assertions.

### Task 3: Filter Dashboard Billing while retaining existing API subscribers

**Files:**
- Modify: `dashboard/src/app/(dashboard)/settings/billing/page.tsx`

- [ ] **Step 1: Apply current-plan-aware filtering**

Import `isPlanVisibleInBilling`, derive the visible array after Billing loads, and render it:

```ts
const visiblePlans = PLANS.filter((plan) => isPlanVisibleInBilling(plan.id, billing?.plan));
```

Replace `PLANS.map` with `visiblePlans.map`. Update the summary to `Free / Basic / Growth / Team`; an existing API subscriber still sees the API current-plan card through the helper.

- [ ] **Step 2: Run the focused test and verify GREEN**

Run:

```bash
cd dashboard
node --test tests/api-plan-visibility-source.test.mjs
```

Expected: PASS.

- [ ] **Step 3: Commit the tested implementation**

```bash
git add dashboard/src/lib/plan-visibility.ts dashboard/src/app/pricing/pricing-page-client.tsx dashboard/src/app/'(dashboard)'/settings/billing/page.tsx dashboard/tests/api-plan-visibility-source.test.mjs
git commit -m "feat: hide API plan from new users"
```

### Task 4: Run full required validation

**Files:**
- Verify only; no planned code changes.

- [ ] **Step 1: Run focused and adjacent source contracts**

Run:

```bash
cd dashboard
node --test tests/api-plan-visibility-source.test.mjs tests/team-plan-contract-source.test.mjs tests/enterprise-pricing-source.test.mjs
```

Expected: PASS.

- [ ] **Step 2: Build Dashboard**

Run:

```bash
cd dashboard
npm run build
```

Expected: Next.js production build succeeds.

- [ ] **Step 3: Run Dashboard browser regression**

Run:

```bash
cd dashboard
npm run test:regression:dashboard
```

Expected: every Playwright regression passes. Any skipped, timed-out, or unavailable test is a failure under repository rules.

- [ ] **Step 4: Review the branch delta**

Run:

```bash
git diff --check origin/staging...HEAD
git log --oneline origin/staging..HEAD
git diff --name-status origin/staging...HEAD
```

Expected: only the approved design/plan, visibility helper, two frontend pages, and focused test are unique to the branch.
