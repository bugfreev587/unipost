# Platform Options Examples Guide Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a task-focused Platform Options Examples guide for the five most error-prone destinations, link it from the Create Post request field, and carry the documentation through development, staging, and production verification.

**Architecture:** Add one static server-rendered guide using the existing docs components, then connect it to the Guides index, sidebar, Create Post reference, YouTube platform guide, and docs search index. A focused source regression test locks the request-shape rules, required examples, visibility semantics, navigation, and discovery links before implementation begins.

**Tech Stack:** Next.js 16 App Router, React 19 Server Components, existing `DocsPage` / `DocsCodeTabs` / `DocsTable` components, Node.js built-in test runner, Playwright regression tests, Vercel deployments.

---

## File Structure

- Create `dashboard/src/app/docs/guides/platform-options/page.tsx` for the complete task guide and copyable examples.
- Create `dashboard/tests/platform-options-guide-source.test.mjs` for the red-green source contract.
- Modify `dashboard/src/app/docs/guides/page.tsx` to make the guide discoverable from the Guides landing page.
- Modify `dashboard/src/app/docs/_components/docs-shell.tsx` to add the guide to Publishing Guides navigation.
- Modify `dashboard/src/app/docs/api/posts/create/content.tsx` to link the request field to the guide without embedding examples in the API reference.
- Modify `dashboard/src/app/docs/platforms/[platform]/_data.tsx` to state the actual API visibility default.
- Modify `dashboard/src/lib/docs-ai-search-index.ts` to make the guide discoverable through docs search.

### Task 1: Lock the documentation contract with a failing test

**Files:**
- Create: `dashboard/tests/platform-options-guide-source.test.mjs`
- Reference: `docs/superpowers/specs/2026-07-15-platform-options-guide-design.md`

- [ ] **Step 1: Write the failing source regression test**

Create the test with the real source files and assertions below:

```javascript
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();
const source = (path) => readFile(join(root, path), "utf8");

test("platform options guide provides safe copyable examples and is discoverable", async () => {
  const [guide, guidesIndex, docsShell, createReference, platformDocs, searchIndex] = await Promise.all([
    source("src/app/docs/guides/platform-options/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/posts/create/content.tsx"),
    source("src/app/docs/platforms/[platform]/_data.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);

  assert.match(guide, /title="Platform options examples"/);
  assert.match(guide, /platform_posts\[\].*flat/i);
  assert.match(guide, /Legacy account_ids/);
  assert.match(guide, /Invalid mixed shape/);
  assert.match(guide, /POST \/v1\/posts\/validate/);

  for (const platform of ["YouTube", "Instagram", "TikTok", "Facebook", "Pinterest"]) {
    assert.match(guide, new RegExp(`<h2 id="${platform.toLowerCase()}"[^>]*>${platform}</h2>`));
  }

  assert.match(guide, /API requests default to <code>private<\/code>/);
  assert.match(guide, /privacy_status/);
  assert.match(guide, /"privacy_status": "public"/);
  assert.match(guide, /"shorts": true/);
  assert.match(guide, /square or vertical/i);
  assert.match(guide, /three minutes/i);
  assert.match(guide, /does not resize, crop, or guarantee/i);
  assert.match(guide, /"mediaType": "story"/);
  assert.match(guide, /"privacy_level": "PUBLIC_TO_EVERYONE"/);
  assert.match(guide, /"brand_content_toggle": true/);
  assert.match(guide, /"mediaType": "reel"/);
  assert.match(guide, /"board_id": "1234567890"/);

  assert.match(guidesIndex, /href="\/docs\/guides\/platform-options"/);
  assert.match(docsShell, /label: "Platform options examples", href: "\/docs\/guides\/platform-options"/);
  assert.match(createReference, /href="\/docs\/guides\/platform-options"/);
  assert.match(createReference, /YouTube, Instagram, TikTok, Facebook, and Pinterest/);
  assert.match(platformDocs, /API requests default to `private`/);
  assert.match(searchIndex, /id: "guide-platform-options"/);
  assert.match(searchIndex, /path: "\/docs\/guides\/platform-options"/);
  assert.match(searchIndex, /YouTube Shorts visibility/);
});
```

