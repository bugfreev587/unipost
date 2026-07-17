# Documentation Enum Status Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render reader-facing enum values in documentation tables as compact, semantic B-style tags, then release and verify the change in development, staging, and production.

**Architecture:** Keep enum recognition in a small pure TypeScript resolver keyed by normalized column header and exact normalized value. `DocsTable` supplies the column header to its cell renderer and uses one exported `DocsEnumTag` component for recognized values; page-local tags are removed, while machine enums and prose continue through the existing rich-content path.

**Tech Stack:** Next.js 16, React 19, TypeScript, component-scoped CSS in `docs-shell.tsx`, Node test runner, Playwright, GitHub Actions, Vercel.

---

## File Map

- Create `dashboard/src/app/docs/_components/docs-table-enum.ts`: pure header/value-to-tone resolver and shared tone type.
- Create `dashboard/tests/docs-enum-status-tags.test.mts`: resolver behavior and source-integration regression coverage.
- Modify `dashboard/src/app/docs/_components/docs-shell.tsx`: shared enum tag component, column-aware table rendering, B-style light/dark CSS.
- Modify `dashboard/src/app/docs/guides/publish-gifs/page.tsx`: replace the local component instances with plain enum strings.
- Modify `dashboard/src/app/docs/platforms/page.tsx`: render the dense matrix `Limited` state through the shared tag.
- Modify `dashboard/tests/publish-gifs-guide-source.test.mjs`: assert shared rendering instead of page-local markup.

### Task 1: Add the Pure Enum Resolver

**Files:**
- Create: `dashboard/src/app/docs/_components/docs-table-enum.ts`
- Create: `dashboard/tests/docs-enum-status-tags.test.mts`

- [ ] **Step 1: Write the failing resolver test**

Create `dashboard/tests/docs-enum-status-tags.test.mts`:

```ts
import assert from "node:assert/strict";
import test from "node:test";
import {
  resolveDocsTableEnumTone,
  type DocsEnumTone,
} from "../src/app/docs/_components/docs-table-enum.ts";

const recognized: Array<[string, string, DocsEnumTone]> = [
  ["Support", "Yes", "success"],
  ["Support", "No", "danger"],
  ["Support", "Partial", "warning"],
  ["Support", "Limited", "warning"],
  ["Available", "Yes", "success"],
  ["Available", "No", "danger"],
  ["Required", "Yes", "success"],
  ["Required", "No", "danger"],
  ["Required", "Required", "info"],
  ["Required", "Optional", "neutral"],
  ["Required", "Rejected", "danger"],
  ["Severity", "Critical", "danger"],
  ["Severity", "High", "caution"],
  ["Severity", "Medium", "warning"],
  ["Default on", "Yes", "success"],
  ["Default on", "No", "danger"],
  ["Use this page?", "Yes", "success"],
  ["Use this page?", "No", "danger"],
  ["Use this page?", "Partially", "warning"],
  ["UniPost status", "Supported", "success"],
  ["UniPost status", "Coming soon", "warning"],
];

test("resolves approved reader-facing table enums", () => {
  for (const [column, value, expected] of recognized) {
    assert.equal(resolveDocsTableEnumTone(column, value), expected, `${column}: ${value}`);
  }
});

test("does not tag prose, machine enums, or descriptive values", () => {
  assert.equal(resolveDocsTableEnumTone("Notes", "Supported"), null);
  assert.equal(resolveDocsTableEnumTone("Meaning", "High"), null);
  assert.equal(resolveDocsTableEnumTone("data.status", "passed"), null);
  assert.equal(resolveDocsTableEnumTone("safety", "read_only"), null);
  assert.equal(resolveDocsTableEnumTone("Required", "Exactly 1 video"), null);
});

test("normalizes harmless whitespace and case", () => {
  assert.equal(resolveDocsTableEnumTone("  unipost STATUS ", " coming SOON "), "warning");
});
```

- [ ] **Step 2: Run the test and verify the missing-module failure**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts
```

Expected: FAIL because `docs-table-enum.ts` does not exist.

- [ ] **Step 3: Implement the minimal resolver**

Create `dashboard/src/app/docs/_components/docs-table-enum.ts`:

```ts
export type DocsEnumTone =
  | "success"
  | "warning"
  | "danger"
  | "info"
  | "neutral"
  | "caution";

