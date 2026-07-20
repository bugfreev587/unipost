import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { test } from "node:test";

const dashboardRoot = process.cwd();

test("dashboard regression uses Clerk's passwordless Playwright helper", async () => {
  const source = await readFile(
    path.join(dashboardRoot, "tests/regression/dashboard-authenticated.spec.ts"),
    "utf8",
  );

  assert.match(source, /from "@clerk\/testing\/playwright"/);
  assert.match(source, /clerk\.signIn\(\{/);
  assert.match(source, /emailAddress/);
  assert.match(source, /page\.goto\("\/pricing", \{ waitUntil: "domcontentloaded" \}\)/);
  assert.doesNotMatch(source, /DASHBOARD_TEST_PASSWORD/);
  assert.doesNotMatch(source, /input\[type="password"\]/);
});

test("dashboard regression runs Clerk setup before Chromium", async () => {
  const config = await readFile(
    path.join(dashboardRoot, "playwright.regression.config.ts"),
    "utf8",
  );
  const setup = await readFile(
    path.join(dashboardRoot, "tests/regression/clerk.setup.ts"),
    "utf8",
  );

  assert.match(config, /name: "clerk-setup"/);
  assert.match(config, /testMatch: \/clerk\\\.setup\\\.ts\$\//);
  assert.match(config, /name: "authenticated-dashboard"/);
  assert.match(config, /testMatch: \/dashboard-authenticated\\\.spec\\\.ts\$\//);
  assert.match(config, /dependencies: \["authenticated-dashboard"\]/);
  assert.match(setup, /clerkSetup\(\)/);
});

test("mobile landing regression resolves every environment without skipping", async () => {
  const source = await readFile(
    path.join(dashboardRoot, "tests/regression/mobile-layout.spec.ts"),
    "utf8",
  );

  assert.match(source, /dev-app\.unipost\.dev/);
  assert.match(source, /dev\.unipost\.dev/);
  assert.match(source, /landing\.localhost/);
  assert.doesNotMatch(source, /test\.skip\(/);
});
