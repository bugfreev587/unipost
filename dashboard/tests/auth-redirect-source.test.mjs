import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const layoutPath = path.join(root, "src/app/layout.tsx");

const publicSignupSurfaces = [
  "src/app/marketing/page.tsx",
  "src/app/blog/page.tsx",
  "src/app/blog/[slug]/page.tsx",
  "src/app/tools/_components/public-analytics-tool.tsx",
];

test("Clerk redirects completed auth to the dashboard app host", async () => {
  const source = await readFile(layoutPath, "utf8");

  assert.match(source, /const APP_URL = process\.env\.NEXT_PUBLIC_APP_URL \|\| "https:\/\/app\.unipost\.dev"/);
  assert.match(source, /const SIGN_UP_REDIRECT_URL = `\$\{APP_URL\}\/welcome`/);
  assert.match(source, /signInForceRedirectUrl=\{APP_URL\}/);
  assert.match(source, /signUpForceRedirectUrl=\{SIGN_UP_REDIRECT_URL\}/);
  assert.doesNotMatch(source, /signInForceRedirectUrl="\/"/);
  assert.doesNotMatch(source, /signUpForceRedirectUrl="\/"/);
});

test("public signup CTAs start Clerk registration instead of opening protected /welcome directly", async () => {
  for (const relativePath of publicSignupSurfaces) {
    const source = await readFile(path.join(root, relativePath), "utf8");

    assert.doesNotMatch(source, /START_BUILDING_URL/);
    assert.doesNotMatch(source, /href=[{]?['"]https:\/\/app\.unipost\.dev\/welcome/);
    assert.match(source, /<MarketingCTA\b/);
  }
});
