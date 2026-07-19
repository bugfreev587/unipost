# Public Canonical Coverage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add exact self-referencing production canonical metadata to the 19 audited public routes and carry the fix through staging, production, and development acceptance.

**Architecture:** Centralize the repeated metadata for eight platform landing pages in one typed builder. Add explicit route metadata to the six docs pages and extend the five existing legal/tools metadata objects. Protect the contract with source regression tests and deployed DOM acceptance.

**Tech Stack:** Next.js 16 metadata API, TypeScript, Node.js test runner, Playwright, GitHub Actions, Vercel, Railway.

---

### Task 1: Platform landing-page canonicals

**Files:**
- Create: `dashboard/src/app/(platforms)/_config/metadata.ts`
- Modify: `dashboard/src/app/(platforms)/bluesky-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/instagram-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/linkedin-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/pinterest-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/threads-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/tiktok-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/twitter-api/page.tsx`
- Modify: `dashboard/src/app/(platforms)/youtube-api/page.tsx`
- Test: `dashboard/tests/seo-public-pages-source.test.mjs`

- [ ] **Step 1: Write the failing platform metadata test**

Append this test block to `dashboard/tests/seo-public-pages-source.test.mjs`:

```js
describe("audited public routes expose self-referencing canonicals", () => {
  const platformRoutes = [
    ["src/app/(platforms)/bluesky-api/page.tsx", "bluesky"],
    ["src/app/(platforms)/instagram-api/page.tsx", "instagram"],
    ["src/app/(platforms)/linkedin-api/page.tsx", "linkedin"],
    ["src/app/(platforms)/pinterest-api/page.tsx", "pinterest"],
    ["src/app/(platforms)/threads-api/page.tsx", "threads"],
    ["src/app/(platforms)/tiktok-api/page.tsx", "tiktok"],
    ["src/app/(platforms)/twitter-api/page.tsx", "twitter"],
    ["src/app/(platforms)/youtube-api/page.tsx", "youtube"],
  ];

  it("builds every platform page metadata through the canonical helper", () => {
    const helperPath = "src/app/(platforms)/_config/metadata.ts";
    assert.equal(existsSync(join(root, helperPath)), true);
    const helper = read(helperPath);
    assert.match(helper, /const canonical = `https:\/\/unipost\.dev\/\$\{platform\.slug\}-api`/);
    assert.match(helper, /alternates:\s*{\s*canonical\s*}/s);

    for (const [routePath, platformName] of platformRoutes) {
      const source = read(routePath);
      assert.match(source, new RegExp(`buildPlatformMetadata\\(${platformName}\\)`));
    }
  });
});
```

- [ ] **Step 2: Run the test and verify RED**

Run from `dashboard/`:

```bash
node --test tests/seo-public-pages-source.test.mjs
```

Expected: FAIL in `builds every platform page metadata through the canonical helper` because `metadata.ts` does not exist.

- [ ] **Step 3: Create the typed metadata builder**

Create `dashboard/src/app/(platforms)/_config/metadata.ts`:

```ts
import type { Metadata } from "next";
import type { PlatformConfig } from "./platforms";

export function buildPlatformMetadata(platform: PlatformConfig): Metadata {
  const canonical = `https://unipost.dev/${platform.slug}-api`;

  return {
    title: platform.seo.title,
    description: platform.seo.description,
    keywords: platform.seo.keywords,
    alternates: { canonical },
    openGraph: {
      title: `${platform.name} API for Developers | UniPost`,
      description: platform.seo.description,
      siteName: "UniPost",
      type: "website",
    },
  };
}
```

- [ ] **Step 4: Wire all eight platform routes to the builder**

In every listed platform route, remove `import type { Metadata } from "next"`, add the helper import, and replace the existing metadata object with the exact matching builder call:

```ts
// bluesky-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(bluesky);

// instagram-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(instagram);

// linkedin-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(linkedin);

// pinterest-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(pinterest);

// threads-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(threads);

// tiktok-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(tiktok);

// twitter-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(twitter);

