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

test("admin users API exposes scheduled post drawer endpoint", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminUserScheduledPost/);
  assert.match(api, /post_id: string;/);
  assert.match(api, /title: string;/);
  assert.match(api, /created_at: string;/);
  assert.match(api, /scheduled_at: string \| null;/);
  assert.match(api, /platforms: string\[\];/);
  assert.match(api, /export async function getAdminUserScheduledPosts/);
  assert.match(api, /\/v1\/admin\/users\/\$\{id\}\/scheduled-posts/);
});

test("admin users API row exposes failed posts this month", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminUserRow/);
  assert.match(api, /failed_posts_this_month: number;/);
});

test("admin users table shows Scheduled before Posts Used", () => {
  const page = source("src/app/admin/users/page.tsx");
  const scheduledHeader = page.indexOf("<th>Scheduled</th>");
  const postsUsedHeader = page.indexOf("<th>Posts Used</th>");

  assert.ok(scheduledHeader > -1, "Scheduled header should be present");
  assert.ok(postsUsedHeader > -1, "Posts Used header should be present");
  assert.ok(scheduledHeader < postsUsedHeader, "Scheduled should appear before Posts Used");
  assert.match(page, /fmtNumber\(u\.scheduled_posts \?\? 0\)/);
  assert.match(page, /colSpan=\{13\}/);
});

test("admin users table links failed counts and only View opens detail", () => {
  const page = source("src/app/admin/users/page.tsx");
  const scheduledHeader = page.indexOf("<th>Scheduled</th>");
  const failedHeader = page.indexOf("<th>Failed</th>");
  const postsUsedHeader = page.indexOf("<th>Posts Used</th>");

  assert.ok(failedHeader > scheduledHeader, "Failed should appear after Scheduled");
  assert.ok(failedHeader < postsUsedHeader, "Failed should appear before Posts Used");
  assert.match(page, /failed_posts_this_month/);
  assert.match(page, /adminUserFailedPostsHref\(u\.id\)/);
  assert.match(page, /period=this_month/);
  assert.match(page, /ad-tbl-wrap ad-tbl-static/);
  assert.doesNotMatch(page, /<tr key=\{u\.id\} onClick=/);
  assert.match(page, /colSpan=\{13\}/);
});

test("admin users detail panel keeps usable height for single-row result sets", () => {
  const page = source("src/app/admin/users/page.tsx");

  assert.match(
    page,
    /className=\{`ad-tbl-wrap ad-tbl-static au-users-table-wrap \$\{selectedUserId \? "au-users-table-wrap-detail-open" : ""\}`\}/,
  );
  assert.match(page, /\.au-users-table-wrap\s*\{\s*position: relative;/);
  assert.match(page, /\.au-users-table-wrap-detail-open\s*\{\s*min-height: clamp\(420px, calc\(100dvh - 260px\), 640px\);/);
  assert.match(page, /@media \(max-width: 860px\) \{[\s\S]*\.au-users-table-wrap-detail-open\s*\{\s*min-height: 0;/);
});

test("admin users scheduled counts open a scheduled-posts drawer", () => {
  const page = source("src/app/admin/users/page.tsx");

  assert.match(page, /getAdminUserScheduledPosts/);
  assert.match(page, /type AdminUserScheduledPost/);
  assert.match(page, /scheduledDrawerUser/);
  assert.match(page, /scheduledDrawerLoading/);
  assert.match(page, /scheduledDrawerError/);
  assert.match(page, /function openScheduledPosts\(u: AdminUserRow\)/);
  assert.match(page, /openScheduledPosts\(u\)/);
  assert.match(page, /className="[^"]*au-scheduled-link/);
  assert.match(page, /className="au-scheduled-drawer"/);
  assert.match(page, /scheduledPosts\.length === 0/);
  assert.match(page, /post\.platforms\.map/);
  assert.match(page, /PlatformIcon key=\{platform\} platform=\{platform\}/);
});

test("admin users API exposes quota reset endpoints", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminUserQuotaResetResult/);
  assert.match(api, /quota_kind: "post" \| "scheduled";/);
  assert.match(api, /affected_workspaces: number;/);
  assert.match(api, /previous_usage: number;/);
  assert.match(api, /export async function resetAdminUserPostQuota/);
  assert.match(api, /export async function resetAdminUserScheduledQuota/);
  assert.match(api, /\/v1\/admin\/users\/\$\{id\}\/quota\/post\/reset/);
  assert.match(api, /\/v1\/admin\/users\/\$\{id\}\/quota\/scheduled\/reset/);
});

test("admin users detail exposes post quota reset actions", () => {
  const page = source("src/app/admin/users/page.tsx");

  assert.match(page, /resetAdminUserPostQuota/);
  assert.match(page, /resetAdminUserScheduledQuota/);
  assert.match(page, /Posts quota reset/);
  assert.match(page, /Reset schedule quota/);
  assert.match(page, /Reset post quota/);
  assert.match(page, /quotaResetPending/);
  assert.match(page, /quotaResetMessage/);
  assert.match(page, /handleQuotaReset\("scheduled"\)/);
  assert.match(page, /handleQuotaReset\("post"\)/);
});

test("admin users page exposes active users filter and filtered total copy", () => {
  const page = source("src/app/admin/users/page.tsx");
  const api = source("src/lib/api.ts");

  assert.match(api, /activity\?: "all" \| "active";/);
  assert.match(api, /params\?\.activity && params\.activity !== "all"/);
  assert.match(api, /qs\.set\("activity", params\.activity\)/);
  assert.match(page, /const \[activity, setActivity\]/);
  assert.match(page, /listAdminUsers\(token, \{ search, plan, activity, sort, limit, offset \}\)/);
  assert.match(page, /<option value="active">Active Users<\/option>/);
  assert.match(page, /activity === "active" \? "active users" : "users"/);
  assert.match(page, /}, \[activity, plan, sort\]\)/);
});
