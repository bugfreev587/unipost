import test from "node:test";
import assert from "node:assert/strict";
import { parseInlineMarkdown } from "../src/lib/blog-inline.ts";

test("parseInlineMarkdown identifies strong text while preserving links and code", () => {
  assert.deepEqual(
    parseInlineMarkdown("Read **Exact API surface** in [docs](/docs) with `post_id`."),
    [
      { type: "text", text: "Read " },
      { type: "strong", text: "Exact API surface" },
      { type: "text", text: " in " },
      { type: "link", text: "docs", href: "/docs" },
      { type: "text", text: " with " },
      { type: "code", text: "post_id" },
      { type: "text", text: "." },
    ],
  );
});

test("parseInlineMarkdown identifies emphasis text", () => {
  assert.deepEqual(parseInlineMarkdown("*This brief is source-backed.*"), [
    { type: "emphasis", text: "This brief is source-backed." },
  ]);
});
