import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const dashboardRootPath = path.join(root, "src/app/(dashboard)/page.tsx");

test("Dashboard root resolver does not leave users on Loading forever", async () => {
  const source = await readFile(dashboardRootPath, "utf8");

  assert.match(source, /const AUTH_LOAD_TIMEOUT_MS = \d+/);
  assert.match(source, /const BOOTSTRAP_TIMEOUT_MS = \d+/);
  assert.match(source, /function withTimeout<T>/);
  assert.match(source, /setAuthTimedOut\(true\)/);
  assert.match(source, /setBootstrapTimedOut\(true\)/);
  assert.match(source, /await withTimeout\(getToken\(\), BOOTSTRAP_TIMEOUT_MS, "Clerk token"\)/);
  assert.match(source, /await withTimeout\(getBootstrap\(token\), BOOTSTRAP_TIMEOUT_MS, "Dashboard bootstrap"\)/);
  assert.match(source, /router\.replace\("\/projects"\)/);
  assert.match(source, /Retry loading dashboard/);
  assert.match(source, /Open profiles/);
  assert.doesNotMatch(source, /if \(!token \|\| cancelled\) return;/);
});
