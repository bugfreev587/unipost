# Pricing Media Retention Non-Retroactivity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a low-emphasis note at the bottom of the public Pricing page explaining that media-retention deadlines do not change retroactively after a Plan change.

**Architecture:** Keep the change local to the existing Pricing client component. Add one source-contract test, one static paragraph after the FAQ grid, and one narrowly scoped CSS rule; do not introduce runtime state, shared components, dependencies, or backend changes.

**Tech Stack:** Next.js App Router, React, inline page-scoped CSS, Node.js built-in test runner, Playwright, Vercel Preview

---

## File Map

- Create `dashboard/tests/pricing-media-retention-policy-source.test.mjs`: protects the approved copy, placement, and low-emphasis styling.
- Modify `dashboard/src/app/pricing/pricing-page-client.tsx`: renders and styles the policy note.
- Do not modify the shared marketing footer, `/docs/pricing`, backend retention policy, or database.

### Task 1: Add the Pricing policy note with TDD

**Files:**
- Create: `dashboard/tests/pricing-media-retention-policy-source.test.mjs`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx:176`
- Modify: `dashboard/src/app/pricing/pricing-page-client.tsx:476-480`

- [ ] **Step 1: Write the failing source-contract test**

Create `dashboard/tests/pricing-media-retention-policy-source.test.mjs`:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing ends with a low-emphasis non-retroactive media retention note", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  const faqIndex = pricing.indexOf('className="pr-faq-grid"');
  const faqCloseIndex = pricing.indexOf("\n        </div>", faqIndex);
  const noteIndex = pricing.indexOf('className="pr-retention-policy-note"');

  assert.ok(faqIndex >= 0, "Pricing FAQ grid should exist");
  assert.ok(faqCloseIndex > faqIndex, "Pricing FAQ grid should have a closing tag");
  assert.ok(noteIndex > faqCloseIndex, "retention policy note should render after the FAQ grid");
  assert.match(
    pricing,
    /Media retention is based on the workspace plan in effect when the retention period begins\./,
  );
  assert.match(
    pricing,
    /Later plan upgrades or downgrades do not retroactively extend or shorten an existing retention period\./,
  );
  assert.match(
    pricing,
    /\.pr-retention-policy-note\{[^}]*border-top:1px solid var\(--pr-border\)[^}]*font-size:12px[^}]*color:var\(--pr-muted2\)/,
  );
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Before the test, verify the owned worktree and branch:

```bash
test "$(git rev-parse --show-toplevel)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit"
test "$(git branch --show-current)" = "dev-pricing-retention-nonretroactive"
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
node --test tests/pricing-media-retention-policy-source.test.mjs
```

Expected: FAIL because `pr-retention-policy-note` and the approved non-retroactivity copy are absent.

- [ ] **Step 3: Add the minimal low-emphasis style**

In the Pricing page `CSS` string, immediately after `.pr-faq-a`, add:

```css
.pr-retention-policy-note{border-top:1px solid var(--pr-border);padding-top:18px;margin:0;max-width:760px;font-size:12px;line-height:1.65;color:var(--pr-muted2)}
```

The rule intentionally has no background, card border, icon, heading, animation,
or centered alignment.

- [ ] **Step 4: Render the approved copy after the FAQ grid**

Immediately after the closing tag for `.pr-faq-grid`, add:

```tsx
<p className="pr-retention-policy-note">
  Media retention is based on the workspace plan in effect when the retention period begins.
  Later plan upgrades or downgrades do not retroactively extend or shorten an existing retention period.
</p>
```

- [ ] **Step 5: Run the focused test and verify GREEN**

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
node --test tests/pricing-media-retention-policy-source.test.mjs
```

Expected: 1 test passes, 0 tests fail.

- [ ] **Step 6: Run adjacent Pricing contract tests**

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
npm run test:team-plan
```

Expected: every Team and Enterprise Pricing source test passes with 0 failures.

- [ ] **Step 7: Review the frontend change against the approved design**

Confirm:

- the note is after the FAQ grid and before the global footer;
- the copy exactly matches the approved text;
- styling uses existing Pricing tokens;
- there is no new dependency, component, state, animation, or feature flag;
- `git diff --check` returns exit code 0.

- [ ] **Step 8: Commit the tested implementation**

```bash
git add dashboard/tests/pricing-media-retention-policy-source.test.mjs \
  dashboard/src/app/pricing/pricing-page-client.tsx
