import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("posts API helper exposes paginated and all-page list calls", async () => {
  const api = await source("src/lib/api.ts");

  assert.match(api, /interface\s+ListSocialPostsParams/);
  assert.match(api, /function\s+socialPostsQuery/);
  assert.match(api, /export\s+async\s+function\s+listSocialPosts\(\s*token:\s*string,\s*params\?:\s*ListSocialPostsParams/);
  assert.match(api, /export\s+async\s+function\s+listAllSocialPosts/);
  assert.match(api, /listSocialPosts\(token,\s*\{[\s\S]*?\.\.\.params[\s\S]*?limit:\s*POSTS_LIST_PAGE_SIZE[\s\S]*?cursor[\s\S]*?\}\)/);
  assert.match(api, /cursor\s*=\s*page\.meta\?\.next_cursor/);
});

test("dashboard post surfaces load all paginated posts before filtering", async () => {
  const calendar = await source("src/components/posts/calendar/posts-calendar-view.tsx");
  const list = await source("src/components/posts/list/posts-legacy-list-view.tsx");
  const analytics = await source("src/app/(dashboard)/projects/[id]/analytics/page.tsx");

  assert.match(calendar, /listAllSocialPosts/);
  assert.doesNotMatch(calendar, /\[\s*listSocialPosts\(token\)/);

  assert.match(list, /listAllSocialPosts/);
  assert.doesNotMatch(list, /\[\s*listSocialPosts\(token\)/);

  assert.match(analytics, /listAllSocialPosts/);
  assert.doesNotMatch(analytics, /listSocialPosts\(token\)/);
});

test("analytics platform filtering includes scheduled posts without result rows", async () => {
  const analytics = await source("src/app/(dashboard)/projects/[id]/analytics/page.tsx");

  assert.match(analytics, /postMatchesPlatform/);
  assert.match(analytics, /post\.target_platforms\?\./);
  assert.match(analytics, /post\.results\?\./);
  assert.match(analytics, /postMatchesPlatform\(p,\s*platformFilter\)/);
});
