import assert from "node:assert/strict";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import test, { after } from "node:test";
import ts from "typescript";

const root = process.cwd();
const tempDir = join(root, ".tmp-docs-ai-answer-formatting");

async function loadFormattingModule() {
  await rm(tempDir, { force: true, recursive: true });
  const outPath = join(tempDir, "src/lib/docs-ai-answer-formatting.cjs");
  const source = await readFile(join(root, "src/lib/docs-ai-answer-formatting.ts"), "utf8");
  const output = ts.transpileModule(source, {
    compilerOptions: {
      esModuleInterop: true,
      module: ts.ModuleKind.CommonJS,
      moduleResolution: ts.ModuleResolutionKind.NodeJs,
      target: ts.ScriptTarget.ES2020,
    },
    fileName: "src/lib/docs-ai-answer-formatting.ts",
  }).outputText;

  await mkdir(dirname(outPath), { recursive: true });
  await writeFile(outPath, output);

  const require = createRequire(import.meta.url);
  return require(outPath);
}

const formatting = await loadFormattingModule();

after(async () => {
  await rm(tempDir, { force: true, recursive: true });
});

test("Docs AI answer paragraph splitting preserves dotted field names", () => {
  const answer = "Use Connect Sessions for customer-owned TikTok account connection. Call POST /v1/connect/sessions with platform set to tiktok, your external_user_id, profile_id when needed, and return_url when you want the browser sent back to your app; then send the returned data.url to the user for TikTok authorization.";
  const paragraphs = formatting.splitDocsAiAnswerParagraphs(answer);

  assert.equal(paragraphs.length, 2);
  assert.equal(paragraphs[0], "Use Connect Sessions for customer-owned TikTok account connection.");
  assert.match(paragraphs[1], /POST \/v1\/connect\/sessions/);
  assert.match(paragraphs[1], /data\.url/);
});

test("Docs AI answer paragraph splitting breaks long semicolon-only answers at action boundaries", () => {
  const answer = "Use the account metrics endpoint for TikTok follower count; then read data.follower_count from the response; store the account id from GET /v1/accounts before calling metrics.";
  const paragraphs = formatting.splitDocsAiAnswerParagraphs(answer);

  assert.ok(paragraphs.length > 1);
  assert.match(paragraphs.join(" "), /GET \/v1\/accounts/);
  assert.match(paragraphs.join(" "), /data\.follower_count/);
});
