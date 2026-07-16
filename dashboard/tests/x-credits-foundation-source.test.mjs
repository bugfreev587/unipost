import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing explains included X Credits with the generated catalog", async () => {
  const [pricing, docsPricing, catalog] = await Promise.all([
    source("src/app/pricing/pricing-page-client.tsx"),
    source("src/app/docs/pricing/page.tsx"),
    source("src/data/x-credits-catalog.generated.ts"),
  ]);

  assert.match(pricing, /X_CREDIT_PLANS/);
  assert.match(pricing, /What your included X Credits can do/);
  assert.match(docsPricing, /X_CREDIT_PLANS/);
  assert.match(docsPricing, /X Credits are separate from posts\/month/);
  assert.match(pricing, /resets each billing period/);
  assert.match(pricing, /hard limit/);
  assert.match(pricing, /Inbox not included/);
  assert.match(pricing, /phased X Inbox support/);

  for (const expected of [
    /"id": "basic"[\s\S]*"normal_posts": 266[\s\S]*"url_posts": 20[\s\S]*"comment_interactions": 200[\s\S]*"dm_interactions": 160/,
    /"id": "growth"[\s\S]*"normal_posts": 800[\s\S]*"url_posts": 60[\s\S]*"comment_interactions": 600[\s\S]*"dm_interactions": 480/,
    /"id": "team"[\s\S]*"normal_posts": 2000[\s\S]*"url_posts": 150[\s\S]*"comment_interactions": 1500[\s\S]*"dm_interactions": 1200/,
  ]) {
    assert.match(catalog, expected);
  }

  for (const forbidden of ["Buy more", "Top-up available", "Auto top-up"]) {
    assert.doesNotMatch(`${pricing}\n${docsPricing}`, new RegExp(forbidden, "i"));
  }
});

test("billing shows every X allowance state and uses the live endpoint", async () => {
  const billing = await source("src/app/(dashboard)/settings/billing/page.tsx");

  assert.match(billing, /getXCreditsAllowance/);
  assert.match(billing, /X Credits/);
  assert.match(billing, /xCreditsLoading/);
  assert.match(billing, /xCreditsError/);
  assert.match(billing, /monthly_allowance == null/);
  assert.match(billing, /monthly_allowance === 0/);
  assert.match(billing, /20 X posts per connected account per UTC day/);
  assert.match(billing, /hard limit/);
});

test("X Credits reference and guide link to each other and all discovery surfaces", async () => {
  const [
    reference,
    guide,
    shell,
    apiIndex,
    guideIndex,
    errors,
    createPost,
    validatePost,
    searchIndex,
    sitemap,
  ] = await Promise.all([
    source("src/app/docs/api/x-credits/page.tsx"),
    source("src/app/docs/guides/x/credits/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/api/errors/page.tsx"),
    source("src/app/docs/api/posts/create/content.tsx"),
    source("src/app/docs/api/posts/validate/page.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
    source("src/app/sitemap.ts"),
  ]);

  assert.match(reference, /\/docs\/guides\/x\/credits/);
  assert.match(reference, /\/docs\/pricing/);
  assert.match(guide, /\/docs\/api\/x-credits/);
  assert.match(guide, /\/docs\/api\/posts\/create/);
  assert.match(guide, /\/docs\/api\/posts\/validate/);

  for (const discoverySource of [shell, apiIndex, searchIndex, sitemap]) {
    assert.match(discoverySource, /\/docs\/api\/x-credits/);
  }
  for (const discoverySource of [shell, guideIndex, searchIndex, sitemap]) {
    assert.match(discoverySource, /\/docs\/guides\/x\/credits/);
  }
  assert.match(errors, /x_monthly_usage_limit_exceeded/);
  assert.match(createPost, /X Credits/);
  assert.match(validatePost, /does not consume X Credits/);
});
