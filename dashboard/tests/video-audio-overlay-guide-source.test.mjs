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
  assert.match(guide, /SDK prerequisite/);
  assert.match(guide, /0\.5\.0/);
  assert.match(guide, /no longer force callers to provide <code>sizeBytes<\/code>/);
  assert.match(guide, /<code>size_bytes<\/code> when reserving media uploads/);
  assert.match(guide, /File size is optional/i);
  assert.match(guide, /provide it when your app already knows the byte length/i);
  assert.match(guide, /omit it when your app does not know it yet/i);
  assert.doesNotMatch(guide, /Do not ask the user to calculate file size/i);
  assert.match(guide, /DocsTable/);
  assert.match(guide, /DocsCodeTabs/);
  assert.match(guide, /<h3 id="step-1-upload-video">Step 1: Upload the video input<\/h3>/);
  assert.match(guide, /<h3 id="step-2-upload-audio">Step 2: Upload the audio input<\/h3>/);
  assert.match(guide, /<h3 id="step-3-generate-overlay">Step 3: Generate the overlay video<\/h3>/);
  assert.match(guide, /<h3 id="step-4-publish-post">Step 4: Publish the processed video<\/h3>/);
  assert.match(guide, /upload_url is not another UniPost JSON endpoint/i);
  assert.match(guide, /label: "Node\.js SDK"/);
  assert.match(guide, /label: "Python SDK"/);
  assert.match(guide, /label: "Go SDK"/);
  assert.match(guide, /label: "Java SDK"/);
  assert.match(guide, /Step 1: Upload the video input/);
  assert.match(guide, /Step 2: Upload the audio input/);
  assert.match(guide, /Step 3: Generate the overlay video/);
  assert.match(guide, /Step 4: Publish the processed video/);
  assert.doesNotMatch(guide, /Step 5:/);
  assert.match(guide, /client\.media\.audio_overlays\.create/);
  assert.match(guide, /client\.Media\.AudioOverlays\.Create/);
  assert.match(guide, /client\.media\(\)\.audioOverlays\(\)\.create/);

  assert.match(guidesIndex, /\/docs\/guides\/video-audio-overlay/);
  assert.match(docsShell, /label: "Video \+ audio overlay", href: "\/docs\/guides\/video-audio-overlay"/);
  assert.match(searchIndex, /id: "guide-video-audio-overlay"/);
  assert.match(searchIndex, /combine video and audio/);
  assert.match(searchIndex, /SDK version 0\.5\.0 or later/);
  assert.match(searchIndex, /sizeBytes or size_bytes/);
  assert.match(searchIndex, /POST \/v1\/media\/audio-overlays/);
});
