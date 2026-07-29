import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("Admin Errors loads debug only from the selected detail endpoint", () => {
  const api = read("src/lib/api.ts");
  const page = read("src/app/admin/errors/page.tsx");

  assert.match(api, /getAdminPostFailureDebug/);
  assert.match(api, /post-failures\/\$\{encodeURIComponent\(id\)\}\/debug/);
  assert.match(api, /debug_curl:\s*string\s*\|\s*null/);
  assert.match(api, /original_bytes:\s*number/);
  assert.match(api, /stored_bytes:\s*number/);
  assert.match(api, /truncated:\s*boolean/);
  assert.match(page, /getAdminPostFailureDebug/);
  assert.match(page, /debugLoading/);
  assert.match(page, /debugDetail\?\.truncated/);
  assert.match(page, /debugDetailID/);
  assert.match(page, /requireSuperAdmin/);
  assert.doesNotMatch(page, /selectedFailure\.debug_curl/);
});
