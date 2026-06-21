import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadBillingFormatModule() {
  const source = readFileSync(resolve("src/lib/billing-format.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

const {
  formatPostLimit,
  formatPlanPostAllowance,
  formatPostUsage,
  usagePercentage,
} = await loadBillingFormatModule();

test("formats negative post limits as unlimited", () => {
  assert.equal(formatPostLimit(-1), "Unlimited");
  assert.equal(formatPlanPostAllowance(-1), "Unlimited posts");
  assert.equal(formatPostUsage(23842, -1), "23,842 / Unlimited posts");
  assert.equal(usagePercentage(23842, -1), 0);
});

test("formats finite post limits with locale separators", () => {
  assert.equal(formatPostLimit(25000), "25,000");
  assert.equal(formatPlanPostAllowance(25000), "25,000 posts");
  assert.equal(formatPostUsage(1200, 25000), "1,200 / 25,000 posts");
  assert.equal(usagePercentage(1200, 25000), 4.8);
});
