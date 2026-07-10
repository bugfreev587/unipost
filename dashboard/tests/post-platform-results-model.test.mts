import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

const modelPath = resolve("src/components/posts/details/post-platform-results-model.ts");

async function loadModel() {
  if (!existsSync(modelPath)) return null;
  const source = readFileSync(modelPath, "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const model = await loadModel();

test("shared platform result model exists", () => {
  assert.ok(model, "post-platform-results-model.ts should exist");
});

test("partitions jobs without leaking other platform jobs", { skip: !model }, () => {
  const jobs = [
    { id: "ig-1", social_post_result_id: "ig-result" },
    { id: "tt-1", social_post_result_id: "tt-result" },
  ];

  assert.deepEqual(model!.getJobsForResult(jobs, "ig-result").map((job: { id: string }) => job.id), ["ig-1"]);
  assert.deepEqual(model!.getJobsForResult(jobs, undefined), []);
});

test("describes loading diagnostics", { skip: !model }, () => {
  assert.deepEqual(model!.getQueueDiagnosticsState([], true, null), {
    kind: "loading",
    label: "Queue diagnostics",
  });
});

test("describes unavailable diagnostics", { skip: !model }, () => {
  assert.deepEqual(model!.getQueueDiagnosticsState([], false, "network failed"), {
    kind: "unavailable",
    label: "Queue diagnostics · Unavailable",
  });
});

test("describes scheduled results that are not queued yet", { skip: !model }, () => {
  assert.deepEqual(model!.getQueueDiagnosticsState([], false, null), {
    kind: "not_queued",
    label: "Queue diagnostics · Not queued yet",
  });
});

test("describes populated diagnostics with a job count", { skip: !model }, () => {
  assert.deepEqual(model!.getQueueDiagnosticsState([{ id: "job-1" }], false, null), {
    kind: "ready",
    label: "Queue diagnostics (1)",
  });
});
