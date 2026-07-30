import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadFreshnessModule() {
  const source = readFileSync(
    resolve("src/app/admin/api-metrics/freshness.ts"),
    "utf8",
  );
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const { combineMetricsFreshness } = await loadFreshnessModule();

test("returns null when none of the metrics responses report freshness", () => {
  assert.equal(combineMetricsFreshness([]), null);
  assert.equal(combineMetricsFreshness([undefined, undefined]), null);
});

test("approximate freshness wins over exact and preserves the largest gap", () => {
  assert.deepEqual(
    combineMetricsFreshness([
      {
        data_state: "exact",
        percentiles_approximate: false,
        missing_rollup_hours: 0,
      },
      {
        data_state: "approximate",
        percentiles_approximate: true,
        missing_rollup_hours: 2,
      },
    ]),
    {
      data_state: "approximate",
      percentiles_approximate: true,
      missing_rollup_hours: 2,
    },
  );
});

test("delayed freshness wins over approximate across all five responses", () => {
  assert.deepEqual(
    combineMetricsFreshness([
      {
        data_state: "approximate",
        percentiles_approximate: true,
        missing_rollup_hours: 1,
      },
      undefined,
      {
        data_state: "delayed",
        percentiles_approximate: false,
        missing_rollup_hours: 4,
      },
      {
        data_state: "exact",
        percentiles_approximate: false,
        missing_rollup_hours: 0,
      },
      undefined,
    ]),
    {
      data_state: "delayed",
      percentiles_approximate: true,
      missing_rollup_hours: 4,
    },
  );
});
