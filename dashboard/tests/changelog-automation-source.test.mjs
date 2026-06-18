import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { test } from "node:test";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..", "..");

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("admin changelog action page requires super admin and confirms explicit signed actions", async () => {
  const page = await source("dashboard/src/app/admin/changelog-actions/page.tsx");

  assert.match(page, /AdminShell/);
  assert.match(page, /requireSuperAdmin/);
  assert.match(page, /useSearchParams/);
  assert.match(page, /getAdminChangelogCandidate/);
  assert.match(page, /confirmAdminChangelogCandidateAction/);
  assert.match(page, /Publish/);
  assert.match(page, /Save for later/);
  assert.match(page, /Discard/);
  assert.match(page, /Already handled/);
});

test("api client exposes changelog candidate preview and action helpers", async () => {
  const api = await source("dashboard/src/lib/api.ts");

  assert.match(api, /AdminChangelogCandidate/);
  assert.match(api, /getAdminChangelogCandidate/);
  assert.match(api, /confirmAdminChangelogCandidateAction/);
  assert.match(api, /\/v1\/admin\/changelog-candidates\/\$\{encodeURIComponent\(candidateId\)\}/);
});

test("publish workflow and daily workflow exist with safe triggers", async () => {
  const daily = await source(".github/workflows/changelog-daily.yml");
  const publish = await source(".github/workflows/changelog-publish.yml");

  assert.match(daily, /timezone:\s*"America\/Los_Angeles"/);
  assert.match(daily, /scripts\/changelog-automation\/daily\.mjs/);
  assert.match(publish, /workflow_dispatch/);
  assert.match(publish, /candidate_id/);
  assert.match(publish, /CHANGELOG_RELEASE_GITHUB_TOKEN/);
});
