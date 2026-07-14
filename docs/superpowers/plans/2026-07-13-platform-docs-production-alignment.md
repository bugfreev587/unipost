# Platform Documentation Production Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align every scoped platform, connection, credential, white-label, and platform-marketing page with current UniPost production behavior and prevent the same high-risk claims from drifting again.

**Architecture:** Treat the production capabilities endpoint and `origin/main` implementation as the product truth, then use a focused Node source-regression test to lock cross-page invariants. Preserve existing page components and routing; update only the platform data/configuration and prose whose claims are disproved by production evidence.

**Tech Stack:** Next.js 16, React 19, TypeScript/TSX, Node.js built-in test runner, Playwright regression suite.

---

### Task 1: Add failing cross-page production-alignment tests

**Files:**
- Create: `dashboard/tests/platform-docs-production-alignment-source.test.mjs`
- Modify: `dashboard/package.json`

- [ ] **Step 1: Write the failing source-regression test**

Create a Node test that loads the scoped source files and asserts the known production invariants:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

test("platform docs do not retain obsolete production claims", async () => {
  const [platformDocs, marketingConfig, marketingPage] = await Promise.all([
    read("src/app/docs/platforms/[platform]/_data.tsx"),
    read("src/app/(platforms)/_config/platforms.ts"),
    read("src/app/(platforms)/_components/platform-page.tsx"),
  ]);

  assert.doesNotMatch(platformDocs, /X removed the shared app path|No shared Quickstart app/);
  assert.doesNotMatch(platformDocs, /FEATURE_FACEBOOK_REELS|facebook_reels_unsupported/);
  assert.match(platformDocs, /Watch time/);
  assert.match(platformDocs, /Subscribers gained/);
  assert.doesNotMatch(marketingConfig, /X\/Twitter[^\n]+Free plan available/);
  assert.match(marketingPage, />9 platforms supported</);
});

test("connection guides keep OAuth and app-password modes distinct", async () => {
  const [quickstart, connectSessions, platformCredentials, whiteLabel, platformDocs] = await Promise.all([
    read("src/app/docs/quickstart/page.tsx"),
    read("src/app/docs/connect-sessions/page.tsx"),
    read("src/app/docs/platform-credentials/page.tsx"),
    read("src/app/docs/white-label/page.tsx"),
    read("src/app/docs/platforms/[platform]/_data.tsx"),
  ]);

  assert.match(quickstart, /X/);
  assert.match(connectSessions, /allow_quickstart_creds=true/);
  assert.match(platformCredentials, /shared OAuth app/);
  assert.match(whiteLabel, /shared OAuth app/);
  assert.match(platformDocs, /Handle \+ app password — no OAuth/);
});
```

- [ ] **Step 2: Add the test to the documentation test script**

Append `tests/platform-docs-production-alignment-source.test.mjs` to `test:docs-ai` in `dashboard/package.json` so CI-equivalent documentation validation runs it.

- [ ] **Step 3: Run the focused test and verify RED**

Run:

```bash
cd dashboard
node --test tests/platform-docs-production-alignment-source.test.mjs
```

Expected: failure on the obsolete X shared-app wording, Facebook Reels feature-flag wording, incomplete YouTube analytics wording, `8 platforms supported`, and the X SEO free-plan claim.

- [ ] **Step 4: Commit the failing regression test**

```bash
git add dashboard/package.json dashboard/tests/platform-docs-production-alignment-source.test.mjs
git commit -m "test: lock platform documentation production facts"
```

### Task 2: Align connection-mode and credential documentation

**Files:**
- Modify: `dashboard/src/app/docs/platforms/[platform]/_data.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/quickstart/page.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/connect-sessions/page.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/api/connect/sessions/create/page.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/platform-credentials/page.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/platform-credentials/[platform]/_data.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/white-label/page.tsx`

- [ ] **Step 1: Record the production connection evidence for all nine platforms**

Compare `GET https://api.unipost.dev/v1/platforms/capabilities`, `connectablePlatforms`, `connectSessionPlatformUsesOAuthApp`, production connector registration, plan gates, and the Bluesky app-password connector. The resulting matrix must identify X, LinkedIn, Instagram, Threads, TikTok, YouTube, Pinterest, and Facebook as OAuth platforms that can use shared Quickstart credentials in Connect Sessions, with X requiring a paid plan; Bluesky remains app-password based.

- [ ] **Step 2: Correct the X setup and limitation rows**

Replace the unavailable Quickstart row with wording equivalent to:

```tsx
["Quickstart", "Use UniPost's shared X OAuth app", "UniPost-managed app", "Requires any paid plan"],
```

Replace the `No shared Quickstart app` limitation with a plan/credential distinction that explains Quickstart is available on paid plans and workspace Platform Credentials remain available for app ownership and quota control.

- [ ] **Step 3: Normalize scoped guide terminology**

Ensure the guides consistently distinguish:

- Quickstart: UniPost shared OAuth credentials.
- Connect Sessions: customer-owned account onboarding.
- Hosted Connect branding: UniPost-hosted pre-OAuth UI.
- Platform Credentials: workspace-owned upstream OAuth application identity.
- Bluesky: handle plus app password, not OAuth credentials.

Do not change prose that already matches those definitions.

- [ ] **Step 4: Run the focused test**

Run `node --test tests/platform-docs-production-alignment-source.test.mjs` from `dashboard/`.

Expected: X and connection-mode assertions pass; remaining Facebook, YouTube, marketing count, and SEO assertions still fail.

- [ ] **Step 5: Commit connection corrections**

```bash
git add dashboard/src/app/docs
git commit -m "docs: align platform connection modes with production"
```

### Task 3: Align platform capabilities, analytics, and runtime status

