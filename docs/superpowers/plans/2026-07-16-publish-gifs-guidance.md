# Publish GIFs Guidance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a task-oriented UniPost guide for sending hosted or local GIFs to X and Facebook, with an accurate nine-platform support matrix and links to every API Reference page used by the workflow.

**Architecture:** Add one server-rendered documentation route using the existing `DocsPage`, `DocsTable`, `DocsCodeTabs`, and `ApiInlineLink` components. Register the route in Guides navigation and Docs AI search, then extend the shared endpoint-to-guide map so the six workflow API Reference pages link back to the guide. Protect the content and cross-link requirements with one source-level Node test before implementation.

**Tech Stack:** Next.js 16 App Router, React 19 server components, existing UniPost docs components, Node.js built-in test runner.

---

## File Structure

- Create `dashboard/tests/publish-gifs-guide-source.test.mjs`: source-level acceptance test for page content, navigation, search indexing, examples, limits, and API backlinks.
- Create `dashboard/src/app/docs/guides/publish-gifs/page.tsx`: complete Guidance page and copyable cURL workflows.
- Modify `dashboard/src/app/docs/guides/page.tsx`: add the Guides index card and publishing-workflow discovery copy.
- Modify `dashboard/src/app/docs/_components/docs-shell.tsx`: add `Publish GIFs` to the Publishing Guides sidebar.
- Modify `dashboard/src/app/docs/api/_components/single-endpoint-page.tsx`: add related-guide backlinks for the six workflow endpoints.
- Modify `dashboard/src/lib/docs-ai-search-index.ts`: make “publish GIF,” supported platforms, local upload, and GIF-to-MP4 questions discover the guide.

### Task 1: Add the failing Guidance acceptance test

**Files:**
- Create: `dashboard/tests/publish-gifs-guide-source.test.mjs`

- [ ] **Step 1: Write the failing source test**

