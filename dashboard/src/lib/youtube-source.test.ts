import assert from "node:assert/strict";
import test from "node:test";

import {
  YOUTUBE_HOME_URL,
  buildYouTubeChannelUrl,
  normalizeYouTubeContentUrl,
} from "./youtube-source.ts";

test("builds an encoded channel URL only for a real channel ID", () => {
  assert.equal(buildYouTubeChannelUrl("UCabc_123-xyz"), "https://www.youtube.com/channel/UCabc_123-xyz");
  assert.equal(buildYouTubeChannelUrl(" channel/id "), "https://www.youtube.com/channel/channel%2Fid");
});

test("rejects empty IDs and disconnected sentinels", () => {
  assert.equal(buildYouTubeChannelUrl(undefined), null);
  assert.equal(buildYouTubeChannelUrl(""), null);
  assert.equal(buildYouTubeChannelUrl("   "), null);
  assert.equal(buildYouTubeChannelUrl("disconnected:account-123"), null);
  assert.equal(buildYouTubeChannelUrl(" DISCONNECTED:account-123 "), null);
});

test("accepts only HTTPS YouTube content destinations", () => {
  for (const value of [
    YOUTUBE_HOME_URL,
    "https://youtube.com/watch?v=abc",
    "https://m.youtube.com/watch?v=abc",
    "https://youtu.be/abc",
  ]) {
    assert.equal(normalizeYouTubeContentUrl(value), value);
  }
  for (const value of [
    undefined,
    "",
    "http://www.youtube.com/watch?v=abc",
    "https://youtube.example/watch?v=abc",
    "https://evil.youtube.com/watch?v=abc",
    "javascript:alert(1)",
  ]) {
    assert.equal(normalizeYouTubeContentUrl(value), null);
  }
});
