import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();
const source = (path) => readFile(join(root, path), "utf8");

test("platform options guide provides safe copyable examples and is discoverable", async () => {
  const [guide, guidesIndex, docsShell, createReference, platformDocs, searchIndex] = await Promise.all([
    source("src/app/docs/guides/platform-options/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/posts/create/content.tsx"),
    source("src/app/docs/platforms/[platform]/_data.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);

  assert.match(guide, /title="Platform options examples"/);
  assert.match(guide, /platform_posts\[\].*flat/i);
  assert.match(guide, /Legacy account_ids/);
  assert.match(guide, /Invalid mixed shape/);
  assert.match(guide, /POST \/v1\/posts\/validate/);

  for (const platform of ["YouTube", "Instagram", "TikTok", "Facebook", "Pinterest"]) {
    assert.match(guide, new RegExp(`<h2 id="${platform.toLowerCase()}"[^>]*>${platform}</h2>`));
  }

  assert.match(guide, /API requests default to <code>private<\/code>/);
  assert.match(guide, /privacy_status/);
  assert.match(guide, /"privacy_status": "public"/);
  assert.match(guide, /"shorts": true/);
  assert.match(guide, /square or vertical/i);
  assert.match(guide, /three minutes/i);
  assert.match(guide, /does not resize, crop, or guarantee/i);
  assert.match(guide, /"mediaType": "story"/);
  assert.match(guide, /"privacy_level": "PUBLIC_TO_EVERYONE"/);
  assert.match(guide, /"brand_content_toggle": true/);
  assert.match(guide, /"mediaType": "reel"/);
  assert.match(guide, /"board_id": "1234567890"/);

  assert.match(guidesIndex, /href="\/docs\/guides\/platform-options"/);
  assert.match(docsShell, /label: "Platform options examples", href: "\/docs\/guides\/platform-options"/);
  assert.match(createReference, /href="\/docs\/guides\/platform-options"/);
  assert.match(createReference, /YouTube, Instagram, TikTok, Facebook, and Pinterest/);
  assert.match(platformDocs, /API requests default to `private`/);
  assert.match(searchIndex, /id: "guide-platform-options"/);
  assert.match(searchIndex, /path: "\/docs\/guides\/platform-options"/);
  assert.match(searchIndex, /YouTube Shorts visibility/);
});
