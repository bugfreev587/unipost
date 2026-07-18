import assert from "node:assert/strict";
import { mkdir, readFile, rm, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import test, { after } from "node:test";
import ts from "typescript";

const root = process.cwd();
const tempDir = join(root, ".tmp-docs-ai-evals");

async function transpileSource(sourcePath, outPath, transforms = []) {
  const source = await readFile(join(root, sourcePath), "utf8");
  const transformed = transforms.reduce((value, transform) => transform(value), source);
  const output = ts.transpileModule(transformed, {
    compilerOptions: {
      esModuleInterop: true,
      module: ts.ModuleKind.CommonJS,
      moduleResolution: ts.ModuleResolutionKind.NodeJs,
      target: ts.ScriptTarget.ES2020,
    },
    fileName: sourcePath,
  }).outputText;

  await mkdir(dirname(outPath), { recursive: true });
  await writeFile(outPath, output);
}

async function loadDocsAiModule() {
  await rm(tempDir, { force: true, recursive: true });
  await mkdir(join(tempDir, "src/lib"), { recursive: true });

  await transpileSource(
    "src/lib/platform-capabilities.ts",
    join(tempDir, "src/lib/platform-capabilities.cjs"),
  );
  await transpileSource(
    "src/lib/docs-ai-search-index.ts",
    join(tempDir, "src/lib/docs-ai-search-index.cjs"),
    [
      (source) => source.replace(
        'from "@/lib/platform-capabilities"',
        'from "./platform-capabilities.cjs"',
      ),
    ],
  );

  const require = createRequire(import.meta.url);
  return require(join(tempDir, "src/lib/docs-ai-search-index.cjs"));
}

const docsAi = await loadDocsAiModule();

after(async () => {
  await rm(tempDir, { force: true, recursive: true });
});

function answerFor(query) {
  const search = docsAi.searchDocsIndex(query, { limit: 5 });
  return {
    search,
    answer: docsAi.buildGroundedDocsAnswer(query, search),
  };
}

function answerForPublicFlags(query, flags) {
  const chunks = docsAi.DOCS_AI_INDEX.filter((chunk) => {
    if (chunk.path === "/docs/guides/x/direct-messages" && !flags.x_dms_v1) return false;
    if (
      ["/docs/guides/x/credits", "/docs/api/x-credits"].includes(chunk.path)
      && !flags.x_credits_billing_v1
    ) return false;
    return !chunk.required_feature || flags[chunk.required_feature];
  });
  const search = docsAi.searchDocsIndex(query, { limit: 5, chunks });
  return docsAi.buildGroundedDocsAnswer(query, search);
}

test("public X comments remain searchable while disabled DM and Credits chunks stay hidden", () => {
  const flags = { x_dms_v1: false, x_credits_billing_v1: false };
  const comments = answerForPublicFlags("How do I reply to X comments?", flags);
  const dms = answerForPublicFlags("How do I use X direct messages?", flags);
  const credits = answerForPublicFlags("How do X Credits work?", flags);

  assert.ok(comments.sources.some((source) => source.path === "/docs/guides/x/comments"));
  assert.ok(dms.sources.every((source) => source.path !== "/docs/guides/x/direct-messages"));
  assert.ok(credits.sources.every((source) => !["/docs/guides/x/credits", "/docs/api/x-credits"].includes(source.path)));
});

test("TikTok connect API questions route to Connect Sessions, not analytics followers", () => {
  const { answer } = answerFor("how to connect tiktok to unipost with API?");

  assert.match(answer.answer, /POST \/v1\/connect\/sessions/);
  assert.match(answer.answer, /data\.url|returned URL|hosted.*URL/i);
  assert.equal(answer.sources[0]?.path, "/docs/connect-sessions");
  assert.equal(answer.sources[1]?.path, "/docs/api/connect/sessions/create");
  assert.notEqual(answer.sources[0]?.path, "/docs/guides/analytics/tiktok-followers");
  assert.ok(
    answer.sources.some((source) => source.path === "/docs/api/connect/sessions/create"),
    "connect API reference should support the guide answer",
  );
});

test("exact create connect session endpoint queries rank API Reference first", () => {
  const { answer } = answerFor("POST /v1/connect/sessions");

  assert.equal(answer.sources[0]?.path, "/docs/api/connect/sessions/create");
  assert.match(answer.answer, /Create a hosted onboarding session|POST \/v1\/connect\/sessions/i);
});

test("TikTok followers questions answer with unified account metrics", () => {
  const { answer } = answerFor("How do I get TikTok followers?");

  assert.match(answer.answer, /GET \/v1\/accounts\/\{account_id\}\/metrics/);
  assert.match(answer.answer, /user\.info\.stats/);
  assert.match(answer.answer, /data\.follower_count/);
  assert.equal(answer.sources[0]?.path, "/docs/guides/analytics/tiktok-followers");
});

test("YouTube Analytics report questions answer with V2 endpoints", () => {
  const { answer } = answerFor("How do I get YouTube watch time trend and top videos?");

  assert.match(answer.answer, /\/youtube\/analytics\/summary/);
  assert.match(answer.answer, /\/youtube\/analytics\/trend/);
  assert.match(answer.answer, /\/youtube\/analytics\/videos/);
  assert.match(answer.answer, /yt-analytics\.readonly/);
});

test("video.list followers questions explain that video.list is not the follower source", () => {
  const { answer } = answerFor("Does video.list give followers?");

  assert.match(answer.answer, /\bno\b/i);
  assert.match(answer.answer, /video\.list/);
  assert.match(answer.answer, /GET \/v1\/accounts\/\{account_id\}\/metrics/);
});

test("unsupported questions return no-answer fallback without cited answer sources", () => {
  const { answer } = answerFor("What is UniPost's refund policy for annual contracts?");

  assert.equal(answer.confidence, "none");
  assert.equal(answer.sources.length, 0);
  assert.match(answer.answer, /could not find enough source coverage|not found in the docs/i);
});
