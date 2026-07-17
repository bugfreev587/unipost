import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing page keeps Enterprise out of the self-serve card grid", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");
  const tiersStart = pricing.indexOf("const TIERS");
  const tiersEnd = pricing.indexOf("];", tiersStart);
  const tiersSource = pricing.slice(tiersStart, tiersEnd);

  assert.doesNotMatch(tiersSource, /id:\s*"enterprise"/);
  assert.match(pricing, /Dedicated support, capacity planning, and custom platform-volume terms for high-scale teams\./);
  assert.match(pricing, /Team has no monthly UniPost post quota/);
  assert.match(pricing, /Contact sales/);
  assert.doesNotMatch(pricing, /Reserved capacity, SLA, and custom platform-volume terms for high-scale teams\./);

  const cardsIndex = pricing.indexOf("{/* CARDS */}");
  const enterpriseIndex = pricing.indexOf("{/* Enterprise */}");
  const compareIndex = pricing.indexOf("{/* Compare */}");

  assert.ok(cardsIndex >= 0, "pricing cards marker should exist");
  assert.ok(enterpriseIndex > cardsIndex, "Enterprise should render after pricing cards");
  assert.ok(compareIndex > enterpriseIndex, "Enterprise should render before the comparison chart");
});

test("pricing page styles Enterprise benefits and nearby cards as one group", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  assert.match(pricing, /\.pr-ent-chip\{[^}]*background:color-mix\(in srgb,var\(--pr-accent\) 10%,#fff\)/);
  assert.match(pricing, /\.pr-ent-chip\{[^}]*font-weight:650/);
  assert.match(pricing, /\.pr-ent\+\.pr-soft\{margin-bottom:18px\}/);
});

test("pricing FAQ explains Team unlimited and Enterprise Custom semantics", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  assert.match(pricing, /What does unlimited Team usage mean\?/);
  assert.match(pricing, /When do I need Enterprise instead of Team\?/);
  assert.match(pricing, /Can Enterprise increase third-party platform quotas\?/);
  assert.match(pricing, /Custom means contract-defined terms/);
  assert.match(pricing, /not a smaller quota than Team/);
});

test("docs pricing describes Enterprise as custom contract terms", async () => {
  const docsPricing = await source("src/app/docs/pricing/page.tsx");

  assert.match(docsPricing, /\["Enterprise",\s*"Custom",\s*"Contract"/);
  assert.match(docsPricing, /Enterprise Custom means contract-defined terms/);
  assert.match(docsPricing, /may include no UniPost monthly post quota/);
  assert.match(docsPricing, /cannot override platform-owned rate limits/);
});
