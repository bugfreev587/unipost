import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const gridPath = resolve("src/components/posts/create-post/account-card-grid.tsx");
const drawerPath = resolve("src/components/posts/create-post/create-post-drawer.tsx");
const accountsPath = resolve("src/app/(dashboard)/projects/[id]/accounts/page.tsx");
const statsPath = resolve("src/components/dashboard/connection-stats.tsx");

async function main() {
  const [gridSource, drawerSource, accountsSource, statsSource] = await Promise.all([
    readFile(gridPath, "utf8"),
    readFile(drawerPath, "utf8"),
    readFile(accountsPath, "utf8"),
    readFile(statsPath, "utf8"),
  ]);

  assert.match(gridSource, /YouTube channel/);
  assert.match(gridSource, /UniPost profile/);
  assert.match(gridSource, /if \(account\.platform === "youtube"\) return "YouTube channel"/);
  assert.match(gridSource, /aria-label=\{`Remove \$\{sourceLabel\}/);
  assert.match(drawerSource, /UniPost capability summary/);
  assert.match(drawerSource, /calculated by UniPost, not YouTube/);
  assert.match(statsSource, /UniPost-managed account health/);
  assert.match(accountsSource, /Source platform/);
  assert.match(statsSource, /YouTube channel/);
  assert.match(accountsSource, /connected through UniPost-managed OAuth/);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
