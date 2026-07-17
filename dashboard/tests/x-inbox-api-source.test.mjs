import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const source = readFileSync(resolve("src/lib/api.ts"), "utf8");

test("X Inbox reply attempts retain the server operation id for reconciliation", () => {
  assert.match(source, /res\.headers\.get\("X-UniPost-Operation-Id"\)/);
  assert.match(source, /X_REMOTE_ACCEPTED_RECONCILING/);
  assert.match(source, /X_WRITE_OUTCOME_PENDING/);
  assert.match(source, /X_USAGE_REVERSAL_PENDING/);
  assert.match(source, /X_WRITE_NEEDS_RECONCILIATION/);
});
