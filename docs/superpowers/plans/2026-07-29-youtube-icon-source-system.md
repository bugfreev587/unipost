# YouTube Icon Source System Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace ambiguous authenticated-Dashboard YouTube branding with a compliant two-tier system: official linked attribution for real YouTube identity/content and a refined neutral glyph for UniPost-owned operations and state.

**Architecture:** A pure URL-policy module validates channel IDs and YouTube content URLs. `YouTubeSourceLink` is the only authenticated component allowed to reference the downloaded official asset, while `YouTubeChannelIdentity` composes avatar, channel name, disclosure, and source link without rendering UniPost status. All operational surfaces use `AccountDestinationIcon`; a source-level compliance suite enforces asset ownership, brand fingerprints, authenticated-surface classification, and required link semantics.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript, CSS Modules, Node test runner, Playwright, GitHub Actions, Vercel Preview, Railway PR Environments.

**Approved design:** `docs/superpowers/specs/2026-07-29-youtube-icon-source-system-design.md`

**Release boundary:** Implement on `dev-youtube-icon-source-system`, open a Draft PR to `dev`, pass Preview Acceptance on the exact head SHA, merge to `dev`, and verify the persistent development deployment. Do not create a staging or production PR.

---

## File map

### New files

- `dashboard/src/lib/youtube-source.ts` — pure channel-ID and YouTube-content URL policy.
- `dashboard/src/lib/youtube-source.test.ts` — executable URL-policy contract.
- `dashboard/src/components/youtube/youtube-source-link.tsx` — sole authenticated renderer of the official YouTube asset.
- `dashboard/src/components/youtube/youtube-channel-identity.tsx` — channel avatar/name/source composition with disconnected fallbacks.
- `dashboard/src/components/youtube/youtube-source.module.css` — clear space, hierarchy, focus, responsive, and theme styling.
- `dashboard/public/brand/youtube/yt_icon_rgb.svg` — unmodified icon downloaded from the official YouTube brand site.
- `docs/compliance/youtube-brand-asset.md` — source URL, retrieval date, SHA-256, permitted use, and public-surface residual risk.
- `dashboard/tests/youtube-source-components-source.test.mjs` — component ownership, semantics, and integration source contracts.
- `dashboard/tests/youtube-published-result-source.test.mjs` — valid-result attribution and invalid-result fallback contract.

### Modified files

- `dashboard/src/components/account-destination-icon.tsx` — replace Lucide camera with neutral outline/video-play SVG.
- `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx` — channel identity in Account cell; status stays in its own column.
- `dashboard/src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx` — channel identity in managed-user detail; status/actions stay separate.
- `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx` — neutral YouTube navigation glyph.
- `dashboard/src/components/analytics/youtube-analytics-view.tsx` — neutral page heading plus channel identity and sentinel-safe URL behavior.
- `dashboard/src/app/(dashboard)/projects/[id]/analytics/page.tsx` — neutral aggregates/status clusters and separate official result links.
- `dashboard/src/components/posts/details/post-platform-results.tsx` — official link only for validated published YouTube URLs.
- `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx` — neutral conversation/status icon, explicit source copy, optional separate valid-content link.
- `dashboard/src/app/admin/users/page.tsx` — neutral platform glyphs for all dynamic platform summaries.
- `dashboard/tests/youtube-compliance-ui-source.test.mjs` — Dashboard-wide default-deny guard.
- `dashboard/tests/youtube-analytics-dashboard-source.test.mjs` — update Analytics expectations from legacy red icon to semantic components.
- `dashboard/tests/regression/authenticated-dashboard.spec.ts` — fixture-backed source-link, theme, focus, prominence, and mobile acceptance.
- `dashboard/package.json` — add the focused compliance test command.
- `.github/workflows/ci.yml` — run the focused compliance command as a required CI step.

## Task 1: Add the pure YouTube URL policy

**Files:**
- Create: `dashboard/src/lib/youtube-source.test.ts`
- Create: `dashboard/src/lib/youtube-source.ts`

- [ ] **Step 1: Write the failing URL-policy test**

