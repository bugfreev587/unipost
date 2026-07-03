import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = join(process.cwd(), "..");

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("smoke regression can validate YouTube V1 account metrics with a dedicated fixture", async () => {
  const smoke = await source("scripts/smoke-test.sh");
  const runner = await source("scripts/regression/run-suite.sh");
  const workflow = await source(".github/workflows/regression-monitor.yml");

  assert.match(smoke, /YOUTUBE_METRICS_ACCOUNT_ID/);
  assert.match(smoke, /\/v1\/accounts\/\$\{YOUTUBE_METRICS_ACCOUNT_ID\}\/metrics/);
  assert.match(smoke, /\.data\.platform'\s+'youtube'/);
  assert.match(smoke, /platform_specific\.view_count/);
  assert.match(smoke, /post_count_public_only/);

  assert.match(runner, /YOUTUBE_METRICS_ACCOUNT_ID/);
  assert.match(workflow, /REGRESSION_YOUTUBE_METRICS_ACCOUNT_ID/);
});

test("smoke regression can validate YouTube Analytics API V2 reports with a dedicated fixture", async () => {
  const smoke = await source("scripts/smoke-test.sh");
  const runner = await source("scripts/regression/run-suite.sh");
  const workflow = await source(".github/workflows/regression-monitor.yml");

  for (const endpoint of ["summary", "trend", "videos"]) {
    assert.match(smoke, new RegExp(`/v1/accounts/\\$\\{YOUTUBE_ANALYTICS_ACCOUNT_ID\\}/youtube/analytics/${endpoint}`));
  }

  for (const expectedField of [
    "metrics.views",
    "metrics.estimated_minutes_watched",
    "rows",
    "videos",
    "required_scopes",
    "yt-analytics.readonly",
  ]) {
    assert.match(smoke, new RegExp(expectedField.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }

  assert.match(runner, /YOUTUBE_ANALYTICS_ACCOUNT_ID/);
  assert.match(workflow, /REGRESSION_YOUTUBE_ANALYTICS_ACCOUNT_ID/);
});
