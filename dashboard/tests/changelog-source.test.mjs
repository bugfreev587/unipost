import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("public marketing nav exposes Developer dropdown with Docs and Change Logs", async () => {
  const nav = await source("src/components/marketing/nav.tsx");

  assert.match(nav, /DropdownMenu/);
  assert.match(nav, /Developer/);
  assert.match(nav, /href:\s*"\/docs"/);
  assert.match(nav, /href:\s*"\/changelog"/);
  assert.match(nav, /label:\s*"Change Logs"/);
  assert.match(nav, /active === "developer"/);
});

test("changelog data keeps verified release metadata and SDK ecosystem fields", async () => {
  const releases = await source("src/app/changelog/releases.ts");

  assert.match(releases, /export type ChangelogCategory/);
  assert.match(releases, /"reliability"/);
  assert.match(releases, /export type SdkEcosystem/);
  assert.match(releases, /"npm" \| "pip" \| "go" \| "maven"/);
  assert.match(releases, /displayDate\?/);
  assert.match(releases, /export type ChangelogImpact = "new" \| "improved" \| "changed" \| "fixed"/);
  assert.match(releases, /impact:\s*ChangelogImpact/);
  assert.match(releases, /isBreaking:\s*boolean/);
  assert.match(releases, /packageName:\s*"@unipost\/sdk"/);
  assert.match(releases, /packageName:\s*"unipost"/);
  assert.match(releases, /packageName:\s*"github\.com\/unipost-dev\/sdk-go"/);
  assert.match(releases, /packageName:\s*"dev\.unipost:sdk-java"/);
  assert.match(releases, /pip install unipost==0\.5\.0/);
  assert.match(releases, /go get github\.com\/unipost-dev\/sdk-go@v0\.5\.0/);
  assert.match(releases, /dev\.unipost:sdk-java:0\.5\.0/);
  assert.doesNotMatch(releases, /@unipost\/sdk-js/);
  assert.match(releases, /sourceLinks/);
  assert.match(releases, /enterprise-pricing-launch/);
  assert.match(releases, /Enterprise pricing positioning launched/);
  assert.match(releases, /sdk-0-5-0/);
  assert.match(releases, /Multi-language SDKs 0\.5\.0/);
  assert.match(releases, /video-audio-overlay-api/);
  assert.match(releases, /Video \+ audio overlay API/);
  assert.match(releases, /\/docs\/api\/media\/audio-overlays/);
  assert.match(releases, /\/docs\/guides\/video-audio-overlay/);
  assert.match(releases, /Developer Logs API/);
  assert.match(releases, /Agent Debug Kit/);
  assert.match(releases, /Local Connect testing/);
  assert.match(releases, /Analytics Explorer API/);
  assert.match(releases, /Posts calendar and scheduled-post editing/);
});

test("changelog page renders table, impact badges, SDK versions, and release links", async () => {
  const page = await source("src/app/changelog/page.tsx");

  assert.match(page, /PublicSiteHeader active="developer"/);
  assert.match(page, /Change Logs/);
  assert.match(page, /Latest release/);
  assert.match(page, /Latest SDKs/);
  assert.match(page, /release-table/);
  assert.match(page, /impactLabel/);
  assert.match(page, /Breaking/);
  assert.match(page, /sdkVersions/);
  assert.match(page, /sourceLinks/);
  assert.match(page, /changelogReleaseRows/);
});

test("SDK docs show the current Java SDK install version", async () => {
  const sdkPage = await source("src/app/docs/sdk/page.tsx");

  assert.match(sdkPage, /dev\.unipost:sdk-java:0\.5\.0/);
  assert.doesNotMatch(sdkPage, /dev\.unipost:sdk-java:0\.4\.0/);
});

test("sitemap includes one canonical changelog page and no hash anchors", async () => {
  const sitemap = await source("src/app/sitemap.ts");
  const changelogMatches = sitemap.match(/\/changelog/g) ?? [];

  assert.equal(changelogMatches.length, 1);
  assert.doesNotMatch(sitemap, /#sdk-|#developer-|#logs-/);
});

test("proxy treats changelog as a public landing page", async () => {
  const proxy = await source("src/proxy.ts");

  assert.ok(proxy.includes('pathname === "/changelog"'));
});
