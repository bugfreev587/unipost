import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("public documentation feature flags fail closed and map every dedicated route", async () => {
  const [sharedFlags, serverFlags] = await Promise.all([
    source("src/lib/docs-feature-flags.ts"),
    source("src/lib/public-feature-flags-server.ts"),
  ]);

  assert.match(sharedFlags, /CLOSED_PUBLIC_DOCS_FLAGS/);
  assert.match(sharedFlags, /x_dms_v1:\s*false/);
  assert.match(sharedFlags, /x_credits_billing_v1:\s*false/);
  assert.match(sharedFlags, /\/docs\/guides\/x\/direct-messages/);
  assert.match(sharedFlags, /\/docs\/guides\/x\/credits/);
  assert.match(sharedFlags, /\/docs\/api\/x-credits/);
  assert.match(sharedFlags, /filterDocsNavigation/);
  assert.match(sharedFlags, /filterDocsSearchChunks/);

  assert.match(serverFlags, /\/v1\/public\/features/);
  assert.match(serverFlags, /cache:\s*"no-store"/);
  assert.match(serverFlags, /CLOSED_PUBLIC_DOCS_FLAGS/);
  assert.match(serverFlags, /notFound\(\)/);
});

test("dedicated X DM and Credits pages return a Next.js 404 while their public flag is off", async () => {
  const [dmGuide, creditsGuide, creditsReference] = await Promise.all([
    source("src/app/docs/guides/x/direct-messages/page.tsx"),
    source("src/app/docs/guides/x/credits/page.tsx"),
    source("src/app/docs/api/x-credits/page.tsx"),
  ]);

  assert.match(dmGuide, /requirePublicDocsFeature\("x_dms_v1"\)/);
  assert.match(creditsGuide, /requirePublicDocsFeature\("x_credits_billing_v1"\)/);
  assert.match(creditsReference, /requirePublicDocsFeature\("x_credits_billing_v1"\)/);
});

test("Docs navigation, local search, landings, platform cards, sitemap, and AI search use public flags", async () => {
  const [
    shell,
    guides,
    api,
    platformPage,
    credentialPage,
    sitemap,
    aiRoute,
  ] = await Promise.all([
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/platforms/[platform]/page.tsx"),
    source("src/app/docs/platform-credentials/[platform]/page.tsx"),
    source("src/app/sitemap.ts"),
    source("src/app/api/docs/answer/route.ts"),
  ]);

  assert.match(shell, /getPublicFeatureFlags/);
  assert.match(shell, /CLOSED_PUBLIC_DOCS_FLAGS/);
  assert.match(shell, /filterDocsNavigation/);
  assert.match(shell, /filterDocsSearchIndex/);

  for (const landing of [guides, api, platformPage]) {
    assert.match(landing, /getPublicDocsFeatureFlags/);
    assert.match(landing, /filterDocsNavigation/);
  }
  assert.match(credentialPage, /getPublicFeatureFlags/);
  assert.match(credentialPage, /CLOSED_PUBLIC_DOCS_FLAGS/);
  assert.match(credentialPage, /filterDocsNavigation/);

  assert.match(sitemap, /getPublicDocsFeatureFlags/);
  assert.match(sitemap, /filterDocsNavigation/);
  assert.match(aiRoute, /getPublicDocsFeatureFlags/);
  assert.match(aiRoute, /filterDocsSearchChunks/);
});

test("shared X Comments, reconnect, and Inbox pages keep public content while omitting disabled-only sections", async () => {
  const paths = [
    "src/app/docs/guides/x/comments/page.tsx",
    "src/app/docs/guides/x/reconnect-permissions/page.tsx",
    "src/app/docs/api/inbox/page.tsx",
    "src/app/docs/api/inbox/list/page.tsx",
    "src/app/docs/api/inbox/reply/page.tsx",
    "src/app/docs/api/inbox/sync/page.tsx",
  ];
  const pages = await Promise.all(paths.map(source));

  for (const [index, page] of pages.entries()) {
    assert.match(page, /getPublicDocsFeatureFlags/, `${paths[index]} must read public flags`);
    assert.match(page, /x_dms_v1/, `${paths[index]} must gate DM-only content`);
  }

  assert.match(pages[0], /x_credits_billing_v1/);
  assert.match(pages[1], /x_credits_billing_v1/);
  assert.match(pages[0], /X comments use OAuth 2\.0/);
  assert.match(pages[2], /x_reply/);
});

test("shared pricing, publishing, capabilities, errors, and cross-links do not expose disabled docs", async () => {
  const [
    pricing,
    capabilities,
    createPost,
    validatePost,
    errors,
    dmGuide,
    creditsGuide,
    creditsReference,
    platformPage,
  ] = await Promise.all([
    source("src/app/docs/pricing/page.tsx"),
    source("src/app/docs/api/accounts/capabilities/page.tsx"),
    source("src/app/docs/api/posts/create/content.tsx"),
    source("src/app/docs/api/posts/validate/page.tsx"),
    source("src/app/docs/api/errors/page.tsx"),
    source("src/app/docs/guides/x/direct-messages/page.tsx"),
    source("src/app/docs/guides/x/credits/page.tsx"),
    source("src/app/docs/api/x-credits/page.tsx"),
    source("src/app/docs/platforms/[platform]/page.tsx"),
  ]);

  assert.match(pricing, /getPublicDocsFeatureFlags/);
  assert.match(pricing, /x_credits_billing_v1/);
  for (const clientPage of [capabilities, createPost, validatePost, errors]) {
    assert.match(clientPage, /usePublicDocsFeatureFlags/);
  }
  assert.match(capabilities, /filterDocsNavigation/);
  assert.match(dmGuide, /publicFeatureFlags\.x_credits_billing_v1/);
  assert.match(creditsGuide, /publicFeatureFlags\.x_dms_v1/);
  assert.match(creditsReference, /publicFeatureFlags\.x_dms_v1/);
  assert.match(platformPage, /capabilityRows/);
  assert.match(platformPage, /inboxRows/);
});
