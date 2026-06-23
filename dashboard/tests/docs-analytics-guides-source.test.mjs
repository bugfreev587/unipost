import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Docs AI Search PRD defines grounded guide-first search requirements", async () => {
  const prd = await source("../docs/prd-docs-ai-search.md");

  assert.match(prd, /^# PRD - Docs AI Search/m);
  assert.match(prd, /guide-first/i);
  assert.match(prd, /grounded/i);
  assert.match(prd, /citation/i);
  assert.match(prd, /no answer without source/i);
  assert.match(prd, /API Reference remains.*Reference/i);
  assert.match(prd, /Analytics Guides/i);
  assert.match(prd, /docs chunk/i);
  assert.match(prd, /GET \/v1\/accounts\/\{id\}\/metrics/);
  assert.match(prd, /GET \/v1\/accounts\/\{account_id\}\/metrics/);
  assert.match(prd, /rendered\/built HTML|built HTML|rendered HTML/i);
  assert.match(prd, /platform-capabilities\.ts/);
  assert.match(prd, /keyword retrieval \+ LLM rerank|LLM rerank/i);
  assert.match(prd, /vector store|pgvector/i);
});

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

test("Publishing guide is grouped under Guides instead of Overview", async () => {
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.match(
    docsShell,
    /if \(current === "\/docs\/publishing"\) return "guides"/,
    "the publishing guide route should activate the Guides primary nav",
  );

  const overviewStart = docsShell.indexOf("overview: [");
  const platformsStart = docsShell.indexOf("  platforms: [", overviewStart);
  const guidesStart = docsShell.indexOf("  guides: [", platformsStart);
  const resourcesStart = docsShell.indexOf("  resources: [", guidesStart);

  assert.notEqual(overviewStart, -1, "overview sidebar nav should exist");
  assert.notEqual(platformsStart, -1, "platforms sidebar nav should follow overview");
  assert.notEqual(guidesStart, -1, "guides sidebar nav should exist");
  assert.notEqual(resourcesStart, -1, "resources sidebar nav should follow guides");

  const overviewSidebar = docsShell.slice(overviewStart, platformsStart);
  const guidesSidebar = docsShell.slice(guidesStart, resourcesStart);

  assert.doesNotMatch(overviewSidebar, /\/docs\/publishing/);
  assert.match(guidesSidebar, /label:\s*"Publishing guide",\s*href:\s*"\/docs\/publishing"/);
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

test("Analytics guides reuse shared docs presentation primitives", async () => {
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");
  const accountMetrics = await source("src/app/docs/guides/analytics/account-metrics/page.tsx");
  const analyticsOverview = await source("src/app/docs/guides/analytics/page.tsx");
  const postAnalytics = await source("src/app/docs/guides/analytics/post-analytics/page.tsx");
  const reconnectScopes = await source("src/app/docs/guides/analytics/reconnect-analytics-scopes/page.tsx");
  const tiktokFollowers = await source("src/app/docs/guides/analytics/tiktok-followers/page.tsx");

  for (const guide of [accountMetrics, analyticsOverview, postAnalytics, reconnectScopes, tiktokFollowers]) {
    assert.doesNotMatch(guide, /_components\/code-block/, "guides should use DocsCodeTabs from the docs shell instead of direct CodeBlock imports");
    assert.doesNotMatch(guide, /<table className="docs-table">/, "guides should use DocsTable so shared table widths and nowrap rules apply");
  }

  assert.match(postAnalytics, /<p>Post analytics availability depends/);
  assert.doesNotMatch(postAnalytics, /id="scope-notes"[\s\S]*?<ul className="docs-step-list">/, "scope notes should render as article paragraphs, not indented step lists");
  assert.doesNotMatch(tiktokFollowers, /id="scope-notes"[\s\S]*?<ul className="docs-step-list">/, "TikTok scope notes should render as article paragraphs, not indented step lists");

  assert.match(docsShell, /case "field\|meaning":/);
  assert.match(docsShell, /case "task\|unipost api\|start here":/);
  assert.match(docsShell, /case "platform\|analytics scopes\|common unipost api":/);
  assert.match(docsShell, /case "field\|meaning":[\s\S]*?return columnIndex === 0;/);
  assert.match(docsShell, /case "task\|unipost api\|start here":[\s\S]*?return columnIndex === 1;/);
  assert.match(docsShell, /case "platform\|analytics scopes\|common unipost api":[\s\S]*?return columnIndex === 2;/);
  assert.match(docsShell, /\.docs-api-inline\{[^}]*white-space:nowrap/s);
  assert.match(docsShell, /\.docs-api-inline-method\{[^}]*color:#16a34a/s);
});

test("Guide step lists do not render as invisible-marker indented body copy", async () => {
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.match(
    docsShell,
    /\.docs-shell-guide-redesign \.docs-page-guide-redesign \.docs-step-list\{[^}]*list-style:none[^}]*padding-left:0/s,
    "guide step lists should remove list markers and the matching phantom indent",
  );
  assert.match(
    docsShell,
    /\.docs-shell-guide-redesign \.docs-page-guide-redesign \.docs-step-list li\{[^}]*padding-left:0/s,
    "guide step list items should align with normal body paragraphs",
  );
});
