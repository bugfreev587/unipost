import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("dashboard API exposes public binding metadata and binding mutations", async () => {
  const api = await source("src/lib/api.ts");

  assert.match(api, /shared_connection\?: boolean/);
  assert.match(api, /bound_profile_ids\?: string\[\]/);
  assert.match(api, /sibling_account_ids\?: string\[\]/);
  assert.match(api, /export async function bindSocialAccount/);
  assert.match(api, /`\/v1\/accounts\/\$\{accountId\}\/bindings`/);
  assert.match(api, /profile_id: profileId/);
  assert.match(api, /external_user_id: externalUserId/);
  assert.match(api, /export async function unbindSocialAccount/);
  assert.match(api, /`\/v1\/accounts\/\$\{accountId\}\/binding`/);
});

test("profile binding controls distinguish binding removal from physical disconnect", async () => {
  const component = await source("src/components/accounts/profile-bindings.tsx");
  const page = await source("src/app/(dashboard)/projects/[id]/accounts/page.tsx");

  assert.match(component, /account\.bound_profile_ids/);
  assert.match(component, /Shared across/);
  assert.match(component, /Add to profile/);
  assert.match(component, /Remove from this Profile/);
  assert.match(component, /Already added/);
  assert.doesNotMatch(component, /connection_id/);

  assert.match(page, /bindSocialAccount/);
  assert.match(page, /unbindSocialAccount/);
  assert.match(page, /ProfileBindings/);
  assert.match(page, /Disconnect physical account/);
  assert.match(page, /affectedProfileNames/);
  assert.doesNotMatch(page, /account\.connection_id/);
});

test("composer prevents selecting sibling bindings for one physical account", async () => {
  const grid = await source("src/components/posts/create-post/account-card-grid.tsx");
  const form = await source("src/components/posts/create-post/use-create-post-form.ts");
  const labels = await source("src/components/posts/create-post/account-labels.ts");

  assert.match(grid, /getAccountIdentityKey/);
  assert.match(grid, /selectedSiblingId/);
  assert.match(grid, /Already selected from another Profile/);
  assert.match(grid, /const isDisabled = disabled \|\| !!selectedSiblingId/);
  assert.match(grid, /disabled=\{isDisabled\}/);
  assert.match(form, /selectedIdentityKeys/);
  assert.match(labels, /account\.sibling_account_ids/);
  assert.match(labels, /::siblings::/);
  assert.match(form, /if \(selectedIdentityKeys\.has\(getAccountIdentityKey\(account\)\)\) return prev/);
  assert.match(form, /if \(duplicateAccountIds\.size > 0\) return false/);
  assert.match(form, /const accountIds = selectedAccounts\.map/);
  assert.doesNotMatch(form, /const accountIds = uniqueSelectedAccounts\.map/);
  assert.match(grid, /Remove one before publishing/);
  assert.doesNotMatch(grid, /only one post per platform account will be sent/);
  assert.doesNotMatch(form, /only publish once/);
});

test("public docs describe binding IDs and duplicate physical target rejection", async () => {
  const list = await source("src/app/docs/api/accounts/list/content.tsx");
  const bind = await source("src/app/docs/api/accounts/bind/page.tsx");
  const index = await source("src/app/docs/api/page.tsx");
  const create = await source("src/app/docs/api/posts/create/content.tsx");

  assert.match(list, /data\[\]\.bound_profile_ids/);
  assert.match(list, /data\[\]\.shared_connection/);
  assert.match(bind, /POST/);
  assert.match(bind, /\/v1\/accounts\/:account_id\/bindings/);
  assert.match(index, /Bind account to Profile/);
  assert.match(create, /DUPLICATE_SOCIAL_CONNECTION/);
  assert.match(create, /platform_posts\[\]\.account_id/);
  assert.doesNotMatch(bind, /connection_id/);
});
