import test from "node:test";
import assert from "node:assert/strict";

import {
  MANAGED_USER_PLATFORMS,
  connectionTypeLabel,
  platformDisplayName,
} from "./managed-user-platforms.ts";

test("MANAGED_USER_PLATFORMS covers every supported Connect platform", () => {
  assert.deepEqual(
    [...MANAGED_USER_PLATFORMS],
    [
      "twitter",
      "linkedin",
      "bluesky",
      "youtube",
      "tiktok",
      "instagram",
      "threads",
      "facebook",
      "pinterest",
    ]
  );
});

test("every supported platform has a distinct human-readable label", () => {
  const labels = MANAGED_USER_PLATFORMS.map(platformDisplayName);

  for (const [index, label] of labels.entries()) {
    assert.notEqual(
      label,
      MANAGED_USER_PLATFORMS[index],
      `${MANAGED_USER_PLATFORMS[index]} falls back to its raw key`
    );
  }
  assert.equal(new Set(labels).size, labels.length, "platform labels are not distinct");
  assert.equal(platformDisplayName("twitter"), "X");
  assert.equal(platformDisplayName("tiktok"), "TikTok");
});

test("platformDisplayName falls back to the raw value for unknown platforms", () => {
  assert.equal(platformDisplayName("mastodon"), "mastodon");
  assert.equal(platformDisplayName(""), "");
});

test("connectionTypeLabel distinguishes Connect-managed accounts from BYO accounts", () => {
  assert.match(connectionTypeLabel("managed"), /Connect/);
  assert.match(connectionTypeLabel("byo"), /BYO/);
  assert.notEqual(connectionTypeLabel("managed"), connectionTypeLabel("byo"));
  assert.equal(connectionTypeLabel("unknown"), "unknown");
});

// Badges render only non-zero counts, so a platform_counts object that is
// complete (nine keys, zeros included) still renders exactly the connected
// platforms — the behavior the four-platform shape got wrong for TikTok.
test("non-zero filtering renders every connected platform and no empty badges", () => {
  const platformCounts = Object.fromEntries(
    MANAGED_USER_PLATFORMS.map((platform) => [platform, 0])
  ) as Record<(typeof MANAGED_USER_PLATFORMS)[number], number>;
  // The reproduced staging case: one X account and one TikTok account.
  platformCounts.twitter = 1;
  platformCounts.tiktok = 1;

  const rendered = MANAGED_USER_PLATFORMS.filter(
    (platform) => (platformCounts[platform] ?? 0) > 0
  );

  assert.deepEqual([...rendered], ["twitter", "tiktok"]);
  const accountCount = 2;
  const sum = MANAGED_USER_PLATFORMS.reduce(
    (total, platform) => total + platformCounts[platform],
    0
  );
  assert.equal(sum, accountCount);
});

test("every supported platform can render a non-zero badge", () => {
  for (const connected of MANAGED_USER_PLATFORMS) {
    const platformCounts = Object.fromEntries(
      MANAGED_USER_PLATFORMS.map((platform) => [platform, platform === connected ? 2 : 0])
    ) as Record<(typeof MANAGED_USER_PLATFORMS)[number], number>;

    const rendered = MANAGED_USER_PLATFORMS.filter(
      (platform) => (platformCounts[platform] ?? 0) > 0
    );

    assert.deepEqual([...rendered], [connected]);
    assert.equal(platformCounts[connected], 2);
  }
});
