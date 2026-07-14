import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const read = (path) => readFile(new URL(`../${path}`, import.meta.url), "utf8");

test("platform docs do not retain obsolete production claims", async () => {
  const [platformDocs, marketingConfig, marketingPage] = await Promise.all([
    read("src/app/docs/platforms/[platform]/_data.tsx"),
    read("src/app/(platforms)/_config/platforms.ts"),
    read("src/app/(platforms)/_components/platform-page.tsx"),
  ]);

  assert.doesNotMatch(platformDocs, /X removed the shared app path|No shared Quickstart app/);
  assert.doesNotMatch(platformDocs, /FEATURE_FACEBOOK_REELS|facebook_reels_unsupported/);
  assert.doesNotMatch(platformDocs, /"Impressions", yes, "Supported"\],\n      \["Reach", yes, "Supported"\],\n      \["Likes", yes, "Supported"/);
  assert.match(platformDocs, /"Saves \/ bookmarks", yes, "Supported"/);
  assert.match(platformDocs, /"Formats", "JPEG, PNG"/);
  assert.doesNotMatch(platformDocs, /WebP and GIF accepted but may fail/);
  assert.match(platformDocs, /Watch time/);
  assert.match(platformDocs, /Subscribers gained/);
  assert.doesNotMatch(marketingConfig, /X\/Twitter[^\n]+Free plan available/);
  assert.match(marketingConfig, /Quickstart uses UniPost's managed X app/);
  assert.doesNotMatch(marketingConfig, /Validated: impressions, reach, likes, comments, shares, clicks/);
  assert.match(marketingConfig, /Production socialActions coverage: likes and comments/);
  assert.doesNotMatch(marketingConfig, /Can I post Instagram Stories\?", a: "Not yet/);
  assert.doesNotMatch(marketingConfig, /Shorts are vertical videos under 60 seconds/);
  assert.match(marketingPage, />9 platforms supported</);
});

test("connection guides keep OAuth and app-password modes distinct", async () => {
  const [quickstart, connectSessions, platformCredentials, whiteLabel, platformDocs] = await Promise.all([
    read("src/app/docs/quickstart/page.tsx"),
    read("src/app/docs/connect-sessions/page.tsx"),
    read("src/app/docs/platform-credentials/page.tsx"),
    read("src/app/docs/white-label/page.tsx"),
    read("src/app/docs/platforms/[platform]/_data.tsx"),
  ]);

  assert.match(quickstart, /X/);
  assert.match(connectSessions, /allow_quickstart_creds=true/);
  assert.match(platformCredentials, /shared OAuth app/);
  assert.match(whiteLabel, /shared OAuth app/);
  assert.match(platformDocs, /Handle \+ app password — no OAuth/);
});

test("Platform Credentials documents every production OAuth callback", async () => {
  const [credentialGuides, credentialOverview, createReference, dashboardCredentials] = await Promise.all([
    read("src/app/docs/platform-credentials/[platform]/_data.tsx"),
    read("src/app/docs/platform-credentials/page.tsx"),
    read("src/app/docs/api/platform-credentials/create/page.tsx"),
    read("src/app/(dashboard)/projects/[id]/credentials/page.tsx"),
  ]);

  for (const platform of ["instagram", "threads", "facebook", "linkedin", "pinterest", "tiktok", "youtube", "twitter"]) {
    assert.match(credentialGuides, new RegExp(`/v1/oauth/callback/${platform}`), `missing workspace callback for ${platform}`);
    assert.match(credentialGuides, new RegExp(`/v1/connect/callback/${platform}`), `missing Connect callback for ${platform}`);
  }

  assert.match(credentialGuides, /slug: "pinterest"/);
  assert.match(credentialOverview, /pinterest: "\/docs\/platform-credentials\/pinterest"/);
  assert.match(createReference, /<code>threads<\/code>/);
  assert.match(dashboardCredentials, /docs: "\/docs\/platform-credentials\/pinterest"/);
});
