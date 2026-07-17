# Publish GIF Status Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Render the Publish GIFs guide's `UniPost status` enum values as compact Slack Docs-style semantic labels.

**Architecture:** Keep the change page-specific by adding a strict local status component to the Publish GIFs page. Add narrowly scoped theme-aware CSS classes to the shared docs shell stylesheet, without changing generic `DocsTable` behavior or other documentation pages.

**Tech Stack:** Next.js App Router, React Server Components, TypeScript, CSS embedded in the docs shell, Node.js source contract tests.

---

### Task 1: Define the status-label contract

**Files:**
- Modify: `dashboard/tests/publish-gifs-guide-source.test.mjs`
- Test: `dashboard/tests/publish-gifs-guide-source.test.mjs`

- [ ] **Step 1: Write the failing test**

Add source assertions that require a strict page-local status component, semantic variants, and usage in all nine status cells:

```js
assert.match(guide, /type PublishGifStatus = "Supported" \| "Coming soon"/);
assert.match(guide, /function PublishGifStatusTag/);
assert.match(guide, /className=\{`publish-gif-status-tag is-\$\{tone\}`\}/);
assert.equal((guide.match(/<PublishGifStatusTag status=/g) || []).length, 9);
assert.match(docsShell, /\.publish-gif-status-tag\{/);
assert.match(docsShell, /\.publish-gif-status-tag\.is-supported\{/);
assert.match(docsShell, /\.publish-gif-status-tag\.is-coming-soon\{/);
```

Update the four exact row assertions so they accept JSX status cells rather than raw string cells.

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
```

Expected: FAIL because `PublishGifStatusTag` and its CSS classes do not exist.

- [ ] **Step 3: Commit the failing contract**

```bash
git add dashboard/tests/publish-gifs-guide-source.test.mjs
git commit -m "test: define publish gif status tags"
```

### Task 2: Implement page-local enum labels

**Files:**
- Modify: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`
- Modify: `dashboard/src/app/docs/_components/docs-shell.tsx`
- Test: `dashboard/tests/publish-gifs-guide-source.test.mjs`

- [ ] **Step 1: Add the strict status component**

Add this component above the page component:

```tsx
type PublishGifStatus = "Supported" | "Coming soon";

function PublishGifStatusTag({ status }: { status: PublishGifStatus }) {
  const tone = status === "Supported" ? "supported" : "coming-soon";
  return <span className={`publish-gif-status-tag is-${tone}`}>{status}</span>;
}
```

- [ ] **Step 2: Replace all nine raw status strings**

Use React nodes in the third cell of every support-matrix row:

```tsx
<PublishGifStatusTag key="x-status" status="Supported" />
<PublishGifStatusTag key="facebook-status" status="Supported" />
<PublishGifStatusTag key="linkedin-status" status="Coming soon" />
```

Apply the same `Coming soon` component pattern to Threads, Instagram, TikTok, Pinterest, YouTube, and Bluesky, using a unique React key for every row.

- [ ] **Step 3: Add compact theme-aware styles**

Add these styles next to the existing docs table styles:

```css
.publish-gif-status-tag{display:inline-flex;align-items:center;justify-content:center;height:24px;padding:0 9px;border-radius:6px;font-size:11.5px;font-weight:700;line-height:1;white-space:nowrap}
.publish-gif-status-tag.is-supported{background:color-mix(in srgb,#10b981 13%,var(--docs-bg-elevated));color:color-mix(in srgb,#047857 92%,var(--docs-text))}
.publish-gif-status-tag.is-coming-soon{background:color-mix(in srgb,#f59e0b 15%,var(--docs-bg-elevated));color:color-mix(in srgb,#9a6500 92%,var(--docs-text))}
html.dark .publish-gif-status-tag.is-supported{background:rgba(16,185,129,.15);color:#6ee7b7}
html.dark .publish-gif-status-tag.is-coming-soon{background:rgba(245,158,11,.16);color:#fcd34d}
```

- [ ] **Step 4: Run the focused test**

Run:

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
```

Expected: PASS.

- [ ] **Step 5: Run the dashboard build**

Run:

```bash
cd dashboard
npm run build
```

Expected: successful Next.js production build including `/docs/guides/publish-gifs`.

- [ ] **Step 6: Commit the implementation**

```bash
git add dashboard/src/app/docs/guides/publish-gifs/page.tsx dashboard/src/app/docs/_components/docs-shell.tsx
git commit -m "docs: style publish gif status values"
```

### Task 3: Integrate and verify development

**Files:**
- No additional source files expected.

- [ ] **Step 1: Update local `dev` and merge the task branch**

```bash
git fetch origin
git switch dev
git pull --ff-only origin dev
git merge --no-ff dev-publish-gif-status-tags
```

- [ ] **Step 2: Re-run focused validation on local `dev`**

```bash
cd dashboard
node --test tests/publish-gifs-guide-source.test.mjs
npm run build
```

Expected: both commands pass.

- [ ] **Step 3: Push local `dev`**

```bash
git push origin dev
```

- [ ] **Step 4: Monitor triggered checks and deployments**

Wait until GitHub Actions, Vercel `unipost-dev`, and any other triggered checks are successful. Inspect and fix any failure before continuing.

- [ ] **Step 5: Verify the real development page**

Open:

```text
https://dev.unipost.dev/docs/guides/publish-gifs#platform-support
```

Verify at desktop and mobile widths in both light and dark themes:

- two green compact `Supported` labels;
- seven amber compact `Coming soon` labels;
- 6px rounded rectangles with no border or dot;
- no clipping, wrapping, or table regression;
- unchanged platform support copy and actions.

- [ ] **Step 6: Hand off for user acceptance**

Report the development URL and the completed automated/deployment/browser checks. Do not promote to staging or production.
