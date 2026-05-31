import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const publishModePanelPath = path.join(root, "src/components/posts/create-post/publish-mode-panel.tsx");

test("Publish mode schedule keeps only the native datetime picker trigger", async () => {
  const source = await readFile(publishModePanelPath, "utf8");

  assert.match(source, /type="datetime-local"/);
  assert.doesNotMatch(source, /setCalendarOpen/);
  assert.doesNotMatch(source, /<Calendar\b/);
  assert.doesNotMatch(source, /function MiniCalendar/);
  assert.doesNotMatch(source, /pr-10/);
});
