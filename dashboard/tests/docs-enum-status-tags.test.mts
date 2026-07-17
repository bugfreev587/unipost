import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";
import {
  resolveDocsTableEnumTone,
  type DocsEnumTone,
} from "../src/app/docs/_components/docs-table-enum.ts";

const root = process.cwd();

const recognized: Array<[string, string, DocsEnumTone]> = [
  ["Support", "Yes", "success"],
  ["Support", "No", "danger"],
  ["Support", "Partial", "warning"],
  ["Support", "Limited", "warning"],
  ["Available", "Yes", "success"],
  ["Available", "No", "danger"],
  ["Required", "Yes", "success"],
  ["Required", "No", "danger"],
  ["Required", "Required", "info"],
  ["Required", "Optional", "neutral"],
  ["Required", "Rejected", "danger"],
  ["Severity", "Critical", "danger"],
  ["Severity", "High", "caution"],
  ["Severity", "Medium", "warning"],
  ["Default on", "Yes", "success"],
  ["Default on", "No", "danger"],
  ["Use this page?", "Yes", "success"],
  ["Use this page?", "No", "danger"],
  ["Use this page?", "Partially", "warning"],
  ["UniPost status", "Supported", "success"],
  ["UniPost status", "Coming soon", "warning"],
];

test("resolves approved reader-facing table enums", () => {
  for (const [column, value, expected] of recognized) {
    assert.equal(resolveDocsTableEnumTone(column, value), expected, `${column}: ${value}`);
  }
});

test("does not tag prose, machine enums, or descriptive values", () => {
  assert.equal(resolveDocsTableEnumTone("Notes", "Supported"), null);
  assert.equal(resolveDocsTableEnumTone("Meaning", "High"), null);
  assert.equal(resolveDocsTableEnumTone("data.status", "passed"), null);
  assert.equal(resolveDocsTableEnumTone("safety", "read_only"), null);
  assert.equal(resolveDocsTableEnumTone("Required", "Exactly 1 video"), null);
});

test("normalizes harmless whitespace and case", () => {
  assert.equal(resolveDocsTableEnumTone("  unipost STATUS ", " coming SOON "), "warning");
});

test("DocsTable renders enums with the shared B-style tag", async () => {
  const docsShell = await readFile(join(root, "src/app/docs/_components/docs-shell.tsx"), "utf8");

  assert.match(docsShell, /export function DocsEnumTag/);
  assert.match(docsShell, /resolveDocsTableEnumTone\(column, cell\)/);
  assert.match(docsShell, /renderDocsTableCell\(cell, columns\[cellIndex\]\)/);
  assert.match(docsShell, /\.docs-enum-tag\{/);
  for (const tone of ["success", "warning", "danger", "info", "neutral", "caution"]) {
    assert.match(docsShell, new RegExp(`\\.docs-enum-tag\\.is-${tone}\\{`));
  }
});

test("the dense platform matrix shares the partial-support tag", async () => {
  const platforms = await readFile(join(root, "src/app/docs/platforms/page.tsx"), "utf8");
  assert.match(platforms, /<DocsEnumTag value="Limited" tone="warning" \/>/);
  assert.doesNotMatch(platforms, /docs-matrix-partial/);
});
