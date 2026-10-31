import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path) => readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("Facebook Pages is available to regular dashboard users", () => {
  const accounts = read("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
  const shell = read("src/components/dashboard/shell.tsx");
  const features = read("src/lib/features-in-dev.ts");

  assert.match(accounts, /id:\s*"facebook"/);
  assert.doesNotMatch(accounts, /isFacebookEnabledForMe|FACEBOOK_PLATFORM/);
  assert.doesNotMatch(shell, /isFacebookEnabledForMe/);
  assert.doesNotMatch(features, /facebook_pages/);
});
