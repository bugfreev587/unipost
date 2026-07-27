import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  try {
    return await readFile(join(root, path), "utf8");
  } catch {
    return "";
  }
}

test("API plan is hidden from new-user pricing and upgrade surfaces", async () => {
  const visibility = await source("src/lib/plan-visibility.ts");
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");
  const billing = await source("src/app/(dashboard)/settings/billing/page.tsx");

  assert.match(visibility, /planId !== "api"/);
  assert.match(visibility, /isPlanVisibleToNewUsers\(planId\) \|\| planId === currentPlanId/);
  assert.match(pricing, /const PUBLIC_TIERS = TIERS\.filter\(\(tier\) => isPlanVisibleToNewUsers\(tier\.id\)\)/);
  assert.match(pricing, /const PUBLIC_COMPARE_PLAN_IDS = COMPARE_PLAN_IDS\.filter\(isPlanVisibleToNewUsers\)/);
  assert.match(pricing, /const PUBLIC_X_CREDIT_PLANS = X_CREDIT_PLANS\.filter\(\(plan\) => isPlanVisibleToNewUsers\(plan\.id\)\)/);
  assert.match(pricing, /PUBLIC_TIERS\.map/);
  assert.match(pricing, /PUBLIC_COMPARE_PLAN_IDS\.map/);
  assert.equal((pricing.match(/PUBLIC_X_CREDIT_PLANS\.map/g) ?? []).length, 2);
  assert.doesNotMatch(pricing, /repeat\(5,/);
  assert.match(pricing, /api: false/);
  assert.match(billing, /PLANS\.filter\(\(plan\) => isPlanVisibleInBilling\(plan\.id, billing\?\.plan\)\)/);
  assert.doesNotMatch(pricing, /Why Free, API, Basic, Growth, Team\?/);
  assert.doesNotMatch(billing, /Free \/ API \/ Basic \/ Growth \/ Team/);
});