- [ ] **Step 2: Run the focused test and verify the red state**

Run from `dashboard/`:

```bash
node --test tests/platform-options-guide-source.test.mjs
```

Expected: FAIL with `ENOENT` for `src/app/docs/guides/platform-options/page.tsx`. This proves the test fails because the requested guide does not exist yet.

- [ ] **Step 3: Commit the failing test**

```bash
git add dashboard/tests/platform-options-guide-source.test.mjs
git commit -m "test: define platform options guide contract"
```

### Task 2: Implement the Platform Options Examples guide

**Files:**
- Create: `dashboard/src/app/docs/guides/platform-options/page.tsx`
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/docs/api/posts/create/content.tsx`
- Modify: `dashboard/src/app/docs/platforms/[platform]/_data.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Test: `dashboard/tests/platform-options-guide-source.test.mjs`

- [ ] **Step 1: Create the guide route using existing server-rendered components**

The new page imports only existing project components:

```tsx
import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";
```

Define copyable JSON snippet arrays with these exact option bodies:

```json
{
  "title": "Launch update",
  "made_for_kids": false,
  "privacy_status": "public",
  "shorts": true
}
```

```json
{
  "mediaType": "story"
}
```

```json
{
  "privacy_level": "PUBLIC_TO_EVERYONE",
  "disable_comment": false,
  "disable_duet": true,
  "disable_stitch": true,
  "brand_organic_toggle": false,
  "brand_content_toggle": false
}
```

```json
{
  "mediaType": "reel"
}
```

```json
{
  "board_id": "1234567890",
  "title": "Launch inspiration",
  "link": "https://example.com/launch"
}
```

Render `DocsPage` with `eyebrow="Publishing Guides"`, `title="Platform options examples"`, `lead="Copy the recommended flat platform_posts[].platform_options shape for YouTube, Instagram, TikTok, Facebook, and Pinterest, then validate the exact payload before publishing."`, and `className="docs-page-wide"`. Inside it, render the request-shape callout and comparison first, the validate-first workflow second, then `<h2>` sections with ids `youtube`, `instagram`, `tiktok`, `facebook`, and `pinterest` in that order. End with the common-mistakes table and reference cards.

The rendered prose must include these exact behavioral statements:

- `YouTube API requests default to private when privacy_status is omitted.`
- `The Dashboard defaults to public, but API callers must send privacy_status: public when they expect public visibility.`
- `shorts: true adds the current Shorts hint; it does not resize, crop, or guarantee that YouTube classifies the upload as a Short.`
- `YouTube automatically classifies eligible square or vertical uploads of no more than three minutes as Shorts.`
- `Do not put a platform name inside platform_posts[].platform_options.`

The request-shape comparison must show these three complete shapes:

```json
{
  "platform_posts": [{
    "account_id": "sa_youtube_123",
    "media_urls": ["https://cdn.example.com/short.mp4"],
    "platform_options": {
      "title": "Launch update",
      "made_for_kids": false,
      "privacy_status": "public",
      "shorts": true
    }
  }]
}
```

```json
{
  "account_ids": ["sa_youtube_123"],
  "media_urls": ["https://cdn.example.com/short.mp4"],
  "platform_options": {
    "youtube": {
      "title": "Launch update",
      "made_for_kids": false,
      "privacy_status": "public",
      "shorts": true
    }
  }
}
```

```json
{
  "platform_posts": [{
    "account_id": "sa_youtube_123",
    "media_urls": ["https://cdn.example.com/short.mp4"],
    "platform_options": {
      "youtube": {
        "title": "Launch update"
      }
    }
  }]
}
```

- [ ] **Step 2: Add navigation and search discovery**

Add the guide card near Instagram Stories on `dashboard/src/app/docs/guides/page.tsx`:

```tsx
<Link href="/docs/guides/platform-options" className="docs-card" style={{ textDecoration: "none" }}>
  <div className="docs-card-title">Platform options examples</div>
  <p>Copy safe platform_posts[] options for YouTube, Instagram, TikTok, Facebook, and Pinterest.</p>
</Link>
```

