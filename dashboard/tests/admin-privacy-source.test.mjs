import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

function source(path) {
  return readFileSync(resolve(path), "utf8");
}

test("admin privacy helper masks user identifiers with stars", () => {
  const helper = source("src/lib/admin-privacy.ts");

  assert.match(helper, /export function adminUserIdentifierLabel/);
  assert.match(helper, /return "\*\*\*\*\*\*\*\*"/);
  assert.match(helper, /return value;/);
});

test("admin users page exposes Privacy control after Sort and masks user column labels", () => {
  const page = source("src/app/admin/users/page.tsx");

  assert.match(page, /adminUserIdentifierLabel/);
  assert.match(page, /const \[hideUsers, setHideUsers\] = useState\(false\)/);
  assert.match(page, /<option value="show">Privacy: Show Users<\/option>/);
  assert.match(page, /<option value="hide">Privacy: Hide Users<\/option>/);
  assert.ok(
    page.indexOf("value={sort}") < page.indexOf("value={hideUsers ? \"hide\" : \"show\"}"),
    "Privacy dropdown should appear to the right of Sort",
  );
  assert.match(page, /adminUserIdentifierLabel\(u\.email, hideUsers\)/);
  assert.match(page, /adminUserIdentifierLabel\(u\.id\.slice\(0, 16\), hideUsers\)/);
});

test("admin posts page exposes Privacy control after days and masks user labels", () => {
  const page = source("src/app/admin/posts/page.tsx");

  assert.match(page, /adminUserIdentifierLabel/);
  assert.match(page, /const \[hideUsers, setHideUsers\] = useState\(false\)/);
  assert.match(page, /<option value="show">Privacy: Show Users<\/option>/);
  assert.match(page, /<option value="hide">Privacy: Hide Users<\/option>/);
  assert.ok(
    page.indexOf("value={days}") < page.indexOf("value={hideUsers ? \"hide\" : \"show\"}"),
    "Privacy dropdown should appear to the right of Last days",
  );
  assert.match(page, /adminUserIdentifierLabel\(u\.email, hideUsers\)/);
  assert.match(page, /adminUserIdentifierLabel\(post\.user_email, hideUsers\)/);
  assert.match(page, /adminUserIdentifierLabel\(selectedPost\.user_email, hideUsers\)/);
});
