import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

test("CLI docs explain setup, configuration, and common fixes", async () => {
  const source = await readFile(join(root, "src/app/docs/cli/page.tsx"), "utf8");

  assert.match(source, /Set up the CLI in 3 steps/);
  assert.match(source, /Configure the CLI/);
  assert.match(source, /Common issues/);
  assert.match(source, /Dashboard setup token/);
  assert.match(source, /npx -y @unipost\/cli/);
  assert.match(source, /does not install a global `unipost` command/);
  assert.match(source, /zsh: command not found: unipost/);
  assert.match(source, /npm install -g @unipost\/cli/);
  assert.match(source, /--base-url https:\/\/api\.unipost\.dev/);
  assert.match(source, /UNIPOST_API_KEY/);
  assert.match(source, /config set base_url/);
  assert.match(source, /auth status --json/);
  assert.match(source, /setup_token_expired/);
  assert.match(source, /setup_token_used/);
  assert.match(source, /keychain_unavailable/);
  assert.match(source, /--dry-run/);
});