const ENUM_TONES: Readonly<Record<string, Readonly<Record<string, DocsEnumTone>>>> = {
  support: {
    yes: "success",
    no: "danger",
    partial: "warning",
    limited: "warning",
  },
  available: {
    yes: "success",
    no: "danger",
  },
  required: {
    yes: "success",
    no: "danger",
    required: "info",
    optional: "neutral",
    rejected: "danger",
  },
  severity: {
    critical: "danger",
    high: "caution",
    medium: "warning",
  },
  "default on": {
    yes: "success",
    no: "danger",
  },
  "use this page?": {
    yes: "success",
    no: "danger",
    partially: "warning",
  },
  "unipost status": {
    supported: "success",
    "coming soon": "warning",
  },
};

function normalize(value: string) {
  return value.trim().toLowerCase();
}

export function resolveDocsTableEnumTone(column: string, value: string): DocsEnumTone | null {
  return ENUM_TONES[normalize(column)]?.[normalize(value)] ?? null;
}
```

- [ ] **Step 4: Run the resolver test and verify it passes**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts
```

Expected: 3 tests pass.

- [ ] **Step 5: Commit the resolver**

```bash
git add dashboard/src/app/docs/_components/docs-table-enum.ts dashboard/tests/docs-enum-status-tags.test.mts
git commit -m "test(docs): define semantic table enum mapping"
```

### Task 2: Integrate the Shared Tag into DocsTable

**Files:**
- Modify: `dashboard/tests/docs-enum-status-tags.test.mts`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`

- [ ] **Step 1: Add failing source-integration assertions**

Append imports and a test to `dashboard/tests/docs-enum-status-tags.test.mts`:

```ts
import { readFile } from "node:fs/promises";
import { join } from "node:path";

const root = process.cwd();

