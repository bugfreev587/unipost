# Welcome Signup Entry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route every public signup CTA through Clerk registration before sending new users to the protected `/welcome` onboarding page.

**Architecture:** Keep `/welcome` as the authenticated post-signup destination. Replace direct public anchors with the existing client-side `MarketingCTA`, extending that component only enough to preserve CTA copy and the homepage arrow treatment.

**Tech Stack:** Next.js 16 App Router, React 19, Clerk 7, Node test runner, Playwright.

---

### Task 1: Add the signup-entry regression test

**Files:**
- Modify: `dashboard/tests/auth-redirect-source.test.mjs`

- [ ] **Step 1: Write a failing source regression test**

Add a test that reads the homepage, blog index, blog article, and public analytics tool sources; rejects `START_BUILDING_URL` and direct `https://app.unipost.dev/welcome` anchors; and requires each source to render `MarketingCTA`.

- [ ] **Step 2: Run the test and verify RED**

Run: `node --test tests/auth-redirect-source.test.mjs`

Expected: FAIL because the public sources still contain direct `/welcome` navigation.

### Task 2: Route public CTAs through Clerk

**Files:**
- Modify: `dashboard/src/components/marketing/nav.tsx`
- Modify: `dashboard/src/app/marketing/page.tsx`
- Modify: `dashboard/src/app/blog/page.tsx`
- Modify: `dashboard/src/app/blog/[slug]/page.tsx`
- Modify: `dashboard/src/app/tools/_components/public-analytics-tool.tsx`

- [ ] **Step 1: Extend `MarketingCTA` presentation props**

Add optional `label` and `showArrow` props. Preserve existing defaults, signed-in dashboard behavior, Clerk `SignUpButton`, and `forceRedirectUrl` behavior.

- [ ] **Step 2: Replace direct signup anchors**

Use `MarketingCTA` on all affected public surfaces. Preserve each CTA's existing CSS classes and visible `Start Building` wording; preserve the homepage arrow via `showArrow`.

- [ ] **Step 3: Run the focused test and verify GREEN**

Run: `node --test tests/auth-redirect-source.test.mjs`

Expected: PASS.

### Task 3: Verify the dashboard surface

**Files:**
- No additional source changes expected.

- [ ] **Step 1: Run the dashboard build**

Run: `npm run build`

Expected: exit code 0.

- [ ] **Step 2: Run the dashboard regression suite**

Run: `npm run test:regression:dashboard`

Expected: all installed-browser tests pass.

- [ ] **Step 3: Review the diff and commit the focused change**

Commit only the signup-entry implementation, regression test, design, and plan.

### Task 4: Integrate and verify development

**Files:**
- No additional source changes expected unless validation finds a defect.

- [ ] **Step 1: Update local `dev` from `origin/dev`, merge the task branch, and rerun required validation**

- [ ] **Step 2: Push local `dev` to `origin/dev`**

- [ ] **Step 3: Monitor all triggered checks and deployments until completion**

- [ ] **Step 4: Verify the signed-out signup CTA and post-signup `/welcome` flow on `https://dev.unipost.dev` and `https://dev-app.unipost.dev`**

Expected: no anonymous 404; registration starts; completed registration reaches `/welcome`.
