# SEO/GEO Emergency Recovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore and protect UniPost's developer/API homepage search intent, then make the deployed sitemap contract a blocking Preview Acceptance check.

**Architecture:** Keep the production change narrow: update the existing Next.js homepage metadata constants and add the missing Twitter metadata, protect the contract with the existing Node source-test suite and required CI, and extend the isolated Vercel Preview regression to fetch the rendered homepage plus every URL emitted by the deployed sitemap. Do not redesign the homepage, add JSON-LD, rewrite CiteLoop content, change authenticated application behavior, or modify the registration funnel in this plan.

**Tech Stack:** Next.js 16 Metadata API, TypeScript/React, Node.js test runner, Playwright, GitHub Actions, Vercel Preview Acceptance.

---

## Scope boundary

This plan implements P0 and the narrowed P1 from `docs/superpowers/specs/2026-07-18-registration-seo-recovery-design.md`.

P2 registration-event and acquisition-funnel work is a separate subsystem and requires its own implementation plan after the SEO/GEO emergency recovery is accepted.

CiteLoop's UniPost writer must remain stopped throughout implementation and promotion. If a new CiteLoop-authored UniPost commit or PR appears before its separate remediation is approved, stop this plan before any merge.

## Owned workspace

Every write, test, commit, push, merge, and deployed acceptance step must begin by proving ownership:

```bash
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-registration-seo-analysis"
test "$(git branch --show-current)" = "dev-registration-seo-analysis"
git status --short --branch
```

Expected: the absolute path and branch tests exit successfully. Unrelated changes or ownership mismatch are a hard stop.

## File map

- Modify `dashboard/src/app/marketing/page.tsx`
  - Owns homepage title, description, canonical, Open Graph, and Twitter metadata.
- Modify `dashboard/tests/seo-public-pages-source.test.mjs`
  - Owns the static homepage SEO regression contract.
- Modify `.github/workflows/ci.yml`
  - Makes `npm run test:seo` a blocking check on pull requests and integration branches.
- Modify `scripts/preview/release-guardrails.test.mjs`
  - Protects CI and Preview Acceptance wiring from being silently removed.
- Create `dashboard/tests/regression/seo-preview.spec.ts`
  - Verifies rendered homepage metadata and every deployed sitemap entry on the exact Vercel Preview.
- Modify `dashboard/playwright.preview.config.ts`
  - Includes both preview-environment identity acceptance and SEO acceptance.
- Modify `dashboard/playwright.regression.config.ts`
  - Keeps preview-only tests out of ordinary local/dashboard smoke regression.

### Task 1: Restore and statically protect homepage search intent

**Files:**

- Modify: `dashboard/tests/seo-public-pages-source.test.mjs`
- Modify: `dashboard/src/app/marketing/page.tsx`

- [ ] **Step 1: Verify the owned workspace**

Run the ownership commands from `Owned workspace`.

Expected: path and branch checks pass; only the committed design/plan history is present.

- [ ] **Step 2: Replace the existing homepage metadata source test with the failing recovery contract**

In `dashboard/tests/seo-public-pages-source.test.mjs`, replace the current `homepage metadata matches the current production brand positioning` test with:

```js
it("homepage metadata protects the developer API search intent", () => {
  const source = read("src/app/marketing/page.tsx");

  assert.match(
    source,
    /const HOMEPAGE_TITLE = "UniPost \| Social Media Posting API for Developers"/,
  );
  assert.match(
    source,
    /UniPost gives developers one API to connect customer social accounts, upload media, schedule posts, and publish across major social platforms\./,
  );
  assert.doesNotMatch(source, /const HOMEPAGE_TITLE = "Unipost"/);
  assert.doesNotMatch(
    source,
    /const HOMEPAGE_TITLE = "Rewrite homepage title and meta description for query relevance"/,
  );
  assert.match(source, /canonical:\s*"https:\/\/unipost\.dev\/"/);
  assert.match(
    source,
    /openGraph:\s*{[\s\S]*title:\s*HOMEPAGE_TITLE,[\s\S]*description:\s*HOMEPAGE_DESCRIPTION,/,
  );
  assert.match(
    source,
    /twitter:\s*{[\s\S]*card:\s*"summary",[\s\S]*title:\s*HOMEPAGE_TITLE,[\s\S]*description:\s*HOMEPAGE_DESCRIPTION,/,
  );
  assert.match(source, /Post to every social platform with one API/);
});
```

