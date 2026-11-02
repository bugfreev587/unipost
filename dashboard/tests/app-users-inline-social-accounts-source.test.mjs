import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

// The App Users list reported the correct account_count while rendering an
// incomplete set of platform icons: the aggregate, the API type, and the badge
// renderer all hard-coded twitter/linkedin/bluesky/youtube. These tests pin the
// fixed contract — complete platform coverage plus inline expansion — across
// the list page, the deep-link page, and the shared account component.

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const listPagePath = path.join(root, "src/app/(dashboard)/projects/[id]/users/page.tsx");
const detailPagePath = path.join(
  root,
  "src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx"
);
const accountsComponentPath = path.join(
  root,
  "src/components/managed-users/managed-user-accounts.tsx"
);
const platformsLibPath = path.join(root, "src/lib/managed-user-platforms.ts");
const apiPath = path.join(root, "src/lib/api.ts");

const SUPPORTED_PLATFORMS = [
  "twitter",
  "linkedin",
  "bluesky",
  "youtube",
  "tiktok",
  "instagram",
  "threads",
  "facebook",
  "pinterest",
];

test("supported Connect platform list covers all nine platforms in badge order", async () => {
  const source = await readFile(platformsLibPath, "utf8");
  const listBlock = source.slice(
    source.indexOf("export const MANAGED_USER_PLATFORMS"),
    source.indexOf("const PLATFORM_LABELS")
  );

  for (const platform of SUPPORTED_PLATFORMS) {
    assert.match(
      listBlock,
      new RegExp(`"${platform}"`),
      `MANAGED_USER_PLATFORMS is missing ${platform}`
    );
  }

  const order = [...listBlock.matchAll(/"([a-z]+)"/g)].map((m) => m[1]);
  assert.deepEqual(order, SUPPORTED_PLATFORMS);

  // Every platform needs a human-readable label for the badge tooltip and the
  // account card's platform line.
  for (const platform of SUPPORTED_PLATFORMS) {
    assert.match(
      source,
      new RegExp(`${platform}:\\s*"`),
      `PLATFORM_LABELS is missing ${platform}`
    );
  }
  // X is the product name for the twitter platform key.
  assert.match(source, /twitter:\s*"X"/);
});

