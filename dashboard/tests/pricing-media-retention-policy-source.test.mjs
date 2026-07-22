import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("pricing ends with a low-emphasis non-retroactive media retention note", async () => {
  const pricing = await source("src/app/pricing/pricing-page-client.tsx");

  const faqIndex = pricing.indexOf('className="pr-faq-grid"');
  const faqCloseIndex = pricing.indexOf("\n        </div>", faqIndex);
  const noteIndex = pricing.indexOf('className="pr-retention-policy-note"');

  assert.ok(faqIndex >= 0, "Pricing FAQ grid should exist");
  assert.ok(faqCloseIndex > faqIndex, "Pricing FAQ grid should have a closing tag");
  assert.ok(noteIndex > faqCloseIndex, "retention policy note should render after the FAQ grid");
  assert.match(
    pricing,
    /Media retention is based on the workspace plan in effect when the retention period begins\./,
  );
  assert.match(
    pricing,
    /Later plan upgrades or downgrades do not retroactively extend or shorten an existing retention period\./,
  );
  assert.match(
    pricing,
    /\.pr-retention-policy-note\{[^}]*border-top:1px solid var\(--pr-border\)[^}]*font-size:12px[^}]*color:var\(--pr-muted2\)/,
  );
});
