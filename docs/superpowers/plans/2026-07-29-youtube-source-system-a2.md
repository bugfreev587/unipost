# YouTube Text Source System A2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve authenticated Dashboard YouTube identity while eliminating official YouTube graphics: channel avatars and validated text links identify sources; a refined neutral glyph represents UniPost operations and state.

**Architecture:** `youtube-source.ts` is the pure URL boundary. `YouTubeSourceLink` renders approved text plus a neutral external-link glyph, and `YouTubeChannelIdentity` composes avatar/name/link without status. Authenticated operational surfaces use `AccountDestinationIcon`; a recursive guard rejects official red/path/assets and unclassified YouTube-capable `PlatformIcon` calls.

**Tech Stack:** Next.js 16, React 19, TypeScript, CSS Modules, Node test runner, Playwright, GitHub Actions, Vercel Preview, Railway PR Environments.

**Approved design:** `docs/superpowers/specs/2026-07-29-youtube-icon-source-system-design.md`

**Release boundary:** Task branch → Draft PR to `dev` → exact-SHA Preview Acceptance → merge to `dev` → real dev acceptance. No staging or production promotion.

---

## Task 1: Pure URL policy — complete

**Files:**
- `dashboard/src/lib/youtube-source.ts`
- `dashboard/src/lib/youtube-source.test.ts`

Commit `2dba8d26` already implements and verifies valid channel IDs, empty IDs, `disconnected:` sentinels, encoding, and permitted HTTPS YouTube content hosts. Re-run this test in every focused suite.

## Task 2: Text source link and channel identity

**Files:**
- Create: `dashboard/src/components/youtube/youtube-source.module.css`
- Create: `dashboard/src/components/youtube/youtube-source-link.tsx`
- Create: `dashboard/src/components/youtube/youtube-channel-identity.tsx`
- Create: `dashboard/tests/youtube-source-components-source.test.mjs`

- [ ] **Step 1: Write failing component source tests**

Use Node's test runner and assert:

```js
assert.match(linkSource, /normalizeYouTubeContentUrl/);
assert.match(linkSource, /ExternalLink/);
assert.match(linkSource, /target="_blank"/);
assert.match(linkSource, /rel="noopener noreferrer"/);
assert.match(linkSource, /data-youtube-source-link/);
assert.match(linkSource, /aria-label=\{accessibleLabel\}/);
assert.doesNotMatch(linkSource, /<img|<svg|#ff0000|yt_icon|youtube.*asset/i);

assert.match(identitySource, /YouTubeSourceLink/);
assert.match(identitySource, /buildYouTubeChannelUrl/);
assert.match(identitySource, /YOUTUBE_HOME_URL/);
assert.match(identitySource, /Disconnected YouTube channel/);
assert.match(identitySource, /data-youtube-channel-avatar/);
assert.doesNotMatch(identitySource, /dbadge|UniPost status|health/);
```

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected red state: component files do not exist.

- [ ] **Step 2: Implement `YouTubeSourceLink`**

```tsx
import type { MouseEventHandler } from "react";
import { ExternalLink } from "lucide-react";

import { normalizeYouTubeContentUrl } from "@/lib/youtube-source";
import styles from "./youtube-source.module.css";

type YouTubeSourceLinkProps = {
  href?: string | null;
  label: string;
  disclosure: "Source: YouTube" | "Data from YouTube" | "View on YouTube";
  onClick?: MouseEventHandler<HTMLAnchorElement>;
};

export function YouTubeSourceLink({ href, label, disclosure, onClick }: YouTubeSourceLinkProps) {
  const safeHref = normalizeYouTubeContentUrl(href);
  if (!safeHref) return null;
  const accessibleLabel = disclosure === "View on YouTube"
    ? `View ${label} on YouTube`
    : label === "YouTube"
      ? "Open YouTube"
      : `Open ${label} on YouTube`;

  return (
    <a
      href={safeHref}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={accessibleLabel}
      data-youtube-source-link
      className={styles.sourceLink}
      onClick={onClick}
    >
      <span>{disclosure}</span>
      <ExternalLink size={14} strokeWidth={1.8} aria-hidden="true" />
    </a>
  );
}
```

CSS: inline-flex, minimum 44px height, muted text, no red styling, 14px neutral icon, high-contrast `:focus-visible` outline, and safe wrapping at 390px.

- [ ] **Step 3: Implement `YouTubeChannelIdentity`**

Use `Pick<SocialAccount, "id" | "account_name" | "account_avatar_url" | "external_account_id" | "status">`. Compute:

