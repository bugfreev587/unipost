import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

async function source(path) {
  return readFile(join(root, path), "utf8");
}

test("media API docs include audio overlay endpoints and optional size guidance", async () => {
  const reserveSource = await source("src/app/docs/api/media/reserve/page.tsx");
  const overlaySource = await source("src/app/docs/api/media/audio-overlays/page.tsx");
  const apiIndexSource = await source("src/app/docs/api/page.tsx");
  const docsShellSource = await source("src/app/docs/_components/docs-shell.tsx");
  const inlineLinkSource = await source("src/app/docs/api/_components/doc-components.tsx");

  assert.match(reserveSource, /size_bytes\?", type: "number"/, "reserve docs should mark size_bytes optional");
  assert.match(reserveSource, /omit size_bytes/i, "reserve docs should tell users UniPost can hydrate size later");
  assert.match(reserveSource, /audio\/mpeg/, "reserve docs should mention audio upload MIME types");

  assert.match(overlaySource, /title="Create audio overlay"/);
  assert.match(overlaySource, /path="\/v1\/media\/audio-overlays"/);
  assert.match(overlaySource, /Idempotency-Key/);
  assert.match(overlaySource, /trim_to_video/);
  assert.match(overlaySource, /loop_to_video/);
  assert.match(overlaySource, /audio_media_id/);

  assert.match(apiIndexSource, /label: "Create audio overlay"/);
  assert.match(apiIndexSource, /path: "\/v1\/media\/audio-overlays"/);
  assert.match(docsShellSource, /label: "Create audio overlay", href: "\/docs\/api\/media\/audio-overlays", method: "POST"/);
  assert.ok(inlineLinkSource.includes("^POST \\/v1\\/media\\/audio-overlays"), "inline link map should resolve POST /v1/media/audio-overlays");
});
