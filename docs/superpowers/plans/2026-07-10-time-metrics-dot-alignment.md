# Time Metrics Dot Alignment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align every Time Metrics timeline dot with its phase-title line while preserving timestamps and right-side Duration placement.

**Architecture:** Keep the shared Time Metrics markup unchanged and adjust only the shared CSS in `post-platform-results.tsx`. Replace the timeline-wide rule with per-event connector segments so the line follows the newly top-aligned dots in both Calendar and List View.

**Tech Stack:** Next.js 16, React 19, TypeScript, styled-jsx global CSS, Node test runner, Playwright.

---

## File Structure

- Modify `dashboard/tests/post-time-metrics-source.test.mjs`: lock the title-aligned dot and per-event connector CSS contract.
- Modify `dashboard/src/components/posts/details/post-platform-results.tsx`: update the shared Time Metrics timeline CSS used by Calendar and List View.

### Task 1: Align dots and connectors to phase titles

**Files:**
- Modify: `dashboard/tests/post-time-metrics-source.test.mjs`
- Modify: `dashboard/src/components/posts/details/post-platform-results.tsx`

- [ ] **Step 1: Write the failing CSS source test**

Add this test to `dashboard/tests/post-time-metrics-source.test.mjs`:

```js
test("Time Metrics dots align with phase titles and connect dot-to-dot", () => {
  const shared = readFileSync(sharedPath, "utf8");

  assert.match(shared, /\.posts-time-metrics-dot\{[^}]*align-self:start[^}]*margin-top:3px/);
  assert.match(shared, /\.posts-time-metrics-event:not\(:last-child\)::before\{[^}]*top:7px[^}]*bottom:-7px/);
  assert.doesNotMatch(shared, /\.posts-time-metrics-timeline::before/);
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run:

```bash
cd dashboard
node --test --test-name-pattern="dots align" tests/post-time-metrics-source.test.mjs
```

Expected: FAIL because the dot still inherits row centering, the connector is timeline-wide, and the new per-event rule is absent.

- [ ] **Step 3: Implement the minimal shared CSS change**

In `PLATFORM_RESULTS_CSS`, replace the current timeline/dot rules with:

```css
.posts-time-metrics-timeline{position:relative;display:flex;flex-direction:column}
.posts-time-metrics-event{position:relative;display:grid;grid-template-columns:10px minmax(0,1fr) max-content;gap:9px;align-items:center;min-height:38px}
.posts-time-metrics-event:not(:last-child)::before{content:"";position:absolute;z-index:0;top:7px;bottom:-7px;left:4px;width:1px;background:var(--dborder)}
.posts-time-metrics-dot{position:relative;z-index:1;align-self:start;width:7px;height:7px;margin-top:3px;border:2px solid var(--surface1);border-radius:999px;background:var(--dmuted2);box-shadow:0 0 0 1px var(--dmuted2)}
```

Do not change `.posts-time-metrics-gap` or the responsive grid rule.

- [ ] **Step 4: Run the focused test and verify GREEN**

Run:

```bash
cd dashboard
node --test tests/post-time-metrics-source.test.mjs tests/post-time-metrics.test.mts
```

Expected: all Time Metrics source and duration tests pass.

- [ ] **Step 5: Run dashboard validation**

Run:

```bash
cd dashboard
npm run lint -- src/components/posts/details/post-platform-results.tsx
npm run build
npm run test:regression:dashboard
```

Expected: lint has no new warnings, build succeeds, and dashboard regression reports 55 passed with only the credential-gated smoke test skipped.

- [ ] **Step 6: Commit**

```bash
git add dashboard/tests/post-time-metrics-source.test.mjs dashboard/src/components/posts/details/post-platform-results.tsx
git commit -m "fix: align time metrics dots with titles"
```

### Task 2: Integrate and verify development deployment

**Files:**
- No additional source files.

- [ ] **Step 1: Merge the task branch into updated local `dev`**

Fetch `origin`, confirm `origin/dev` is an ancestor or merge it into the task branch if needed, then merge `dev-time-metrics-dot-alignment` into local `dev` without overwriting unrelated worktree changes.

- [ ] **Step 2: Re-run validation on local `dev`**

Run the focused Time Metrics tests, `npm run build`, and `npm run test:regression:dashboard` from the local `dev` integration worktree.

Expected: the focused tests and build pass; dashboard regression reports 55 passed and one credential-gated skip.

- [ ] **Step 3: Push and monitor `origin/dev`**

Push local `dev` to `origin/dev`, wait for GitHub CI, Vercel development deployment, and Railway development deployment to complete successfully.

- [ ] **Step 4: Verify the real development UI**

Open a multi-platform published post in `https://dev-app.unipost.dev`, expand Time Metrics, and verify:

- each dot center matches its phase-title line;
- connector segments run between adjacent dots and stop at Published;
- timestamp and Duration positions are unchanged;
- Calendar and List View render the same alignment;
- the browser console has no new errors.

- [ ] **Step 5: Clean up the short-lived branch/worktree**

After deployment verification passes, remove the owned temporary worktrees, prune worktree metadata, and delete `dev-time-metrics-dot-alignment` after confirming it is merged into `origin/dev`.
