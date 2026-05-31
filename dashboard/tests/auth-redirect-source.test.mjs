import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const layoutPath = path.join(root, "src/app/layout.tsx");

test("Clerk redirects completed auth to the dashboard app host", async () => {
  const source = await readFile(layoutPath, "utf8");

  assert.match(source, /const APP_URL = process\.env\.NEXT_PUBLIC_APP_URL \|\| "https:\/\/app\.unipost\.dev"/);
  assert.match(source, /const SIGN_UP_REDIRECT_URL = `\$\{APP_URL\}\/welcome`/);
  assert.match(source, /signInForceRedirectUrl=\{APP_URL\}/);
  assert.match(source, /signUpForceRedirectUrl=\{SIGN_UP_REDIRECT_URL\}/);
  assert.doesNotMatch(source, /signInForceRedirectUrl="\/"/);
  assert.doesNotMatch(source, /signUpForceRedirectUrl="\/"/);
});