```ts
import assert from "node:assert/strict";
import test from "node:test";

import {
  YOUTUBE_HOME_URL,
  buildYouTubeChannelUrl,
  normalizeYouTubeContentUrl,
} from "./youtube-source.ts";

test("builds an encoded channel URL only for a real channel ID", () => {
  assert.equal(buildYouTubeChannelUrl("UCabc_123-xyz"), "https://www.youtube.com/channel/UCabc_123-xyz");
  assert.equal(buildYouTubeChannelUrl(" channel/id "), "https://www.youtube.com/channel/channel%2Fid");
});

test("rejects empty IDs and disconnected sentinels", () => {
  assert.equal(buildYouTubeChannelUrl(undefined), null);
  assert.equal(buildYouTubeChannelUrl(""), null);
  assert.equal(buildYouTubeChannelUrl("   "), null);
  assert.equal(buildYouTubeChannelUrl("disconnected:account-123"), null);
  assert.equal(buildYouTubeChannelUrl(" DISCONNECTED:account-123 "), null);
});

test("accepts only HTTPS YouTube content destinations", () => {
  for (const value of [
    YOUTUBE_HOME_URL,
    "https://youtube.com/watch?v=abc",
    "https://m.youtube.com/watch?v=abc",
    "https://youtu.be/abc",
  ]) {
    assert.equal(normalizeYouTubeContentUrl(value), value);
  }
  for (const value of [
    undefined,
    "",
    "http://www.youtube.com/watch?v=abc",
    "https://youtube.example/watch?v=abc",
    "https://evil.youtube.com/watch?v=abc",
    "javascript:alert(1)",
  ]) {
    assert.equal(normalizeYouTubeContentUrl(value), null);
  }
});
```

- [ ] **Step 2: Run the focused test and verify the red state**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts`

Expected: FAIL with `ERR_MODULE_NOT_FOUND` for `youtube-source.ts`.

- [ ] **Step 3: Implement the minimal pure policy**

```ts
export const YOUTUBE_HOME_URL = "https://www.youtube.com/";

const YOUTUBE_CONTENT_HOSTS = new Set([
  "youtube.com",
  "www.youtube.com",
  "m.youtube.com",
  "youtu.be",
]);

export function buildYouTubeChannelUrl(rawChannelId?: string | null): string | null {
  const channelId = rawChannelId?.trim();
  if (!channelId || channelId.toLowerCase().startsWith("disconnected:")) return null;
  return `https://www.youtube.com/channel/${encodeURIComponent(channelId)}`;
}

export function normalizeYouTubeContentUrl(rawUrl?: string | null): string | null {
  const value = rawUrl?.trim();
  if (!value) return null;
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "https:" || !YOUTUBE_CONTENT_HOSTS.has(parsed.hostname.toLowerCase())) return null;
    return parsed.toString();
  } catch {
    return null;
  }
}
```

- [ ] **Step 4: Run the focused test and verify the green state**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts`

Expected: PASS, 3 tests, 0 failures.

- [ ] **Step 5: Commit the URL contract**

```bash
git add dashboard/src/lib/youtube-source.ts dashboard/src/lib/youtube-source.test.ts
git commit -m "feat: validate YouTube source destinations"
```

## Task 2: Add the official asset and source components

**Files:**
- Create: `dashboard/public/brand/youtube/yt_icon_rgb.svg`
- Create: `docs/compliance/youtube-brand-asset.md`
- Create: `dashboard/src/components/youtube/youtube-source.module.css`
- Create: `dashboard/src/components/youtube/youtube-source-link.tsx`
- Create: `dashboard/src/components/youtube/youtube-channel-identity.tsx`
- Create: `dashboard/tests/youtube-source-components-source.test.mjs`

- [ ] **Step 1: Write the failing source-component contract**

Create a Node source test that reads the two new component files and asserts all of these exact contracts:

```js
test("official YouTube attribution is owned by one accessible link component", async () => {
  const source = await read("src/components/youtube/youtube-source-link.tsx");
  assert.match(source, /\/brand\/youtube\/yt_icon_rgb\.svg/);
  assert.match(source, /normalizeYouTubeContentUrl/);
  assert.match(source, /target="_blank"/);
  assert.match(source, /rel="noopener noreferrer"/);
  assert.match(source, /data-youtube-source-link/);
  assert.match(source, /data-youtube-source-mark/);
  assert.match(source, /aria-label=\{accessibleLabel\}/);
});

test("channel identity owns source disclosure but not UniPost status", async () => {
  const source = await read("src/components/youtube/youtube-channel-identity.tsx");
  assert.match(source, /YouTubeSourceLink/);
  assert.match(source, /buildYouTubeChannelUrl/);
  assert.match(source, /YOUTUBE_HOME_URL/);
  assert.match(source, /Disconnected YouTube channel/);
  assert.match(source, /Source: YouTube/);
  assert.doesNotMatch(source, /dbadge|UniPost status|health/);
});
```

Use this complete shared reader at the top of the file:

```js
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();
const read = (path) => readFile(join(root, path), "utf8");
```

- [ ] **Step 2: Run the component contract and verify the red state**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected: FAIL because the component files do not exist.

- [ ] **Step 3: Obtain and record the official asset without redrawing it**

