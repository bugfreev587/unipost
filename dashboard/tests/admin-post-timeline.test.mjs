import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadTimelineModule() {
  const source = readFileSync(resolve("src/app/admin/posts/timeline.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const { getAdminPostPublishTimeline, formatAdminDurationSeconds } = await loadTimelineModule();

test("scheduled admin posts use their scheduled publish time", () => {
  assert.deepEqual(
    getAdminPostPublishTimeline({
      status: "scheduled",
      scheduled_at: "2026-06-01T16:00:00Z",
      published_at: undefined,
    }),
    { label: "scheduled", at: "2026-06-01T16:00:00Z" },
  );
});

test("published admin posts use their actual published time", () => {
  assert.deepEqual(
    getAdminPostPublishTimeline({
      status: "published",
      scheduled_at: "2026-06-01T16:00:00Z",
      published_at: "2026-06-01T16:08:42Z",
    }),
    { label: "published", at: "2026-06-01T16:08:42Z" },
  );
});

test("admin posts without a publish timeline render no value", () => {
  assert.equal(
    getAdminPostPublishTimeline({
      status: "failed",
      scheduled_at: "2026-06-01T16:00:00Z",
      published_at: undefined,
    }),
    null,
  );
});

test("admin duration renders integer seconds and rejects invalid values", () => {
  assert.equal(formatAdminDurationSeconds(98), "98 s");
  assert.equal(formatAdminDurationSeconds(0), "0 s");
  assert.equal(formatAdminDurationSeconds(undefined), "—");
  assert.equal(formatAdminDurationSeconds(-1), "—");
  assert.equal(formatAdminDurationSeconds(Number.NaN), "—");
});