Add the sidebar entry to Publishing Guides:

```tsx
{ label: "Platform options examples", href: "/docs/guides/platform-options" },
```

Add this search chunk after the publishing overview entry:

```tsx
chunk({
  id: "guide-platform-options",
  title: "Platform options examples",
  path: "/docs/guides/platform-options",
  section_id: "request-shape",
  primary_nav: "Guides",
  section_title: "Use flat platform options",
  product_area: "publishing",
  tags: [
    "platform_options",
    "platform_posts",
    "YouTube Shorts visibility",
    "Instagram Stories",
    "TikTok privacy",
    "Facebook Reels",
    "Pinterest board_id",
  ],
  intent_tags: ["posting"],
  endpoint_aliases: ["POST /v1/posts", "POST /v1/posts/validate", "/v1/posts", "/v1/posts/validate"],
  platforms: ["youtube", "instagram", "tiktok", "facebook", "pinterest"],
  content:
    "Use platform_posts[].platform_options as a flat object. Do not nest a platform name inside platform_posts. YouTube API requests default to private when privacy_status is omitted, so set privacy_status to public for a public video or Short. The guide includes Instagram mediaType, TikTok privacy and interaction controls, Facebook feed and Reel options, and Pinterest board_id examples.",
}),
```

- [ ] **Step 3: Link the Create Post field without embedding examples**

Import `Link` from `next/link` and replace the string description with concise JSX:

```tsx
description: <>
  Flat destination options for this platform post. Do not nest by platform name inside <code>platform_posts</code>;
  platform-scoped nesting belongs only to the legacy <code>account_ids</code> shape. See{" "}
  <Link href="/docs/guides/platform-options">common platform options examples</Link> for YouTube, Instagram,
  TikTok, Facebook, and Pinterest.
</>,
```

- [ ] **Step 4: Correct the existing YouTube visibility note**

Replace the YouTube field note with:

```tsx
["platform_options.youtube.privacy_status", "Optional", "private / public / unlisted", "API requests default to `private` when omitted. The Dashboard sends `public` by default. Unverified Google API projects may still force uploads to private."],
```

- [ ] **Step 5: Run the focused test and verify the green state**

Run from `dashboard/`:

```bash
node --test tests/platform-options-guide-source.test.mjs
```

Expected: PASS with one passing test and zero failures.

- [ ] **Step 6: Commit the guide implementation**

```bash
git add \
  dashboard/src/app/docs/guides/platform-options/page.tsx \
  dashboard/src/app/docs/guides/page.tsx \
  dashboard/src/app/docs/_components/docs-shell.tsx \
  dashboard/src/app/docs/api/posts/create/content.tsx \
  'dashboard/src/app/docs/platforms/[platform]/_data.tsx' \
  dashboard/src/lib/docs-ai-search-index.ts
git commit -m "docs: add platform options examples guide"
```

### Task 3: Validate the documentation surface on the task branch

**Files:**
- Verify: `dashboard/tests/platform-options-guide-source.test.mjs`
- Verify: `dashboard/src/app/docs/guides/platform-options/page.tsx`
- Verify: all modified documentation sources

- [ ] **Step 1: Run focused and adjacent source regression tests**

From `dashboard/`:

```bash
node --test \
  tests/platform-options-guide-source.test.mjs \
  tests/platform-docs-production-alignment-source.test.mjs \
  tests/video-audio-overlay-guide-source.test.mjs \
  tests/docs-analytics-guides-source.test.mjs
```

Expected: all listed tests pass with zero failures.

- [ ] **Step 2: Run the documentation search suite**

```bash
npm run test:docs-ai
```

Expected: all docs AI and documentation-source tests pass.

- [ ] **Step 3: Build the dashboard**

```bash
npm run build
```

Expected: Next.js production build exits 0 and includes `/docs/guides/platform-options`.

- [ ] **Step 4: Run dashboard Playwright regression when browsers are installed**

