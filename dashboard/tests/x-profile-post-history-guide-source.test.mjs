import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("X profile and authored-post guide documents the complete integration workflow", async () => {
  const guide = await source("src/app/docs/guides/x/profile-and-post-history/page.tsx");

  for (const value of [
    "users.read",
    "tweet.read",
    "offline.access",
    "external_user_id",
    "Idempotency-Key",
    "next_cursor",
    "start_time",
    "end_time",
    "inclusive",
    "exclusive",
    "accounting_enabled",
    "bypass_reason",
    "feature_disabled",
    "customer_x_app",
    "INSUFFICIENT_X_CREDITS",
    "INVALID_CURSOR",
    "IDEMPOTENCY_CONFLICT",
    "READ_IN_PROGRESS",
    "READ_SETTLEMENT_PENDING",
    "ACCOUNT_REAUTHORIZATION_REQUIRED",
    "RATE_LIMITED",
    "X_UPSTREAM_ERROR",
    "is_retriable",
    "Retry-After",
    "/docs/guides/x/reconnect-permissions",
    "/docs/api/accounts/profile",
    "/docs/api/accounts/posts",
  ]) {
    assert.match(guide, new RegExp(value.replace(/[./-]/g, "\\$&")));
  }

  for (const sdkMethod of [
    "accounts.getProfile",
    "accounts.get_profile",
    "Accounts.Profile",
    "accounts().profile",
    "accounts.listPosts",
    "accounts.list_posts",
    "Accounts.ListPosts",
    "accounts().listPosts",
  ]) {
    assert.match(guide, new RegExp(sdkMethod.replace(/[().]/g, "\\$&")));
  }
  assert.match(guide, /request_id/);
  assert.match(guide, /meta(?:\.|\["?)credits/);
  assert.match(guide, /caller-owned|caller owned/i);
  assert.match(guide, /same key/i);
  assert.match(guide, /new (?:logical-page )?key|new key/i);
  assert.match(guide, /persisted X app identity/i);
});

test("guide discovery is public and Credits AI chunks remain feature gated", async () => {
  const [shell, guides, api, search, sitemap, ai, flags] = await Promise.all([
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/sitemap.ts"),
    source("src/lib/docs-ai-search-index.ts"),
    source("src/lib/docs-feature-flags.ts"),
  ]);
  const route = "/docs/guides/x/profile-and-post-history";

  for (const discovery of [shell, guides, search, sitemap, ai]) {
    assert.match(discovery, new RegExp(route.replace(/[/-]/g, "\\$&")));
  }
  assert.match(api, /\/docs\/api\/accounts\/profile/);
  assert.match(api, /\/docs\/api\/accounts\/posts/);
  assert.doesNotMatch(flags, new RegExp(route.replace(/[/-]/g, "\\$&")));

  const generalChunk = ai.indexOf('id: "guide-x-profile-post-history"');
  const creditsChunk = ai.indexOf('id: "guide-x-profile-post-history-credits"');
  assert.notEqual(generalChunk, -1);
  assert.notEqual(creditsChunk, -1);
  assert.ok(generalChunk < creditsChunk);
  assert.doesNotMatch(
    ai.slice(generalChunk, creditsChunk),
    /required_feature:\s*"x_credits_billing_v1"/,
  );
  assert.match(
    ai.slice(creditsChunk, creditsChunk + 1800),
    /required_feature:\s*"x_credits_billing_v1"/,
  );
});
