import test from "node:test";
import assert from "node:assert/strict";
import path from "node:path";
import {
  loadGeneratedBlogPostsFromDirectory,
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

test("parseGeneratedBlogPostFromSource keeps generated markdown block semantics", () => {
  const post = parseGeneratedBlogPostFromSource(
    "content/citeloop/blog/markdown-blocks.mdx",
    `---
source: citeloop
slug: "markdown-blocks"
title: "Markdown Blocks"
description: "Generated article description."
---

# Markdown Blocks

Intro paragraph.

- **Exact API surface** (endpoints, SDK languages, authentication model)
- **Pricing mechanics** (posts, accounts, seats)

---

> **Pricing signal to watch:** Per-account fees compound quickly.
`,
  );

  assert.ok(post);
  assert.deepEqual(post.blocks, [
    { type: "lead", text: "Intro paragraph." },
    {
      type: "list",
      items: [
        "**Exact API surface** (endpoints, SDK languages, authentication model)",
        "**Pricing mechanics** (posts, accounts, seats)",
      ],
    },
    { type: "divider" },
    { type: "blockquote", text: "**Pricing signal to watch:** Per-account fees compound quickly." },
  ]);
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

test("parseGeneratedBlogPostFromSource excludes development fixture posts", () => {
  assert.equal(
    parseGeneratedBlogPostFromSource(
      "content/citeloop/blog/citeloop-dev-verification.mdx",
      `---
source: citeloop
citeloop_article_id: "dev-fixture"
slug: "citeloop-dev-verification"
title: "CiteLoop Dev Verification"
description: "Fixture"
---

This fixture should not publish.
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

test("generated blog posts spread the June 16 article dates across the target cadence", () => {
  const posts = loadGeneratedBlogPostsFromDirectory(path.join(process.cwd(), "..", "content", "citeloop", "blog"));
  const datesBySlug = new Map(posts.map((post) => [post.slug, post.publishedAt]));

  assert.deepEqual(
    [
      "multi-platform-social-api-integration",
      "real-time-delivery-tracking-for-multi-platform-social-posts-webhooks-status-apis",
      "rest-api-to-mcp-server-ai-native-social-publishing",
      "scheduling-posts-and-optimal-timing-integrating-temporal-logic-with-multi-platform-publishing",
    ].map((slug) => [slug, datesBySlug.get(slug)]),
    [
      ["multi-platform-social-api-integration", "2026-06-16"],
      ["real-time-delivery-tracking-for-multi-platform-social-posts-webhooks-status-apis", "2026-06-14"],
      ["rest-api-to-mcp-server-ai-native-social-publishing", "2026-06-12"],
      ["scheduling-posts-and-optimal-timing-integrating-temporal-logic-with-multi-platform-publishing", "2026-06-09"],
    ],
  );
});
