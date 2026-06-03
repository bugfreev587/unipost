import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

test("CLI docs explain setup, configuration, and common fixes", async () => {
  const source = await readFile(join(root, "src/app/docs/cli/page.tsx"), "utf8");

  assert.match(source, /Choose your setup path/);
  assert.match(source, /Path A: Command line only/);
  assert.match(source, /Path B: Local AI agent/);
  assert.match(source, /setup token only logs the UniPost CLI in/);
  assert.match(source, /It does not install or configure Codex, Claude Code, or any other agent/);
  assert.match(source, /Run this command exactly as copied; do not shorten it to `unipost`/);
  assert.match(source, /Only use bare `unipost` commands after `npm install -g @unipost\/cli`/);
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

test("API key setup dialog does not imply Codex is configured by auth alone", async () => {
  const source = await readFile(join(root, "src/app/(dashboard)/projects/[id]/api-keys/page.tsx"), "utf8");

  assert.match(source, /Set up UniPost CLI for agents/);
  assert.match(source, /Create a short-lived setup token that signs the UniPost CLI in/);
  assert.match(source, /This does not install or configure the selected AI agent/);
});

test("CLI README explains npx-first usage and separate agent install", async () => {
  const source = await readFile(join(root, "../cli/README.md"), "utf8");

  assert.match(source, /Use `npx -y @unipost\/cli \.\.\.` by default/);
  assert.match(source, /does not install a global `unipost` command/);
  assert.match(source, /To let a local AI agent use UniPost/);
  assert.match(source, /npx -y @unipost\/cli agent install --client codex --json/);
});