- [ ] **Step 3: Run the focused SEO test and verify the recovery contract fails**

Run:

```bash
cd dashboard
npm run test:seo
```

Expected: FAIL in `homepage metadata protects the developer API search intent` because the source still contains `const HOMEPAGE_TITLE = "Unipost"` and has no `twitter` metadata.

- [ ] **Step 4: Implement the minimal homepage metadata recovery**

In `dashboard/src/app/marketing/page.tsx`, replace the title and description constants with:

```ts
const HOMEPAGE_TITLE = "UniPost | Social Media Posting API for Developers";
const HOMEPAGE_DESCRIPTION =
  "UniPost gives developers one API to connect customer social accounts, upload media, schedule posts, and publish across major social platforms.";
```

Extend the existing `metadata` object immediately after `openGraph`:

```ts
  twitter: {
    card: "summary",
    title: HOMEPAGE_TITLE,
    description: HOMEPAGE_DESCRIPTION,
  },
```

Do not change homepage JSX, CTA behavior, canonical URL, or add JSON-LD.

- [ ] **Step 5: Run the focused SEO test and verify it passes**

Run:

```bash
cd dashboard
npm run test:seo
```

Expected: all `test:seo` tests PASS.

- [ ] **Step 6: Inspect the focused diff**

Run:

```bash
git diff --check
git diff -- dashboard/src/app/marketing/page.tsx dashboard/tests/seo-public-pages-source.test.mjs
```

Expected: only the approved metadata fields and their regression test changed.

- [ ] **Step 7: Commit the homepage recovery**

Run the ownership commands, then:

```bash
git add dashboard/src/app/marketing/page.tsx dashboard/tests/seo-public-pages-source.test.mjs
git commit -m "fix: restore homepage API search intent"
```

Expected: one focused commit containing exactly the two listed files.

### Task 2: Make the SEO regression a blocking CI check

**Files:**

- Modify: `scripts/preview/release-guardrails.test.mjs`
- Modify: `.github/workflows/ci.yml`

- [ ] **Step 1: Verify the owned workspace**

Run the ownership commands from `Owned workspace`.

- [ ] **Step 2: Add a failing release-guardrail assertion for required SEO CI**

Add this test to `scripts/preview/release-guardrails.test.mjs`:

```js
test("CI makes the dashboard SEO regression blocking", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  assert.match(
    workflow,
    /Run dashboard SEO source regression[\s\S]*npm run test:seo/,
  );
});
```

- [ ] **Step 3: Run the release guardrails and verify failure**

Run from the repository worktree root:

```bash
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: FAIL in `CI makes the dashboard SEO regression blocking` because `.github/workflows/ci.yml` does not run `npm run test:seo`.

- [ ] **Step 4: Wire the SEO suite into the dashboard CI job**

In `.github/workflows/ci.yml`, add this step immediately after `Run dashboard source regression`:

```yaml
      - name: Run dashboard SEO source regression
        run: npm run test:seo
```

- [ ] **Step 5: Run the release guardrails and focused SEO test**

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
cd dashboard
npm run test:seo
```

Expected: both commands PASS.

- [ ] **Step 6: Commit the blocking CI wiring**

Run the ownership commands, then:

```bash
git add .github/workflows/ci.yml scripts/preview/release-guardrails.test.mjs
git commit -m "ci: require dashboard SEO regression"
```

Expected: one focused commit containing exactly the workflow and its contract test.

### Task 3: Add exact-preview homepage and sitemap acceptance

**Files:**

- Modify: `scripts/preview/release-guardrails.test.mjs`
- Create: `dashboard/tests/regression/seo-preview.spec.ts`
- Modify: `dashboard/playwright.preview.config.ts`
- Modify: `dashboard/playwright.regression.config.ts`

- [ ] **Step 1: Verify the owned workspace**

Run the ownership commands from `Owned workspace`.

- [ ] **Step 2: Add failing guardrails for the new preview-only SEO spec**

In the existing `Preview Acceptance is fail-closed and tied to the exact PR head` test in `scripts/preview/release-guardrails.test.mjs`, add:

```js
  assert.match(previewConfig, /seo-preview\.spec\.ts/);

  const seoPreviewTest = await read(
    "dashboard/tests/regression/seo-preview.spec.ts",
  );
  assert.match(seoPreviewTest, /\/sitemap\.xml/);
  assert.match(seoPreviewTest, /maxRedirects:\s*0/);
  assert.match(seoPreviewTest, /noindex/i);
  assert.match(seoPreviewTest, /UniPost \| Social Media Posting API for Developers/);
```

Replace the existing `ordinary dashboard regression excludes deployed preview-only acceptance` assertion with:

```js
test("ordinary dashboard regression excludes deployed preview-only acceptance", async () => {
  const config = await read("dashboard/playwright.regression.config.ts");
  assert.match(config, /testIgnore:\s*\[[\s\S]*preview-environment\.spec\.ts/);
  assert.match(config, /testIgnore:\s*\[[\s\S]*seo-preview\.spec\.ts/);
});
```

- [ ] **Step 3: Run the release guardrails and verify failure**

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
```

Expected: FAIL because `seo-preview.spec.ts` does not exist and the Playwright configs do not include/exclude it.

- [ ] **Step 4: Create the deployed SEO acceptance spec**

Create `dashboard/tests/regression/seo-preview.spec.ts` with:

```ts
import { expect, test, type APIRequestContext } from "@playwright/test";

const dashboardBaseURL = process.env.DASHBOARD_BASE_URL;
const automationBypassSecret =
  process.env.VERCEL_AUTOMATION_BYPASS_SECRET;

if (!dashboardBaseURL || !automationBypassSecret) {
  throw new Error(
    "DASHBOARD_BASE_URL and VERCEL_AUTOMATION_BYPASS_SECRET are required",
  );
}

const bypassHeaders = {
  "x-vercel-protection-bypass": automationBypassSecret,
  "x-vercel-set-bypass-cookie": "true",
};
const productionOrigin = "https://unipost.dev";
const expectedTitle = "UniPost | Social Media Posting API for Developers";
const expectedDescription =
  "UniPost gives developers one API to connect customer social accounts, upload media, schedule posts, and publish across major social platforms.";

function normalizePath(pathname: string): string {
  if (pathname === "/") {
    return pathname;
  }
  return pathname.replace(/\/+$/, "");
}

