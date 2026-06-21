import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("public pricing presents Team as unlimited posts", async () => {
  const pricing = await source("src/app/pricing/page.tsx");

  assert.match(pricing, /posts:\s*"Unlimited posts\/mo"/);
  assert.match(pricing, /team:\s*"Unlimited"/);
  assert.doesNotMatch(pricing, /Need more than 25,000 posts\/month or custom terms/);
  assert.doesNotMatch(pricing, /25,000 posts\/mo/);
});

test("billing settings and docs no longer describe Team as a 25,000 post plan", async () => {
  const billing = await source("src/app/(dashboard)/settings/billing/page.tsx");
  const docsPricing = await source("src/app/docs/pricing/page.tsx");
  const unipostData = await source("src/data/competitors/unipost.ts");
  const terms = await source("src/app/terms/page.tsx");

  assert.match(billing, /id:\s*"team"[^}]*post_limit:\s*-1/s);
  assert.doesNotMatch(billing, /Need more than 25,000 posts\/month or custom terms/);

  assert.match(docsPricing, /\["Team",\s*"\$149",\s*"Unlimited"/);
  assert.match(unipostData, /label:\s*"Team"[^}]*posts:\s*"Unlimited"/s);
  assert.doesNotMatch(terms, /Each plan includes a monthly post limit/);
});
