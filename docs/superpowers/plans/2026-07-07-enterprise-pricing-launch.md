# Enterprise Pricing Launch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the Phase 2 Enterprise pricing-page and docs changes from `docs/prd-enterprise-plan.md`, then promote and verify them in production.

**Architecture:** Keep Free/API/Basic/Growth/Team as the only self-serve card grid. Upgrade the existing Enterprise footer banner into a distinct sales-led Enterprise section, add Team fair-use language and Enterprise FAQ semantics, and align developer pricing docs with the buyer-facing page. Add source-level regression tests to guard the plan boundary.

**Tech Stack:** Next.js 16 App Router, React 19, inline pricing page CSS, Node `node:test`, Playwright regression suite for visual/public route checks.

---

### Task 1: Source Regression Coverage

**Files:**
- Create: `dashboard/tests/enterprise-pricing-source.test.mjs`

- [ ] **Step 1: Add the failing source tests**

Create `dashboard/tests/enterprise-pricing-source.test.mjs` with:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing page keeps Enterprise out of the self-serve card grid", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");
  const tiersStart = pricing.indexOf("const TIERS");
  const tiersEnd = pricing.indexOf("];", tiersStart);
  const tiersSource = pricing.slice(tiersStart, tiersEnd);

  assert.doesNotMatch(tiersSource, /id:\s*"enterprise"/);
  assert.match(pricing, /Priority support, capacity planning, and custom platform-volume terms for high-scale teams\./);
  assert.match(pricing, /Team has no monthly UniPost post quota/);
  assert.match(pricing, /Contact sales/);
  assert.doesNotMatch(pricing, /Reserved capacity, SLA, and custom platform-volume terms for high-scale teams\./);
});

test("pricing FAQ explains Team unlimited and Enterprise Custom semantics", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  assert.match(pricing, /What does unlimited Team usage mean\?/);
  assert.match(pricing, /When do I need Enterprise instead of Team\?/);
  assert.match(pricing, /Can Enterprise increase third-party platform quotas\?/);
  assert.match(pricing, /Custom means contract-defined terms/);
  assert.match(pricing, /not a smaller quota than Team/);
});

test("docs pricing describes Enterprise as custom contract terms", async () => {
  const docsPricing = await source("src/app/docs/pricing/page.tsx");

  assert.match(docsPricing, /\["Enterprise",\s*"Custom",\s*"Contract"/);
  assert.match(docsPricing, /Enterprise Custom means contract-defined terms/);
  assert.match(docsPricing, /may include no UniPost monthly post quota/);
  assert.match(docsPricing, /cannot override platform-owned rate limits/);
});
```

- [ ] **Step 2: Run the new test and confirm it fails**

Run:

```bash
cd dashboard && node --test tests/enterprise-pricing-source.test.mjs
```

Expected: FAIL because the new Enterprise copy and FAQ/docs semantics do not exist yet.

### Task 2: Pricing Page Implementation

**Files:**
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx`

- [ ] **Step 1: Update the Enterprise banner copy and visual structure**

In `pricing-page-client.tsx`, replace the current Enterprise title, description, chips, and CTA text with copy centered on:

- `Enterprise`
- `Priority support, capacity planning, and custom platform-volume terms for high-scale teams.`
- `Capacity planning`
- `Platform-volume planning`
- `SLA and security`
- `Contact sales`

Do not add `enterprise` to `TIERS`.

- [ ] **Step 2: Add Team fair-use boundary near quota behavior**

Add public copy stating:

```text
Team has no monthly UniPost post quota. Platform safety limits, third-party API quotas, abuse controls, and shared-infrastructure fairness still apply. Customers needing capacity planning, SLA, or custom platform-volume terms should use Enterprise.
```

- [ ] **Step 3: Add the Enterprise FAQ entries**

Add these questions and answer semantics to `FAQS`:

- `What does unlimited Team usage mean?`
- `When do I need Enterprise instead of Team?`
- `Can Enterprise increase third-party platform quotas?`

Ensure the answers say Enterprise Custom is contract-defined and not a smaller quota than Team.

- [ ] **Step 4: Run the source test again**

Run:

```bash
cd dashboard && node --test tests/enterprise-pricing-source.test.mjs
```

Expected: the pricing-page assertions pass, with docs assertions still failing until Task 3.

### Task 3: Docs Pricing Alignment

**Files:**
- Modify: `dashboard/src/app/docs/pricing/page.tsx`

- [ ] **Step 1: Update Enterprise plan behavior copy**

Update the Enterprise row and usage notes so docs state:

- Enterprise posts are `Custom`.
- Enterprise quota behavior is `Contract`.
- Enterprise Custom means contract-defined terms.
- Contract terms may include no UniPost monthly post quota.
- Third-party platform rate limits and review processes still apply.

- [ ] **Step 2: Run the source test**

Run:

```bash
cd dashboard && node --test tests/enterprise-pricing-source.test.mjs
```

Expected: PASS.

### Task 4: Full Local Validation

**Files:**
- No additional file edits expected.

- [ ] **Step 1: Run dashboard build**

Run:

```bash
cd dashboard && npm run build
```

Expected: PASS.

- [ ] **Step 2: Run relevant source tests**

Run:

```bash
cd dashboard && node --test tests/enterprise-pricing-source.test.mjs tests/team-unlimited-posts-source.test.mjs tests/docs-pricing-guide-source.test.mjs
```

Expected: PASS.

- [ ] **Step 3: Run dashboard regression if browsers are installed**

Run:

```bash
cd dashboard && npm run test:regression:dashboard
```

Expected: PASS. If Playwright browsers are not installed, record the exact skipped reason.

### Task 5: Release and Environment Verification

**Files:**
- No source edits expected.

- [ ] **Step 1: Commit the implementation**

Run:

```bash
git add docs/prd-enterprise-plan.md docs/superpowers/plans/2026-07-07-enterprise-pricing-launch.md dashboard/src/app/pricing/pricing-page-client.tsx dashboard/src/app/docs/pricing/page.tsx dashboard/tests/enterprise-pricing-source.test.mjs
git commit -m "feat: launch enterprise pricing positioning"
```

- [ ] **Step 2: Merge to local dev and push origin/dev**

Update local `dev` from `origin/dev`, merge this branch, rerun required validation, and push `dev` to `origin/dev`.

- [ ] **Step 3: Verify development**

Wait for the development deployment and checks. Verify `https://dev.unipost.dev/pricing` and `https://dev.unipost.dev/docs/pricing` show the Enterprise section, Team fair-use copy, and Enterprise Custom docs semantics.

- [ ] **Step 4: Promote through staging**

Create and merge a PR from `dev` to `staging` after checks pass. Wait for staging deployment. Verify `https://staging.unipost.dev/pricing` and `https://staging.unipost.dev/docs/pricing`.

- [ ] **Step 5: Promote to production**

Create and merge a PR from `staging` to `main` after checks pass. Wait for production deployment. Verify `https://unipost.dev/pricing` and `https://unipost.dev/docs/pricing`.
