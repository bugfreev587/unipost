import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";
import ts from "typescript";

async function loadReplyWindowModule() {
  const source = readFileSync(
    resolve("src/app/(dashboard)/projects/[id]/inbox/reply-window.ts"),
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

const { isMetaDMReplyWindowClosed } = await loadReplyWindowModule();

const now = Date.parse("2026-07-16T12:00:00Z");

test("Instagram DM reply window closes after 24 hours", () => {
  assert.equal(
    isMetaDMReplyWindowClosed("ig_dm", "2026-07-15T11:59:59Z", now),
    true,
  );
});

test("Facebook DM reply window closes after 24 hours", () => {
  assert.equal(
    isMetaDMReplyWindowClosed("fb_dm", "2026-07-15T11:59:59Z", now),
    true,
  );
});

test("Meta DM reply window stays open through 24 hours", () => {
  assert.equal(
    isMetaDMReplyWindowClosed("ig_dm", "2026-07-15T12:00:00Z", now),
    false,
  );
});

test("reply-window guard ignores non-DM and invalid timestamps", () => {
  assert.equal(
    isMetaDMReplyWindowClosed("ig_comment", "2026-07-15T11:59:59Z", now),
    false,
  );
  assert.equal(isMetaDMReplyWindowClosed("ig_dm", "not-a-date", now), false);
});
