import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const linkPath = new URL(
  "../src/components/youtube/youtube-source-link.tsx",
  import.meta.url,
);
const identityPath = new URL(
  "../src/components/youtube/youtube-channel-identity.tsx",
  import.meta.url,
);

test("YouTube source links are validated, accessible text links without brand artwork", async () => {
  const source = await readFile(linkPath, "utf8");

  assert.match(source, /normalizeYouTubeContentUrl/);
  assert.match(source, /ExternalLink/);
  assert.match(source, /target="_blank"/);
  assert.match(source, /rel="noopener noreferrer"/);
  assert.match(source, /data-youtube-source-link/);
  assert.match(source, /aria-label=\{accessibleLabel\}/);
  assert.doesNotMatch(source, /<img|<svg|#ff0000|yt_icon|youtube.*asset/i);
});

test("YouTube channel identity composes avatar, name, and source without UniPost state", async () => {
  const source = await readFile(identityPath, "utf8");

  assert.match(source, /YouTubeSourceLink/);
  assert.match(source, /buildYouTubeChannelUrl/);
  assert.match(source, /YOUTUBE_HOME_URL/);
  assert.match(source, /Disconnected YouTube channel/);
  assert.match(source, /data-youtube-channel-avatar/);
  assert.doesNotMatch(source, /dbadge|UniPost status|health/i);
});
