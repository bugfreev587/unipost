import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

const gridPath = resolve("src/components/posts/create-post/account-card-grid.tsx");
const drawerPath = resolve("src/components/posts/create-post/create-post-drawer.tsx");

async function main() {
  const [gridSource, drawerSource] = await Promise.all([
    readFile(gridPath, "utf8"),
    readFile(drawerPath, "utf8"),
  ]);

  assert.match(gridSource, /YouTube channel/);
  assert.match(gridSource, /UniPost profile/);
  assert.match(gridSource, /if \(account\.platform === "youtube"\) return "YouTube channel"/);
  assert.match(gridSource, /aria-label=\{`Remove \$\{sourceLabel\}/);
  assert.match(drawerSource, /UniPost capability summary/);
  assert.match(drawerSource, /calculated by UniPost, not YouTube/);
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
