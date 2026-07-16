import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

const apiPages = [
  "src/app/docs/api/inbox/page.tsx",
  "src/app/docs/api/inbox/list/page.tsx",
  "src/app/docs/api/inbox/reply/page.tsx",
  "src/app/docs/api/inbox/sync/page.tsx",
];

const guidePages = [
  "src/app/docs/guides/x/comments/page.tsx",
  "src/app/docs/guides/x/direct-messages/page.tsx",
  "src/app/docs/guides/x/reconnect-permissions/page.tsx",
];

const publicPaths = [
  "/docs/api/inbox/list",
  "/docs/api/inbox/reply",
  "/docs/api/inbox/sync",
  "/docs/guides/x/comments",
  "/docs/guides/x/direct-messages",
  "/docs/guides/x/reconnect-permissions",
];

test("Inbox docs publish only the shipped normalized source contract", async () => {
  const inboxReference = await source(apiPages[0]);
  for (const value of ["ig_comment", "ig_dm", "threads_reply", "fb_comment", "fb_dm", "x_reply", "x_dm"]) {
    assert.match(inboxReference, new RegExp(`\\b${value}\\b`), `${value} should be documented`);
  }
  assert.doesNotMatch(inboxReference, /youtube_comment/);
});

test("X Inbox references and guides link to each other in both directions", async () => {
  const [list, reply, sync, comments, directMessages, reconnect] = await Promise.all([
    source(apiPages[1]),
    source(apiPages[2]),
    source(apiPages[3]),
    source(guidePages[0]),
    source(guidePages[1]),
    source(guidePages[2]),
  ]);

  for (const [reference, path] of [
    [list, "/docs/guides/x/comments"],
    [list, "/docs/guides/x/direct-messages"],
    [reply, "/docs/guides/x/comments"],
    [reply, "/docs/guides/x/direct-messages"],
    [sync, "/docs/guides/x/comments"],
    [sync, "/docs/guides/x/direct-messages"],
    [sync, "/docs/guides/x/reconnect-permissions"],
  ]) {
    assert.match(reference, new RegExp(path.replaceAll("/", "\\/")));
  }

  for (const guide of [comments, directMessages, reconnect]) {
    for (const path of ["/docs/api/inbox/list", "/docs/api/inbox/reply", "/docs/api/inbox/sync"]) {
      assert.match(guide, new RegExp(path.replaceAll("/", "\\/")));
    }
    for (const endpoint of ["GET /v1/inbox", "POST /v1/inbox/:id/reply", "POST /v1/inbox/sync"]) {
      assert.match(guide, new RegExp(endpoint.replaceAll("/", "\\/")));
    }
    assert.match(guide, /curl "https:\/\/api\.unipost\.dev\/v1\/inbox/);
  }
});

test("X platform, credential, and workflow pages expose the complete Inbox discovery matrix", async () => {
  const [platformData, platformPage, credentialData, credentialPage, comments, directMessages, reconnect] = await Promise.all([
    source("src/app/docs/platforms/[platform]/_data.tsx"),
    source("src/app/docs/platforms/[platform]/page.tsx"),
    source("src/app/docs/platform-credentials/[platform]/_data.tsx"),
    source("src/app/docs/platform-credentials/[platform]/page.tsx"),
    source(guidePages[0]),
    source(guidePages[1]),
    source(guidePages[2]),
  ]);
  const requiredReferences = [
    "/docs/api/inbox",
    "/docs/api/inbox/list",
    "/docs/api/inbox/reply",
    "/docs/api/inbox/sync",
    "/docs/api/x-credits",
    "/docs/guides/x/credits",
  ];
  const surfaces = [
    ["X platform page", platformData + platformPage, [
      ...requiredReferences,
      "/docs/guides/x/comments",
      "/docs/guides/x/direct-messages",
      "/docs/guides/x/reconnect-permissions",
    ]],
    ["X credential page", credentialData + credentialPage, [
      ...requiredReferences,
      "/docs/guides/x/comments",
      "/docs/guides/x/direct-messages",
      "/docs/guides/x/reconnect-permissions",
    ]],
    ["X comments guide", comments, [
      ...requiredReferences,
      "/docs/guides/x/direct-messages",
      "/docs/guides/x/reconnect-permissions",
    ]],
    ["X direct-message guide", directMessages, [
      ...requiredReferences,
      "/docs/guides/x/comments",
      "/docs/guides/x/reconnect-permissions",
    ]],
    ["X reconnect guide", reconnect, [
      ...requiredReferences,
      "/docs/guides/x/comments",
      "/docs/guides/x/direct-messages",
    ]],
  ];
  for (const [label, corpus, paths] of surfaces) {
    for (const path of paths) {
      assert.match(corpus, new RegExp(path.replaceAll("/", "\\/")), label + " is missing " + path);
    }
  }
  assert.match(platformPage, /data\.inbox\.links\.map/);
  assert.match(credentialPage, /guide\.relatedLinks\.map/);
});

test("X comments and DM guides show the complete confirmation follow-up request", async () => {
  for (const path of [guidePages[0], guidePages[1]]) {
    const guide = await source(path);
    assert.ok(
      guide.match(/curl -X POST "https:\/\/api\.unipost\.dev\/v1\/inbox\/sync"/g)?.length >= 2,
      path + " must show both estimate and confirmed sync calls",
    );
    assert.match(guide, /confirmation_token/);
    assert.match(guide, /CONFIRMATION_TOKEN/);
  }
});

test("X reply reference documents URL and URL-free billing operations exactly", async () => {
  const reply = await source("src/app/docs/api/inbox/reply/page.tsx");
  assert.match(reply, /post\.reply_summoned[^\n]{0,160}10/);
  assert.match(reply, /post\.create_url[^\n]{0,160}200/);
  assert.match(reply, /docs\.example\.com\/releases/);
  assert.match(reply, /"x_credits_counted": 200/);
  assert.match(reply, /"x_credit_operation": "post\.create_url"/);
});

test("X Inbox pages are registered in sidebar, indexes, search, and sitemap", async () => {
  const [shell, apiIndex, guideIndex, searchIndex, sitemap] = await Promise.all([
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
    source("src/app/sitemap.ts"),
  ]);

  for (const path of publicPaths) {
    assert.match(shell, new RegExp(path.replaceAll("/", "\\/")), `${path} missing from sidebar`);
    assert.match(searchIndex, new RegExp(path.replaceAll("/", "\\/")), `${path} missing from search`);
    assert.match(sitemap, new RegExp(path.replaceAll("/", "\\/")), `${path} missing from sitemap`);
  }
  for (const path of publicPaths.slice(0, 3)) {
    assert.match(apiIndex, new RegExp(path.replaceAll("/", "\\/")), `${path} missing from API index`);
  }
  for (const path of publicPaths.slice(3)) {
    assert.match(guideIndex, new RegExp(path.replaceAll("/", "\\/")), `${path} missing from Guides index`);
  }
});

test("X Inbox docs disclose modes, eligibility, caps, errors, and reconnect steps without deferred commercial claims", async () => {
  const files = await Promise.all([
    ...apiPages.map(source),
    ...guidePages.map(source),
    source("src/app/docs/api/accounts/capabilities/page.tsx"),
    source("src/app/docs/api/errors/page.tsx"),
    source("src/app/docs/platforms/[platform]/_data.tsx"),
    source("src/app/docs/platform-credentials/[platform]/_data.tsx"),
  ]);
  const corpus = files.join("\n");

  for (const phrase of [
    "unipost_managed_app",
    "workspace_x_app",
    "Basic plan or higher",
    "x_monthly_usage_limit_exceeded",
    "x_inbound_daily_cap_exceeded",
    "dm.read",
    "dm.write",
    "tweet.read",
    "tweet.write",
    "users.read",
    "offline.access",
  ]) {
    assert.match(corpus, new RegExp(phrase.replaceAll(".", "\\."), "i"), `${phrase} disclosure missing`);
  }

  for (const excluded of [
    /youtube_comment/i,
    /top[- ]?up/i,
    /auto[- ]?top[- ]?up/i,
    /xchat/i,
    /upstream[^\n]{0,80}\$\d/i,
    /\bmargin(?:s)?\b/i,
  ]) {
    assert.doesNotMatch(corpus, excluded);
  }
});
