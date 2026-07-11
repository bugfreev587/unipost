import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const postsPagePath = path.join(root, "src/app/(dashboard)/projects/[id]/posts/page.tsx");
const postsListPagePath = path.join(root, "src/app/(dashboard)/projects/[id]/posts/list/page.tsx");
const legacyViewPath = path.join(root, "src/components/posts/list/posts-legacy-list-view.tsx");
const platformResultsPath = path.join(root, "src/components/posts/details/post-platform-results.tsx");

test("Posts routes use calendar by default and keep the legacy list route", async () => {
  const [postsPage, postsListPage, legacyView] = await Promise.all([
    readFile(postsPagePath, "utf8"),
    readFile(postsListPagePath, "utf8"),
    readFile(legacyViewPath, "utf8"),
  ]);

  assert.match(postsPage, /PostsCalendarView/);
  assert.doesNotMatch(postsPage, /useFeatureFlags|FEATURE_FLAG_KEYS|PostsLegacyListView/);
  assert.doesNotMatch(postsListPage, /router\.replace|useFeatureFlags|FEATURE_FLAG_KEYS/);
  assert.match(postsListPage, /PostsLegacyListView showCalendarLink/);
  assert.match(legacyView, /showCalendarLink/);
  assert.match(legacyView, /Calendar View/);
  assert.match(legacyView, /posts-view-switch/);
  assert.match(legacyView, /searchParams\.get\("post"\)/);
  assert.match(legacyView, /setPendingExpandedPostId\(focusPostId\)/);
});

test("Legacy posts list keeps management workflows after route split", async () => {
  const [legacyView, platformResults] = await Promise.all([
    readFile(legacyViewPath, "utf8"),
    readFile(platformResultsPath, "utf8"),
  ]);

  assert.match(legacyView, /\(\["all", "published", "scheduled", "failed", "draft", "archived"\] as FilterTab\[\]\)/);
  assert.match(legacyView, /placeholder="Search posts\.\.\."/);
  assert.match(legacyView, /<option value="all">All platforms<\/option>/);
  assert.match(legacyView, /checked=\{filtered\.length > 0 && filtered\.every\(\(post\) => selectedPostIds\.has\(post\.id\)\)\}/);
  assert.match(legacyView, /requestArchive\(\[...selectedPostIds\]\)/);
  assert.match(legacyView, /requestRestore\(\[...selectedPostIds\]\)/);
  assert.match(legacyView, /requestDelete\(\[...selectedPostIds\]\)/);
  assert.match(legacyView, /archiveSocialPost\(token, id\)/);
  assert.match(legacyView, /restoreSocialPost\(token, id\)/);
  assert.match(legacyView, /deleteSocialPost\(token, id\)/);
  assert.match(legacyView, /setExpandedPostId\(\(current\) => current === post\.id \? null : post\.id\)/);
  assert.match(legacyView, /Platform Results/);
  assert.match(legacyView, /PostPlatformResults/);
  assert.match(platformResults, /retrySocialPostResult\(token, post\.id, result\.id\)/);
  assert.match(legacyView, /onRetryComplete=\{async \(\) => \{\s*await loadData\(\);\s*\}\}/);
  assert.match(legacyView, /openRescheduleDialog\(post\)/);
  assert.match(legacyView, /rescheduleSocialPost\(token, reschedulePost\.id, nextTime\.toISOString\(\)\)/);
  assert.match(legacyView, /<CreatePostDrawer/);
  assert.match(legacyView, /onCreated=\{async \(postId\) => \{/);
  assert.match(legacyView, /setPendingExpandedPostId\(postId\)/);
  assert.match(legacyView, /searchParams\.get\("action"\) === "new"/);
  assert.match(legacyView, /searchParams\.get\("template"\) === "welcome"/);
  assert.match(legacyView, /readStoredReplay\(\)\?\.selectedAccountId/);
  assert.match(legacyView, /consumeStoredQuickstartSelectedAccountId\(\)/);
});
