import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const postsPagePath = path.join(root, "src/app/(dashboard)/projects/[id]/posts/page.tsx");
const postsListPagePath = path.join(root, "src/app/(dashboard)/projects/[id]/posts/list/page.tsx");
const legacyViewPath = path.join(root, "src/components/posts/list/posts-legacy-list-view.tsx");
const featureFlagsPath = path.join(root, "src/lib/feature-flags.ts");

test("Posts routes are feature-flagged between calendar and legacy list", async () => {
  const [postsPage, postsListPage, legacyView, featureFlags] = await Promise.all([
    readFile(postsPagePath, "utf8"),
    readFile(postsListPagePath, "utf8"),
    readFile(legacyViewPath, "utf8"),
    readFile(featureFlagsPath, "utf8"),
  ]);

  assert.match(featureFlags, /postsCalendarViewV1:\s*"posts\.calendar_view_v1"/);
  assert.match(postsPage, /useFeatureFlags/);
  assert.match(postsPage, /FEATURE_FLAG_KEYS\.postsCalendarViewV1/);
  assert.match(postsPage, /PostsCalendarView/);
  assert.match(postsPage, /PostsLegacyListView/);
  assert.match(postsListPage, /router\.replace\(`\/projects\/\$\{params\.id\}\/posts`\)/);
  assert.match(postsListPage, /PostsLegacyListView showCalendarLink/);
  assert.match(legacyView, /showCalendarLink/);
  assert.match(legacyView, /Calendar View/);
  assert.match(legacyView, /posts-view-switch/);
  assert.match(legacyView, /searchParams\.get\("post"\)/);
  assert.match(legacyView, /setPendingExpandedPostId\(focusPostId\)/);
});