**Files:**
- Modify: `dashboard/src/app/docs/platforms/page.tsx`
- Modify: `dashboard/src/app/docs/platforms/[platform]/_data.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/api/analytics/_data/platform-analytics-docs.tsx`
- Modify if evidence requires: `dashboard/src/app/docs/api/analytics/_components/platform-analytics-doc-pages.tsx`

- [ ] **Step 1: Audit all nine platform detail records**

For every platform, compare summaries, capability rows, media specifications, analytics rows, inbox rows, examples, errors, and limitations against production capabilities plus the `origin/main` adapters and tests. Keep the narrower UniPost-enforced limit when it differs from an upstream maximum.

- [ ] **Step 2: Remove obsolete Facebook Reels gates**

Remove all `FEATURE_FACEBOOK_REELS` requirements, the `facebook_reels_unsupported` error row, and the feature-flag limitation. Describe Reels as the deployed `platform_options.facebook.mediaType="reel"` path while retaining its actual vertical-video and media restrictions.

- [ ] **Step 3: Update YouTube analytics wording**

Distinguish post-level metrics from the deployed dedicated YouTube analytics explorer. The platform page must mention views, likes, comments, shares, watch time, average view duration/percentage, and subscriber gains/losses where those production endpoints expose them, while preserving any reconnect-scope caveat.

- [ ] **Step 4: Correct any additional evidence-backed drift**

Update only claims disproved during the nine-platform audit. Typical corrections include obsolete rollout labels, missing newly deployed surfaces, stale analytics scope notes, and limits that differ from the production capabilities response.

- [ ] **Step 5: Run source tests**

Run:

```bash
cd dashboard
node --test tests/platform-docs-production-alignment-source.test.mjs tests/platform-analytics-docs-source.test.mjs tests/youtube-analytics-regression-source.test.mjs tests/tiktok-analytics-docs-source.test.mjs
```

Expected: all tests pass except any marketing assertion intentionally left for Task 4.

- [ ] **Step 6: Commit capability corrections**

```bash
git add dashboard/src/app/docs/platforms dashboard/src/app/docs/api/analytics
git commit -m "docs: sync platform capabilities with production"
```

### Task 4: Align platform marketing and shared SEO claims

**Files:**
- Modify: `dashboard/src/app/(platforms)/_components/platform-page.tsx`
- Modify: `dashboard/src/app/(platforms)/_config/platforms.ts`
- Modify if evidence requires: `dashboard/src/data/seo-growth-pages.ts`

- [ ] **Step 1: Audit all eight existing platform marketing configurations**

Compare hero copy, capability lists, Quickstart/native mode cards, analytics metrics, FAQs, plan availability, and metadata against the corrected platform documentation. Do not add a Facebook marketing page in this task because none currently exists.

- [ ] **Step 2: Correct the shared platform count**

Change the shared badge from `8 platforms supported` to `9 platforms supported`.

- [ ] **Step 3: Correct plan and mode contradictions**

Remove `Free plan available` from X metadata and any other X marketing claim that contradicts the paid-plan gate. Preserve the accurate statement that the other eight platforms are available on Free.

- [ ] **Step 4: Correct any additional marketing drift found by the audit**

Use the same production-backed terminology and limits as the platform documentation. Avoid layout, icon, animation, or visual-style changes.

- [ ] **Step 5: Run the focused and SEO tests**

Run:

```bash
cd dashboard
node --test tests/platform-docs-production-alignment-source.test.mjs tests/seo-public-pages-source.test.mjs
```

Expected: PASS.

- [ ] **Step 6: Commit marketing corrections**

```bash
git add 'dashboard/src/app/(platforms)' dashboard/src/data/seo-growth-pages.ts
git commit -m "docs: align platform marketing claims with production"
```

### Task 5: Complete local validation and task-branch review

**Files:**
- Review: all files changed by Tasks 1–4

- [ ] **Step 1: Run all documentation source tests**

Run `npm run test:docs-ai` and `npm run test:seo` from `dashboard/`.

Expected: PASS.

- [ ] **Step 2: Build the dashboard**

Run `npm run build` from `dashboard/`.

Expected: successful Next.js production build.

- [ ] **Step 3: Run dashboard regression tests**

Run `npm run test:regression:dashboard` from `dashboard/` when browsers are installed.

Expected: PASS. If the browser executable is unavailable, report that exact skipped check before any push.

- [ ] **Step 4: Review the final diff**

Run `git diff origin/dev...HEAD --check`, inspect every changed claim against its evidence, confirm no unrelated untracked files are included, and verify the working tree contains only the user's pre-existing untracked files.

### Task 6: Integrate into dev and verify the deployed environment

**Files:**
- No new source files unless validation finds a scoped defect.

- [ ] **Step 1: Refresh and merge into local dev**

Fetch `origin`, switch to local `dev` only after confirming the working tree is safe, update it from `origin/dev`, and merge `dev-audit-platform-docs` without including unrelated files.

- [ ] **Step 2: Repeat required validation on updated local dev**

Run `npm run test:docs-ai`, `npm run test:seo`, `npm run build`, and the dashboard regression suite when available.

- [ ] **Step 3: Push local dev**

Push local `dev` directly to `origin/dev` only after all required validation passes.

- [ ] **Step 4: Monitor remote checks and deployments**

Wait until GitHub Actions, Vercel `unipost-dev`, Railway development, and any other triggered checks finish successfully. Inspect and fix any in-scope failure before continuing.

- [ ] **Step 5: Perform real development acceptance**

Open `https://dev.unipost.dev` in a real browser and verify the platform overview, all nine platform docs, Quickstart, Connect Sessions, Platform Credentials, White-label, X marketing, and representative non-X marketing pages. Confirm the corrected facts render without layout or routing regressions.