Open `https://brand.youtube/` from the official link in the [YouTube API Branding Guidelines](https://developers.google.com/youtube/terms/branding-guidelines). Download the current RGB YouTube Icon vector package, copy the unmodified SVG to `dashboard/public/brand/youtube/yt_icon_rgb.svg`, and run:

```bash
shasum -a 256 dashboard/public/brand/youtube/yt_icon_rgb.svg
```

Create `docs/compliance/youtube-brand-asset.md` containing the official source URL, retrieval date, the exact emitted SHA-256, the original package filename, the published minimum-size rule, the rule that only `YouTubeSourceLink` may reference it in authenticated UI, and the public `PlatformIcon` inventory listed in Task 8. Do not copy the private audit PDF into the repository. If the current official minimum is larger than 24 CSS pixels, use that published minimum consistently in the component, CSS, source test, and browser measurements instead of 24.

- [ ] **Step 4: Implement `YouTubeSourceLink`**

Implement this public interface and behavior:

```tsx
import type { MouseEventHandler } from "react";

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
      <span className={styles.markFrame} aria-hidden="true">
        <img
          src="/brand/youtube/yt_icon_rgb.svg"
          alt=""
          width={24}
          height={17}
          data-youtube-source-mark
          className={styles.mark}
        />
      </span>
      <span className={styles.disclosure}>{disclosure}</span>
    </a>
  );
}
```

Use this CSS module, increasing the 24px artwork dimensions only if the current official minimum is larger:

```css
.sourceLink {
  display: inline-flex;
  min-width: 44px;
  min-height: 44px;
  align-items: center;
  gap: 8px;
  border-radius: 10px;
  color: var(--dmuted);
  text-decoration: none;
}

.sourceLink:hover {
  color: var(--dtext);
}

.sourceLink:focus-visible {
  outline: 3px solid var(--daccent);
  outline-offset: 2px;
}

.markFrame {
  display: inline-flex;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--dborder);
  border-radius: 10px;
  background: var(--surface);
}

.mark {
  display: block;
  width: 24px;
  height: auto;
  filter: none;
}

.disclosure {
  min-width: 0;
  font-size: 12px;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.identity {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.avatar,
.avatarFallback {
  width: 40px;
  height: 40px;
  border: 1px solid var(--dborder);
  border-radius: 11px;
  background: var(--surface2);
}

.avatar {
  display: block;
  object-fit: cover;
}

.avatarFallback {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--dmuted);
  font-size: 12px;
  font-weight: 650;
}

.compact .avatar,
.compact .avatarFallback {
  width: 32px;
  height: 32px;
  border-radius: 9px;
}

.identityBody {
  min-width: 0;
}

.channelName {
  overflow: hidden;
  color: var(--dtext);
  font-size: 13px;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 520px) {
  .sourceLink {
    max-width: 100%;
  }
}
```

- [ ] **Step 5: Implement `YouTubeChannelIdentity`**

Implement the component with this complete state and fallback contract:

```tsx
"use client";

import { useEffect, useState } from "react";

import { AccountDestinationIcon } from "@/components/account-destination-icon";
import type { SocialAccount } from "@/lib/api";
import { buildYouTubeChannelUrl, YOUTUBE_HOME_URL } from "@/lib/youtube-source";
import styles from "./youtube-source.module.css";
import { YouTubeSourceLink } from "./youtube-source-link";

type YouTubeIdentityAccount = Pick<
  SocialAccount,
  "id" | "account_name" | "account_avatar_url" | "external_account_id" | "status"
>;

type YouTubeChannelIdentityProps = {
  account: YouTubeIdentityAccount;
  density?: "standard" | "compact";
  disclosure: "Source: YouTube" | "Data from YouTube";
};

const warnedMissingChannelIds = new Set<string>();

function initialsFor(value: string): string {
  const parts = value.trim().split(/\s+/).filter(Boolean);
  return parts.slice(0, 2).map((part) => part[0]?.toUpperCase()).join("") || "";
}

export function YouTubeChannelIdentity({
  account,
  density = "standard",
  disclosure,
}: YouTubeChannelIdentityProps) {
  const [failedAvatar, setFailedAvatar] = useState<string | null>(null);
const disconnected = account.status === "disconnected"
  || account.external_account_id?.trim().toLowerCase().startsWith("disconnected:") === true;
const channelUrl = buildYouTubeChannelUrl(account.external_account_id);
const displayName = disconnected
  ? "Disconnected YouTube channel"
  : account.account_name?.trim() || "YouTube channel";
const sourceHref = disconnected ? null : channelUrl || YOUTUBE_HOME_URL;
  const avatarUrl = account.account_avatar_url?.trim() || null;
  const showAvatar = Boolean(avatarUrl && failedAvatar !== avatarUrl);
  const initials = account.account_name ? initialsFor(account.account_name) : "";

  useEffect(() => {
    if (disconnected || channelUrl || warnedMissingChannelIds.has(account.id)) return;
    warnedMissingChannelIds.add(account.id);
    console.warn("[youtube-source] Active account is missing a valid channel ID", { accountId: account.id });
  }, [account.id, channelUrl, disconnected]);

  return (
    <div className={`${styles.identity} ${density === "compact" ? styles.compact : ""}`}>
      {showAvatar ? (
        <img
          src={avatarUrl || ""}
          alt={`${displayName} avatar`}
          className={styles.avatar}
          data-youtube-channel-avatar
          onError={() => setFailedAvatar(avatarUrl)}
        />
      ) : (
        <span className={styles.avatarFallback} data-youtube-channel-avatar aria-hidden="true">
          {initials || <AccountDestinationIcon platform="youtube" size={density === "compact" ? 16 : 18} />}
        </span>
      )}
      <div className={styles.identityBody}>
        <div className={styles.channelName}>{displayName}</div>
        <YouTubeSourceLink href={sourceHref} label={displayName} disclosure={disclosure} />
      </div>
    </div>
  );
}
```

Do not add status, health, selection, disconnect, or editing controls to this component.

- [ ] **Step 6: Run source and URL tests**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts tests/youtube-source-components-source.test.mjs`

Expected: PASS, all tests, 0 failures.

- [ ] **Step 7: Commit the owned official source component**

```bash
git add dashboard/public/brand/youtube/yt_icon_rgb.svg docs/compliance/youtube-brand-asset.md dashboard/src/components/youtube dashboard/tests/youtube-source-components-source.test.mjs
git commit -m "feat: add compliant YouTube source identity"
```

## Task 3: Refine the neutral operational glyph

**Files:**
- Modify: `dashboard/tests/youtube-compliance-ui-source.test.mjs`
- Modify: `dashboard/src/components/account-destination-icon.tsx`

- [ ] **Step 1: Add failing neutral-glyph assertions**

Add assertions requiring `data-youtube-destination-icon`, an internal `<svg>`, `stroke="currentColor"`, and no Lucide `Video` import, official red fill, official path prefix, or official asset reference.

- [ ] **Step 2: Run the current compliance test and verify the red state**

Run: `cd dashboard && node --test tests/youtube-compliance-ui-source.test.mjs`

Expected: FAIL because the current component imports and renders Lucide `Video`.

- [ ] **Step 3: Replace only the YouTube branch with the neutral primitive**

```tsx
return (
  <span style={style} aria-hidden="true" data-youtube-destination-icon>
    <svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <rect x="3" y="5" width="18" height="14" rx="4" stroke="currentColor" strokeWidth="1.8" />
      <path d="M10 9.25L15 12L10 14.75Z" stroke="currentColor" strokeWidth="1.6" strokeLinejoin="round" />
    </svg>
  </span>
);
```

Remove `Video` from the Lucide import. Leave non-YouTube delegation to `PlatformIcon` unchanged.

- [ ] **Step 4: Re-run the focused compliance test**

Run: `cd dashboard && node --test tests/youtube-compliance-ui-source.test.mjs`

Expected: PASS, 0 failures.

- [ ] **Step 5: Commit the neutral glyph**

```bash
git add dashboard/src/components/account-destination-icon.tsx dashboard/tests/youtube-compliance-ui-source.test.mjs
git commit -m "feat: refine neutral YouTube destination icon"
```

## Task 4: Integrate channel identity into Accounts and managed-user detail

**Files:**
- Modify: `dashboard/tests/youtube-source-components-source.test.mjs`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx`

- [ ] **Step 1: Add failing integration assertions**

Assert both pages import `YouTubeChannelIdentity`; Accounts passes `density="compact"` and `disclosure="Source: YouTube"`; managed-user detail does the same for `acc.platform === "youtube"`; and both keep their status markup outside the identity component.

- [ ] **Step 2: Run the source-component test and verify the red state**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected: FAIL because neither page imports the identity component.

- [ ] **Step 3: Replace the Accounts account-cell visual conditionally**

For YouTube, render:

```tsx
<YouTubeChannelIdentity
  account={a}
  density="compact"
  disclosure="Source: YouTube"
/>
```

For other platforms preserve the existing `AccountDestinationIcon` and name/source copy. Do not move the existing `UniPost Profile`, `Source platform`, `Connected`, `UniPost status`, or Inbox capability columns. This preserves physical separation between source identity and UniPost state.

- [ ] **Step 4: Replace the managed-user account identity conditionally**

Render the same compact identity for YouTube and preserve the existing non-YouTube icon/name block. Keep the status badge and disconnect/dismiss button as sibling regions after the flexing identity region.

- [ ] **Step 5: Run both focused source suites**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs tests/youtube-compliance-ui-source.test.mjs`

Expected: PASS, 0 failures.

- [ ] **Step 6: Commit account identity adoption**

```bash
git add 'dashboard/src/app/(dashboard)/projects/[id]/accounts/page.tsx' 'dashboard/src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx' dashboard/tests/youtube-source-components-source.test.mjs
git commit -m "feat: show linked YouTube channel identity"
```

## Task 5: Migrate Analytics and published-result attribution

**Files:**
- Modify: `dashboard/tests/youtube-analytics-dashboard-source.test.mjs`
- Create: `dashboard/tests/youtube-published-result-source.test.mjs`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx`
- Modify: `dashboard/src/components/analytics/youtube-analytics-view.tsx`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/analytics/page.tsx`
- Modify: `dashboard/src/components/posts/details/post-platform-results.tsx`

- [ ] **Step 1: Update Analytics tests to the new semantic contract**

Change the old expectation for `<PlatformIcon platform="youtube"` to `AccountDestinationIcon platform="youtube"` in the platform navigation. Add assertions that `youtube-analytics-view.tsx` uses `YouTubeChannelIdentity`, contains `Data from YouTube`, contains no `<PlatformIcon platform="youtube"`, and no longer contains the direct channel-URL template prefix `https://www.youtube.com/channel/`.

- [ ] **Step 2: Write failing published-result tests**

The new test must assert both Analytics `ResultCard` and `PostPlatformResults`:

```js
assert.match(source, /normalizeYouTubeContentUrl/);
assert.match(source, /YouTubeSourceLink/);
assert.match(source, /View on YouTube/);
assert.match(source, /AccountDestinationIcon/);
```

It must also assert the official source-link branch requires `platform === "youtube"` and a normalized URL, while the status cluster retains `AccountDestinationIcon`.

- [ ] **Step 3: Run the Analytics and result tests and verify the red state**

Run: `cd dashboard && node --test tests/youtube-analytics-dashboard-source.test.mjs tests/youtube-published-result-source.test.mjs`

Expected: FAIL on legacy `PlatformIcon`, missing identity component, direct URL construction, and missing source links.

- [ ] **Step 4: Migrate Analytics navigation and aggregate/status glyphs**

- In `platform-analytics-list.tsx`, use `AccountDestinationIcon` for the YouTube navigation card.
- In `analytics/page.tsx`, replace the three dynamic `PlatformIcon` call sites in By Platform, post rows, and result status clusters with `AccountDestinationIcon`.
- Preserve literal non-YouTube platform-specific analytics icons.

- [ ] **Step 5: Migrate the YouTube Analytics page**

- Replace the page-heading `PlatformIcon` with `AccountDestinationIcon platform="youtube"`.
- Replace the local `ChannelAvatar` and direct channel URL with `YouTubeChannelIdentity account={account} disclosure="Data from YouTube"`.
- Remove the obsolete `ChannelAvatar` helper and `next/link` import only if no remaining use requires them.
- Keep freshness, report window, subscriber semantics, readiness, and reconnect state outside the identity region.

- [ ] **Step 6: Add separate official attribution to valid YouTube results**

At each result-card site, compute:

```ts
const youtubeSourceUrl = platform === "youtube"
  ? normalizeYouTubeContentUrl(url)
  : null;
```

Keep `AccountDestinationIcon` beside the UniPost result status. In the separate link/action region, render:

```tsx
<YouTubeSourceLink
  href={youtubeSourceUrl}
  label={accountName || "published post"}
  disclosure="View on YouTube"
  onClick={(event) => event.stopPropagation()}
/>
```

Only use that branch when `youtubeSourceUrl` is non-null. Preserve the existing external-link treatment for non-YouTube platforms. If a YouTube result URL is absent or fails host/protocol validation, render no official mark and retain the neutral status glyph.

- [ ] **Step 7: Run Analytics, result, URL, and compliance tests**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts tests/youtube-analytics-dashboard-source.test.mjs tests/youtube-published-result-source.test.mjs tests/youtube-compliance-ui-source.test.mjs`

Expected: PASS, all tests, 0 failures.

- [ ] **Step 8: Commit Analytics and result migration**

```bash
git add 'dashboard/src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx' dashboard/src/components/analytics/youtube-analytics-view.tsx 'dashboard/src/app/(dashboard)/projects/[id]/analytics/page.tsx' dashboard/src/components/posts/details/post-platform-results.tsx dashboard/tests/youtube-analytics-dashboard-source.test.mjs dashboard/tests/youtube-published-result-source.test.mjs
git commit -m "feat: separate YouTube analytics source attribution"
```

## Task 6: Migrate Inbox source attribution

**Files:**
- Modify: `dashboard/tests/youtube-source-components-source.test.mjs`
- Modify: `dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx`

- [ ] **Step 1: Add failing Inbox assertions**

Assert the Inbox imports `AccountDestinationIcon`, uses visible `Source: YouTube` text for `youtube_comment`, contains no `PlatformIcon` import, validates any optional YouTube item URL with `normalizeYouTubeContentUrl`, and places `YouTubeSourceLink` in a distinct `data-youtube-inbox-source-action` region.

- [ ] **Step 2: Run the source-component test and verify the red state**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected: FAIL on the legacy `PlatformIcon` import and missing source disclosure/action region.

- [ ] **Step 3: Replace all Inbox platform glyph call sites**

Replace the current Twitter literals and dynamic group/context calls with `AccountDestinationIcon`. For non-YouTube platforms this delegates to the existing icon, while YouTube resolves to the neutral glyph. Update the stale `platformFromSource` comment so it describes destination-glyph routing, not `PlatformIcon` fallback behavior.

- [ ] **Step 4: Make YouTube source disclosure explicit**

Change `sourceLabel` to return `Source: YouTube` for `youtube_comment` and preserve existing short labels for other sources. Keep unread count and `StatusPill` in their current UniPost-managed positions.

- [ ] **Step 5: Add the optional separated content link**

For a selected YouTube comment group, normalize the first available `item.url`. If valid, render `YouTubeSourceLink` with `View on YouTube` in a dedicated action row below the source/context heading and above the content body. Do not place it inside the Resolve/Re-open toolbar, timestamp, unread, reply, publish-status, or thread-status clusters. If the URL is absent or invalid, render no official mark.

- [ ] **Step 6: Run Inbox, URL, and compliance source tests**

Run: `cd dashboard && node --test src/lib/youtube-source.test.ts tests/youtube-source-components-source.test.mjs tests/youtube-compliance-ui-source.test.mjs`

Expected: PASS, all tests, 0 failures.

- [ ] **Step 7: Commit Inbox migration**

```bash
git add 'dashboard/src/app/(dashboard)/projects/[id]/inbox/page.tsx' dashboard/tests/youtube-source-components-source.test.mjs
git commit -m "feat: clarify YouTube sources in Inbox"
```

## Task 7: Migrate authenticated Admin platform summaries

**Files:**
- Modify: `dashboard/tests/youtube-source-components-source.test.mjs`
- Modify: `dashboard/src/app/admin/users/page.tsx`

- [ ] **Step 1: Add failing Admin assertions**

Assert `app/admin/users/page.tsx` imports and uses `AccountDestinationIcon` for all three dynamic platform collections and contains no `PlatformIcon` import.

- [ ] **Step 2: Run the source-component test and verify the red state**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected: FAIL because Admin still imports and dynamically renders `PlatformIcon`.

- [ ] **Step 3: Replace all three Admin dynamic collections**

Use `AccountDestinationIcon` at the existing 14px sizes in the user table, detail panel, and scheduled-post drawer. Preserve current wrappers, titles, counts, and status colors; only the YouTube visual changes because non-YouTube values delegate to `PlatformIcon`.

- [ ] **Step 4: Run the source-component test**

Run: `cd dashboard && node --test tests/youtube-source-components-source.test.mjs`

Expected: PASS, 0 failures.

- [ ] **Step 5: Commit Admin migration**

```bash
git add dashboard/src/app/admin/users/page.tsx dashboard/tests/youtube-source-components-source.test.mjs
git commit -m "fix: neutralize YouTube admin status icons"
```

## Task 8: Enforce Dashboard-wide default deny and record public risk

**Files:**
- Modify: `dashboard/tests/youtube-compliance-ui-source.test.mjs`
- Modify: `dashboard/package.json`
- Modify: `.github/workflows/ci.yml`
- Modify: `docs/compliance/youtube-brand-asset.md`

- [ ] **Step 1: Inventory the public legacy uses**

Record these visually unchanged public consumers in the compliance document, with exact paths and the note “not remediated or certified by this authenticated-Dashboard task”:

```text
dashboard/src/app/marketing/page.tsx
dashboard/src/app/about/page.tsx
dashboard/src/app/blog/_components/blog-cover.tsx
dashboard/src/app/tools/_components/public-analytics-tool.tsx
dashboard/src/components/tools/ToolCard.tsx
dashboard/src/components/platform-icons.tsx
```

- [ ] **Step 2: Write the failing Dashboard-wide guard**

Expand `youtube-compliance-ui-source.test.mjs` to recursively read `.ts` and `.tsx` under `src/app/(dashboard)`, `src/app/admin`, and `src/components`. Add these exact policies:

1. `yt_icon_rgb.svg` is referenced only by `youtube-source-link.tsx`.
2. Outside `youtube-source-link.tsx` and the explicit legacy definition `platform-icons.tsx`, reject `#ff0000`, `#f00`, `rgb(255, 0, 0)`, the current official path prefix `M23.498 6.186`, and the approved asset filename.
3. Under `src/app/(dashboard)` and `src/app/admin`, reject literal `<PlatformIcon platform="youtube"` and every `PlatformIcon` element whose `platform` prop is a JSX expression.
4. Under shared components, reject literal YouTube uses. Allow dynamic `PlatformIcon` only in `account-destination-icon.tsx`, `meta-platform-analytics-view.tsx`, and the documented public `ToolCard.tsx`; assert the first has an early YouTube neutral branch and the latter two cannot introduce an authenticated YouTube path.
5. Hash every repository SVG and fail if another file duplicates the approved asset bytes.
6. Assert `YouTubeSourceLink` contains the official asset, URL normalization, a real anchor, permitted disclosure union, `target="_blank"`, `rel="noopener noreferrer"`, accessible label, and 44px target styling.
7. Assert Analytics, Inbox, Admin, Accounts, managed-user detail, Create Post, Calendar, Connection Stats, and aggregate badges follow their assigned neutral/identity treatment.

Run the guard before migration completion to confirm it detects at least one current legacy use, then finish the remaining migration and require green.

- [ ] **Step 3: Add a focused package script and CI gate**

Add:

```json
"test:youtube-compliance": "node --test src/lib/youtube-source.test.ts tests/youtube-compliance-ui-source.test.mjs tests/youtube-source-components-source.test.mjs tests/youtube-published-result-source.test.mjs tests/youtube-analytics-dashboard-source.test.mjs"
```

Add this CI step immediately before `Build dashboard`:

```yaml
      - name: Run YouTube UI compliance contracts
        run: npm run test:youtube-compliance
```

- [ ] **Step 4: Run the complete focused compliance suite**

Run: `cd dashboard && npm run test:youtube-compliance`

Expected: PASS, all files and tests, 0 failures.

- [ ] **Step 5: Commit guardrails and evidence inventory**

```bash
git add dashboard/tests/youtube-compliance-ui-source.test.mjs dashboard/package.json .github/workflows/ci.yml docs/compliance/youtube-brand-asset.md
git commit -m "test: enforce YouTube attribution boundaries"
```

## Task 9: Add fixture-backed browser acceptance

**Files:**
- Modify: `dashboard/tests/regression/authenticated-dashboard.spec.ts`

- [ ] **Step 1: Add a fixture-backed authenticated test**

Reuse `createSyntheticClerkUser`, `signInSyntheticUser`, `bootstrapSyntheticUser`, and cleanup. After bootstrap and before navigating to Accounts, use `page.route` to fulfill the profile's accounts endpoint with an active YouTube account:

```ts
{
  id: "youtube-source-fixture",
  profile_id: profileID,
  platform: "youtube",
  account_name: "UniPost YouTube Fixture",
  external_account_id: "UCfixture123",
  account_avatar_url: "data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='64' height='64'%3E%3Crect width='64' height='64' fill='%23111827'/%3E%3Ctext x='32' y='39' text-anchor='middle' fill='white' font-size='20'%3EUF%3C/text%3E%3C/svg%3E",
  connected_at: "2026-07-29T00:00:00Z",
  status: "active",
  connection_type: "managed",
  scope: ["youtube.readonly", "yt-analytics.readonly"]
}
```

Fulfill in the API response envelope used by `request()`. Keep all other requests on the isolated Preview/Railway environment.

- [ ] **Step 2: Assert hierarchy, link semantics, and prominence**

On the Accounts page:

```ts
const sourceLink = page.locator("[data-youtube-source-link]");
await expect(sourceLink).toBeVisible();
await expect(sourceLink).toHaveAttribute("href", "https://www.youtube.com/channel/UCfixture123");
await expect(sourceLink).toHaveAttribute("target", "_blank");
await expect(sourceLink).toHaveAttribute("rel", "noopener noreferrer");
await expect(page.getByText("Source: YouTube", { exact: true })).toBeVisible();
await expect(page.getByText("Active", { exact: true })).toBeVisible();
```

Measure `[data-youtube-source-mark]` and `[data-youtube-channel-avatar]`; require the artwork width to be 24px, the click target to be at least 44px in both dimensions, and the avatar to be larger than the artwork. Focus the link and require a visible nonzero outline. Capture a light screenshot.

- [ ] **Step 3: Assert dark and mobile behavior**

Set `unipost-theme` to `dark`, reload, and repeat visibility/link checks. Set the viewport to 390x844, require no horizontal document overflow, require the link target to remain at least 44x44, and capture dark/mobile screenshots in the Playwright output.

- [ ] **Step 4: Run authenticated browser acceptance locally when credentials are present**

Run:

```bash
cd dashboard
DASHBOARD_BASE_URL=https://dev-app.unipost.dev \
DASHBOARD_TEST_CLERK_SECRET_KEY="$DASHBOARD_TEST_CLERK_SECRET_KEY" \
DASHBOARD_TEST_CLERK_PUBLISHABLE_KEY="$DASHBOARD_TEST_CLERK_PUBLISHABLE_KEY" \
npm run test:regression:dashboard:authenticated
```

Expected: PASS, including the YouTube source fixture. If required credentials are unavailable or any test cannot start, treat it as failed and stop before push/merge until the required environment supplies the check.

- [ ] **Step 5: Commit browser acceptance**

```bash
git add dashboard/tests/regression/authenticated-dashboard.spec.ts
git commit -m "test: cover YouTube source hierarchy"
```

## Task 10: Run complete local verification

**Files:**
- Verify all changed Dashboard and documentation files.

- [ ] **Step 1: Verify worktree and branch ownership**

Run: `pwd -P && git branch --show-current && git status --short --branch`

Expected: the exclusive worktree path and branch `dev-youtube-icon-source-system`; no unrelated changes.

- [ ] **Step 2: Run focused compliance tests**

Run: `cd dashboard && npm run test:youtube-compliance`

Expected: PASS, 0 failures.

- [ ] **Step 3: Run the existing Dashboard source suites affected by Analytics**

Run: `cd dashboard && npm run test:docs-ai`

Expected: PASS, 0 failures.

- [ ] **Step 4: Build the Dashboard**

Run: `cd dashboard && npm run build`

Expected: exit 0 with a successful Next.js production build.

- [ ] **Step 5: Run required public Dashboard regression**

Run: `cd dashboard && npm run test:regression:dashboard`

Expected: PASS, 0 failed/skipped/cancelled tests.

- [ ] **Step 6: Run required authenticated Dashboard regression**

Run the credentialed command from Task 9 against the applicable local/dev target.

Expected: PASS, 0 failed/skipped/cancelled tests.

- [ ] **Step 7: Audit the exact branch contents**

Run:

```bash
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
git diff --check origin/dev...HEAD
```

Expected: only the approved PRD/plan, YouTube source-system implementation, focused tests, asset provenance, package script, and CI gate. Any unrelated file is a hard blocker.

## Task 11: Draft PR, Preview Acceptance, and merge to dev only

**Files:**
- No new source files unless a failed required check reveals an in-scope defect.

- [ ] **Step 1: Push only the owned task branch**

After re-verifying worktree and branch, run:

```bash
git push -u origin dev-youtube-icon-source-system
```

Expected: successful push of the task branch; no direct update to `dev`, `staging`, or `main`.

- [ ] **Step 2: Open a Draft PR to dev**

```bash
gh pr create \
  --draft \
  --base dev \
  --head dev-youtube-icon-source-system \
  --title "feat: add compliant YouTube icon source system" \
  --body-file docs/superpowers/specs/2026-07-29-youtube-icon-source-system-design.md
```

Expected: Draft PR whose head SHA exactly matches the pushed branch.

- [ ] **Step 3: Monitor every triggered check to a terminal result**

Use `gh pr checks --watch` plus the GitHub run/deployment links to monitor GitHub CI, Vercel Preview, Railway PR Environment, deployed preview regression, and every visibly triggered check. A failure, timeout, cancellation, skip, inability to start, or SHA mismatch is a hard stop; record the workflow, job, suite/test, exact error, log excerpt, run URL, artifact URLs, and deployment state before fixing the owned source branch.

- [ ] **Step 4: Perform Codex Preview browser acceptance on the exact head SHA**

Open the Vercel Preview wired to the Railway PR API. Verify:

- Accounts identity: avatar primary, official linked mark secondary, visible `Source: YouTube`, separate `UniPost status`.
- Managed-user detail: same identity/status separation.
- Analytics: neutral page/navigation/status glyphs, linked channel identity, valid-result `View on YouTube` only.
- Inbox: neutral YouTube conversation glyph, `Source: YouTube`, no official mark in unread/status/Resolve clusters.
- Admin: neutral YouTube platform summaries.
- Create Post, Calendar, Connection Stats, capability summaries, counts: neutral glyph only.
- Official links: exact allowed YouTube host, new tab, noopener+noreferrer, keyboard focus, 44px target.
- Light, dark, desktop, 390px mobile; no overflow; official artwork subordinate to avatar/content/title.

Capture redacted screenshots for the evidence package. Do not persist private account/channel data without approval.

- [ ] **Step 5: Mark ready and merge only after all Preview gates pass**

First rerun the promotion content audit (`git log` and `git diff --name-status` against current `origin/dev`). Then:

```bash
gh pr ready
gh pr merge --merge
```

Expected: the PR merges into `dev`. Do not create a staging or production PR.

- [ ] **Step 6: Monitor the persistent development deployment**

Wait for every check and deployment triggered by the `dev` merge, including Vercel `unipost-dev` and Railway `dev`. Confirm each successful result applies to the merge SHA. Any non-success result is a hard stop and must be reported with the required evidence.

- [ ] **Step 7: Verify the real dev environment**

Open `https://dev-app.unipost.dev` and repeat the changed critical flows against `https://dev-api.unipost.dev`: Accounts, managed-user detail, Analytics, Inbox, Admin where authorized, Create Post, Calendar, result links, light/dark, keyboard, and mobile. Confirm the official asset/link contract and absence of legacy red YouTube icons on authenticated operational surfaces.

- [ ] **Step 8: Report dev completion without promoting further**

Report the merged dev SHA, PR URL, CI/deployment URLs, Preview and dev acceptance evidence, exact test results, and remaining public-branding follow-up risk. State explicitly that no staging or production promotion occurred.
