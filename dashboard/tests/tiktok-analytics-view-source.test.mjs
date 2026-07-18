import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const source = readFileSync(resolve("src/components/analytics/tiktok-analytics-view.tsx"), "utf8");

test("TikTok analytics view isolates requests by generation and selected account", () => {
  assert.match(source, /useRef/);
  assert.match(source, /requestGenerationRef\.current\s*\+=\s*1/);
  assert.match(source, /requestGenerationRef\.current\s*===\s*generation/);
  assert.match(source, /activeAccountIdRef\.current\s*===\s*account\.id/);
});

test("TikTok analytics view settles primary resources independently", () => {
  assert.match(source, /Promise\.allSettled\s*\(\s*\[/);
  for (const request of ["getTikTokProfile", "getAccountMetrics", "getTikTokVideos", "listAllSocialPosts"]) {
    assert.match(source, new RegExp(`${request}\\(`));
  }
});
