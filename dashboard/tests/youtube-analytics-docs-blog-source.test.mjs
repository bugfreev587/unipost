import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const dashboardRoot = process.cwd();
const repoRoot = join(dashboardRoot, "..");

async function dashboardSource(path) {
  return readFile(join(dashboardRoot, path), "utf8");
}

async function repoSource(path) {
  return readFile(join(repoRoot, path), "utf8");
}

test("YouTube analytics docs explain V1 account metrics, V2 reports, and dashboard visibility", async () => {
  const [overview, accountMetricsGuide, reconnectGuide] = await Promise.all([
    dashboardSource("src/app/docs/api/analytics/youtube/page.tsx"),
    dashboardSource("src/app/docs/guides/analytics/account-metrics/page.tsx"),
    dashboardSource("src/app/docs/guides/analytics/reconnect-analytics-scopes/page.tsx"),
  ]);

  assert.match(overview, /V1[\s\S]*\/v1\/accounts\/\{account_id\}\/metrics/);
  assert.match(overview, /V2[\s\S]*\/youtube\/analytics\/summary/);
  assert.match(overview, /Analytics\s*-\s*Platforms\s*-\s*YouTube/);
  assert.match(accountMetricsGuide, /YouTube[\s\S]*youtube\.readonly/);
  assert.match(reconnectGuide, /YouTube Analytics V2[\s\S]*yt-analytics\.readonly/);
});

test("SDK coverage matrix records optional live regression coverage for YouTube V1 and V2", async () => {
  const matrix = await repoSource("docs/sdk-api-coverage-matrix.md");

  assert.match(matrix, /\| `GET \/v1\/accounts\/\{id\}\/metrics` \| Optional smoke/);
  assert.match(matrix, /\| `GET \/v1\/accounts\/\{id\}\/youtube\/analytics\/summary` \| Optional smoke/);
  assert.match(matrix, /\| `GET \/v1\/accounts\/\{id\}\/youtube\/analytics\/trend` \| Optional smoke/);
  assert.match(matrix, /\| `GET \/v1\/accounts\/\{id\}\/youtube\/analytics\/videos` \| Optional smoke/);
});

test("YouTube analytics blog draft is ready for product review before publication", async () => {
  const draft = await repoSource("docs/blog-drafts/youtube-analytics-api.md");

  assert.match(draft, /^# YouTube Analytics API for Apps That Publish Video/m);
  assert.match(draft, /V1[\s\S]*basic account metrics/i);
  assert.match(draft, /V2[\s\S]*YouTube Analytics API/i);
  assert.match(draft, /yt-analytics\.readonly/);
  assert.match(draft, /does not include monetary or revenue metrics/i);
  assert.match(draft, /Status: Draft for PM review/);
});
