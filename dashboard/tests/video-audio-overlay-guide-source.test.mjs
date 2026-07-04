import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("video and audio overlay guide explains the user workflow and API sequence", async () => {
  const guide = await source("src/app/docs/guides/video-audio-overlay/page.tsx");
  const guidesIndex = await source("src/app/docs/guides/page.tsx");
  const docsShell = await source("src/app/docs/_components/docs-shell.tsx");
  const searchIndex = await source("src/lib/docs-ai-search-index.ts");

  assert.match(guide, /title="Overlay user audio onto a video"/);
  assert.match(guide, /TikTok and Instagram API publishing do not expose the same manual editor flow/i);
  assert.match(guide, /POST \/v1\/media/);
  assert.match(guide, /POST \/v1\/media\/audio-overlays/);
  assert.match(guide, /GET \/v1\/media\/audio-overlays\/&#123;id&#125;/);
  assert.match(guide, /POST \/v1\/posts/);
  assert.match(guide, /mode: "mix"/);
  assert.match(guide, /mode: "replace"/);
  assert.match(guide, /fit: "trim_to_video"/);
  assert.match(guide, /fit: "loop_to_video"/);
  assert.match(guide, /Idempotency-Key/);
  assert.match(guide, /output_media_id/);
  assert.match(guide, /Do not ask the user to calculate file size/i);
  assert.match(guide, /DocsTable/);
  assert.match(guide, /DocsCodeTabs/);

  assert.match(guidesIndex, /\/docs\/guides\/video-audio-overlay/);
  assert.match(docsShell, /label: "Video \+ audio overlay", href: "\/docs\/guides\/video-audio-overlay"/);
  assert.match(searchIndex, /id: "guide-video-audio-overlay"/);
  assert.match(searchIndex, /combine video and audio/);
  assert.match(searchIndex, /POST \/v1\/media\/audio-overlays/);
});
