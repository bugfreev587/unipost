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

test("docs pricing usage limits use aligned key-value rows", async () => {
  const pricing = await source("src/app/docs/pricing/page.tsx");
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.doesNotMatch(pricing, /className="docs-checklist"/);
  assert.match(pricing, /className="docs-guide-key-values"/);
  assert.match(pricing, /className="docs-guide-key-item"/);
  assert.match(pricing, /className="docs-guide-key-label"/);
  assert.match(pricing, /className="docs-guide-key-copy"/);

  assert.match(docsShell, /\.docs-guide-key-values/);
  assert.match(docsShell, /\.docs-guide-key-item/);
  assert.match(docsShell, /\.docs-guide-key-label/);
  assert.match(docsShell, /-webkit-line-clamp:2/);
});

test("docs pricing tables keep the plan column readable and aligned", async () => {
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.match(docsShell, /case "plan\|posts\/month\|quota behavior\|active scheduled\|api behavior":/);
  assert.match(docsShell, /case "plan\|after success\|after failed, partial, or cancelled":/);
  assert.match(docsShell, /return \["16%", "16%", "18%", "18%", "32%"\]/);
  assert.match(docsShell, /return \["20%", "22%", "58%"\]/);
  assert.match(docsShell, /case "plan\|posts\/month\|quota behavior\|active scheduled\|api behavior":\n\s*case "plan\|after success\|after failed, partial, or cancelled":\n\s*return columnIndex === 0;/);
});
