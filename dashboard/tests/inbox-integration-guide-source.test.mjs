import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  try {
    return await readFile(join(root, path), "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") return "";
    throw error;
  }
}

const guidePath = "src/app/docs/guides/inbox-integration/page.tsx";
const overviewPath = "src/app/docs/api/inbox/page.tsx";

test("Inbox API overview matches the deployed router and auth contract", async () => {
  const overview = await source(overviewPath);
  for (const endpoint of [
    "GET /v1/inbox",
    "GET /v1/inbox/unread-count",
    "GET /v1/inbox/{id}",
    "POST /v1/inbox/{id}/read",
    "POST /v1/inbox/mark-all-read",
    "POST /v1/inbox/{id}/reply",
    "POST /v1/inbox/{id}/thread-state",
    "GET /v1/inbox/{id}/media-context",
    "POST /v1/inbox/sync",
    "GET /v1/inbox/x-outbound-operations/{requestID}",
    "GET /v1/inbox/ws",
  ]) {
    assert.match(overview, new RegExp(endpoint.replaceAll("/", "\\/")));
  }
  assert.match(overview, /selected scope/i);
  assert.match(overview, /INBOX_SCOPE_LOOKUP_FAILED/);
  assert.match(overview, /scope[^\n]{0,160}before[^\n]{0,160}plan/i);
  assert.match(overview, /Clerk[^\n]{0,160}token[^\n]{0,160}query/i);
  assert.match(overview, /API key[^\n]{0,160}Authorization[^\n]{0,160}header/i);
  assert.match(overview, /\/docs\/guides\/inbox-integration/);
});

test("Inbox integration guide derives stable managed-user scope on the server", async () => {
  const guide = await source(guidePath);
  assert.match(guide, /https:\/\/unipost\.dev\/docs\/guides\/inbox-integration/);
  assert.match(guide, /app_usr_7f4c91/);
  assert.match(guide, /external_user_id/);
  assert.match(guide, /opaque/i);
  assert.match(guide, /immutable/i);
  assert.match(guide, /email[^\n]{0,120}(?:mutable|primary)/i);
  assert.match(guide, /authenticated app user/i);
  assert.match(guide, /server-side/i);
  assert.match(guide, /inbox_scope=managed_user/);
  assert.match(guide, /inbox_scope=workspace/);
  assert.doesNotMatch(guide, /youtube_comment/);
});

test("Inbox integration guide covers connect, reads, writes, realtime, errors, and A-B acceptance", async () => {
  const guide = await source(guidePath);
  for (const endpoint of [
    "POST /v1/connect/sessions",
    "GET /v1/inbox",
    "GET /v1/inbox/unread-count",
    "GET /v1/inbox/{id}",
    "POST /v1/inbox/{id}/read",
    "POST /v1/inbox/mark-all-read",
    "POST /v1/inbox/{id}/reply",
    "POST /v1/inbox/{id}/thread-state",
    "POST /v1/inbox/sync",
    "GET /v1/inbox/ws",
  ]) {
    assert.match(guide, new RegExp(endpoint.replaceAll("/", "\\/")));
  }
  assert.match(guide, /npm install ws/);
  assert.match(guide, /Authorization[^\n]{0,160}Bearer/);
  assert.match(guide, /browser[^\n]{0,180}(?:never|must not)[^\n]{0,180}API key/i);
  assert.match(guide, /default[^\n]{0,80}50[^\n]{0,80}(?:cap|maximum)[^\n]{0,80}500/i);
  assert.match(guide, /INBOX_SCOPE_LOOKUP_FAILED/);
  assert.match(guide, /bounded exponential backoff/i);
  assert.match(guide, /ownership conflict/i);
  assert.match(guide, /409/);
  assert.match(guide, /x_backfill/);
  assert.match(guide, /confirmation_token/);
  assert.match(guide, /X Credits/);
  assert.match(guide, /synthetic user A/i);
  assert.match(guide, /synthetic user B/i);
  assert.match(guide, /cross-scope/i);
  assert.match(guide, /remove[^\n]{0,100}synthetic/i);
});

test("Inbox integration guide is discoverable everywhere", async () => {
  const [guideIndex, shell, sitemap, aiIndex] = await Promise.all([
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/sitemap.ts"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);
  for (const corpus of [guideIndex, shell, sitemap, aiIndex]) {
    assert.match(corpus, /\/docs\/guides\/inbox-integration/);
  }
  assert.match(shell, /Inbox Guides/);
  assert.match(aiIndex, /managed user[^\n]{0,200}inbox|inbox[^\n]{0,200}managed user/i);
});