git commit -m "feat(pricing): clarify media retention timing"
```

### Task 2: Validate build and browser presentation

**Files:**
- Verify: `dashboard/src/app/pricing/pricing-page-client.tsx`
- Verify: `dashboard/tests/pricing-media-retention-policy-source.test.mjs`

- [ ] **Step 1: Run the Dashboard production build**

```bash
test "$(git rev-parse --show-toplevel)" = "/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit"
test "$(git branch --show-current)" = "dev-pricing-retention-nonretroactive"
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
npm run build
```

Expected: Next.js production build exits 0.

- [ ] **Step 2: Run Dashboard regression**

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
npm run test:regression:dashboard
```

Expected: Playwright exits 0 with no failed, skipped, cancelled, or timed-out required test.

- [ ] **Step 3: Start the local Dashboard**

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
npm run dev
```

Expected: the local Next.js server reports its localhost URL and remains running for browser verification.

- [ ] **Step 4: Verify `/pricing` in a desktop browser**

Open the local `/pricing` route at a 1440px-wide viewport and confirm:

- the note is visible only after scrolling past the FAQ grid;
- it is visually quieter than FAQ answers;
- the single top divider is visible;
- the paragraph is left-aligned and does not look like a card;
- no horizontal overflow or console errors occur.

- [ ] **Step 5: Verify `/pricing` in a mobile browser**

Repeat at a 390px-wide viewport and confirm:

- the paragraph wraps without clipping or horizontal overflow;
- it remains after the FAQ grid;
- the global footer follows it normally;
- no console errors occur.

- [ ] **Step 6: Stop the local server and confirm clean scope**

```bash
git status --short
git diff origin/dev...HEAD --name-only
git log --oneline origin/dev..HEAD
```

Expected changed files:

```text
dashboard/src/app/pricing/pricing-page-client.tsx
dashboard/tests/pricing-media-retention-policy-source.test.mjs
docs/superpowers/plans/2026-07-19-pricing-media-retention-nonretroactive.md
docs/superpowers/specs/2026-07-19-pricing-media-retention-nonretroactive-design.md
```

Expected commits are the design commit, this implementation-plan commit, and the focused implementation commit only.

### Task 3: Draft PR and Preview Acceptance

**Files:**
- Audit only the four files listed in Task 2.

- [ ] **Step 1: Re-run the final local gate on the exact branch head**

```bash
cd /Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-r2-lifetime-audit/dashboard
node --test tests/pricing-media-retention-policy-source.test.mjs
npm run test:team-plan
npm run build
npm run test:regression:dashboard
```

Expected: every command exits 0 on the SHA returned by `git rev-parse HEAD`.

- [ ] **Step 2: Audit promotion content**

```bash
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: only the approved design, plan, Pricing component, and source test are unique to the branch.

- [ ] **Step 3: Push only the owned task branch**

```bash
git push -u origin dev-pricing-retention-nonretroactive
```

- [ ] **Step 4: Open a Draft pull request to `dev`**

Create a Draft PR with:

- base: `dev`
- head: `dev-pricing-retention-nonretroactive`
- title: `Clarify non-retroactive media retention on Pricing`
- summary: adds an unobtrusive policy note after the Pricing FAQ
- tests: focused source test, Team Pricing contracts, production build, Dashboard regression

- [ ] **Step 5: Monitor exact-head Preview gates**

Record the PR head SHA and wait for all triggered checks to finish:

- GitHub CI;
- Railway PR Environment;
- Vercel Preview wired to the PR API;
- deployed regression;
- Preview Acceptance.

Any failure, error, timeout, cancellation, skipped required test, missing result,
or result for another SHA is a hard stop.

- [ ] **Step 6: Perform Codex browser acceptance on the Vercel Preview**

On the exact PR-head Vercel URL, repeat desktop and 390px mobile checks from
Task 2. Confirm the deployed page contains the approved copy, the note remains
low emphasis after the FAQ, and the browser console is clean.

- [ ] **Step 7: Merge to `dev` only after every Preview gate passes**

Before merge, repeat the commit/file audit from Step 2. Mark the PR ready and
merge only when every required result is successful on the exact head SHA.

- [ ] **Step 8: Verify the official development environment**

Wait for the persistent development deployment to finish, then open
`https://dev.unipost.dev/pricing` and repeat the desktop/mobile acceptance.
Do not report the task complete until the development deployment and browser
acceptance both succeed.