```ts
const disconnected = account.status === "disconnected"
  || account.external_account_id?.trim().toLowerCase().startsWith("disconnected:") === true;
const channelUrl = buildYouTubeChannelUrl(account.external_account_id);
const displayName = disconnected
  ? "Disconnected YouTube channel"
  : account.account_name?.trim() || "YouTube channel";
const sourceHref = disconnected ? null : channelUrl || YOUTUBE_HOME_URL;
```

Render a 40px standard or 32px compact avatar, initials fallback, then neutral destination-glyph fallback. Warn once per active UniPost account ID when using the home fallback. Render `YouTubeSourceLink`; do not render status or controls.

- [ ] **Step 4: Verify and commit**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts tests/youtube-source-components-source.test.mjs`

Expected: PASS, 0 failures.

Commit: `feat: add YouTube text source identity`

## Task 3: Neutral glyph and account identity integration

**Files:**
- Modify: `dashboard/src/components/account-destination-icon.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx`
- Modify: `dashboard/tests/youtube-compliance-ui-source.test.mjs`
- Modify: `dashboard/tests/youtube-source-components-source.test.mjs`

- [ ] **Step 1: Add failing tests**

Require `data-youtube-destination-icon`, internal outline SVG, `currentColor`, no Lucide `Video`, no official fill/path/asset. Require both concrete-account pages to import `YouTubeChannelIdentity` and pass compact `Source: YouTube`; status remains outside the identity component.

Run the two source tests; expect failure on current Lucide icon and missing identities.

- [ ] **Step 2: Implement neutral SVG**

```tsx
<span style={style} aria-hidden="true" data-youtube-destination-icon>
  <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
    <rect x="3" y="5" width="18" height="14" rx="4" stroke="currentColor" strokeWidth="1.8" />
    <path d="M10 9.25L15 12L10 14.75Z" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
  </svg>
</span>
```

- [ ] **Step 3: Integrate identities**

In Accounts and managed-user detail, use `YouTubeChannelIdentity` only when `platform === "youtube"`; preserve existing non-YouTube visuals. Keep Accounts status in the `UniPost status` column and managed-user status/actions as sibling regions.

- [ ] **Step 4: Verify and commit**

Run both focused source tests plus URL tests. Expected: PASS.

Commit: `feat: show neutral YouTube destinations and channel identity`

## Task 4: Analytics and published-result migration

**Files:**
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx`
- Modify: `dashboard/src/components/analytics/youtube-analytics-view.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/page.tsx`
- Modify: `dashboard/src/components/posts/details/post-platform-results.tsx`
- Modify: `dashboard/tests/youtube-analytics-dashboard-source.test.mjs`
- Create: `dashboard/tests/youtube-published-result-source.test.mjs`

- [ ] **Step 1: Write failing semantic tests**

Require neutral navigation/aggregate/status glyphs; `YouTubeChannelIdentity` with `Data from YouTube`; no direct channel URL template; result source links require `platform === "youtube"` and `normalizeYouTubeContentUrl`; status clusters keep `AccountDestinationIcon`.

- [ ] **Step 2: Migrate navigation and channel identity**

Use `AccountDestinationIcon` for YouTube Analytics navigation/page heading and every dynamic aggregate/post/status platform icon. Replace the local channel avatar/direct URL block with `YouTubeChannelIdentity` and keep readiness/freshness outside it.

- [ ] **Step 3: Migrate valid result links**

Compute a normalized YouTube URL only for YouTube. Keep the neutral status glyph; in the separate action area render `YouTubeSourceLink disclosure="View on YouTube"`. Invalid/missing YouTube URLs render no source link. Preserve non-YouTube external links.

- [ ] **Step 4: Verify and commit**

Run URL, component, Analytics, published-result, and compliance tests. Expected: PASS.

Commit: `feat: clarify YouTube analytics and result sources`

## Task 5: Inbox and Admin migration

