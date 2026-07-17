import assert from "node:assert/strict";
import fs from "node:fs";
import test from "node:test";

const read = (path) => fs.readFileSync(new URL(`../${path}`, import.meta.url), "utf8");

test("GIF conversion API reference documents the complete async contract", () => {
  const source = read("src/app/docs/api/media/gif-conversions/page.tsx");
  for (const expected of [
    "/v1/media/gif-conversions",
    "GET /v1/media/gif-conversions/&#123;id&#125;",
    "gif_media_id",
    "background_color",
    "universal_mp4_v1",
    "output_media_id",
    "Idempotency-Key",
    "Retry-After",
    "gif_conversion_rate_limit_exceeded",
    "media_processing_capacity_exceeded",
    "Plan retention",
    "Conversion and publishing are separate operations",
  ]) {
    assert.match(source, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
  for (const language of ["Node.js", "Python", "Go", "Java"]) {
    assert.match(source, new RegExp(`label: "${language}"`));
  }
});

test("GIF guidance keeps direct support scoped and links conversion", () => {
  const source = read("src/app/docs/guides/publish-gifs/page.tsx");
  assert.match(source, /Publish GIFs to X and Facebook/);
  assert.match(source, /LinkedIn and Threads direct GIF paths remain coming soon/);
  assert.match(source, /POST \/v1\/media\/gif-conversions/);
  assert.match(source, /Conversion does not publish, edit a draft, or replace the original GIF/);
  assert.match(source, /TikTok requires both output dimensions to be at least 360 pixels/);
});

test("GIF conversion appears in docs navigation, endpoint links, and AI search", () => {
  const combined = [
    read("src/app/docs/api/page.tsx"),
    read("src/app/docs/_components/docs-shell.tsx"),
    read("src/app/docs/api/_components/doc-components.tsx"),
    read("src/lib/docs-ai-search-index.ts"),
  ].join("\n");
  assert.match(combined, /\/docs\/api\/media\/gif-conversions/);
  assert.match(combined, /POST \/v1\/media\/gif-conversions/);
  assert.match(combined, /GET \/v1\/media\/gif-conversions\/\{id\}/);
});

test("Media and Audio Overlay references explain shared lifecycle and capacity", () => {
  const combined = [
    read("src/app/docs/api/media/audio-overlays/page.tsx"),
    read("src/app/docs/guides/video-audio-overlay/page.tsx"),
    read("src/app/docs/api/media/reserve/page.tsx"),
    read("src/app/docs/api/media/get/page.tsx"),
    read("src/app/docs/api/posts/create/content.tsx"),
  ].join("\n");
  assert.match(combined, /share.*active Media Processing capacity/is);
  assert.match(combined, /Audio Overlay does not consume the rolling GIF conversion allowance/);
  assert.match(combined, /active Media Processing jobs/);
  assert.match(combined, /output_media_id/);
});