function canonicalFromHTML(html: string): string | null {
  return html.match(
    /<link[^>]+rel=["']canonical["'][^>]+href=["']([^"']+)["']/i,
  )?.[1] ??
    html.match(
      /<link[^>]+href=["']([^"']+)["'][^>]+rel=["']canonical["']/i,
    )?.[1] ??
    null;
}

function robotsFromHTML(html: string): string | null {
  return html.match(
    /<meta[^>]+name=["']robots["'][^>]+content=["']([^"']+)["']/i,
  )?.[1] ??
    html.match(
      /<meta[^>]+content=["']([^"']+)["'][^>]+name=["']robots["']/i,
    )?.[1] ??
    null;
}

async function fetchPreviewRoute(
  request: APIRequestContext,
  pathname: string,
) {
  return request.get(pathname, {
    headers: bypassHeaders,
    maxRedirects: 0,
  });
}

test("homepage renders the protected developer API metadata", async ({ page }) => {
  await page.setExtraHTTPHeaders(bypassHeaders);
  await page.goto("/", { waitUntil: "domcontentloaded" });

  await expect(page).toHaveTitle(expectedTitle);
  await expect(page.locator('meta[name="description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
  await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
    "href",
    `${productionOrigin}/`,
  );
  await expect(page.locator('meta[property="og:title"]')).toHaveAttribute(
    "content",
    expectedTitle,
  );
  await expect(page.locator('meta[property="og:description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
  await expect(page.locator('meta[name="twitter:card"]')).toHaveAttribute(
    "content",
    "summary",
  );
  await expect(page.locator('meta[name="twitter:title"]')).toHaveAttribute(
    "content",
    expectedTitle,
  );
  await expect(page.locator('meta[name="twitter:description"]')).toHaveAttribute(
    "content",
    expectedDescription,
  );
});

test("every deployed sitemap entry is directly indexable", async ({ request }) => {
  const sitemapResponse = await fetchPreviewRoute(request, "/sitemap.xml");
  expect(sitemapResponse.status()).toBe(200);
  expect(sitemapResponse.headers()["content-type"]).toContain("xml");

  const xml = await sitemapResponse.text();
  const sitemapURLs = [...xml.matchAll(/<loc>([^<]+)<\/loc>/g)].map(
    (match) => match[1],
  );
  expect(sitemapURLs.length).toBeGreaterThan(0);
  expect(new Set(sitemapURLs).size).toBe(sitemapURLs.length);

  for (let index = 0; index < sitemapURLs.length; index += 8) {
    const batch = sitemapURLs.slice(index, index + 8);
    const results = await Promise.all(
      batch.map(async (sitemapURL) => {
        const productionURL = new URL(sitemapURL);
        expect(productionURL.origin, sitemapURL).toBe(productionOrigin);

        const response = await fetchPreviewRoute(
          request,
          `${productionURL.pathname}${productionURL.search}`,
        );
        const html = await response.text();
        return {
          sitemapURL,
          productionURL,
          status: response.status(),
          html,
        };
      }),
    );

    for (const result of results) {
      expect(result.status, result.sitemapURL).toBe(200);
      expect(
        robotsFromHTML(result.html)?.toLowerCase() ?? "",
        result.sitemapURL,
      ).not.toContain("noindex");

      const canonical = canonicalFromHTML(result.html);
      if (canonical) {
        const canonicalURL = new URL(canonical);
        expect(canonicalURL.origin, result.sitemapURL).toBe(productionOrigin);
        expect(
          normalizePath(canonicalURL.pathname),
          result.sitemapURL,
        ).toBe(normalizePath(result.productionURL.pathname));
      }
    }
  }
});
```

- [ ] **Step 5: Update the Playwright test collection boundaries**

In `dashboard/playwright.preview.config.ts`, replace:

```ts
  testMatch: "preview-environment.spec.ts",
```

with:

```ts
  testMatch: [
    "preview-environment.spec.ts",
    "seo-preview.spec.ts",
  ],
```

In `dashboard/playwright.regression.config.ts`, replace:

```ts
  testIgnore: "preview-environment.spec.ts",
```

with:

```ts
  testIgnore: [
    "preview-environment.spec.ts",
    "seo-preview.spec.ts",
  ],
```

- [ ] **Step 6: Run static guardrails and verify Playwright collection**

Run:

```bash
node --test scripts/preview/release-guardrails.test.mjs
cd dashboard
DASHBOARD_BASE_URL=https://preview.example.vercel.app \
EXPECTED_PREVIEW_API_URL=https://preview-api-example.up.railway.app \
EXPECTED_PREVIEW_SHA=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa \
VERCEL_AUTOMATION_BYPASS_SECRET=dummy \
npx playwright test --config=playwright.preview.config.ts --list
```

Expected:

- Release guardrails PASS.
- Playwright lists three tests: the existing preview environment pairing test plus the new homepage metadata and sitemap tests.
- No network request is made by `--list`.

- [ ] **Step 7: Commit the deployed SEO acceptance**

Run the ownership commands, then:

```bash
git add \
  scripts/preview/release-guardrails.test.mjs \
  dashboard/tests/regression/seo-preview.spec.ts \
  dashboard/playwright.preview.config.ts \
  dashboard/playwright.regression.config.ts
git commit -m "test: add deployed SEO preview acceptance"
```

Expected: one focused commit containing exactly the four listed files.

### Task 4: Complete local verification

**Files:**

- Verify only; no planned file changes.

- [ ] **Step 1: Verify the owned workspace and exact commit scope**

Run:

```bash
test "$(pwd)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-registration-seo-analysis"
test "$(git branch --show-current)" = "dev-registration-seo-analysis"
git status --short --branch
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: only the two design commits, the implementation-plan commit, and the three focused SEO implementation commits are unique. Changed files match the design and this plan.

- [ ] **Step 2: Run source and release-contract tests**

Run:

```bash
node --test scripts/preview/*.test.mjs
cd dashboard
npm run test:seo
```

Expected: every test PASS with zero skipped, cancelled, or missing results.

- [ ] **Step 3: Run the dashboard production build**

Run:

```bash
cd dashboard
npm run build
```

Expected: Next.js production build completes successfully.

- [ ] **Step 4: Run dashboard smoke regression**

First prove that the conversation-owned port is free, then start the already-built
dashboard from this worktree on that dedicated port:

```bash
cd dashboard
lsof -nP -iTCP:3100 -sTCP:LISTEN
npm run start -- --hostname 127.0.0.1 --port 3100
```

Expected: the `lsof` command prints no listener before startup, and the Next.js
server process has this worktree as its current working directory.

In a second shell rooted at the same owned worktree, run:

```bash
cd dashboard
CI=1 \
DASHBOARD_BASE_URL=http://localhost:3100 \
DASHBOARD_WEB_SERVER=0 \
npm run test:regression:dashboard
```

Expected: all enabled Playwright dashboard regression tests PASS against port
3100. The preview-only specs are not collected. Stop the owned server after the
suite completes. Never use `reuseExistingServer`, port 3000, or a server whose
process belongs to another worktree.

- [ ] **Step 5: Perform the final local diff audit**

Run:

```bash
git diff --check origin/dev...HEAD
git status --short --branch
```

Expected: no whitespace errors and a clean worktree.

Any failure, timeout, skip, cancellation, missing browser, or inability to start is a failed gate. Stop, preserve the evidence, fix the cause on this owned branch, and rerun the complete required suite.

### Task 5: Draft PR and exact-SHA Preview Acceptance

**Files:**

- No new planned source files.

- [ ] **Step 1: Re-fetch and audit task-branch uniqueness**

Run the ownership commands, then:

```bash
git fetch origin
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: no unrelated, unidentified, unfinished, or unaccepted changes.

- [ ] **Step 2: Push only the owned task branch**

Run:

```bash
git push -u origin dev-registration-seo-analysis
```

Expected: `origin/dev-registration-seo-analysis` points to the exact locally verified head SHA. Do not push `dev`, `staging`, or `main`.

- [ ] **Step 3: Open a Draft pull request to `dev`**

Create a Draft PR with:

- Base: `dev`
- Head: `dev-registration-seo-analysis`
- Title: `fix: recover and protect UniPost homepage SEO intent`
- Body: summarize the metadata recovery, blocking SEO CI, deployed sitemap contract, local test results, rollback boundary, and that CiteLoop remains stopped.

Expected: Draft PR exists and triggers GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and Preview Acceptance for the exact head SHA.

- [ ] **Step 4: Monitor every triggered gate to terminal success**

Record:

- PR head SHA.
- GitHub workflow run URLs.
- Railway PR environment and deployment.
- Vercel Preview deployment URL.
- Preview regression artifacts.

Expected: every required and visibly triggered result is `success` on the exact PR head SHA. Any failure, error, timeout, cancellation, skip, inability to start, missing result, or SHA mismatch is a hard stop.

- [ ] **Step 5: Perform Codex browser acceptance on the exact Preview**

On the Vercel Preview associated with the exact head SHA:

1. Open `/`.
2. Verify the visible homepage and signed-out `Start Building` CTA still work.
3. Inspect the rendered title, description, canonical, Open Graph, and Twitter metadata.
4. Open `/sitemap.xml`.
5. Confirm the sitemap is XML and representative homepage, docs, platform, tool, commercial, resource, and blog entries open without redirect or error on the Preview.
6. Confirm no authenticated app, posting, billing, or account-connection behavior was changed by the diff.

Expected: browser acceptance passes with no 5xx responses or console errors caused by this change.

- [ ] **Step 6: Re-audit and merge to `dev` only after every gate passes**

Immediately before merge, list:

```bash
git fetch origin
git log --oneline origin/dev..origin/dev-registration-seo-analysis
git diff --name-status origin/dev...origin/dev-registration-seo-analysis
```

Expected: the exact accepted commits and files only. Mark the PR ready and merge it to `dev` only if all Preview gates and the final audit pass.

- [ ] **Step 7: Monitor official development deployment and self-accept**

Wait for every deployment triggered by the merge to `dev`, then verify:

- `https://dev.unipost.dev/`
- `https://dev.unipost.dev/sitemap.xml`

Repeat the homepage metadata, CTA, sitemap, and representative-URL checks on the official development domain.

Expected: the real development environment matches the accepted Preview and all triggered checks/deployments succeed.

## Stop boundary after development

This plan does not promote to `staging` or `main` and does not submit Search Console. After development acceptance, report the exact commit/PR/deployment evidence and request explicit release authorization.

Production Search Console submission happens only after:

1. An explicitly authorized standard release reaches production.
2. Production homepage metadata and sitemap pass the same deployed checks.
3. The user is informed immediately before the external Search Console submission.
