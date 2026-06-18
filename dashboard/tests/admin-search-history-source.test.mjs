import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin search history API helpers use the shared admin endpoint", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /export interface AdminSearchHistoryItem/);
  assert.match(api, /export type AdminSearchHistoryFieldKey/);
  assert.match(api, /export async function listAdminSearchHistory/);
  assert.match(api, /export async function saveAdminSearchHistory/);
  assert.match(api, /export async function deleteAdminSearchHistory/);
  assert.match(api, /\/v1\/admin\/search-history/);
  assert.match(api, /method: "POST"/);
  assert.match(api, /method: "DELETE"/);
});

test("search history input loads, saves, deletes, and exposes combobox semantics", () => {
  const component = source("src/app/admin/_components/search-history-input.tsx");

  assert.match(component, /listAdminSearchHistory/);
  assert.match(component, /saveAdminSearchHistory/);
  assert.match(component, /deleteAdminSearchHistory/);
  assert.match(component, /onFocus/);
  assert.match(component, /onKeyDown/);
  assert.match(component, /aria-expanded/);
  assert.match(component, /role="combobox"/);
  assert.match(component, /role="listbox"/);
  assert.match(component, /role="option"/);
});

test("requested admin filters are wired to server-backed search history", () => {
  const pages = [
    {
      path: "src/app/admin/logs/page.tsx",
      fieldKeys: ["admin.logs.q", "admin.logs.workspace_id", "admin.logs.owner_email"],
    },
    {
      path: "src/app/admin/errors/page.tsx",
      fieldKeys: ["admin.errors.search"],
    },
    {
      path: "src/app/admin/api-metrics/page.tsx",
      fieldKeys: ["admin.api_metrics.workspace_id"],
    },
    {
      path: "src/app/admin/posts/page.tsx",
      fieldKeys: ["admin.posts.search"],
    },
    {
      path: "src/app/admin/users/page.tsx",
      fieldKeys: ["admin.users.search"],
    },
  ];

  for (const page of pages) {
    const pageSource = source(page.path);
    assert.match(pageSource, /SearchHistoryInput/, `${page.path} should use SearchHistoryInput`);
    for (const fieldKey of page.fieldKeys) {
      assert.match(pageSource, new RegExp(`fieldKey="${fieldKey.replaceAll(".", "\\.")}"`));
    }
  }
});
