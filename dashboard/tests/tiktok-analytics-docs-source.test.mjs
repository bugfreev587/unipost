import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("TikTok Analytics docs are not promoted as a separate primary Analytics API category", async () => {
  const [apiIndex, docsShell, toolsConfig] = await Promise.all([
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/tools/_components/public-analytics-tool.tsx"),
  ]);

  assert.doesNotMatch(apiIndex, /title:\s*"TikTok Analytics"/);
  assert.doesNotMatch(apiIndex, /\/docs\/api\/analytics\/tiktok\/profile/);
  assert.doesNotMatch(apiIndex, /\/docs\/api\/analytics\/tiktok\/account-metrics/);
  assert.doesNotMatch(apiIndex, /\/docs\/api\/analytics\/tiktok\/videos/);

  assert.doesNotMatch(docsShell, /label:\s*"TikTok Analytics"/);
  assert.doesNotMatch(docsShell, /\/docs\/api\/analytics\/tiktok\/profile/);
  assert.doesNotMatch(docsShell, /\/docs\/api\/analytics\/tiktok\/account-metrics/);
  assert.doesNotMatch(docsShell, /\/docs\/api\/analytics\/tiktok\/videos/);

  assert.match(toolsConfig, /docsHref:\s*"\/docs\/api\/analytics\/platforms"/);
});

test("TikTok Analytics endpoint pages document production readiness and scopes", async () => {
  const [overview, profile, metrics, videos, platformData] = await Promise.all([
    source("src/app/docs/api/analytics/tiktok/page.tsx"),
    source("src/app/docs/api/analytics/tiktok/profile/page.tsx"),
    source("src/app/docs/api/analytics/tiktok/account-metrics/page.tsx"),
    source("src/app/docs/api/analytics/tiktok/videos/page.tsx"),
    source("src/app/docs/platforms/[platform]/_data.tsx"),
  ]);

  for (const page of [overview, profile, metrics, videos]) {
    assert.match(page, /production/i);
    assert.match(page, /approved|public-ready|ready/i);
    assert.match(page, /optional native drilldown|native drilldown/i);
    assert.match(page, /user\.info\.(profile|stats)|video\.list/);
    assert.match(page, /reconnect|newly connected|connected accounts/i);
    assert.doesNotMatch(page, /tiktok\.analytics_scopes|FEATURE_TIKTOK_ANALYTICS_SCOPES|FEATURE_DISABLED|feature flag/i);
    assert.doesNotMatch(page, /until (TikTok approves|approval is complete)|Keep the flag off in production/i);
  }

  assert.match(profile, /\/v1\/accounts\/:account_id\/tiktok\/profile/);
  assert.match(metrics, /\/v1\/accounts\/:account_id\/metrics/);
  assert.match(videos, /\/v1\/accounts\/:account_id\/tiktok\/videos/);
  assert.match(platformData, /analytics scopes are approved/i);
  assert.doesNotMatch(platformData, /tiktok\.analytics_scopes|FEATURE_TIKTOK_ANALYTICS_SCOPES|FEATURE_DISABLED|feature flag/i);
  assert.doesNotMatch(platformData, /until TikTok approves/i);
});
