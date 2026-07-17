import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadInboxModel() {
  const source = readFileSync(resolve("src/lib/inbox-model.ts"), "utf8");
  const compiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ES2022,
      target: ts.ScriptTarget.ES2022,
    },
  });
  const dataUrl = `data:text/javascript;base64,${Buffer.from(compiled.outputText).toString("base64")}`;
  return import(dataUrl);
}

test("X reply is modeled as a public X comment", async () => {
  const { getInboxSourceDefinition } = await loadInboxModel();
  assert.deepEqual(getInboxSourceDefinition("x_reply"), {
    source: "x_reply",
    platform: "twitter",
    kind: "public_comment",
    tab: "comments",
    label: "X comment",
    shortLabel: "Comment",
    private: false,
  });
});

test("X DM is modeled as a private X message", async () => {
  const { getInboxSourceDefinition } = await loadInboxModel();
  assert.deepEqual(getInboxSourceDefinition("x_dm"), {
    source: "x_dm",
    platform: "twitter",
    kind: "private_message",
    tab: "dms",
    label: "X DM",
    shortLabel: "DM",
    private: true,
  });
});
