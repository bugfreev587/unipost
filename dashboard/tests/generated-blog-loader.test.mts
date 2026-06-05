import test from "node:test";
import assert from "node:assert/strict";
import {
  mergeBlogPosts,
  parseGeneratedBlogPostFromSource,
  type BlogPost,
} from "../src/lib/blog.ts";

test("parseGeneratedBlogPostFromSource maps CiteLoop MDX into BlogPost blocks", () => {
  const post = parseGeneratedBlogPostFromSource(
    "content/citeloop/blog/citeloop-demo.mdx",
    `---
source: citeloop
slug: "citeloop-demo"
title: "CiteLoop Demo"
seo_title: "CiteLoop Demo SEO"
description: "Generated article description."
excerpt: "Generated article excerpt."
published_at: "2026-06-05"
updated_at: "2026-06-05"
author: "UniPost"
category: "Engineering"
keywords: ["citeloop", "social publishing"]
canonical: "https://dev.unipost.dev/blog/citeloop-demo"
---

# CiteLoop Demo

The first paragraph becomes the article lead with an [internal link](/docs).

## Why it matters

The second paragraph stays a paragraph with \`inline code\`.

- First item
- Second item

\`\`\`ts
console.log("hello");
\`\`\`
`,
  );

  assert.ok(post);
  assert.equal(post.slug, "citeloop-demo");
  assert.equal(post.title, "CiteLoop Demo");
  assert.equal(post.seoTitle, "CiteLoop Demo SEO");
  assert.equal(post.description, "Generated article description.");
  assert.equal(post.keywords[1], "social publishing");
  assert.equal(post.blocks[0].type, "lead");
  assert.deepEqual(post.blocks[1], { type: "heading", text: "Why it matters" });
  assert.deepEqual(post.blocks[3], { type: "list", items: ["First item", "Second item"] });
  assert.deepEqual(post.blocks[4], { type: "code", language: "ts", code: 'console.log("hello");' });
});

test("parseGeneratedBlogPostFromSource rejects unsafe generated MDX", () => {
  assert.equal(
    parseGeneratedBlogPostFromSource(
      "content/citeloop/blog/unsafe.mdx",
      `---
slug: "unsafe"
title: "Unsafe"
description: "Unsafe"
---

import Widget from "./widget";
`,
    ),
    null,
  );
  assert.equal(
    parseGeneratedBlogPostFromSource(
      "content/citeloop/blog/script.mdx",
      `---
slug: "script"
title: "Script"
description: "Script"
---

<script>alert(1)</script>
`,
    ),
    null,
  );
});

test("mergeBlogPosts keeps existing posts and orders generated posts by date", () => {
  const existing: BlogPost = {
    slug: "existing",
    title: "Existing",
    seoTitle: "Existing",
    description: "Existing",
    excerpt: "Existing",
    publishedAt: "2026-05-01",
    updatedAt: "2026-05-01",
    readingTime: "1 min read",
    category: "Engineering",
    author: "UniPost",
    keywords: [],
    blocks: [{ type: "lead", text: "Existing" }],
  };
  const generated: BlogPost = { ...existing, slug: "generated", publishedAt: "2026-06-05", updatedAt: "2026-06-05" };
  const duplicate: BlogPost = { ...existing, title: "Duplicate should not replace" };

  const merged = mergeBlogPosts([existing], [generated, duplicate]);

  assert.equal(merged[0].slug, "generated");
  assert.equal(merged[1].slug, "existing");
  assert.equal(merged.find((post) => post.slug === "existing")?.title, "Existing");
});
