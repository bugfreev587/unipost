import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadStateModule() {
  const source = readFileSync(resolve("src/components/analytics/tiktok-analytics-state.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const { scopeReadinessState, tiktokAnalyticsIssue } = await loadStateModule();

function apiError(rawCode, reason) {
  return Object.assign(new Error("provider detail"), {
    rawCode,
    details: reason ? { reason } : undefined,
  });
}

test("TikTok analytics errors map stable API reasons to user guidance", () => {
  const cases = [
    ["ACCOUNT_DISCONNECTED", "account_disconnected", "This TikTok account is disconnected. Reconnect it to continue."],
    ["NEEDS_RECONNECT", "account_token_invalid", "Your TikTok connection has expired. Reconnect the account."],
    ["NEEDS_RECONNECT", "analytics_scope_required", "Reconnect TikTok to grant the permissions required for analytics."],
    ["UPSTREAM_RATE_LIMITED", "provider_rate_limited", "TikTok is temporarily rate limiting analytics requests. Try again later."],
    ["TIKTOK_TEMPORARY_ERROR", "provider_temporary_error", "TikTok analytics are temporarily unavailable. Try again later."],
    ["TIKTOK_ANALYTICS_UNAVAILABLE", "video_not_found", "TikTok analytics are not available for this video yet."],
    ["TIKTOK_ANALYTICS_UNAVAILABLE", "video_not_ready", "TikTok analytics are not available for this video yet."],
  ];
  for (const [code, reason, message] of cases) {
    assert.deepEqual(tiktokAnalyticsIssue(apiError(code, reason)), { code, reason, message });
  }
});

test("legacy NEEDS_RECONNECT falls back to expired-token guidance", () => {
  assert.equal(
    tiktokAnalyticsIssue(apiError("NEEDS_RECONNECT")).message,
    "Your TikTok connection has expired. Reconnect the account.",
  );
});

test("stored missing scopes alone do not claim reconnect is required", () => {
  const state = scopeReadinessState(["video.list"], undefined);
  assert.equal(state.badge, "Verify");
  assert.notEqual(state.title, "Reconnect required for analytics");
});

test("runtime scope response makes readiness show reconnect required", () => {
  const state = scopeReadinessState(["video.list"], "analytics_scope_required");
  assert.equal(state.badge, "Reconnect");
  assert.equal(state.title, "Reconnect required for analytics");
});
