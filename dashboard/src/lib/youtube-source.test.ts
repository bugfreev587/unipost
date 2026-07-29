import assert from "node:assert/strict";
import test from "node:test";

import {
  YOUTUBE_HOME_URL,
  buildYouTubeChannelUrl,
  getYouTubeIdentityInitials,
  isDisconnectedYouTubeAccount,
  normalizeYouTubeContentUrl,
} from "./youtube-source.ts";

test("disconnected YouTube identities never retain stale initials", () => {
  assert.equal(isDisconnectedYouTubeAccount("disconnected", "UC-real"), true);
  assert.equal(isDisconnectedYouTubeAccount("active", "disconnected:account-123"), true);
  assert.equal(isDisconnectedYouTubeAccount("active", "UC-real"), false);
  assert.equal(getYouTubeIdentityInitials("Bob Alpha", true), null);
  assert.equal(getYouTubeIdentityInitials("Bob Alpha", false), "BA");
});

test("builds an encoded channel URL only for a real channel ID", () => {
  assert.equal(buildYouTubeChannelUrl("UCabc_123-xyz"), "https://www.youtube.com/channel/UCabc_123-xyz");
  assert.equal(buildYouTubeChannelUrl(" channel/id "), "https://www.youtube.com/channel/channel%2Fid");
  assert.equal(buildYouTubeChannelUrl("channel:id"), "https://www.youtube.com/channel/channel%3Aid");
  assert.equal(
    normalizeYouTubeContentUrl(buildYouTubeChannelUrl("channel:id")),
    "https://www.youtube.com/channel/channel%3Aid",
  );
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
    "https://www.youtube.com@evil.com/watch?v=abc",
    "https://youtube.com.evil.com/watch?v=abc",
    "javascript:alert(1)",
  ]) {
    assert.equal(normalizeYouTubeContentUrl(value), null);
  }
});