Create a Node test that reads the planned route plus the existing navigation and mapping files:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Publish GIFs guide covers support, workflows, navigation, and API backlinks", async () => {
  const [guide, guidesIndex, docsShell, endpointGuides, searchIndex] = await Promise.all([
    source("src/app/docs/guides/publish-gifs/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/_components/single-endpoint-page.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);

  assert.match(guide, /title="Publish GIFs to X and Facebook"/);
  for (const platform of ["X / Twitter", "Facebook Page", "LinkedIn", "Threads", "Instagram", "TikTok", "Pinterest", "YouTube", "Bluesky"]) {
    assert.match(guide, new RegExp(platform.replace("/", "\\/")));
  }
  assert.match(guide, /X \\/ Twitter"?, "Yes[^"]*direct GIF[^"]*", "Supported"/i);
  assert.match(guide, /Facebook Page"?, "Yes[^"]*GIF[^"]*", "Supported"/i);
  assert.match(guide, /LinkedIn"?, "Yes[^"]*LinkedIn image APIs[^"]*", "Coming soon"/i);
  assert.match(guide, /Threads"?, "Yes[^"]*provider-backed GIF attachments[^"]*", "Coming soon"/i);
  for (const platform of ["Instagram", "TikTok", "Pinterest", "YouTube", "Bluesky"]) {
    assert.match(guide, new RegExp(`${platform}[\\s\\S]{0,240}GIF[ -]to[ -]MP4[\\s\\S]{0,120}Coming soon`, "i"));
  }

  for (const endpoint of [
    "GET /v1/accounts",
    "POST /v1/media",
    "GET /v1/media/:media_id",
    "POST /v1/posts/validate",
    "POST /v1/posts",
    "GET /v1/posts/:post_id",
  ]) {
    assert.match(guide, new RegExp(endpoint.replace(/[/:]/g, "\\$&")));
  }

  assert.match(guide, /"content_type": "image\\/gif"/);
  assert.match(guide, /"account_id": "sa_twitter_123"/);
  assert.match(guide, /"account_id": "sa_facebook_123"/);
  assert.match(guide, /"platform_posts"/);
  assert.match(guide, /5 MB or smaller/i);
  assert.match(guide, /Facebook GIF immediately/i);
  assert.match(guide, /GIF-to-MP4 conversion option is coming soon/i);
  assert.doesNotMatch(guide, /GIF-to-MP4 conversion is available/i);

  assert.match(guidesIndex, /href="\\/docs\\/guides\\/publish-gifs"/);
  assert.match(docsShell, /label: "Publish GIFs", href: "\\/docs\\/guides\\/publish-gifs"/);
  assert.match(searchIndex, /id: "guide-publish-gifs"/);

  for (const pattern of [
    /method: "GET",[\s\S]{0,120}path: \/\^\\\/v1\\\/accounts\$\//,
    /method: "POST",[\s\S]{0,120}path: \/\^\\\/v1\\\/media\$\//,
    /method: "GET",[\s\S]{0,160}path: \/\^\\\/v1\\\/media\\\/:\[\^\\\/\]\+\$\//,
    /method: "POST",[\s\S]{0,160}path: \/\^\\\/v1\\\/posts\\\/validate\$\//,
    /method: "POST",[\s\S]{0,120}path: \/\^\\\/v1\\\/posts\$\//,
    /method: "GET",[\s\S]{0,160}path: \/\^\\\/v1\\\/posts\\\/:\[\^\\\/\]\+\$\//,
  ]) {
    const match = endpointGuides.match(pattern);
    assert.ok(match, `missing endpoint mapping ${pattern}`);
    const mappingWindow = endpointGuides.slice(match.index, match.index + 700);
    assert.match(mappingWindow, /label: "Publish GIFs", href: "\\/docs\\/guides\\/publish-gifs"/);
  }
});
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
```

Expected: FAIL because `src/app/docs/guides/publish-gifs/page.tsx` does not exist.

- [ ] **Step 3: Commit the failing test**

```bash
git add dashboard/tests/publish-gifs-guide-source.test.mjs
git commit -m "test: define Publish GIFs guide contract"
```

### Task 2: Implement the Guidance page

**Files:**
- Create: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`
- Test: `dashboard/tests/publish-gifs-guide-source.test.mjs`

- [ ] **Step 1: Create hosted and local cURL snippets**

Define:

- `HOSTED_GIF_SNIPPETS` with one `POST /v1/posts/validate` request and one `POST /v1/posts` request using the same `platform_posts[]` payload, `sa_twitter_123`, `sa_facebook_123`, and `https://cdn.example.com/launch.gif`.
- `LOCAL_GIF_SNIPPETS` with cURL commands for `GET /v1/accounts`, `POST /v1/media`, PUT to `upload_url`, polling `GET /v1/media/$MEDIA_ID`, validation, publishing, and polling `GET /v1/posts/$POST_ID`.
- Use `"content_type": "image/gif"` and one shared `media_id` in both destination entries.

- [ ] **Step 2: Build the page from existing docs components**

Create a server component with:

```tsx
import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

export default function PublishGifsGuidePage() {
  return (
    <DocsPage
      eyebrow="Publishing Guides"
      title="Publish GIFs to X and Facebook"
      lead="Publish a hosted or local GIF to X and Facebook, understand direct GIF support across UniPost destinations, and prepare for upcoming conversion workflows."
      className="docs-page-wide"
    >
      {/* support matrix, workflows, limits, roadmap, errors, references */}
    </DocsPage>
  );
}
```

The page must contain these sections in order:

1. platform support matrix;
2. current supported workflow;
3. hosted GIF example;
4. local GIF workflow;
5. X/Facebook limits;
6. coming-soon paths;
7. common errors;
8. API Reference cards and X/Facebook platform cards.

- [ ] **Step 3: Implement the exact support matrix**

Use `DocsTable` with columns:

```tsx
["Platform", "Official GIF support", "UniPost status", "Recommended action"]
```

Use the nine approved rows from the design spec. Keep LinkedIn and Threads described as officially supported upstream but coming soon in UniPost. For Instagram, TikTok, Pinterest, YouTube, and Bluesky, state that the UniPost GIF-to-MP4 conversion option is coming soon.

- [ ] **Step 4: Implement limits and common errors**

Document:

- X: exactly one GIF, 5 MB, no mixed media.
- Facebook: exactly one GIF, 10 MB, no other media, no Facebook link option, immediate publish only.
- Shared X + Facebook request: GIF must be 5 MB or smaller.
- `unsupported_format`, `file_too_large`, `media_not_uploaded`, `mixed_media_unsupported`, `facebook_scheduled_media_unsupported`, and asynchronous per-destination failure handling.

- [ ] **Step 5: Add all inline and card links**

Use `ApiInlineLink` for:

```tsx
<ApiInlineLink endpoint="GET /v1/accounts" />
<ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />
<ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" />
<ApiInlineLink endpoint="POST /v1/posts/validate" href="/docs/api/posts/validate" />
<ApiInlineLink endpoint="POST /v1/posts" href="/docs/api/posts/create" />
<ApiInlineLink endpoint="GET /v1/posts/:post_id" href="/docs/api/posts/get" />
```

End with `docs-next-card` links to the six API pages plus `/docs/platforms/twitter` and `/docs/platforms/facebook`.

- [ ] **Step 6: Run the targeted test**

Run:

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
```

Expected: the route-content assertions pass; navigation and backlink assertions still fail.

- [ ] **Step 7: Commit the page**

```bash
git add dashboard/src/app/docs/guides/publish-gifs/page.tsx
git commit -m "docs: add Publish GIFs guidance"
```

### Task 3: Add discovery, API backlinks, and Docs AI search

**Files:**
- Modify: `dashboard/src/app/docs/guides/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Modify: `dashboard/src/app/docs/api/_components/single-endpoint-page.tsx`
- Modify: `dashboard/src/lib/docs-ai-search-index.ts`
- Test: `dashboard/tests/publish-gifs-guide-source.test.mjs`

- [ ] **Step 1: Add Guides index discovery**

Add this card near the other publishing guides:

```tsx
<Link href="/docs/guides/publish-gifs" className="docs-card" style={{ textDecoration: "none" }}>
  <div className="docs-card-title">Publish GIFs</div>
  <p>Publish a hosted or local GIF to X and Facebook, compare platform support, and prepare for upcoming conversion workflows.</p>
</Link>
```

Update the publishing workflow prose to link “publish a GIF” to the new guide.

- [ ] **Step 2: Add the sidebar entry**

In the `Publishing Guides` items, add:

```ts
{ label: "Publish GIFs", href: "/docs/guides/publish-gifs" },
```

- [ ] **Step 3: Add all six API Reference backlinks**

In `ENDPOINT_GUIDE_LINKS`, add `{ label: "Publish GIFs", href: "/docs/guides/publish-gifs" }` while preserving existing guide links for:

- `GET /v1/accounts`
- `POST /v1/media`
- `GET /v1/media/:media_id`
- `POST /v1/posts/validate`
- `POST /v1/posts`
- `GET /v1/posts/:post_id`

- [ ] **Step 4: Add the Docs AI search chunk**

Add a `chunk` with:

```ts
chunk({
  id: "guide-publish-gifs",
  title: "Publish GIFs to X and Facebook",
  path: "/docs/guides/publish-gifs",
  section_id: "platform-support",
  primary_nav: "Guides",
  section_title: "Platform support",
  product_area: "publishing",
  tags: [
    "publish GIF",
    "GIF post",
    "animated GIF",
    "X GIF",
    "Twitter GIF",
    "Facebook GIF",
    "image/gif",
    "GIF to MP4",
    "local GIF upload",
  ],
  intent_tags: ["posting"],
  endpoint_aliases: [
    "GET /v1/accounts",
    "POST /v1/media",
    "GET /v1/media/{media_id}",
    "POST /v1/posts/validate",
    "POST /v1/posts",
    "GET /v1/posts/{post_id}",
  ],
  platforms: ["twitter", "facebook", "linkedin", "threads", "instagram", "tiktok", "pinterest", "youtube", "bluesky"],
  content:
    "UniPost directly supports GIF publishing to X and Facebook. Publish a hosted GIF with platform_posts[].media_urls, or reserve a local image/gif upload with POST /v1/media, PUT bytes to upload_url, poll GET /v1/media/{media_id}, then validate and publish with media_ids. A GIF sent to both X and Facebook must be 5 MB or smaller. LinkedIn and Threads native GIF integration is coming soon. For Instagram, TikTok, Pinterest, YouTube, and Bluesky, a UniPost GIF-to-MP4 conversion option is coming soon.",
}),
```

- [ ] **Step 5: Run the targeted test and verify GREEN**

Run:

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
```

Expected: PASS.

- [ ] **Step 6: Run related docs source tests**

Run:

```bash
cd dashboard
node --test \
  tests/publish-gifs-guide-source.test.mjs \
  tests/video-audio-overlay-guide-source.test.mjs \
  tests/platform-docs-production-alignment-source.test.mjs
```

Expected: all tests PASS.

- [ ] **Step 7: Commit discovery and backlinks**

```bash
git add \
  dashboard/src/app/docs/guides/page.tsx \
  dashboard/src/app/docs/_components/docs-shell.tsx \
  dashboard/src/app/docs/api/_components/single-endpoint-page.tsx \
  dashboard/src/lib/docs-ai-search-index.ts
git commit -m "docs: link Publish GIFs workflow"
```

### Task 4: Validate the task branch

**Files:**
- Verify all files changed in Tasks 1–3.

- [ ] **Step 1: Run formatting checks**

```bash
git diff --check origin/dev...HEAD
```

Expected: no output.

- [ ] **Step 2: Run the dashboard build**

```bash
cd dashboard
npm run build
```

Expected: successful Next.js production build including `/docs/guides/publish-gifs`.

- [ ] **Step 3: Run dashboard regression tests if browsers are installed**

```bash
cd dashboard
npm run test:regression:dashboard
```

Expected: PASS. If Playwright browsers are unavailable, record the exact missing-browser output and continue only in accordance with the repository validation rule.

- [ ] **Step 4: Review the task-branch diff**

```bash
git status --short
git diff --stat origin/dev...HEAD
git log --oneline origin/dev..HEAD
```

Expected: only the approved spec, implementation plan, source test, Guidance page, navigation, related-guide mapping, and Docs AI search changes are present.

### Task 5: Integrate into dev, push, and verify the deployed Guidance

**Files:**
- No new files; integrate the validated task branch.

- [ ] **Step 1: Fetch and update local dev**

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
```

Expected: local `dev` matches the latest `origin/dev`.

- [ ] **Step 2: Merge the task branch**

```bash
git merge --no-ff dev-publish-gif-guidance
```

Expected: clean merge. If unrelated local changes prevent switching or merging, stop without stashing or overwriting them.

- [ ] **Step 3: Re-run validation on local dev**

```bash
cd dashboard
node --test \
  tests/publish-gifs-guide-source.test.mjs \
  tests/video-audio-overlay-guide-source.test.mjs \
  tests/platform-docs-production-alignment-source.test.mjs
npm run build
```

Also run `npm run test:regression:dashboard` when browsers are installed.

Expected: all required checks PASS on local `dev`.

- [ ] **Step 4: Push dev**

```bash
git push origin dev
```

Expected: `origin/dev` advances successfully.

- [ ] **Step 5: Monitor all triggered checks and deployments**

Use GitHub and deployment status tooling to wait until GitHub Actions, Vercel `unipost-dev`, Railway `dev`, and any other triggered checks finish successfully.

- [ ] **Step 6: Verify the real development environment**

Open:

`https://dev.unipost.dev/docs/guides/publish-gifs`

Verify:

- the page loads without console errors;
- the nine-platform matrix appears at the top;
- X and Facebook show `Supported`;
- all other rows show `Coming soon` with the approved distinction;
- hosted and local cURL examples render;
- all API Reference and platform links navigate correctly;
- the Guides index and sidebar link to the page;
- the six API Reference pages show `Publish GIFs` as a related guide.

- [ ] **Step 7: Hand off for user review**

Report the live development URL and completed checks. Do not claim completion until deployment and live dev verification have passed.

### Task 6: Prepare the follow-up GIF-to-MP4 PRD

This task begins only after the Guidance implementation and dev verification are complete.

- [ ] **Step 1: Start a separate product-design cycle**

Use the brainstorming workflow to define the GIF-to-MP4 conversion product surface, API contract, lifecycle, destination behavior, limits, storage, failures, quotas, and validation.

- [ ] **Step 2: Write the PRD for user review**

Save the PRD in the repository's established PRD location and present it to the user for review. Do not implement conversion behavior as part of the Guidance branch.
