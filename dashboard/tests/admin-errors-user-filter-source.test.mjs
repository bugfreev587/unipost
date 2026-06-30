import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin errors API supports exact user and this-month filters", () => {
  const api = source("src/lib/api.ts");

  assert.match(api, /user_id\?: string;/);
  assert.match(api, /period\?: "this_month";/);
  assert.match(api, /qs\.set\("user_id", params\.user_id\)/);
  assert.match(api, /qs\.set\("period", params\.period\)/);
});

test("admin errors page reads user_id and period from URL", () => {
  const page = source("src/app/admin/errors/page.tsx");

  assert.match(page, /params\.get\("user_id"\)/);
  assert.match(page, /params\.get\("period"\) === "this_month"/);
  assert.match(page, /user_id: userIdFilter \|\| undefined/);
  assert.match(page, /period: range === "this_month" \? "this_month" : undefined/);
  assert.match(page, /value="this_month"/);
});