test("DocsTable renders enums with the shared B-style tag", async () => {
  const docsShell = await readFile(join(root, "src/app/docs/_components/docs-shell.tsx"), "utf8");

  assert.match(docsShell, /export function DocsEnumTag/);
  assert.match(docsShell, /resolveDocsTableEnumTone\(column, cell\)/);
  assert.match(docsShell, /renderDocsTableCell\(cell, columns\[cellIndex\]\)/);
  assert.match(docsShell, /\.docs-enum-tag\{/);
  for (const tone of ["success", "warning", "danger", "info", "neutral", "caution"]) {
    assert.match(docsShell, new RegExp(`\\.docs-enum-tag\\.is-${tone}\\{`));
  }
});
```

- [ ] **Step 2: Run the test and verify the integration assertions fail**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts
```

Expected: resolver tests pass and the new source-integration test fails because the shared component is absent.

- [ ] **Step 3: Add the component and column-aware renderer**

In `dashboard/src/app/docs/_components/docs-shell.tsx`, import the resolver:

```ts
import { resolveDocsTableEnumTone, type DocsEnumTone } from "./docs-table-enum";
```

Replace the global `Yes`/`No` icon branches with:

```tsx
export function DocsEnumTag({ value, tone }: { value: string; tone: DocsEnumTone }) {
  return <span className={`docs-enum-tag is-${tone}`}>{value}</span>;
}

function renderDocsTableCell(cell: React.ReactNode, column: string) {
  if (typeof cell !== "string") {
    return cell;
  }

  const tone = resolveDocsTableEnumTone(column, cell);
  if (tone) {
    return <DocsEnumTag value={cell.trim()} tone={tone} />;
  }

  return renderDocsRichContent(cell);
}
```

Update the table body call:

```tsx
{renderDocsTableCell(cell, columns[cellIndex])}
```

- [ ] **Step 4: Add the approved B-style CSS**

Replace the page-specific Publish GIF tag rules in `docs-shell.tsx` with:

```css
.docs-enum-tag{display:inline-flex;align-items:center;justify-content:center;min-height:24px;padding:0 9px;border-radius:6px;font-size:11.5px;font-weight:700;line-height:1;white-space:nowrap}
.docs-enum-tag.is-success{background:color-mix(in srgb,#10b981 13%,var(--docs-bg-elevated));color:color-mix(in srgb,#047857 92%,var(--docs-text))}
.docs-enum-tag.is-warning{background:color-mix(in srgb,#f59e0b 15%,var(--docs-bg-elevated));color:color-mix(in srgb,#9a6500 92%,var(--docs-text))}
.docs-enum-tag.is-danger{background:color-mix(in srgb,#ef4444 13%,var(--docs-bg-elevated));color:color-mix(in srgb,#b91c1c 92%,var(--docs-text))}
.docs-enum-tag.is-info{background:color-mix(in srgb,#3b82f6 13%,var(--docs-bg-elevated));color:color-mix(in srgb,#1d4ed8 92%,var(--docs-text))}
.docs-enum-tag.is-neutral{background:color-mix(in srgb,var(--docs-text-muted) 12%,var(--docs-bg-elevated));color:var(--docs-text-soft)}
.docs-enum-tag.is-caution{background:color-mix(in srgb,#f97316 14%,var(--docs-bg-elevated));color:color-mix(in srgb,#c2410c 92%,var(--docs-text))}
html.dark .docs-enum-tag.is-success{background:rgba(16,185,129,.15);color:#6ee7b7}
html.dark .docs-enum-tag.is-warning{background:rgba(245,158,11,.16);color:#fcd34d}
html.dark .docs-enum-tag.is-danger{background:rgba(239,68,68,.16);color:#fca5a5}
html.dark .docs-enum-tag.is-info{background:rgba(59,130,246,.16);color:#93c5fd}
html.dark .docs-enum-tag.is-neutral{background:rgba(148,163,184,.14);color:#cbd5e1}
html.dark .docs-enum-tag.is-caution{background:rgba(249,115,22,.16);color:#fdba74}
```

- [ ] **Step 5: Run the focused test and verify it passes**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts
```

Expected: 4 tests pass.

- [ ] **Step 6: Commit the shared renderer**

```bash
git add dashboard/src/app/docs/_components/docs-shell.tsx dashboard/tests/docs-enum-status-tags.test.mts
git commit -m "feat(docs): render semantic table enum tags"
```

### Task 3: Migrate Publish GIFs and the Dense Platform Matrix

**Files:**
- Modify: `dashboard/tests/publish-gifs-guide-source.test.mjs`
- Modify: `dashboard/tests/docs-enum-status-tags.test.mts`
- Modify: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`
- Modify: `dashboard/src/app/docs/platforms/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`

- [ ] **Step 1: Change source tests to require shared tags**

In `publish-gifs-guide-source.test.mjs`, replace local-component assertions with:

```js
assert.doesNotMatch(guide, /PublishGifStatusTag|publish-gif-status-tag/);
assert.equal((guide.match(/"Supported"/g) || []).length, 2);
assert.equal((guide.match(/"Coming soon"/g) || []).length, 7);
assert.match(docsShell, /\.docs-enum-tag\.is-success\{/);
assert.match(docsShell, /\.docs-enum-tag\.is-warning\{/);
```

Change the four platform row assertions to expect plain status strings, for example:

```js
assert.match(
  guide,
  /"X \/ Twitter",\s*"Yes — direct GIF media upload",\s*"Supported"/,
);
assert.match(
  guide,
  /"LinkedIn",\s*"Yes — through LinkedIn image APIs",\s*"Coming soon"/,
);
```

Append to `docs-enum-status-tags.test.mts`:

```ts
test("the dense platform matrix shares the partial-support tag", async () => {
  const platforms = await readFile(join(root, "src/app/docs/platforms/page.tsx"), "utf8");
  assert.match(platforms, /<DocsEnumTag value="Limited" tone="warning" \/>/);
  assert.doesNotMatch(platforms, /docs-matrix-partial/);
});
```

- [ ] **Step 2: Run the focused tests and verify they fail**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts tests/publish-gifs-guide-source.test.mjs
```

Expected: FAIL because both pages still use their local markup.

- [ ] **Step 3: Migrate Publish GIFs to plain enum strings**

Delete `PublishGifStatus` and `PublishGifStatusTag` from `publish-gifs/page.tsx`.
Replace all nine component cells with exact strings:

```tsx
"Supported"
```

or:

```tsx
"Coming soon"
```

The `UniPost status` header makes the shared renderer apply the correct tone.

- [ ] **Step 4: Migrate the matrix partial state**

Import `DocsEnumTag` in `platforms/page.tsx`:

```tsx
import { DocsCode, DocsEnumTag, DocsPage, DocsTable } from "../_components/docs-shell";
```

Replace `partialCell` with:

```tsx
function partialCell() {
  return <DocsEnumTag value="Limited" tone="warning" />;
}
```

Remove `.docs-matrix-partial` and its `:has(...)` selector from `docs-shell.tsx`.

- [ ] **Step 5: Run focused tests and verify they pass**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts tests/publish-gifs-guide-source.test.mjs
```

Expected: all focused tests pass.

- [ ] **Step 6: Commit the page migrations**

```bash
git add dashboard/src/app/docs/_components/docs-shell.tsx dashboard/src/app/docs/guides/publish-gifs/page.tsx dashboard/src/app/docs/platforms/page.tsx dashboard/tests/docs-enum-status-tags.test.mts dashboard/tests/publish-gifs-guide-source.test.mjs
git commit -m "refactor(docs): adopt shared enum status tags"
```

### Task 4: Complete Local Verification

**Files:**
- Verify only; no new files expected.

- [ ] **Step 1: Run focused and related documentation tests**

Run:

```bash
cd dashboard
node --test tests/docs-enum-status-tags.test.mts tests/publish-gifs-guide-source.test.mjs tests/platform-docs-production-alignment-source.test.mjs
```

Expected: all tests pass.

- [ ] **Step 2: Run the Dashboard production build**

Run:

```bash
cd dashboard
npm run build
```

Expected: Next.js exits 0 with no type or build errors.

- [ ] **Step 3: Run Dashboard regression tests when browsers are installed**

Run:

```bash
cd dashboard
npm run test:regression:dashboard
```

Expected: Playwright exits 0. If browser binaries are unavailable, install the configured Chromium binary and rerun before release.

- [ ] **Step 4: Inspect the final diff**

Run:

```bash
git diff --check origin/dev...HEAD
git status --short --branch
git log --oneline origin/dev..HEAD
```

Expected: no whitespace errors; only the approved design, plan, resolver, shared renderer, page migrations, and tests are present.

### Task 5: Merge and Verify Development

**Files:**
- Git integration and deployed environment verification only.

- [ ] **Step 1: Update local `dev` and merge the task branch**

Use the existing clean integration worktree after checking its status:

```bash
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration status --short --branch
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration fetch origin
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration switch dev
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration pull --ff-only origin dev
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration merge --no-ff dev-docs-enum-status-tags -m "Merge dev-docs-enum-status-tags into dev"
```

Expected: the worktree is clean before switching and the merge completes without conflicts.

- [ ] **Step 2: Repeat required validation on merged local `dev`**

Run from the integration worktree's `dashboard/` directory:

```bash
node --test tests/docs-enum-status-tags.test.mts tests/publish-gifs-guide-source.test.mjs tests/platform-docs-production-alignment-source.test.mjs
npm run build
npm run test:regression:dashboard
```

Expected: all commands exit 0.

- [ ] **Step 3: Push `dev` and monitor every triggered check**

```bash
git -C /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-x-inbox-dev-integration push origin dev
gh run list --branch dev --limit 20
```

Expected: push succeeds and all GitHub Actions, Vercel, Railway, and other visible development checks finish successfully.

- [ ] **Step 4: Browser-verify the real development site**

Inspect light and dark themes on:

- `https://dev.unipost.dev/docs/guides/publish-gifs#platform-support`
- `https://dev.unipost.dev/docs/platforms/tiktok`
- `https://dev.unipost.dev/docs/resources/notifications`
- `https://dev.unipost.dev/docs/quickstart`
- `https://dev.unipost.dev/docs/platforms`

Expected: tag geometry and colors match the approved design; descriptive and machine enum values remain unchanged; no console errors or horizontal-layout regressions appear.

### Task 6: Promote and Verify Staging

**Files:**
- GitHub promotion and deployed environment verification only.

- [ ] **Step 1: Create the `dev` to `staging` promotion PR**

```bash
gh pr create --base staging --head dev --title "Release documentation enum status tags to staging" --body "Promotes the verified shared documentation enum tag renderer from development to staging."
```

Expected: GitHub returns the promotion PR URL.

- [ ] **Step 2: Monitor checks and merge the PR**

```bash
STAGING_PR=$(gh pr list --base staging --head dev --state open --json number --jq '.[0].number')
gh pr checks "$STAGING_PR" --watch
gh pr merge "$STAGING_PR" --merge
```

Expected: all required checks pass and the PR merges into `staging`.

- [ ] **Step 3: Monitor deployment and verify staging**

Wait for all triggered checks and deployments, then inspect the same representative paths under `https://staging.unipost.dev` in light and dark themes.

Expected: staging matches the accepted development behavior with no console or layout regressions.

### Task 7: Promote and Verify Production

**Files:**
- GitHub promotion and deployed environment verification only.

- [ ] **Step 1: Create the `staging` to `main` production PR**

```bash
gh pr create --base main --head staging --title "Release documentation enum status tags" --body "Promotes the staging-verified shared documentation enum tag renderer to production."
```

Expected: GitHub returns the production PR URL.

- [ ] **Step 2: Monitor checks and merge the PR**

```bash
PRODUCTION_PR=$(gh pr list --base main --head staging --state open --json number --jq '.[0].number')
gh pr checks "$PRODUCTION_PR" --watch
gh pr merge "$PRODUCTION_PR" --merge
```

Expected: all required checks pass and the PR merges into `main`.

- [ ] **Step 3: Monitor production deployment and verify production**

Wait for all triggered checks and deployments, then inspect the same representative paths under `https://unipost.dev` in light and dark themes.

Expected: production is healthy, the shared enum tags match staging, Publish GIFs still communicates all nine platform states, and machine enums remain code-styled.
