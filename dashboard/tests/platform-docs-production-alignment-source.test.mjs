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
  assert.match(platformDocs, /Watch time/);
  assert.match(platformDocs, /Subscribers gained/);
  assert.doesNotMatch(marketingConfig, /X\/Twitter[^\n]+Free plan available/);
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
