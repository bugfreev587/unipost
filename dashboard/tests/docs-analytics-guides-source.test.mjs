import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Analytics guides are exposed as Guides, not as API Reference endpoint groups", async () => {
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");
  const docsHome = await source("src/app/docs/page.tsx");

  assert.match(docsShell, /type DocsPrimaryKey = .*"guides"/);
  assert.match(docsShell, /key:\s*"guides",\s*label:\s*"Guides",\s*href:\s*"\/docs\/guides"/);
  assert.match(docsShell, /if \(current\.startsWith\("\/docs\/guides"\)\) return "guides"/);
  assert.match(docsShell, /label:\s*"Get TikTok followers"/);
  assert.match(docsHome, /\/docs\/guides\/analytics/);

  const apiReferenceSection = docsShell.slice(docsShell.indexOf('"api-reference": ['));
  assert.doesNotMatch(apiReferenceSection, /Get TikTok followers/);
  assert.doesNotMatch(apiReferenceSection, /\/docs\/guides\/analytics\/tiktok-followers/);
});

test("TikTok followers guide points users to the unified account metrics API", async () => {
  const guide = await source("src/app/docs/guides/analytics/tiktok-followers/page.tsx");

  assert.match(guide, /GET \/v1\/accounts\/\{account_id\}\/metrics/);
  assert.match(guide, /follower_count/);
  assert.match(guide, /user\.info\.stats/);
  assert.match(guide, /video\.list[\s\S]*public videos/i);
  assert.match(guide, /\/docs\/api\/accounts\/metrics/);
  assert.match(guide, /\/docs\/api\/accounts\/list/);
  assert.doesNotMatch(guide, /tiktok\.analytics_scopes|FEATURE_TIKTOK_ANALYTICS_SCOPES|feature flag/i);
});
