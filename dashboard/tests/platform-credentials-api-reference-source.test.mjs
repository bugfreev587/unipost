import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

test("Platform Credentials API docs use the API Reference endpoint pattern", async () => {
  const source = await readFile(join(root, "src/app/docs/api/platform-credentials/page.tsx"), "utf8");

  assert.match(source, /ApiReferencePage/, "page should use the API Reference page shell");
  assert.match(source, /ApiEndpointCard/, "page should render endpoint cards");
  assert.doesNotMatch(source, /<DocsPage/, "API Reference page should not use the generic guide shell");

  assert.match(source, /method="POST"[\s\S]*?path="\/v1\/platform-credentials"/, "upload endpoint should be visible");
  assert.match(source, /method="GET"[\s\S]*?path="\/v1\/platform-credentials"/, "list endpoint should be visible");
  assert.match(source, /method="DELETE"[\s\S]*?path="\/v1\/platform-credentials\/:platform"/, "delete endpoint should be visible");

  assert.match(source, /client_secret/, "upload request fields should document client_secret");
  assert.match(source, /client_secret[^]*never returned|never[^]*client_secret/i, "response docs should say secrets are never returned");
});
