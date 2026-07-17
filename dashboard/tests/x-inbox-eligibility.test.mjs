import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadEligibility() {
  const source = readFileSync(resolve("src/lib/x-inbox-eligibility.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

test("missing X DM scopes requests reconnect without hiding publishing or comments", async () => {
  const { evaluateXInboxEligibility } = await loadEligibility();
  const result = evaluateXInboxEligibility(
    {
      status: "active",
      scope: ["tweet.read", "tweet.write", "users.read"],
    },
    {
      comments_enabled: true,
      dms_enabled: false,
      missing_scopes: ["dm.read", "dm.write"],
      reconnect_required: true,
      delivery_status: "pending",
      app_mode: "unipost_managed_app",
      missing_app_credentials: [],
    },
  );

  assert.equal(result.publishingEnabled, true);
  assert.equal(result.commentsEnabled, true);
  assert.equal(result.dmsEnabled, false);
  assert.equal(result.reconnectRequired, true);
  assert.deepEqual(result.missingScopes, ["dm.read", "dm.write"]);
  assert.match(result.summary, /Reconnect.*DM/i);
});

test("plan-paused X Inbox does not suggest reconnect", async () => {
  const { evaluateXInboxEligibility } = await loadEligibility();
  const result = evaluateXInboxEligibility(
    {
      status: "active",
      scope: ["tweet.read", "tweet.write", "users.read", "dm.read", "dm.write"],
    },
    {
      comments_enabled: false,
      dms_enabled: false,
      missing_scopes: [],
      reconnect_required: false,
      delivery_status: "paused_plan",
      app_mode: "unipost_managed_app",
      missing_app_credentials: [],
    },
  );

  assert.equal(result.publishingEnabled, true);
  assert.equal(result.reconnectRequired, false);
  assert.match(result.summary, /plan/i);
});
