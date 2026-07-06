import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("docs pricing uses the shared guide surface instead of legacy card grids", async () => {
  const pricing = await source("src/app/docs/pricing/page.tsx");
  const connectSessions = await source("src/app/docs/connect-sessions/page.tsx");
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");

  for (const className of [
    "docs-summary-grid",
    "docs-summary-card",
    "docs-next-grid",
    "docs-next-card",
  ]) {
    assert.doesNotMatch(pricing, new RegExp(className));
  }

  assert.match(pricing, /className="docs-guide-badges"/);
  assert.match(pricing, /className="docs-guide-next"/);
  assert.match(connectSessions, /className="docs-guide-badges"/);
  assert.match(connectSessions, /className="docs-guide-next"/);
  assert.doesNotMatch(connectSessions, /dangerouslySetInnerHTML/);

  assert.match(docsShell, /\.docs-guide-badges/);
  assert.match(docsShell, /\.docs-guide-next-card/);
  assert.match(docsShell, /current === "\/docs\/pricing"/);
});
