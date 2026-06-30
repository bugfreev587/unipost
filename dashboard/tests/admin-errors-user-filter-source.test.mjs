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

test("admin errors page syncs filters when URL params change client-side", () => {
  const page = source("src/app/admin/errors/page.tsx");

  assert.match(page, /useSearchParams/);
  assert.match(page, /const searchParams = useSearchParams\(\)/);
  assert.match(page, /<Suspense fallback=\{<AdminShell title="Errors" loading><div \/><\/AdminShell>\}>/);
  assert.match(page, /const \[userIdFilter, setUserIdFilter\]/);
  assert.match(page, /setSearchInput\(nextFilters\.search\)/);
  assert.match(page, /setUserIdFilter\(nextFilters\.userId\)/);
  assert.match(page, /setRange\(nextFilters\.range\)/);
  assert.match(page, /\[searchParams\]/);
});

test("admin errors page ignores stale failure loads after URL filter changes", () => {
  const page = source("src/app/admin/errors/page.tsx");

  assert.match(page, /useRef/);
  assert.match(page, /const loadRequestSeq = useRef\(0\)/);
  assert.match(page, /const requestSeq = loadRequestSeq\.current \+ 1/);
  assert.match(page, /loadRequestSeq\.current = requestSeq/);
  assert.match(page, /if \(requestSeq !== loadRequestSeq\.current\) return;\s+setFailures\(res\.data\)/);
  assert.match(page, /if \(requestSeq === loadRequestSeq\.current\) \{\s+setLoading\(false\)/);
});
