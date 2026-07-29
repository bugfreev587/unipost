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
const destinationIconPath = new URL(
  "../src/components/account-destination-icon.tsx",
  import.meta.url,
);
const accountsPath = new URL(
  "../src/app/(dashboard)/projects/[id]/accounts/page.tsx",
  import.meta.url,
);
const managedUserPath = new URL(
  "../src/app/(dashboard)/projects/[id]/users/[external_user_id]/page.tsx",
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

test("YouTube operational destinations use an original neutral glyph", async () => {
  const source = await readFile(destinationIconPath, "utf8");

  assert.match(source, /data-youtube-destination-icon/);
  assert.match(source, /<svg/);
  assert.match(source, /stroke="currentColor"/);
  assert.doesNotMatch(source, /\bVideo\b/);
  assert.doesNotMatch(source, /#ff0000|M23\.498 6\.186|yt_icon|youtube.*asset/i);
});

test("concrete account pages use channel identity while keeping status separate", async () => {
  const [accountsSource, managedUserSource] = await Promise.all([
    readFile(accountsPath, "utf8"),
    readFile(managedUserPath, "utf8"),
  ]);

  for (const source of [accountsSource, managedUserSource]) {
    assert.match(source, /YouTubeChannelIdentity/);
    assert.match(source, /compact/);
    assert.match(source, /disclosure="Source: YouTube"/);
  }
  assert.match(accountsSource, /<th>UniPost status<\/th>/);
  assert.match(managedUserSource, /data-unipost-account-status/);
});
