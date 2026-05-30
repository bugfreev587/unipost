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

test("sidebar unread count loads only when profile and plan allow inbox", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      planAllowsInbox: true,
    }),
    true,
  );
});

test("sidebar unread count does not load for plans without inbox", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      planAllowsInbox: false,
    }),
    false,
  );
});

test("sidebar unread count waits until plan allowance has loaded", () => {
  assert.equal(
    shouldLoadGlobalInboxUnreadCount({
      profileId: "profile_123",
      planAllowsInbox: null,
    }),
    false,
  );
});

test("dashboard shell does not fetch global limits just to gate inbox unread state", () => {
  const source = readFileSync(resolve("src/components/dashboard/shell.tsx"), "utf8");
  assert.equal(source.includes("getApiLimits"), false);
  assert.equal(source.includes("FEATURE_FLAG_KEYS.inbox"), false);
});
