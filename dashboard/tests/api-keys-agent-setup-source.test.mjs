import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { test } from "node:test";

const root = process.cwd();

test("api client exposes CLI setup-token creation", async () => {
  const source = await readFile(join(root, "src/lib/api.ts"), "utf8");

  assert.match(source, /export interface CliSetupTokenResponse/);
  assert.match(source, /export async function createCliSetupToken/);
  assert.match(source, /\/v1\/cli\/setup-tokens/);
});

test("api keys page offers agent setup commands for Claude Code and Codex", async () => {
  const source = await readFile(join(root, "src/app/(dashboard)/projects/[id]/api-keys/page.tsx"), "utf8");

  assert.match(source, /createCliSetupToken/);
  assert.match(source, /Connect with Claude Code \/ Codex/);
  assert.match(source, /claude-code/);
  assert.match(source, /codex/);
  assert.match(source, /agent bootstrap --client/);
  assert.match(source, /--setup-token/);
  assert.match(source, /--base-url/);
  assert.match(source, /NEXT_PUBLIC_API_URL/);
  assert.match(source, /npx -y @unipost\/cli agent bootstrap --client/);
  assert.doesNotMatch(source, /`unipost agent bootstrap --client/);
  assert.doesNotMatch(source, /setNewKey\(res\.data\.key\).*setup/i);
});