**Files:**
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx`
- Modify: `dashboard/src/app/admin/users/page.tsx`
- Modify: `dashboard/tests/youtube-source-components-source.test.mjs`

- [ ] **Step 1: Add failing tests**

Inbox must have no `PlatformIcon` import, use `AccountDestinationIcon`, show `Source: YouTube`, and render an optional validated `YouTubeSourceLink` in a `data-youtube-inbox-source-action` region outside controls. Admin must use `AccountDestinationIcon` for all three dynamic platform collections and contain no `PlatformIcon` import.

- [ ] **Step 2: Migrate Inbox**

Replace all Inbox platform glyph calls with `AccountDestinationIcon` (non-YouTube remains visually delegated). Return `Source: YouTube` for `youtube_comment`. Normalize the first available YouTube item URL; if valid, render the text link in a separate context action row, never in unread/thread/Resolve/Reply/status clusters.

- [ ] **Step 3: Migrate Admin**

Replace the three dynamic `PlatformIcon` collections with `AccountDestinationIcon` at the existing size; preserve wrappers, titles, counts, and state colors.

- [ ] **Step 4: Verify and commit**

Run URL, component, and compliance tests. Expected: PASS.

Commit: `fix: remove authenticated red YouTube icons`

## Task 6: Dashboard-wide guard, CI, and evidence

**Files:**
- Modify: `dashboard/tests/youtube-compliance-ui-source.test.mjs`
- Modify: `dashboard/package.json`
- Modify: `.github/workflows/ci.yml`
- Create: `docs/compliance/youtube-dashboard-source-system.md`

- [ ] **Step 1: Implement recursive default-deny guard**

Scan `.ts`/`.tsx` under authenticated Dashboard/Admin and shared components. Reject official-red variants, the legacy path prefix `M23.498 6.186`, YouTube brand-asset references, `<PlatformIcon platform="youtube"`, and dynamic route-level `PlatformIcon` expressions. Assert `YouTubeSourceLink` has no image/SVG/brand reference and satisfies URL/anchor/target/rel/accessibility/text contracts.

- [ ] **Step 2: Record compliance evidence**

Document the audit finding, current brand-site 100px minimum, A2 decision, exact source URLs, semantic matrix, and these out-of-scope public consumers:

```text
dashboard/src/app/marketing/page.tsx
dashboard/src/app/about/page.tsx
dashboard/src/app/blog/_components/blog-cover.tsx
dashboard/src/app/tools/_components/public-analytics-tool.tsx
dashboard/src/components/tools/ToolCard.tsx
dashboard/src/components/platform-icons.tsx
```

- [ ] **Step 3: Wire package and CI**

Add `test:youtube-compliance` running URL, compliance, component, result, and Analytics source tests. Add `Run YouTube UI compliance contracts` before the Dashboard build in `.github/workflows/ci.yml`.

- [ ] **Step 4: Verify and commit**

Run `cd dashboard && npm run test:youtube-compliance`. Expected: PASS.

Commit: `test: enforce YouTube text attribution boundaries`

## Task 7: Fixture-backed browser acceptance

**Files:**
- Modify: `dashboard/tests/regression/authenticated-dashboard.spec.ts`

- [ ] **Step 1: Add a fixture-backed YouTube account**

After synthetic user bootstrap, intercept the profile accounts request with an active YouTube account containing channel name, avatar data URL, real-looking channel ID, managed connection type, and Analytics scopes. Leave other calls on the target API.

- [ ] **Step 2: Assert visual/interaction contract**

Require visible channel avatar/name/`Source: YouTube`, exact channel href, target/rel, 44px minimum link height, separate `Active` status, neutral destination icon on operational surfaces, visible focus outline, no `[data-youtube-official-mark]`, no overflow at 390x844, and successful light/dark screenshots.

- [ ] **Step 3: Verify and commit**

Run authenticated regression with required Clerk credentials. Missing credentials or a non-starting/skipped test is a failure and blocks push.

Commit: `test: cover YouTube text source hierarchy`

## Task 8: Local verification, Draft PR, Preview, and dev only

- [ ] Verify exclusive path/branch before every write/test/commit/push.
- [ ] Run `npm run test:youtube-compliance`, `npm run test:docs-ai`, `npm run build`, public Dashboard regression, and authenticated regression; require zero failure/error/timeout/cancel/skip.
- [ ] Audit `git log --oneline origin/dev..HEAD`, `git diff --name-status origin/dev...HEAD`, and `git diff --check origin/dev...HEAD`; unrelated content blocks release.
- [ ] Push only `origin/dev-youtube-icon-source-system`.
- [ ] Open Draft PR to `dev`; monitor GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and all triggered checks on the exact head SHA.
- [ ] Perform Codex Preview acceptance for Accounts, managed user, Analytics, results, Inbox, Admin, Create Post, Calendar, health/counts/capabilities, light/dark, keyboard, and mobile.
- [ ] After every Preview gate passes, mark ready, re-audit content, and merge the PR to `dev`.
- [ ] Monitor persistent `dev` deployments and repeat acceptance on `https://dev-app.unipost.dev` with `https://dev-api.unipost.dev`.
- [ ] Report SHAs, PR/check/deployment URLs, evidence, test results, and public residual risk. Do not open staging or production PRs.
