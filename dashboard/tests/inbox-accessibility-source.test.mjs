import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import test from "node:test";

const source = readFileSync(
  resolve("src/app/(dashboard)/projects/[id]/inbox/page.tsx"),
  "utf8",
);

test("Inbox search and public reply inputs have explicit accessible names", () => {
  assert.match(source, /aria-label="Search Inbox conversations"/);
  assert.match(source, /aria-label=\{`Reply to \$\{item\.author_name \|\| "this comment"\}`\}/);
});

test("DM composer and icon-only send button stay named when disabled", () => {
  assert.match(source, /aria-label=\{selectedGroup\.source === "x_dm" \? "Write an X direct message" : "Write a direct message"\}/);
  assert.match(source, /aria-label=\{windowClosed \? "Cannot send: reply window closed" : "Send direct message"\}/);
});
