import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("workspace restriction client and result retention contract are typed", () => {
  const api = read("src/lib/api.ts");

  assert.match(api, /export\s+async\s+function\s+getPublishingRestrictions/);
  assert.match(api, /\/v1\/publishing-restrictions/);
  assert.match(api, /media_retained_until\?:\s*string/);
});

test("shared Composer refreshes restrictions on open and focus", () => {
  const drawer = read("src/components/posts/create-post/create-post-drawer.tsx");

  assert.match(drawer, /getPublishingRestrictions/);
  assert.match(drawer, /window\.addEventListener\("focus"/);
  assert.match(drawer, /window\.removeEventListener\("focus"/);
  assert.match(drawer, /const \[restrictionsLoaded, setRestrictionsLoaded\] = useState\(false\)/);
  assert.match(drawer, /if \(!restrictionsLoaded\) return/);
  assert.match(drawer, /finally\s*\{[\s\S]*setRestrictionsLoaded\(true\)/);
  assert.match(drawer, /filterAllowedAccountIds/);
  assert.match(drawer, /PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE/);
  assert.match(drawer, /isPlanPlatformPublishingRestrictedError/);
});

test("Composer disables restricted accounts with an accessible persistent notice", () => {
  const grid = read("src/components/posts/create-post/account-card-grid.tsx");

  assert.match(grid, /disabledIds/);
  assert.match(grid, /const isDisabled = disabled \|\| !!selectedSiblingId/);
  assert.match(grid, /disabled=\{isDisabled\}/);
  assert.match(grid, /aria-describedby/);
  assert.match(grid, /restrictionNoticeId/);

  const unusedLargeCard = read("src/components/posts/create-post/account-card.tsx");
  assert.doesNotMatch(unusedLargeCard, /describedBy/);
  assert.doesNotMatch(unusedLargeCard, /disabled\?:/);
});

test("shared Posts and Calendar result cards show restriction deadline and Retry states", () => {
  const results = read("src/components/posts/details/post-platform-results.tsx");

  assert.match(results, /media_retained_until/);
  assert.match(results, /describePublishingRestrictionRetry/);
  assert.match(results, /Media retained until/);
  assert.match(results, /publishingRestrictionRetry\s*!==\s*null\s*&&\s*publishingRestrictionRetry\?\.state\s*!==\s*"ready"/);
});
