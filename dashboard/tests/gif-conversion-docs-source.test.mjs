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
    "POST always returns 202",
    "GET returns 200",
    "Idempotency-Key applies only to POST",
    "4096 pixels per dimension",
    "2,000 frames",
    "1.5 billion decoded pixels",
    "60-second animation cycle",
    "five-minute processing limit",
    "gif_dimensions_exceeded",
    "gif_frame_count_exceeded",
    "gif_decode_budget_exceeded",
    "gif_duration_exceeded",
    "gif_probe_failed",
    "gif_decode_failed",
    "processing_timeout",
    "output_size_exceeded",
    "gif_conversion_failed",
    "Free: 1 active / 10 GIF conversions",
    "API: 2 / 50",
    "Basic: 2 / 100",
    "Growth: 4 / 300",
    "Team: 6 / 1,000",
    "Enterprise: 6 / 1,000 by default",
    "Published UniPost SDK packages do not yet include GIF conversion helpers",
    "DEADLINE=$((SECONDS + 900))",
    "curl -fSs",
    "jq -er",
    "Unexpected conversion status",
    "does not cancel the server-side job",
    "at least five seconds",
    "After a successful conversion",
    "After a failed conversion",
    "Free 1 day",
    "API 2 days",
    "Basic 4 days",
    "Growth 15 days",
    "Team and Enterprise 30 days",
  ]) {
    assert.match(source, new RegExp(expected.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")));
  }
  for (const language of ["Node.js", "Python", "Go", "Java"]) {
    assert.doesNotMatch(source, new RegExp(`label: "${language}"`));
  }
  assert.doesNotMatch(source, /while true/);
});

test("GIF guidance keeps direct support scoped and links conversion", () => {
  const source = read("src/app/docs/guides/publish-gifs/page.tsx");
  assert.match(source, /Publish GIFs to X and Facebook/);
  assert.match(source, /LinkedIn and Threads direct GIF paths remain coming soon/);
  assert.match(source, /POST \/v1\/media\/gif-conversions/);
  assert.match(source, /Conversion does not publish, edit a draft, or replace the original GIF/);
  assert.match(source, /TikTok requires both output dimensions to be at least 360 pixels/);
  assert.equal(
    source.match(/GIF-to-MP4 conversion available; destination-specific publishing guidance coming soon/g)?.length,
    5,
  );
  assert.doesNotMatch(source, /MP4 conversion supported; GIF guidance coming soon/);
  assert.doesNotMatch(source, /prepare for upcoming conversion workflows/);
  assert.doesNotMatch(source, /No stable direct GIF file publishing path exposed by UniPost/);
  assert.match(source, /No direct GIF media type in the documented image and video embed APIs/);
  assert.match(source, /URL path must end in <code>\.gif/);
  assert.match(source, /Query strings are allowed/);
  assert.match(source, /extensionless URL/);
  assert.match(source, /upload it and use <code>media_ids/);
  assert.match(source, /does not download hosted media bytes/);
  assert.match(source, /cannot confirm the hosted file&apos;s actual MIME type,\s+dimensions, or size/);
  assert.match(source, /Hosted GIF validation failed/);
  assert.match(source, /dispatching\|retrying/);
  assert.match(source, /partial\|failed\|cancelled\).*exit 1/);
  assert.match(source, /require video to preserve GIF animation/);
  assert.match(source, /To preserve animation on Instagram, TikTok, Pinterest, YouTube, and Bluesky/);
  assert.match(source, /does not currently enforce X&apos;s\s+one-GIF-only rule/);
  assert.doesNotMatch(source, /video-only destinations/);
  assert.doesNotMatch(source, /need video media instead of an unchanged GIF/);
  assert.doesNotMatch(source, /while true/);
  assert.match(
    source,
    /Destination-specific publishing guides and\s+the Dashboard conversion control are still coming soon/,
  );
});

test("Guides index presents GIF conversion as available", () => {
  const source = read("src/app/docs/guides/page.tsx");
  assert.match(source, /convert GIFs to MP4 for video destinations/);
  assert.match(source, /available GIF-to-MP4 API workflow/);
  assert.doesNotMatch(source, /upcoming conversion workflows/);
  assert.doesNotMatch(source, /planned GIF-to-MP4 path/);
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
