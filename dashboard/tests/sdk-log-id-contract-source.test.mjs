import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const repoSource = (path) => readFile(new URL(`../../${path}`, import.meta.url), "utf8");

test("observability v2 stays locked while public SDK validation is numeric-only", async () => {
  const [js, python, go, java, definitions, adminTest, runbook, plan] = await Promise.all([
    repoSource("scripts/sdk-validation/js/unipost-sdk-test.mjs"),
    repoSource("scripts/sdk-validation/python/unipost_sdk_test.py"),
    repoSource("scripts/sdk-validation/go/main.go"),
    repoSource("scripts/sdk-validation/java/src/main/java/dev/unipost/validation/UnipostSdkTest.java"),
    repoSource("api/internal/featureflags/featureflags.go"),
    repoSource("api/internal/handler/admin_feature_flags_test.go"),
    repoSource("docs/feature-flags-unleash.md"),
    repoSource("docs/superpowers/plans/2026-07-29-observability-reads-v2.md"),
  ]);

  assert.match(js, /Number\(log\.id\) === Number\(firstLog\.id\)/);
  assert.match(python, /int\(payload\["id"\]\) == int\(log_id\)/);
  assert.match(go, /firstLog\.ID > 0/);
  assert.match(java, /firstLog\.value\.path\("id"\)\.canConvertToLong\(\)/);
  const observabilityDefinition = definitions.match(/\{\s*Key:\s*ObservabilityReadsV2,[\s\S]*?\n\s*\},/u)?.[0] ?? "";
  assert.doesNotMatch(observabilityDefinition, /ActivationReady:\s*true/);
  assert.match(observabilityDefinition, /historical Metrics coverage\/parity/i);
  assert.match(observabilityDefinition, /retention safety/i);
  assert.match(adminTest, /StatusConflict/);
  assert.match(adminTest, /FLAG_NOT_READY/);
  assert.match(adminTest, /historical Metrics coverage\/parity/i);
  assert.match(adminTest, /retention-safety/i);
  assert.match(runbook, /all four public SDKs/i);
  assert.match(runbook, /activation_ready=false/i);
  assert.match(runbook, /bounded.*OnPersistedBatch/i);
  assert.match(runbook, /exactly one canonical request/i);
  assert.match(runbook, /OFF.*SSE.*numeric.*no-gap/i);
  assert.match(runbook, /selector failure.*legacy/i);
  assert.match(runbook, /WebSocket.*within 1\.5 seconds/i);
  assert.match(runbook, /one-second.*500ms.*1\.5-second/i);
  assert.match(runbook, /backfill or sufficient warm-up.*every supported 7-90-day Metrics window/i);
  assert.match(runbook, /completion coverage for all target hours/i);
  assert.match(runbook, /legacy parity for counts, status, trends, and percentile behavior/i);
  assert.match(runbook, /sealed workspace-hour.*prevents destructive recompute and data loss/i);
  assert.doesNotMatch(plan, /is now activation-ready/i);
  assert.match(plan, /remains activation-locked pending compatible releases and opaque log ID validation from all four public SDKs/i);
  assert.match(plan, /bounded.*OnPersistedBatch/i);
  assert.match(plan, /exactly one canonical request/i);
  assert.match(plan, /OFF.*SSE.*numeric.*no-gap/i);
  assert.match(plan, /selector failure.*legacy/i);
  assert.match(plan, /WebSocket.*within 1\.5 seconds/i);
  assert.match(plan, /one-second.*500ms.*1\.5-second/i);
  assert.match(plan, /backfill or sufficient warm-up.*every supported 7-90-day Metrics window/i);
  assert.match(plan, /completion coverage for all target hours/i);
  assert.match(plan, /legacy parity for counts, status, trends, and percentile behavior/i);
  assert.match(plan, /sealed workspace-hour.*prevents destructive recompute and data loss/i);
});