test("ManagedUserListEntry types platform_counts from the canonical Connect platform set", async () => {
  const source = await readFile(apiPath, "utf8");
  const entry = source.slice(
    source.indexOf("export interface ManagedUserListEntry"),
    source.indexOf("export interface ManagedUserDetail")
  );

  assert.match(entry, /platform_counts:\s*Record<ConnectSessionPlatform,\s*number>/);
  // The four-platform literal shape is exactly what let TikTok disappear.
  assert.doesNotMatch(entry, /platform_counts:\s*\{/);
});

test("App Users list renders a badge for every supported platform with a non-zero count", async () => {
  const source = await readFile(listPagePath, "utf8");

  assert.match(source, /MANAGED_USER_PLATFORMS/);
  assert.match(source, /\(u\.platform_counts\?\.\[platform\] \?\? 0\) > 0/);
  assert.match(source, /count=\{u\.platform_counts\[platform\]\}/);

  // No per-platform conditional may remain: that shape is what has to be
  // updated in nine places when a platform is added, and was updated in four.
  for (const platform of SUPPORTED_PLATFORMS) {
    assert.doesNotMatch(
      source,
      new RegExp(`platform_counts\\.${platform}`),
      `list page still hard-codes a ${platform} badge`
    );
  }
});

test("App Users rows start collapsed, expand independently, and expose accessible state", async () => {
  const source = await readFile(listPagePath, "utf8");

  // Independent expansion state keyed by external_user_id, empty on first render.
  assert.match(source, /useState<Set<string>>\(new Set\(\)\)/);
  assert.match(source, /const isExpanded = expanded\.has\(u\.external_user_id\)/);
  assert.match(source, /next\.delete\(externalUserId\)/);
  assert.match(source, /next\.add\(externalUserId\)/);

  // Accessibility contract for the disclosure control.
  assert.match(source, /aria-expanded=\{isExpanded\}/);
  assert.match(source, /aria-controls=\{panelId\}/);
  assert.match(source, /id=\{panelId\}/);
  assert.match(source, /aria-label=\{`\$\{isExpanded \? "Collapse" : "Expand"\}/);

  // Expansion must not navigate: the row-level detail link moved into the
  // expanded area as `Open full detail`.
  assert.match(source, /Open full detail/);
  assert.doesNotMatch(source, />\s*Detail <ArrowRight/);
});

test("App Users expansion lazy-loads detail, caches it, and never caches a failure", async () => {
  const source = await readFile(listPagePath, "utf8");

  assert.match(source, /getManagedUser\(token, profileId, externalUserId\)/);
  // Cache hit: a row already loaded must not refetch when reopened.
  assert.match(source, /cached\?\.status !== "ready"/);
  assert.match(source, /!expanded\.has\(externalUserId\) && cached\?\.status !== "ready"/);
  // A failure is stored as an error state, so Retry issues a fresh request.
  assert.match(source, /\[externalUserId\]:\s*\{\s*status:\s*"error"/);
  assert.match(source, /onRetry=\{\(\) => loadDetail\(u\.external_user_id\)\}/);

  // Only the list request fires on page load.
  const effectBlock = source.slice(source.indexOf("useEffect(() => {"));
  assert.match(effectBlock.slice(0, 120), /load\(\);/);
  assert.doesNotMatch(effectBlock.slice(0, 120), /loadDetail/);
});

test("App Users mutations refresh both the expanded detail and the list aggregates", async () => {
  const source = await readFile(listPagePath, "utf8");

  assert.match(
    source,
    /await Promise\.all\(\[loadDetail\(externalUserId\), load\(\)\]\)/
  );
  assert.match(source, /onMutated=\{\(\) => refreshAfterMutation\(u\.external_user_id\)\}/);
});

test("nested App Users actions do not toggle the row", async () => {
  const source = await readFile(listPagePath, "utf8");

  // Dismiss, the chevron, and the expanded panel all stop propagation so the
  // row's click handler does not also fire.
  const stopPropagationCount = (source.match(/stopPropagation\(\)/g) || []).length;
  assert.ok(
    stopPropagationCount >= 3,
    `expected nested actions to stop propagation, found ${stopPropagationCount}`
  );
  assert.match(source, /onClick=\{\(e\) => e\.stopPropagation\(\)\}/);
});

test("expanded App Users row keeps a neutral surface rather than a selected-state fill", async () => {
  const source = await readFile(listPagePath, "utf8");

  const panelStart = source.indexOf("id={panelId}");
  const panel = source.slice(panelStart, source.indexOf("</div>", panelStart));

  // Disclosure is communicated by the chevron and a divider, not a fill.
  assert.match(panel, /border-t border-dashed/);
  assert.match(source, /isExpanded \? "rotate-90" : ""/);

  // No accent/primary/selected background may be applied when expanded.
  assert.doesNotMatch(source, /isExpanded[^\n]*bg-\[var\(--(primary|accent|success|info)/);
  assert.doesNotMatch(source, /data-expanded[^\n]*bg-\[var\(--(primary|accent)/);
  assert.doesNotMatch(panel, /background:\s*"var\(--(primary|accent|success|info)/);
});

test("shared account component renders identity, both timestamps, connection type, status, and actions", async () => {
  const source = await readFile(accountsComponentPath, "utf8");

  // Reuses the shared identity rule instead of re-deriving labels.
  assert.match(source, /accountIdentityLabels\(account\)/);
  assert.match(source, /identity\.handle/);
  assert.match(source, /TIKTOK_IDENTITY_RECONNECT_GUIDANCE/);
  assert.match(source, /identity\.identityRefreshRequired/);
  assert.match(source, /YouTubeChannelIdentity/);
  assert.match(source, /AccountDestinationIcon/);
  // Legacy state comes from the server flag, never from a username/display-name
  // comparison.
  assert.doesNotMatch(source, /username\s*===\s*.*display_name/);

  // connected_at and last_connected_at are distinct events, both labeled, and a
  // missing last_connected_at is never backfilled from last_refreshed_at.
  assert.match(source, /label="First connected"[\s\S]{0,80}account\.connected_at/);
  assert.match(source, /label="Last connected"[\s\S]{0,80}account\.last_connected_at/);
  assert.match(source, /"Not recorded"/);
  assert.doesNotMatch(source, /account\.last_refreshed_at/);

  assert.match(source, /connectionTypeLabel\(account\.connection_type\)/);
  assert.match(source, /platformDisplayName\(account\.platform\)/);

  // Existing actions and confirmations.
  assert.match(source, /disconnectSocialAccount\(token, profileId, accountId\)/);
  assert.match(source, /dismissSocialAccount\(token, profileId, accountId\)/);
  assert.match(source, /<ConfirmModal/);
});

test("shared account component reports action failures to the user, not only the console", async () => {
  const source = await readFile(accountsComponentPath, "utf8");

  assert.match(source, /setActionError\(/);
  assert.match(source, /data-managed-user-action-error/);
  assert.match(source, /role="alert"/);
  assert.doesNotMatch(source, /console\.error\("Disconnect failed/);
  assert.doesNotMatch(source, /console\.error\("Dismiss failed/);
});

test("shared account component owns scoped loading, empty, and error states with Retry", async () => {
  const source = await readFile(accountsComponentPath, "utf8");

  assert.match(source, /status === "loading" \? \(\s*<AccountsSkeleton/);
  assert.match(source, /No managed accounts found for this App User\./);
  assert.match(source, /onRetry \? \(/);
  assert.match(source, /Retry/);
  assert.match(source, /aria-busy="true"/);

  // The component presents accounts; fetching and authorization stay with the
  // page and the API.
  assert.doesNotMatch(source, /listManagedUsers/);
  assert.doesNotMatch(source, /getManagedUser/);
});

test("deep-link detail route stays available and renders the shared component", async () => {
  const source = await readFile(detailPagePath, "utf8");

  // Still fetches independently of the list page's in-memory cache.
  assert.match(source, /getManagedUser\(token, profileId, externalUserID\)/);
  assert.match(source, /<ManagedUserAccounts/);
  assert.match(source, /onRetry=\{retry\}/);
  assert.match(source, /onMutated=\{load\}/);
  assert.match(source, /Back to users/);

  // The duplicated account-card markup is gone, so the two entry points cannot
  // drift in identity fields, timestamps, or actions.
  assert.doesNotMatch(source, /accountIdentityLabels/);
  assert.doesNotMatch(source, /AccountDestinationIcon/);
  assert.doesNotMatch(source, /disconnectSocialAccount/);
  assert.doesNotMatch(source, /dismissSocialAccount/);
});

test("App Users content reflows at narrow widths instead of clipping", async () => {
  const listSource = await readFile(listPagePath, "utf8");
  const accountsSource = await readFile(accountsComponentPath, "utf8");

  // Summary cells stack below `md` and the header labels hide with them.
  assert.match(listSource, /grid grid-cols-1 \$\{ROW_GRID\}/);
  assert.match(listSource, /hidden md:grid \$\{ROW_GRID\}/);
  // The strict desktop table would have needed horizontal scrolling for the
  // expanded account content.
  assert.doesNotMatch(listSource, /<table/);
  assert.doesNotMatch(listSource, /overflow-x-auto/);

  assert.match(accountsSource, /flex-wrap/);
  assert.match(accountsSource, /grid-cols-1 gap-x-6 gap-y-2[^\n]*sm:grid-cols-3/);
  assert.match(accountsSource, /break-words/);
});
