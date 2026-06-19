import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

test("API reference overview does not list the error contract as an endpoint", async () => {
  const source = await readFile(join(root, "src/app/docs/api/page.tsx"), "utf8");

  assert.doesNotMatch(source, /label:\s*"Error contract"/, "the error contract is documentation, not an API endpoint card");
  assert.doesNotMatch(source, /method:\s*"GET"[\s\S]*?path:\s*"\/docs\/api\/errors"/, "documentation pages must not be labeled as GET API routes");
  assert.doesNotMatch(source, /path:\s*"\/docs\/api\/errors"/, "documentation paths should stay out of the API endpoint overview");
});