// youtube-api/page.tsx
import { buildPlatformMetadata } from "../_config/metadata";
export const metadata = buildPlatformMetadata(youtube);
```

Keep each route component and JSON-LD block unchanged.

- [ ] **Step 5: Run the platform test and verify GREEN**

Run from `dashboard/`:

```bash
node --test tests/seo-public-pages-source.test.mjs
```

Expected: all SEO source tests pass.

- [ ] **Step 6: Commit the platform change**

```bash
git add dashboard/tests/seo-public-pages-source.test.mjs dashboard/src/app/\(platforms\)
git commit -m "fix(seo): add platform self-canonicals"
```

### Task 2: Docs canonicals

**Files:**
- Modify: `dashboard/src/app/docs/page.tsx`
- Modify: `dashboard/src/app/docs/api/inbox/list/page.tsx`
- Modify: `dashboard/src/app/docs/api/inbox/reply/page.tsx`
- Modify: `dashboard/src/app/docs/api/inbox/sync/page.tsx`
- Modify: `dashboard/src/app/docs/guides/x/comments/page.tsx`
- Modify: `dashboard/src/app/docs/guides/x/reconnect-permissions/page.tsx`
- Test: `dashboard/tests/seo-public-pages-source.test.mjs`

- [ ] **Step 1: Write the failing docs canonical test**

Add this test inside the existing `audited public routes expose self-referencing canonicals` describe block:

```js
it("declares exact self-canonicals for audited docs routes", () => {
  const routes = [
    ["src/app/docs/page.tsx", "https://unipost.dev/docs"],
    ["src/app/docs/api/inbox/list/page.tsx", "https://unipost.dev/docs/api/inbox/list"],
    ["src/app/docs/api/inbox/reply/page.tsx", "https://unipost.dev/docs/api/inbox/reply"],
    ["src/app/docs/api/inbox/sync/page.tsx", "https://unipost.dev/docs/api/inbox/sync"],
    ["src/app/docs/guides/x/comments/page.tsx", "https://unipost.dev/docs/guides/x/comments"],
    ["src/app/docs/guides/x/reconnect-permissions/page.tsx", "https://unipost.dev/docs/guides/x/reconnect-permissions"],
  ];

  for (const [routePath, canonical] of routes) {
    const source = read(routePath);
    assert.equal(source.includes(canonical), true);
    assert.match(source, /alternates:\s*{\s*canonical:/s);
  }
});
```

- [ ] **Step 2: Run the test and verify RED**

Run from `dashboard/`:

```bash
node --test tests/seo-public-pages-source.test.mjs
```

Expected: FAIL because `/docs` does not contain `https://unipost.dev/docs`.

- [ ] **Step 3: Add exact docs route metadata**

For each docs module, add `import type { Metadata } from "next";` unless already present, then add the matching metadata export after imports:

```ts
export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs" },
};

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/api/inbox/list" },
};

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/api/inbox/reply" },
};

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/api/inbox/sync" },
};

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/guides/x/comments" },
};

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs/guides/x/reconnect-permissions" },
};
```

Apply these blocks in the same order as the files listed under this task. Do not change page content or feature-flag logic.

- [ ] **Step 4: Run the docs test and verify GREEN**

Run from `dashboard/`:

```bash
node --test tests/seo-public-pages-source.test.mjs
```

Expected: all SEO source tests pass.

- [ ] **Step 5: Commit the docs change**

```bash
git add dashboard/tests/seo-public-pages-source.test.mjs dashboard/src/app/docs
git commit -m "fix(seo): add docs self-canonicals"
```

### Task 3: Legal and tools canonicals

**Files:**
- Modify: `dashboard/src/app/privacy/page.tsx`
- Modify: `dashboard/src/app/terms/page.tsx`
- Modify: `dashboard/src/app/tools/page.tsx`
- Modify: `dashboard/src/app/tools/agentpost/page.tsx`
- Modify: `dashboard/src/app/tools/character-counter/page.tsx`
- Test: `dashboard/tests/seo-public-pages-source.test.mjs`

- [ ] **Step 1: Write the failing legal/tools canonical test**

Add this test inside the existing `audited public routes expose self-referencing canonicals` describe block:

```js
it("declares exact self-canonicals for audited legal and tools routes", () => {
  const routes = [
    ["src/app/privacy/page.tsx", "https://unipost.dev/privacy"],
    ["src/app/terms/page.tsx", "https://unipost.dev/terms"],
    ["src/app/tools/page.tsx", "https://unipost.dev/tools"],
    ["src/app/tools/agentpost/page.tsx", "https://unipost.dev/tools/agentpost"],
    ["src/app/tools/character-counter/page.tsx", "https://unipost.dev/tools/character-counter"],
  ];

  for (const [routePath, canonical] of routes) {
    const source = read(routePath);
    assert.equal(source.includes(canonical), true);
    assert.match(source, /alternates:\s*{\s*canonical:/s);
  }
});
```

- [ ] **Step 2: Run the test and verify RED**

Run from `dashboard/`:

```bash
node --test tests/seo-public-pages-source.test.mjs
```

Expected: FAIL because `/privacy` does not contain `https://unipost.dev/privacy`.

- [ ] **Step 3: Extend the five existing metadata objects**

Add the matching field to each existing `metadata` object:

```ts
// privacy/page.tsx
alternates: { canonical: "https://unipost.dev/privacy" },

// terms/page.tsx
alternates: { canonical: "https://unipost.dev/terms" },

// tools/page.tsx
alternates: { canonical: "https://unipost.dev/tools" },

// tools/agentpost/page.tsx
alternates: { canonical: "https://unipost.dev/tools/agentpost" },

// tools/character-counter/page.tsx
alternates: { canonical: "https://unipost.dev/tools/character-counter" },
```

Do not change titles, descriptions, keywords, or components.

- [ ] **Step 4: Run the SEO test and verify GREEN**

Run from `dashboard/`:

```bash
npm run test:seo
```

Expected: all SEO and YouTube analytics source tests pass.

- [ ] **Step 5: Commit the legal/tools change**

```bash
git add dashboard/tests/seo-public-pages-source.test.mjs dashboard/src/app/privacy/page.tsx dashboard/src/app/terms/page.tsx dashboard/src/app/tools/page.tsx dashboard/src/app/tools/agentpost/page.tsx dashboard/src/app/tools/character-counter/page.tsx
git commit -m "fix(seo): complete public self-canonicals"
```

### Task 4: Local CI-equivalent verification

**Files:**
- Verify all files changed in Tasks 1–3

- [ ] **Step 1: Verify formatting and exact scope**

Run from repository root:

```bash
git diff --check origin/staging..HEAD
git diff --name-status origin/staging..HEAD
```

Expected: no whitespace errors; only the spec, plan, direct test, platform metadata helper, and 19 route modules are changed.

- [ ] **Step 2: Run SEO and docs source regressions**

Run from `dashboard/`:

```bash
npm run test:seo
npm run test:docs-ai
```

Expected: both commands pass.

- [ ] **Step 3: Build the dashboard**

Run from `dashboard/`:

```bash
npm run build
```

Expected: successful production build with all 19 public routes generated.

- [ ] **Step 4: Run dashboard Playwright regression**

Run from `dashboard/`:

```bash
npm run test:regression:dashboard
```

Expected: all required tests pass; only the previously approved authenticated local smoke may be skipped.

### Task 5: Staging, production, and development release

**Files:**
- No additional source files

- [ ] **Step 1: Push and open the hotfix PR to staging**

From repository root, after proving the owned path and branch:

```bash
git push --set-upstream origin hotfix-public-canonical-20260719
gh pr create --repo bugfreev587/unipost --base staging --head hotfix-public-canonical-20260719 --draft --title "fix(seo): add public self-canonicals" --body "Adds exact self-referencing production canonicals to the 19 audited public routes. No sitemap, content, UI, authentication, or CiteLoop changes."
```

Expected: a Draft PR targeting `staging`.

- [ ] **Step 2: Complete staging gates and acceptance**

Require all checks on the exact PR head: API, Dashboard, Railway PR environment, Vercel Preview, and Preview Acceptance. Mark ready only after all succeed, re-audit commits/files, merge to staging, and monitor the exact staging merge SHA through GitHub CI, Vercel, and Railway.

Verify each of the 19 staging URLs returns HTTP 200, has no `noindex`, and renders exactly one canonical equal to the corresponding `https://unipost.dev/...` URL. Recheck homepage SEO metadata, API health, and sitemap health.

- [ ] **Step 3: Promote staging to production**

Open or update the `staging` to `main` PR only after staging acceptance. Audit all staging-unique commits and files, require all exact-head checks, merge without bypassing protection, and monitor the exact `main` merge SHA through GitHub CI, Vercel production, and all Railway production services.

Verify the same 19 production URLs, homepage metadata, API health, sitemap entries, and critical landing-page behavior. Any failed, skipped, cancelled, timed-out, or mismatched-SHA gate is a hard stop.

- [ ] **Step 4: Back-sync the owned branch to dev**

Fetch the latest `origin/dev` and merge it into `hotfix-public-canonical-20260719`. Stop and ask the user if there is any conflict. If the merge is clean and produces a diff, rerun the required local checks, push, open a Draft PR to `dev`, complete full Preview Acceptance, merge, monitor development deployments, and verify the development domains. If `origin/dev` already contains an equivalent tree, record the zero-diff evidence and do not create an empty commit or PR.
