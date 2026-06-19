import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("Docs AI search has a grounded retrieval index with analytics task coverage", async () => {
  const index = await source("src/lib/docs-ai-search-index.ts");

  assert.match(index, /DOCS_AI_INDEX/);
  assert.match(index, /PLATFORM_METRICS/);
  assert.match(index, /searchDocsIndex/);
  assert.match(index, /buildGroundedDocsAnswer/);
  assert.match(index, /no answer without source|source coverage/i);
  assert.match(index, /GET \/v1\/accounts\/\{id\}\/metrics/);
  assert.match(index, /GET \/v1\/accounts\/\{account_id\}\/metrics/);
  assert.match(index, /GET \/v1\/analytics\/posts\/export/);
  assert.match(index, /TikTok followers/i);
  assert.match(index, /user\.info\.stats/);
  assert.match(index, /video\.list/);
  assert.match(index, /follower_count/);
  assert.match(index, /instagram|threads|pinterest|facebook|youtube|twitter/i);
});

test("Docs answer API uses AI SDK only after retrieval and keeps a deterministic fallback", async () => {
  const packageJson = JSON.parse(await source("package.json"));
  const route = await source("src/app/api/docs/answer/route.ts");

  assert.ok(packageJson.dependencies?.ai, "dashboard should depend on the Vercel AI SDK");
  assert.match(route, /generateText/);
  assert.match(route, /DOCS_AI_MODEL/);
  assert.match(route, /RATE_LIMIT_WINDOW_MS/);
  assert.match(route, /searchDocsIndex/);
  assert.match(route, /buildGroundedDocsAnswer/);
  assert.match(route, /rate_limited/);
  assert.match(route, /No matching docs|not found in the docs|not enough source coverage/i);
  assert.match(route, /sources/);
  assert.match(route, /confidence/);
  assert.match(route, /docs_ai_answer/);
  assert.match(route, /coverage_reason/);
  assert.match(route, /generated_by/);
});

test("Docs AI feedback endpoint records helpfulness and missing-doc signals", async () => {
  const route = await source("src/app/api/docs/feedback/route.ts");

  assert.match(route, /docs_ai_feedback/);
  assert.match(route, /helpful/);
  assert.match(route, /not_helpful/);
  assert.match(route, /missing_docs/);
  assert.match(route, /query/);
  assert.match(route, /confidence/);
  assert.match(route, /generated_by/);
  assert.match(route, /related/);
});

test("Docs shell preserves classic keyword search while adding task-shaped AI answers", async () => {
  const shell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.match(shell, /Ask UniPost Docs/);
  assert.match(shell, /Classic search/);
  assert.match(shell, /\/api\/docs\/answer/);
  assert.match(shell, /\/api\/docs\/feedback/);
  assert.match(shell, /Search results/);
  assert.match(shell, /Source|Sources/);
  assert.match(shell, /Helpful|Not helpful|Missing docs/);
  assert.match(shell, /confidence:\s*answer\?\.confidence/);
  assert.match(shell, /generated_by:\s*answer\?\.generated_by/);
  assert.match(shell, /related:\s*answer\?\.related/);
});

test("Docs AI answer panel renders answers as structured rich content", async () => {
  const shell = await source("src/app/docs/_components/docs-shell.tsx");

  assert.match(shell, /splitDocsAiAnswerParagraphs/);
  assert.match(shell, /renderDocsRichContent\(paragraph\)/);
  assert.match(shell, /renderDocsRichContent\(step\)/);
  assert.doesNotMatch(shell, /<p>\{answer\.answer\}<\/p>/);
  assert.match(shell, /docs-ai-answer-body/);
  assert.match(shell, /docs-ai-answer-lead/);
  assert.match(shell, /docs-ai-answer-detail/);
  assert.match(shell, /docs-ai-step-marker/);
  assert.match(shell, /docs-ai-source-title/);
});

test("Docs AI browser routes stay public for unauthenticated docs visitors", async () => {
  const proxy = await source("src/proxy.ts");

  assert.match(proxy, /\/api\/docs\/answer/);
  assert.match(proxy, /\/api\/docs\/feedback/);
  assert.match(proxy, /isPublicDocsApi/);
  assert.match(proxy, /isPublicPage \|\| isPublicDocsApi/);
});