```bash
npm run test:regression:dashboard
```

Expected: all dashboard regression tests pass. If Playwright browsers are unavailable, record the exact installation error before promotion.

- [ ] **Step 5: Inspect the final task-branch diff**

```bash
git status --short
git diff --check origin/dev...HEAD
git diff --stat origin/dev...HEAD
```

Expected: only the approved spec, plan, focused test, guide page, navigation, API-reference link, YouTube note, and search-index changes appear.

### Task 4: Merge into dev and verify the real development environment

**Files:**
- Merge the complete `dev-youtube-shorts-docs` branch into local `dev`.

- [ ] **Step 1: Update local dev from origin and merge the task branch**

```bash
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-youtube-shorts-docs
```

Expected: merge succeeds without unrelated conflicts.

- [ ] **Step 2: Re-run required validation on local dev**

From `dashboard/`:

```bash
node --test tests/platform-options-guide-source.test.mjs
npm run test:docs-ai
npm run build
npm run test:regression:dashboard
```

Expected: all available checks pass.

- [ ] **Step 3: Push dev and monitor every triggered check and deployment**

```bash
git push origin dev
gh run list --branch dev --limit 20
```

Wait until GitHub Actions, Vercel `unipost-dev`, Railway `dev`, and every visibly triggered check reaches a terminal successful state.

- [ ] **Step 4: Verify the development docs in a browser**

Open these development URLs:

- `https://dev.unipost.dev/docs/guides/platform-options`
- `https://dev.unipost.dev/docs/api/posts/create`
- `https://dev.unipost.dev/docs/guides`

Verify the guide loads, the five platform sections and copyable code tabs render, the Create Post request field links to the guide, and the Guides card and sidebar route correctly.

### Task 5: Promote dev to staging and verify staging

**Files:**
- Promotion only; no new source changes unless a staging failure requires a fix on `dev`.

- [ ] **Step 1: Create the dev-to-staging promotion PR**

```bash
gh pr create --base staging --head dev --title "Release platform options examples guide to staging" --body "Promotes the Platform Options Examples guide, Create Post reference link, corrected YouTube visibility default, navigation, search indexing, and regression coverage."
```

- [ ] **Step 2: Monitor checks, merge, and monitor staging deployments**

Capture and monitor the exact PR, then merge only after checks pass:

```bash
staging_pr=$(gh pr view dev --json number --jq '.number')
gh pr checks "$staging_pr" --watch
gh pr merge "$staging_pr" --merge
```

Monitor GitHub Actions, Vercel `unipost-staging`, Railway `staging`, and every triggered deployment until successful.

- [ ] **Step 3: Verify staging in a browser**

Open:

- `https://staging.unipost.dev/docs/guides/platform-options`
- `https://staging.unipost.dev/docs/api/posts/create`
- `https://staging.unipost.dev/docs/guides`

Repeat the development acceptance checks against staging.

### Task 6: Promote staging to production and verify production

**Files:**
- Promotion only; no new source changes unless a production PR check requires a fix on `staging` and corresponding sync back to `dev`.

- [ ] **Step 1: Create the staging-to-main production PR**

```bash
gh pr create --base main --head staging --title "Release platform options examples guide" --body "Promotes the verified staging documentation changes for error-prone platform options examples and YouTube visibility guidance."
```

- [ ] **Step 2: Monitor checks, merge, and monitor production deployments**

Capture and monitor the exact PR, then merge only after checks pass:

```bash
production_pr=$(gh pr view staging --json number --jq '.number')
gh pr checks "$production_pr" --watch
gh pr merge "$production_pr" --merge
```

Monitor GitHub Actions, Vercel `unipost`, Railway `production`, and every triggered deployment until successful.

- [ ] **Step 3: Verify production health and the changed docs flow**

Open:

- `https://unipost.dev/docs/guides/platform-options`
- `https://unipost.dev/docs/api/posts/create`
- `https://unipost.dev/docs/guides`

Verify the page content, navigation, Create Post field link, copyable examples, and corrected YouTube API-default language in production. Confirm existing docs pages still load without console errors.
