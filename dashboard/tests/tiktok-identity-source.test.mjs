import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

// TikTok managed account identity accuracy PRD (2026-07-31): one shared label
// rule (display name primary, @username handle), legacy detection through the
// server flag only, and inline reconnect guidance that reuses the existing
// Connect flow.
test("shared account identity helper implements the PRD label rule", async () => {
  const helper = await source("src/lib/account-identity.ts");

  assert.match(helper, /export function accountIdentityLabels/);
  assert.match(helper, /export const TIKTOK_IDENTITY_RECONNECT_GUIDANCE/);
  assert.match(helper, /Reconnect TikTok to refresh the account username/);
  // Primary label chain: display_name → username → account_name → id.
  assert.match(helper, /displayName \|\| username \|\| legacyName \|\| account\.id/);
  // Handle renders only for a verified username.
  assert.match(helper, /@\$\{username\}/);
  // Legacy detection uses the server flag, never value equality.
  assert.match(helper, /identity_refresh_required === true/);
  assert.doesNotMatch(helper, /username\s*===?\s*(account\.)?display_name/i);
});

test("SocialAccount type carries the additive identity fields", async () => {
  const api = await source("src/lib/api.ts");
  for (const field of [
    "username?: string | null",
    "display_name?: string | null",
    "identity_refresh_required?: boolean",
    "last_connected_at?: string | null",
  ]) {
    assert.ok(api.includes(field), `api.ts missing ${field}`);
  }
});

test("accounts page and managed-user detail consume the shared rule with inline guidance", async () => {
  const accountsPage = await source("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
  const userDetail = await source(
    "src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx",
  );

  for (const [name, page] of [
    ["accounts page", accountsPage],
    ["managed-user detail", userDetail],
  ]) {
    assert.match(page, /accountIdentityLabels/, `${name} must use the shared label rule`);
    assert.match(
      page,
      /TIKTOK_IDENTITY_RECONNECT_GUIDANCE/,
      `${name} must render the inline reconnect guidance`,
    );
    assert.match(
      page,
      /data-identity-refresh-guidance/,
      `${name} guidance must stay adjacent to the account identity`,
    );
  }
});

test("create-post account labels route through the shared identity rule", async () => {
  const labels = await source("src/components/posts/create-post/account-labels.ts");
  assert.match(labels, /accountIdentityLabels/);
});

test("List Accounts docs match the production contract", async () => {
  const docs = await source("src/app/docs/api/accounts/list/content.tsx");
  // PRD §9.7: stored TikTok avatar URLs are expiring provider URLs and are
  // not part of the P0 list contract.
  assert.doesNotMatch(docs, /account_avatar_url/);
  for (const field of ["username", "display_name", "identity_refresh_required", "last_connected_at"]) {
    assert.ok(docs.includes(`"${field}"`), `docs example missing ${field}`);
  }
});
