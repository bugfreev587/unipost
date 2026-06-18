import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

async function sourceOrNull(path) {
  try {
    return await source(path);
  } catch (error) {
    if (error?.code === "ENOENT") {
      return null;
    }
    throw error;
  }
}

const platformDocs = [
  {
    label: "Instagram Analytics",
    slug: "instagram",
    paths: [
      "/docs/api/analytics/instagram",
      "/docs/api/analytics/instagram/profile",
      "/docs/api/analytics/instagram/account-metrics",
      "/docs/api/analytics/instagram/media",
    ],
    scopes: ["instagram_business_basic", "instagram_business_manage_insights"],
    routes: [
      "/v1/accounts/:account_id/instagram/profile",
      "/v1/accounts/:account_id/metrics",
      "/v1/accounts/:account_id/instagram/media",
      "/v1/posts/:post_id/analytics",
    ],
  },
  {
    label: "Threads Analytics",
    slug: "threads",
    paths: [
      "/docs/api/analytics/threads",
      "/docs/api/analytics/threads/profile",
      "/docs/api/analytics/threads/account-metrics",
      "/docs/api/analytics/threads/posts",
    ],
    scopes: ["threads_basic", "threads_manage_insights"],
    routes: [
      "/v1/accounts/:account_id/threads/profile",
      "/v1/accounts/:account_id/metrics",
      "/v1/accounts/:account_id/threads/posts",
      "/v1/posts/:post_id/analytics",
    ],
  },
  {
    label: "Pinterest Analytics",
    slug: "pinterest",
    paths: [
      "/docs/api/analytics/pinterest",
      "/docs/api/analytics/pinterest/boards",
      "/docs/api/analytics/pinterest/post-analytics",
    ],
    scopes: ["pins:read", "boards:read", "user_accounts:read"],
    routes: [
      "/v1/accounts/:account_id/pinterest/boards",
      "/v1/posts/:post_id/analytics",
    ],
  },
  {
    label: "Facebook Page Analytics",
    slug: "facebook",
    paths: [
      "/docs/api/analytics/facebook",
      "/docs/api/analytics/facebook/page-analytics",
      "/docs/api/analytics/facebook/page-insights",
    ],
    scopes: ["pages_read_engagement", "read_insights"],
    routes: [
      "/v1/accounts/:account_id/facebook/page-analytics",
      "/v1/accounts/:account_id/facebook/page-insights",
      "/v1/posts/:post_id/analytics",
    ],
  },
];

test("Analytics docs stay unified-first instead of exposing each platform as a primary API category", async () => {
  const [apiIndex, docsShell] = await Promise.all([
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
  ]);

  assert.match(apiIndex, /title:\s*"Analytics"/);
  assert.match(apiIndex, /path:\s*"\/v1\/analytics\/platforms"/);
  assert.match(apiIndex, /Platform capabilities/);
  assert.match(docsShell, /label:\s*"Analytics"/);
  assert.match(docsShell, /label:\s*"Platform capabilities"/);

  for (const platform of platformDocs) {
    assert.doesNotMatch(apiIndex, new RegExp(`title:\\s*"${platform.label}"`), `${platform.label} should not be a primary API index group`);
    assert.doesNotMatch(docsShell, new RegExp(`label:\\s*"${platform.label}"`), `${platform.label} should not be a primary docs sidebar group`);
    for (const path of platform.paths) {
      assert.doesNotMatch(apiIndex, new RegExp(path.replaceAll("/", "\\/")), `${path} should not be promoted from the API index`);
      assert.doesNotMatch(docsShell, new RegExp(path.replaceAll("/", "\\/")), `${path} should not be promoted from the docs sidebar`);
    }
  }
});

test("platform Analytics docs describe optional native drilldowns behind the unified Analytics API", async () => {
  const dataSource = await source("src/app/docs/api/analytics/_data/platform-analytics-docs.tsx");

  for (const platform of platformDocs) {
    const pagePath = `src/app/docs/api/analytics/${platform.slug}/page.tsx`;
    const overview = await sourceOrNull(pagePath);
    assert.ok(overview, `${pagePath} should exist`);
    assert.match(dataSource, new RegExp(`label:\\s*"${platform.label}"`), `${platform.label} should be defined in platform analytics docs data`);
    assert.match(dataSource, /public-ready|production|public API/i, `${platform.label} should describe public readiness`);
    assert.match(dataSource, /optional native drilldown|native drilldown/i, `${platform.label} should be positioned as a drilldown, not the primary integration path`);

    for (const scope of platform.scopes) {
      assert.match(dataSource, new RegExp(scope.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")), `${platform.label} should document ${scope}`);
    }
    for (const route of platform.routes) {
      assert.match(dataSource, new RegExp(route.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")), `${platform.label} should link ${route}`);
    }
  }
});

test("public Analytics tools link back to unified platform capabilities docs", async () => {
  const toolsConfig = await source("src/app/tools/_components/public-analytics-tool.tsx");

  for (const slug of ["tiktok", "instagram", "threads", "pinterest"]) {
    assert.match(
      toolsConfig,
      new RegExp(`slug:\\s*"${slug}"[\\s\\S]*?docsHref:\\s*"\\/docs\\/api\\/analytics\\/platforms"`),
      `${slug} public tool should link to unified platform capabilities docs`,
    );
  }
});
