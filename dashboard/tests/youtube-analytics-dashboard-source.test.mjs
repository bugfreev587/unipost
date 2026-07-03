import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Dashboard platform analytics list exposes YouTube as a first-class platform", async () => {
  const listSource = await source("src/app/(dashboard)/projects/[id]/analytics/platforms/platform-analytics-list.tsx");

  assert.match(listSource, /analytics\/platforms\/youtube/);
  assert.match(listSource, /<PlatformIcon platform="youtube"/);
  assert.match(listSource, /youtube\.readonly\s*\/\s*yt-analytics\.readonly/);
  assert.match(listSource, /Subscribers, channel views, watch time, and top videos/);
  assert.doesNotMatch(listSource, /YouTube channel stats[\s\S]*same drilldown pattern later/);
});

test("Dashboard YouTube platform route renders the YouTube analytics component", async () => {
  const pageSource = await source("src/app/(dashboard)/projects/[id]/analytics/platforms/youtube/page.tsx");

  assert.match(pageSource, /YouTubeAnalyticsView/);
  assert.match(pageSource, /params:\s*Promise<\{\s*id:\s*string\s*\}>/);
});

test("YouTube analytics dashboard combines V1 account metrics and V2 Analytics API reports", async () => {
  const componentSource = await source("src/components/analytics/youtube-analytics-view.tsx");

  for (const call of [
    "listSocialAccounts",
    "getAccountMetrics",
    "getYouTubeAnalyticsSummary",
    "getYouTubeAnalyticsTrend",
    "getYouTubeAnalyticsVideos",
  ]) {
    assert.match(componentSource, new RegExp(call), `expected ${call} in YouTube dashboard component`);
  }

  for (const requiredText of [
    "YouTube Analytics",
    "youtube.readonly",
    "yt-analytics.readonly",
    "Basic channel metrics",
    "Analytics report",
    "Daily trend",
    "Top videos",
    "Reconnect required for YouTube Analytics",
  ]) {
    assert.match(componentSource, new RegExp(requiredText.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
});
