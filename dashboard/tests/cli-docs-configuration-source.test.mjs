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
  assert.match(source, /Install once/);
  assert.match(source, /npm install -g @unipost\/cli/);
  assert.match(source, /After install, use <code>unipost<\/code> commands/);
  assert.match(source, /Update with <code>unipost upgrade<\/code>/);
  assert.match(source, /CLI self-management/);
  assert.match(source, /unipost upgrade/);
  assert.match(source, /unipost self help/);
  assert.match(source, /Choose Terminal/);
  assert.match(source, /Copy the generated `unipost auth login/);
  assert.match(source, /--client terminal/);
  assert.match(source, /Path B is for Codex, Claude Code, Cursor/);
  assert.match(source, /Configure the CLI/);
  assert.match(source, /Common issues/);
  assert.match(source, /Dashboard setup token/);
  assert.doesNotMatch(source, /npx -y @unipost\/cli/);
  assert.doesNotMatch(source, /No-install one-off alternative/);
  assert.match(source, /zsh: command not found: unipost/);
  assert.match(source, /--base-url https:\/\/api\.unipost\.dev/);
  assert.match(source, /UNIPOST_API_KEY/);
  assert.match(source, /config set base_url/);
  assert.match(source, /auth status --json/);
  assert.match(source, /setup_token_expired/);
  assert.match(source, /setup_token_used/);
  assert.match(source, /keychain_unavailable/);
  assert.match(source, /--dry-run/);
});

test("CLI docs split overview from a grouped command reference", async () => {
  const shellSource = await readFile(join(root, "src/app/docs/_components/docs-shell.tsx"), "utf8");
  const overviewSource = await readFile(join(root, "src/app/docs/cli/page.tsx"), "utf8");
  const referenceSource = await readFile(join(root, "src/app/docs/cli/reference/page.tsx"), "utf8");

  assert.match(shellSource, /CLI/);
  assert.match(shellSource, /Overview/);
  assert.match(shellSource, /Reference/);
  assert.match(shellSource, /\/docs\/cli\/reference/);
  assert.match(shellSource, /@media \(max-width:640px\)\{\.docs-auth-actions\{display:none\}/);

  assert.match(overviewSource, /title="CLI - Overview"/);
  assert.match(overviewSource, /\/docs\/cli\/reference/);

  assert.match(referenceSource, /title="CLI - Reference"/);
  assert.match(referenceSource, /CLI_REFERENCE_SECTIONS/);
  assert.match(referenceSource, /Setup & Diagnostics/);
  assert.match(referenceSource, /Auth & Config/);
  assert.match(referenceSource, /Profiles & Connect/);
  assert.match(referenceSource, /Accounts/);
  assert.match(referenceSource, /Posts/);
  assert.match(referenceSource, /Media/);
  assert.match(referenceSource, /Analytics/);
  assert.match(referenceSource, /Examples/);
  assert.match(referenceSource, /Agent & MCP/);
  assert.match(referenceSource, /Self-management & Shell/);
  assert.match(referenceSource, /cli-reference-group/);
  assert.match(referenceSource, /cli-command-row/);
  assert.match(referenceSource, /cli-command-summary/);
  assert.match(referenceSource, /\.cli-command-example\{[^}]*font-weight:650/);
  assert.match(referenceSource, /<details/);
  assert.match(referenceSource, /<summary/);
  assert.doesNotMatch(referenceSource, /command-reference-card/);
  assert.doesNotMatch(referenceSource, /command-reference-examples/);
  assert.match(referenceSource, /Example response/);
  assert.match(referenceSource, /unipost accounts list --json/);
  assert.match(referenceSource, /"command": "accounts list"/);
  assert.match(referenceSource, /unipost agent capabilities --json/);
});

test("API key setup dialog does not imply Codex is configured by auth alone", async () => {
  const source = await readFile(join(root, "src/app/(dashboard)/projects/[id]/api-keys/page.tsx"), "utf8");

  assert.match(source, /Set up UniPost CLI/);
  assert.match(source, /Install once with npm install -g @unipost\/cli/);
  assert.match(source, /create a short-lived setup token that signs the UniPost CLI in/i);
  assert.match(source, /Choose Terminal for command line only/);
  assert.match(source, /Choose Codex or Claude Code only when a local agent will use UniPost/);
});

test("CLI README explains install-first usage and separate agent install", async () => {
  const source = await readFile(join(root, "../cli/README.md"), "utf8");

  assert.match(source, /Install once:/);
  assert.match(source, /npm install -g @unipost\/cli/);
  assert.match(source, /Use `unipost \.\.\.` by default/);
  assert.match(source, /Update with `unipost upgrade`/);
  assert.match(source, /unipost self help/);
  assert.match(source, /unipost auth login --setup-token ust_\.\.\. --client terminal/);
  assert.doesNotMatch(source, /npx -y @unipost\/cli/);
  assert.doesNotMatch(source, /No-install one-off alternative/);
  assert.match(source, /To let a local AI agent use UniPost/);
  assert.match(source, /unipost agent install --client codex --json/);
});
