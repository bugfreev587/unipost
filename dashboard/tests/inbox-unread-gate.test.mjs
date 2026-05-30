import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadGateModule() {
  const source = readFileSync(resolve("src/components/dashboard/inbox-unread-gate.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const { shouldLoadGlobalInboxUnreadCount } = await loadGateModule();

test("sidebar unread count loads only when profile, flag, and plan all allow inbox", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      inboxFeatureEnabled: true,
      planAllowsInbox: true,
    }),
    true,
  );
});

test("sidebar unread count does not load for plans without inbox", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      inboxFeatureEnabled: true,
      planAllowsInbox: false,
    }),
    false,
  );
});

test("sidebar unread count waits until plan allowance has loaded", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      inboxFeatureEnabled: true,
      planAllowsInbox: null,
    }),
    false,
  );
});
