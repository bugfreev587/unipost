# GIF Guidance Status Copy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the GIF Guidance unambiguous that GIF-to-MP4 conversion is available while destination-specific publishing guidance remains coming soon.

**Architecture:** Keep the current documentation page and table structure. Add a source-contract regression assertion first, then make the minimum copy-only changes in the page and validate the complete docs surface.

**Tech Stack:** Next.js App Router, React Server Components, Node.js built-in test runner.

---

### Task 1: Lock the user-facing status contract

**Files:**
- Modify: `dashboard/tests/gif-conversion-docs-source.test.mjs`

- [ ] **Step 1: Add the failing assertions**

Add these assertions to `GIF guidance keeps direct support scoped and links conversion`:

```js
assert.equal(
  source.match(/GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon/g)?.length,
  5,
);
assert.doesNotMatch(source, /MP4 conversion supported; GIF guidance coming soon/);
assert.doesNotMatch(source, /prepare for upcoming conversion workflows/);
assert.match(source, /Destination-specific publishing guides and the Dashboard conversion control are still coming soon/);
```

- [ ] **Step 2: Run the test and verify RED**

Run:

```bash
cd dashboard
node --test tests/gif-conversion-docs-source.test.mjs
```

Expected: the GIF Guidance subtest fails because the new wording is absent.

- [ ] **Step 3: Commit the test**

```bash
git add dashboard/tests/gif-conversion-docs-source.test.mjs
git commit -m "test: clarify GIF guidance availability copy"
```

### Task 2: Make conversion availability explicit

**Files:**
- Modify: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`

- [ ] **Step 1: Update the lead**

Replace:

```tsx
lead="Publish a hosted or local GIF to X and Facebook, compare official platform support with UniPost support, and prepare for upcoming conversion workflows."
```

with:

```tsx
lead="Publish a hosted or local GIF to X and Facebook, compare official platform support with UniPost support, and convert GIFs for video-only destinations."
```

- [ ] **Step 2: Update the five status cells**

For Instagram, TikTok, Pinterest, YouTube, and Bluesky, replace the status with:

```tsx
"GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon",
```

- [ ] **Step 3: Clarify the final availability note**

Replace:

```tsx
Destination-specific GIF guidance and the Dashboard conversion control are still coming soon.
```

with:

```tsx
Destination-specific publishing guides and the Dashboard conversion control are still coming soon.
```

- [ ] **Step 4: Run the targeted test and verify GREEN**

Run:

```bash
cd dashboard
node --test tests/gif-conversion-docs-source.test.mjs
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit the copy**

```bash
git add dashboard/src/app/docs/guides/publish-gifs/page.tsx
git commit -m "docs: clarify GIF conversion availability"
```

### Task 3: Review and validate the complete docs experience

**Files:**
- Review: `dashboard/src/app/docs/guides/publish-gifs/page.tsx`
- Review: `dashboard/src/app/docs/api/media/gif-conversions/page.tsx`

- [ ] **Step 1: Apply verified PM feedback**

Accept only feedback that identifies a factual contradiction, ambiguity, or user workflow problem. Keep the guide scoped to X, Facebook, and a support-status summary for other platforms.

- [ ] **Step 2: Run docs AI tests**

Run:

```bash
cd dashboard
npm run test:docs-ai
```

Expected: all tests pass.

- [ ] **Step 3: Run the production build**

Run:

```bash
cd dashboard
npm run build
```

Expected: Next.js production build succeeds.

- [ ] **Step 4: Merge and validate local dev**

Update local `dev` from `origin/dev`, merge `dev-gif-guidance-copy`, rerun the targeted source test and production build, then push local `dev` to `origin/dev`.

- [ ] **Step 5: Monitor and verify deployed dev**

Wait for GitHub, Vercel, and Railway checks to finish. Open `https://dev.unipost.dev/docs/guides/publish-gifs` in a browser and verify:

- all five revised status cells render;
- the lead presents conversion as available;
- the conversion section distinguishes API availability from upcoming destination-specific guides;
- no browser console error is introduced.
