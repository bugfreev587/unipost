import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join, relative, resolve } from "node:path";
import test from "node:test";

const root = process.cwd();
const authenticatedRoots = [
  resolve("src/app/(dashboard)"),
  resolve("src/app/admin"),
];
const sharedComponentsRoot = resolve("src/components");
const platformIconDefinition = "src/components/platform-icons.tsx";
const allowedDynamicSharedPlatformIcons = new Set([
  "src/components/account-destination-icon.tsx", // rejects YouTube before delegating
  "src/components/analytics/meta-platform-analytics-view.tsx", // type-constrained to Instagram/Threads
  "src/components/tools/ToolCard.tsx", // public tool surface, recorded as out of scope
]);

async function source(path) {
  return readFile(resolve(path), "utf8");
}

async function codeFiles(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = join(directory, entry.name);
    if (entry.isDirectory()) return codeFiles(path);
    return /\.(?:css|ts|tsx)$/.test(entry.name) ? [path] : [];
  }));
  return nested.flat();
}

function repositoryPath(path) {
  return relative(root, path).split("\\").join("/");
}

test("authenticated Dashboard defaults to no official YouTube graphics", async () => {
  const files = (await Promise.all([
    ...authenticatedRoots.map(codeFiles),
    codeFiles(sharedComponentsRoot),
  ])).flat();

  for (const path of files) {
    const repoPath = repositoryPath(path);
    if (repoPath === platformIconDefinition) continue;
    const contents = await readFile(path, "utf8");

    assert.doesNotMatch(contents, /#(?:ff0000|f00)\b/i, `${repoPath} contains official YouTube red`);
    assert.doesNotMatch(contents, /M23\.498\s+6\.186/i, `${repoPath} contains the legacy YouTube path`);
    assert.doesNotMatch(
      contents,
      /(?:youtube|yt)[\w./-]*(?:icon|logo)[\w./-]*\.(?:png|svg|webp)/i,
      `${repoPath} references a YouTube brand asset`,
    );
    assert.doesNotMatch(
      contents,
      /<PlatformIcon\s+[^>]*platform=["']youtube["']/,
      `${repoPath} renders the official YouTube PlatformIcon`,
    );

    const dynamicPlatformIcon = /<PlatformIcon\s+[^>]*platform=\{/m.test(contents);
    if (dynamicPlatformIcon) {
      assert.ok(
        allowedDynamicSharedPlatformIcons.has(repoPath),
        `${repoPath} has an unclassified dynamic PlatformIcon`,
      );
    }
  }
});

test("the text source component owns validation and never embeds brand artwork", async () => {
  const linkSource = await source("src/components/youtube/youtube-source-link.tsx");

  assert.match(linkSource, /normalizeYouTubeContentUrl/);
  assert.match(linkSource, /<a/);
  assert.match(linkSource, /target="_blank"/);
  assert.match(linkSource, /rel="noopener noreferrer"/);
  assert.match(linkSource, /aria-label=\{accessibleLabel\}/);
  assert.match(linkSource, /Source: YouTube/);
  assert.match(linkSource, /Data from YouTube/);
  assert.match(linkSource, /View on YouTube/);
  assert.doesNotMatch(linkSource, /<img|<svg|#(?:ff0000|f00)|M23\.498\s+6\.186/i);
});

test("known operational surfaces use the neutral destination component", async () => {
  const paths = [
    "src/components/posts/create-post/account-card-grid.tsx",
    "src/components/posts/create-post/platform-editor-block.tsx",
    "src/components/posts/create-post/account-card.tsx",
    "src/components/dashboard/create-post-modal.tsx",
    "src/components/dashboard/connection-stats.tsx",
    "src/app/(dashboard)/projects/[id]/accounts/page.tsx",
    "src/app/(dashboard)/projects/[id]/inbox/page.tsx",
    "src/app/admin/users/page.tsx",
  ];

  for (const path of paths) {
    const contents = await source(path);
    assert.match(contents, /AccountDestinationIcon/, `${path} must use the neutral destination component`);
  }

  const accountsSource = await source("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
  const statsSource = await source("src/components/dashboard/connection-stats.tsx");
  const drawerSource = await source("src/components/posts/create-post/create-post-drawer.tsx");
  assert.match(accountsSource, /YouTubeChannelIdentity/);
  assert.match(accountsSource, /Source platform/);
  assert.match(accountsSource, /connected through UniPost-managed OAuth/);
  assert.match(statsSource, /UniPost-managed account health/);
  assert.match(drawerSource, /calculated by UniPost, not YouTube/);
});
