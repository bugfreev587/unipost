import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Platform Credentials API docs are split into endpoint pages", async () => {
  const groupSource = await readFile(join(root, "src/app/docs/api/platform-credentials/page.tsx"), "utf8");

  assert.match(groupSource, /redirect\("\/docs\/api\/platform-credentials\/create"\)/, "group page should redirect to the create endpoint");
  assert.doesNotMatch(groupSource, /ApiEndpointCard/, "group page should not contain the combined endpoint reference");

  const createSource = await source("src/app/docs/api/platform-credentials/create/page.tsx");
  const listSource = await source("src/app/docs/api/platform-credentials/list/page.tsx");
  const deleteSource = await source("src/app/docs/api/platform-credentials/delete/page.tsx");

  assert.match(createSource, /SingleEndpointReferencePage/, "create page should use the standard single-endpoint shell");
  assert.match(createSource, /title="Upload platform credentials"/);
  assert.match(createSource, /method="POST"/);
  assert.match(createSource, /path="\/v1\/platform-credentials"/);
  assert.match(createSource, /client_secret/, "upload request fields should document client_secret");
  assert.match(createSource, /client_secret[^]*never returned|never[^]*client_secret/i, "create response docs should say secrets are never returned");
  assert.doesNotMatch(createSource, /method="GET"/, "create page should not render the list endpoint");
  assert.doesNotMatch(createSource, /method="DELETE"/, "create page should not render the delete endpoint");

  assert.match(listSource, /SingleEndpointReferencePage/, "list page should use the standard single-endpoint shell");
  assert.match(listSource, /title="List platform credentials"/);
  assert.match(listSource, /method="GET"/);
  assert.match(listSource, /path="\/v1\/platform-credentials"/);
  assert.match(listSource, /data\[\]\.client_id/);
  assert.match(listSource, /client_secret[^]*never returned|never[^]*client_secret/i, "list page should say secrets are never returned");
  assert.doesNotMatch(listSource, /method="POST"/, "list page should not render the create endpoint");
  assert.doesNotMatch(listSource, /method="DELETE"/, "list page should not render the delete endpoint");

  assert.match(deleteSource, /SingleEndpointReferencePage/, "delete page should use the standard single-endpoint shell");
  assert.match(deleteSource, /title="Delete platform credentials"/);
  assert.match(deleteSource, /method="DELETE"/);
  assert.match(deleteSource, /path="\/v1\/platform-credentials\/:platform"/);
  assert.match(deleteSource, /204 No Content/);
  assert.doesNotMatch(deleteSource, /method="POST"/, "delete page should not render the create endpoint");
  assert.doesNotMatch(deleteSource, /method="GET"/, "delete page should not render the list endpoint");
});

test("API Reference sidebar lists Errors as the final Core item", async () => {
  const docsShellSource = await source("src/app/docs/_components/docs-shell.tsx");
  const apiReferenceStart = docsShellSource.indexOf('"api-reference": [');
  assert.notEqual(apiReferenceStart, -1, "API Reference sidebar config should exist");
  const apiReferenceSource = docsShellSource.slice(apiReferenceStart);
  const coreStart = apiReferenceSource.indexOf('title: "Core"');
  const publishingStart = apiReferenceSource.indexOf('title: "Publishing"', coreStart);
  assert.notEqual(coreStart, -1, "Core sidebar group should exist");
  assert.notEqual(publishingStart, -1, "Publishing sidebar group should follow Core");
  const coreGroup = apiReferenceSource.slice(coreStart, publishingStart);
  const platformGroupMatch = apiReferenceSource.match(/label: "Platform Credentials",\s*children: \[[\s\S]*?\n\s*\],\n\s*\}/);

  assert.ok(platformGroupMatch, "Platform Credentials should be a sidebar group with children");
  const platformGroup = platformGroupMatch[0];

  assert.match(platformGroup, /label: "Upload credentials", href: "\/docs\/api\/platform-credentials\/create", method: "POST"/);
  assert.match(platformGroup, /label: "List credentials", href: "\/docs\/api\/platform-credentials\/list", method: "GET"/);
  assert.match(platformGroup, /label: "Delete credentials", href: "\/docs\/api\/platform-credentials\/delete", method: "DELETE"/);
  assert.doesNotMatch(platformGroup, /label: "Errors", href: "\/docs\/api\/errors"/);

  assert.match(coreGroup, /label: "Errors", href: "\/docs\/api\/errors"/, "Errors should be a standalone Core item");
  assert.match(coreGroup, /label: "Platform Credentials"[\s\S]*?\},\n\s*\{ label: "Errors", href: "\/docs\/api\/errors" \},\n\s*\],\n\s*\}/, "Errors should be the final Core item after Platform Credentials");
});
