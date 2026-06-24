import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin users API row allows scheduled post counts to be absent during API rollout", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminUserRow/);
  assert.match(api, /scheduled_posts\?: number;/);
});

test("admin users table shows Scheduled before Posts Used", () => {
  const page = source("src/app/admin/users/page.tsx");
  const scheduledHeader = page.indexOf("<th>Scheduled</th>");
  const postsUsedHeader = page.indexOf("<th>Posts Used</th>");

  assert.ok(scheduledHeader > -1, "Scheduled header should be present");
  assert.ok(postsUsedHeader > -1, "Posts Used header should be present");
  assert.ok(scheduledHeader < postsUsedHeader, "Scheduled should appear before Posts Used");
  assert.match(page, /fmtNumber\(u\.scheduled_posts \?\? 0\)/);
  assert.match(page, /colSpan=\{12\}/);
});
