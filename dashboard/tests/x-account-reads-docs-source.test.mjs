import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("X profile and authored-post references document their public contracts", async () => {
  const [profile, posts] = await Promise.all([
    source("src/app/docs/api/accounts/profile/page.tsx"),
    source("src/app/docs/api/accounts/posts/page.tsx"),
  ]);

  assert.match(profile, /\/v1\/accounts\/:account_id\/profile/);
  assert.match(profile, /external_user_id/);
  assert.match(profile, /Idempotency-Key/);
  assert.match(profile, /users\.read/);
  assert.match(profile, /INSUFFICIENT_X_CREDITS/);
  assert.match(profile, /accounting_enabled/);
  assert.match(profile, /X_CREDITS_CATALOG_VERSION/);

  assert.match(posts, /\/v1\/accounts\/:account_id\/posts/);
  assert.match(posts, /between 5 and 100/);
  assert.match(posts, /scanned_count/);
  assert.match(posts, /exclude_reposts/);
  assert.match(posts, /exclude_replies_to_others/);
  assert.match(posts, /cursor_expires_at/);
  assert.match(posts, /retry_cursor/);
  assert.match(posts, /external_post_id/);
  assert.match(posts, /thread_id/);
  assert.match(posts, /position is not exposed/i);
  assert.match(posts, /INSUFFICIENT_X_CREDITS/);
  assert.match(posts, /X_CREDITS_CATALOG_VERSION/);
});

test("capabilities and Credits docs expose billing policy without post content", async () => {
  const [capabilities, credits] = await Promise.all([
    source("src/app/docs/api/accounts/capabilities/page.tsx"),
    source("src/app/docs/api/x-credits/page.tsx"),
  ]);

  assert.match(capabilities, /schema_version": "1\.8"/);
  assert.match(capabilities, /x_account_reads\.profile_read/);
  assert.match(capabilities, /x_account_reads\.own_post_history_read/);
  assert.match(capabilities, /feature_disabled/);
  assert.match(capabilities, /customer_x_app/);

  assert.match(credits, /monthly_finalized/);
  assert.match(credits, /monthly_pending/);
  assert.match(credits, /monthly_effective/);
  assert.match(credits, /\/v1\/billing\/x-credits\/events/);
  assert.match(credits, /operation_id/);
  assert.match(credits, /external_user_id/);
  assert.match(credits, /estimated/);
  assert.match(credits, /reserved/);
  assert.match(credits, /charged/);
  assert.match(credits, /released/);
  assert.match(credits, /never includes post text/i);
});

test("new X reads are discoverable while Credits accounting remains flag gated", async () => {
  const [shell, apiIndex, searchIndex, sitemap, docsFlags] = await Promise.all([
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/page.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
    source("src/app/sitemap.ts"),
    source("src/lib/docs-feature-flags.ts"),
  ]);

  for (const discoverySource of [shell, apiIndex, searchIndex, sitemap]) {
    assert.match(discoverySource, /\/docs\/api\/accounts\/profile/);
    assert.match(discoverySource, /\/docs\/api\/accounts\/posts/);
  }
  assert.match(docsFlags, /\/docs\/api\/x-credits/);
  assert.doesNotMatch(docsFlags, /\/docs\/api\/accounts\/(?:profile|posts)/);
});
