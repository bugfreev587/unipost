import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const gridPath = resolve("src/components/posts/create-post/account-card-grid.tsx");
const drawerPath = resolve("src/components/posts/create-post/create-post-drawer.tsx");
const accountsPath = resolve("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
const statsPath = resolve("src/components/dashboard/connection-stats.tsx");
const editorPath = resolve("src/components/posts/create-post/platform-editor-block.tsx");
const destinationIconPath = resolve("src/components/account-destination-icon.tsx");
const legacyCardPath = resolve("src/components/posts/create-post/account-card.tsx");
const legacyModalPath = resolve("src/components/dashboard/create-post-modal.tsx");

async function readOptional(path) {
  try {
    return await readFile(path, "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") return "";
    throw error;
  }
}

async function main() {
  const [
    gridSource,
    drawerSource,
    accountsSource,
    statsSource,
    editorSource,
    destinationIconSource,
    legacyCardSource,
    legacyModalSource,
  ] = await Promise.all([
    readFile(gridPath, "utf8"),
    readFile(drawerPath, "utf8"),
    readFile(accountsPath, "utf8"),
    readFile(statsPath, "utf8"),
    readFile(editorPath, "utf8"),
    readOptional(destinationIconPath),
    readFile(legacyCardPath, "utf8"),
    readFile(legacyModalPath, "utf8"),
  ]);

  assert.match(gridSource, /YouTube channel/);
  assert.match(gridSource, /UniPost profile/);
  assert.match(gridSource, /if \(account\.platform === "youtube"\) return "YouTube channel"/);
  assert.match(gridSource, /aria-label=\{`Remove \$\{sourceLabel\}/);
  assert.match(gridSource, /AccountDestinationIcon/);
  assert.doesNotMatch(gridSource, /PlatformIcon/);
  assert.match(drawerSource, /UniPost capability summary/);
  assert.match(drawerSource, /calculated by UniPost, not YouTube/);
  assert.match(statsSource, /UniPost-managed account health/);
  assert.match(statsSource, /AccountDestinationIcon/);
  assert.doesNotMatch(statsSource, /PlatformIcon/);
  assert.match(accountsSource, /Source platform/);
  assert.doesNotMatch(statsSource, /quickstartSourceLabel/);
  assert.match(accountsSource, /connected through UniPost-managed OAuth/);
  assert.match(accountsSource, /AccountDestinationIcon/);
  assert.doesNotMatch(accountsSource, /PlatformIcon/);
  assert.match(editorSource, /AccountDestinationIcon/);
  assert.doesNotMatch(editorSource, /PlatformIcon/);
  assert.match(legacyCardSource, /AccountDestinationIcon/);
  assert.doesNotMatch(legacyCardSource, /PlatformIcon/);
  assert.match(legacyModalSource, /AccountDestinationIcon/);
  assert.doesNotMatch(legacyModalSource, /PlatformIcon/);
  assert.match(destinationIconSource, /platform === "youtube"/);
  assert.doesNotMatch(destinationIconSource, /fill="#ff0000"/);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
