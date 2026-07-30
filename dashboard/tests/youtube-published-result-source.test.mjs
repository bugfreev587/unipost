import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import test from "node:test";

const root = process.cwd();

test("published YouTube results separate neutral status from validated source links", async () => {
  const paths = [
    "src/app/(dashboard)/projects/[id]/analytics/page.tsx",
    "src/components/posts/details/post-platform-results.tsx",
  ];

  for (const path of paths) {
    const source = await readFile(join(root, path), "utf8");
    assert.match(source, /AccountDestinationIcon/);
    assert.match(source, /YouTubeSourceLink/);
    assert.match(source, /normalizeYouTubeContentUrl/);
    assert.match(source, /platform === "youtube"/);
    assert.match(source, /disclosure="View on YouTube"/);
    assert.doesNotMatch(source, /<PlatformIcon platform=\{platform\}/);
  }
});
