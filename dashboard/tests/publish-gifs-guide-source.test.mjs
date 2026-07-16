import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

function endpointMapping(sourceText, method, pathSource) {
  const pathIndex = sourceText.indexOf(pathSource);
  if (pathIndex < 0) {
    return "";
  }

  const blockStart = sourceText.lastIndexOf("  {", pathIndex);
  const blockEnd = sourceText.indexOf("\n  },", pathIndex);
  const block = sourceText.slice(blockStart, blockEnd >= 0 ? blockEnd + 5 : sourceText.length);

  assert.match(block, new RegExp(`method: "${method}"`));
  return block;
}

test("Publish GIFs guide covers support, workflows, navigation, and API backlinks", async () => {
  const [guide, guidesIndex, docsShell, endpointGuides, searchIndex] = await Promise.all([
    source("src/app/docs/guides/publish-gifs/page.tsx"),
    source("src/app/docs/guides/page.tsx"),
    source("src/app/docs/_components/docs-shell.tsx"),
    source("src/app/docs/api/_components/single-endpoint-page.tsx"),
    source("src/lib/docs-ai-search-index.ts"),
  ]);

  assert.match(guide, /title="Publish GIFs to X and Facebook"/);
  for (const platform of [
    "X / Twitter",
    "Facebook Page",
    "LinkedIn",
    "Threads",
    "Instagram",
    "TikTok",
    "Pinterest",
    "YouTube",
    "Bluesky",
  ]) {
    assert.match(guide, new RegExp(platform.replace("/", "\\/")));
  }

  assert.match(guide, /"X \/ Twitter", "Yes — direct GIF media upload", "Supported"/);
  assert.match(guide, /"Facebook Page", "Yes — GIF photo post", "Supported"/);
  assert.match(guide, /"LinkedIn", "Yes — through LinkedIn image APIs", "Coming soon"/);
  assert.match(guide, /"Threads", "Yes — through provider-backed GIF attachments", "Coming soon"/);

  for (const platform of ["Instagram", "TikTok", "Pinterest", "YouTube", "Bluesky"]) {
    assert.match(
      guide,
      new RegExp(`${platform}[\\s\\S]{0,420}GIF-to-MP4 conversion option is coming soon`, "i"),
    );
  }

  for (const endpoint of [
    "GET /v1/accounts",
    "POST /v1/media",
    "GET /v1/media/:media_id",
    "POST /v1/posts/validate",
    "POST /v1/posts",
    "GET /v1/posts/:post_id",
  ]) {
    assert.match(guide, new RegExp(endpoint.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }

  assert.match(guide, /"content_type": "image\/gif"/);
  assert.match(guide, /"account_id": "sa_twitter_123"/);
  assert.match(guide, /"account_id": "sa_facebook_123"/);
  assert.match(guide, /"platform_posts"/);
  assert.match(guide, /5 MB or smaller/i);
  assert.match(guide, /publish the Facebook GIF immediately/i);
  assert.match(guide, /GIF-to-MP4 conversion option is coming soon/i);
  assert.doesNotMatch(guide, /GIF-to-MP4 conversion is available/i);

  assert.match(guidesIndex, /href="\/docs\/guides\/publish-gifs"/);
  assert.match(docsShell, /label: "Publish GIFs", href: "\/docs\/guides\/publish-gifs"/);
  assert.match(searchIndex, /id: "guide-publish-gifs"/);

  const endpointMappings = [
    endpointMapping(endpointGuides, "GET", "path: /^\\/v1\\/accounts$/"),
    endpointMapping(endpointGuides, "POST", "path: /^\\/v1\\/media$/"),
    endpointMapping(endpointGuides, "GET", "path: /^\\/v1\\/media\\/:[^/]+$/"),
    endpointMapping(endpointGuides, "POST", "path: /^\\/v1\\/posts\\/validate$/"),
    endpointMapping(endpointGuides, "POST", "path: /^\\/v1\\/posts$/"),
    endpointMapping(endpointGuides, "GET", "path: /^\\/v1\\/posts\\/:[^/]+$/"),
  ];

  for (const mapping of endpointMappings) {
    assert.notEqual(mapping, "", "missing endpoint-to-guide mapping");
    assert.match(mapping, /label: "Publish GIFs", href: "\/docs\/guides\/publish-gifs"/);
  }
});
