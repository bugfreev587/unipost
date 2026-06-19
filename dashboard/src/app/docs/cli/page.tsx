import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const CLI_AUTH_SNIPPETS = [
  {
    label: "Install and sign in",
    lang: "bash",
    code: `npm install -g @unipost/cli

# Dashboard: Project -> API Keys -> Set up UniPost CLI
# Choose Terminal, then copy and run the generated command.
unipost auth login --setup-token ust_... --client terminal --base-url https://api.unipost.dev --json
unipost auth status --json`,
  },
  {
    label: "Existing API key",
    lang: "bash",
    code: `unipost init --api-key up_live_... --base-url https://api.unipost.dev --json
unipost auth status --json`,
  },
  {
    label: "CI or one-off",
    lang: "bash",
    code: `export UNIPOST_API_KEY=up_live_...
unipost auth status --json
unipost doctor verify --json`,
  },
];

const AGENT_SETUP_SNIPPETS = [
  {
    label: "Codex",
    lang: "bash",
    code: `# Finish CLI auth first. This teaches Codex how to use UniPost;
# it does not install the Codex CLI itself.
unipost agent install --client codex --json
unipost agent bootstrap --client codex --json
unipost agent capabilities --client codex --json
unipost agent context --json`,
  },
  {
    label: "Claude Code",
    lang: "bash",
    code: `# Finish CLI auth first. This prints the Claude Code instruction package setup.
unipost agent install --client claude-code --json
unipost agent bootstrap --client claude-code --json
unipost agent capabilities --client claude-code --json
unipost agent mcp-config --client claude-code --json`,
  },
];

const CONFIG_SNIPPETS = [
  {
    label: "Check current config",
    lang: "bash",
    code: `unipost config path --json
unipost config show --json
unipost auth status --json`,
  },
  {
    label: "Use dev API",
    lang: "bash",
    code: `unipost config set base_url https://dev-api.unipost.dev --json
unipost config show --json`,
  },
  {
    label: "Set defaults",
    lang: "bash",
    code: `unipost profiles list --json
unipost config set default_profile_id pr_... --json
unipost accounts list --json
unipost accounts health --account sa_... --json`,
  },
];

const SAFE_POST_SNIPPETS = [
  {
    label: "Validate and draft",
    lang: "bash",
    code: `unipost posts validate --account sa_... --caption "Shipping with UniPost CLI."
unipost posts draft --account sa_... --caption "Shipping with UniPost CLI."`,
  },
  {
    label: "Dry-run before approval",
    lang: "bash",
    code: `unipost posts create \\
  --from-file post.json \\
  --dry-run \\
  --json`,
  },
  {
    label: "Approved scheduled publish",
    lang: "bash",
    code: `unipost posts schedule \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI." \\
  --at 2026-06-10T09:00:00Z \\
  --yes \\
  --idempotency-key user-approved-2026-06-03-001 \\
  --json

unipost posts wait post_... --timeout 120 --json`,
  },
  {
    label: "Cancel or retry",
    lang: "bash",
    code: `unipost posts cancel post_... --yes --json
unipost posts retry post_... --result result_... --yes --json`,
  },
  {
    label: "Media upload",
    lang: "bash",
    code: `unipost media upload ./video.mp4 --json
unipost media wait med_... --timeout 120 --json

unipost posts create \\
  --from-file post-with-media.json \\
  --dry-run \\
  --json`,
  },
  {
    label: "Native fetch example",
    lang: "bash",
    code: `unipost examples posts.create \\
  --lang node \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI." \\
  --json`,
  },
  {
    label: "Structured agent execute",
    lang: "bash",
    code: `unipost agent plan \\
  --intent create_draft_post \\
  --from-file post.json \\
  --json > safe-plan.json

unipost agent execute \\
  --plan safe-plan.json \\
  --json`,
  },
];

const JSON_ENVELOPE = [
  {
    label: "Success",
    lang: "json",
    code: `{
  "ok": true,
  "data": {},
  "warnings": [],
  "meta": {
    "request_id": "req_...",
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.3.1",
    "command": "accounts list",
    "source": "cli"
  }
}`,
  },
  {
    label: "Error",
    lang: "json",
    code: `{
  "ok": false,
  "error": {
    "code": "unauthorized",
    "message": "API key is missing or invalid.",
    "hint": "Set UNIPOST_API_KEY or pass --api-key.",
    "docs_url": "https://unipost.dev/docs/quickstart"
  },
  "warnings": [],
  "meta": {
    "request_id": "req_...",
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.3.1",
    "command": "auth status",
    "source": "cli"
  }
}`,
  },
];

const COMMAND_ROWS = [
  ["CLI self-management", "`upgrade`, `self update`, `self help`, `--version`, `--help`, and `completion`."],
  ["Auth and config", "`init`, `auth login --api-key`, Dashboard setup-token login, `auth logout`, `auth status`, `config path`, `config show`, `config set base_url`, and `config set default_profile_id`. `auth list` and `auth use` remain compatibility commands for the single current binding."],
  ["Quickstart", "`init`, `doctor`, `quickstart`, `profiles list/create/get/use`, and `connect create/get/wait`."],
  ["Accounts", "`accounts list`, `accounts get`, `accounts health`, `accounts capabilities`, and `accounts metrics`."],
  ["Posts", "`posts list`, `posts get`, `posts analytics`, `posts validate`, `posts draft`, `posts create --dry-run`, `posts create`, `posts schedule`, `posts publish-draft`, `posts wait`, `posts cancel`, and `posts retry`."],
  ["Media", "`media upload`, `media get`, and `media wait`."],
  ["Analytics", "`analytics summary`, `analytics posts`, `analytics platforms`, and `analytics platform`."],
  ["Examples", "`examples posts.create` plus `examples mcp.claude-code` for hosted MCP setup."],
  ["Agent", "`agent bootstrap`, `agent capabilities`, `agent guide`, `agent context`, `agent mcp-config`, `agent mcp-test`, `agent install`, `agent plan`, `agent plan-publish`, and restricted `agent execute`."],
] as const;

const USE_CASES = [
  {
    title: "Connect UniPost faster",
    label: "Build integration code",
    body: "Codex or Claude Code can read your project, add UniPost API, SDK, webhook, and environment-variable code, then run local tests and CLI validation against the same workspace.",
    command: "unipost examples posts.create --lang node --json",
  },
  {
    title: "Inspect real workspace data",
    label: "Query connected accounts",
    body: "Authenticated agents can retrieve profiles, connected accounts, account health, platform capabilities, and metrics before generating code or planning social workflows.",
    command: "unipost agent bootstrap --client codex --json",
  },
  {
    title: "Validate and draft safely",
    label: "Plan publishing work",
    body: "Agents can generate post payloads, validate them, dry-run publish requests, and create drafts. Live or scheduled publishing is never automatic and still needs explicit user approval.",
    command: "unipost posts validate --account sa_... --json",
  },
] as const;

const START_STEPS = [
  ["1. Install and sign in", "Install once with `npm install -g @unipost/cli`, then run `unipost init` or the Dashboard setup-token command. After install, use <code>unipost</code> commands everywhere. Update with <code>unipost upgrade</code> when you need the latest CLI."],
  ["2. Choose who will use it", "Use Terminal for human command-line work. Use Codex or Claude Code when a local agent should read project code, query UniPost context, and help implement or validate workflows."],
  ["3. Ground before action", "Run `auth status`, `profiles list`, `accounts list`, or `agent bootstrap` before drafting. Agents should validate, dry-run, or draft first; live publishing requires `--yes` and `--idempotency-key`."],
] as const;

const TROUBLESHOOTING_ROWS = [
  ["`zsh: command not found: unipost`", "Install the CLI once with `npm install -g @unipost/cli`, then open a new terminal and run `unipost --help`. If it still fails, check that your npm global bin directory is on PATH with `npm bin -g`."],
  ["Need the latest CLI", "Update with `unipost upgrade`, then run `unipost --version` to confirm the installed version."],
  ["Codex or Claude Code still does not know UniPost after setup-token login", "The setup token signs in the UniPost CLI only. Run `unipost agent install --client codex --json` or `unipost agent install --client claude-code --json`, then follow the returned instruction package setup in that agent."],
  ["`codex` or `claude` command is missing", "Install or open that AI agent separately. UniPost CLI does not install Codex, Claude Code, or any other local agent executable."],
  ["API key is missing or invalid", "Use the Dashboard setup token flow first. If you are running in CI, set `UNIPOST_API_KEY`, then run `unipost auth status --json`."],
  ["Replacing the local account binding is blocked", "UniPost CLI keeps one local binding. Run `unipost auth logout --json` first, or rerun the login/init command with `--yes` after confirming the new workspace should replace the old one."],
  ["`setup_token_invalid`, `setup_token_expired`, or `setup_token_used`", "Create a fresh Dashboard setup token. Setup tokens are short-lived and single-use, so copy the newest command from Dashboard before retrying."],
  ["`keychain_unavailable`", "The CLI could not store the named key in secure local storage. On Linux, Windows, or CI, use `UNIPOST_API_KEY`, pass `--api-key` for one-off commands, or rerun with `--metadata-only` if you only want redacted metadata."],
  ["Wrong API URL", "Copy the newest Dashboard setup command; it includes `--base-url` for the current environment. If you are configuring manually, run `unipost config set base_url https://dev-api.unipost.dev --json` for dev validation."],
  ["No profile or account IDs", "Run `unipost profiles list --json`, `unipost quickstart --name \"Brand\" --json`, or `unipost connect create --json`, then check `unipost accounts list --json`."],
  ["Live publish is blocked", "Start with `posts validate`, `posts draft`, or `posts create --dry-run`. Only add `--yes` and `--idempotency-key` after the user explicitly approves live or scheduled publishing."],
] as const;

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Developer tools"
      title="CLI - Overview"
      lead="Use UniPost from a terminal, or give a local AI agent a safe UniPost toolchain. Pick the setup path that matches what you are trying to do."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <section className="cli-use-cases" aria-labelledby="agent-use-cases">
        <div className="cli-use-cases-copy">
          <div className="cli-status-label">What the CLI unlocks</div>
          <h2 id="agent-use-cases">What UniPost CLI lets agents do</h2>
          <p>
            The CLI gives Codex, Claude Code, and terminal users a safe way to use your real UniPost workspace. It can speed up integration work, inspect connected social accounts, and prepare publishing workflows without turning live publishing into an automatic action.
          </p>
        </div>
        <div className="cli-use-case-grid">
          {USE_CASES.map((item) => (
            <article className="cli-use-case" key={item.title}>
              <div className="cli-use-case-label">{item.label}</div>
              <h3>{item.title}</h3>
              <p>{item.body}</p>
              <code>{item.command}</code>
            </article>
          ))}
        </div>
      </section>

      <h2 id="start">Start in three steps</h2>
      <DocsTable columns={["Step", "What to do"]} rows={START_STEPS} />

      <div className="cli-status">
        <div>
          <div className="cli-status-label">Beta status</div>
          <p>
            Dashboard setup tokens, npm-installed <code>unipost</code> commands, and existing API-key login are available now. On macOS, setup-token and API-key login store a secure local CLI credential; <code>UNIPOST_API_KEY</code> remains the CI-friendly fallback. Browser/device auth remains a later auth surface; for direct production integrations, use the <Link href="/docs/api">REST API</Link>, <Link href="/docs/sdk">SDKs</Link>, or <Link href="/docs/mcp">MCP</Link>.
          </p>
        </div>
        <div className="cli-phase-pill">Agent setup beta</div>
      </div>

      <h2 id="before-running">Before you run a command</h2>
      <DocsTable
        columns={["What you run", "What it actually does"]}
        rows={[
          ["`npm install -g @unipost/cli`", "Install once. This creates the persistent `unipost` command in your shell."],
          ["`unipost ...`", "The normal command prefix after install. Use this in Dashboard setup-token commands, terminal workflows, and agent setup."],
          ["`unipost upgrade`", "Updates the installed CLI package with npm, then you can run `unipost --version` to confirm."],
          ["`unipost self help`", "Shows CLI install, update, version, and help commands."],
          ["Dashboard setup-token command", "Exchanges the one-time token for a named CLI key when no valid local binding exists. If the current binding is still valid, the CLI returns `already_configured` and does not consume the token."],
          ["`codex`, `claude`, or another agent command", "A separate local AI agent. UniPost CLI does not install those programs."],
        ]}
      />

      <h2 id="setup-paths">Choose your setup path</h2>
      <p>
        The setup token only logs the UniPost CLI in. It does not install or configure Codex, Claude Code, or any other agent. If you only want to use UniPost from a terminal, follow Path A. If you want a local AI agent to use UniPost, finish Path A first, then follow Path B.
      </p>

      <h3 id="path-command-line">Path A: Command line only</h3>
      <DocsTable
        columns={["Step", "What to do"]}
        rows={[
          ["1. Install once", "Run `npm install -g @unipost/cli`, then confirm `unipost --help` works."],
          ["2. Choose a credential path", "Run `unipost init --api-key up_live_... --json` when you already have a key, or open Dashboard -> Project -> API Keys -> Set up UniPost CLI and copy the generated setup-token command."],
          ["3. Run it exactly", "Run the copied Dashboard command as-is, or run `unipost init --api-key <key> --json`. Dashboard commands should include `--setup-token`, `--client terminal`, `--base-url`, and `--json`. To replace an existing local binding, run `unipost auth logout --json` first or rerun with `--replace-key` after confirming the new workspace."],
          ["4. Verify auth", "Run `unipost auth status --json`. Confirm the credential source and `base_url` match the environment you are using."],
          ["5. Discover real IDs", "Run `unipost profiles list --json`, `unipost accounts list --json`, and `unipost accounts health --account sa_... --json` before validating or drafting posts."],
        ]}
      />
      <DocsCodeTabs snippets={CLI_AUTH_SNIPPETS} />

      <h3 id="path-local-agent">Path B: Local AI agent</h3>
      <p>
        Path B is for Codex, Claude Code, Cursor, or another local agent that should operate UniPost safely. This path does not install the agent executable itself; install or open that agent separately if its own command is missing.
      </p>
      <DocsTable
        columns={["Step", "What to do"]}
        rows={[
          ["1. Finish CLI auth", "Complete Path A first. The agent will reuse the same UniPost CLI credentials and config."],
          ["2. Add UniPost instructions", "Run `unipost agent install --client codex --json` or `unipost agent install --client claude-code --json`, then follow the returned instruction package setup."],
          ["3. Start the agent with grounding", "In the agent session, have it run `unipost agent bootstrap --client codex --json`, `agent capabilities`, and `agent context` before it plans or writes anything. Agent context includes workspace, profiles, accounts, recent posts, and a recent post summary."],
          ["4. Keep publish safe", "Agents should validate, draft, or dry-run first. Live or scheduled publish still requires explicit user approval plus `--yes` and `--idempotency-key`."],
        ]}
      />
      <DocsCodeTabs snippets={AGENT_SETUP_SNIPPETS} />

      <h2 id="configure">Configure the CLI</h2>
      <p>
        The CLI stores non-secret settings such as <code>base_url</code> and <code>default_profile_id</code> in its local config file. Production defaults to <code>https://api.unipost.dev</code>. When validating the development environment, set <code>base_url</code> to <code>https://dev-api.unipost.dev</code>.
      </p>
      <DocsCodeTabs snippets={CONFIG_SNIPPETS} />

      <h2 id="common-issues">Common issues</h2>
      <DocsTable columns={["Problem", "Fix"]} rows={TROUBLESHOOTING_ROWS} />

      <h2 id="command-groups">Command groups</h2>
      <p>
        This overview keeps the setup path and safety model short. For every supported command, copyable examples, and sample JSON responses, open the <Link href="/docs/cli/reference">CLI Reference</Link>. For plain-language prompts that tell Codex or Claude Code how to use the CLI, open the <Link href="/docs/cli/agents">AI Agent Guide</Link>.
      </p>
      <DocsTable columns={["Group", "Commands"]} rows={COMMAND_ROWS} />

      <h2 id="safe-publishing">Safe publishing model</h2>
      <p>
        Phase 5 keeps publish-capable commands behind the same guardrails. Validation, draft creation, media upload, readiness waits, dry-runs, MCP auth tests, and instruction setup stay safe without <code>--yes</code>; live and scheduled publishing require explicit approval plus a stable idempotency key. The <code>agent execute</code> beta only runs current <code>agent plan --json</code> envelopes with matching <code>catalog_version</code> and structured read-only, validate, or draft-write actions; it rejects stale plans, live publish actions, pending confirmations, and raw command strings.
      </p>
      <DocsTable
        columns={["Action", "Non-interactive rule"]}
        rows={[
          ["Validate", "Allowed without `--yes`."],
          ["Draft", "Allowed without `--yes`; it does not publish externally."],
          ["Dry-run", "Allowed without `--yes` because it validates and previews only."],
          ["Live publish", "Requires `--yes` and `--idempotency-key`."],
          ["Scheduled publish", "Requires `--yes` and `--idempotency-key` because it eventually posts externally."],
          ["Cancel or retry", "Requires explicit resource IDs and `--yes`."],
        ]}
      />
      <DocsCodeTabs snippets={SAFE_POST_SNIPPETS} />

      <h2 id="agent-contract">Agent contract</h2>
      <p>
        Codex, Claude Code, Cursor, and other agents should use <code>agent bootstrap</code>, <code>agent capabilities</code>, <code>agent context</code>, <code>agent mcp-config</code>, <code>agent mcp-test</code>, <code>agent install</code>, and <code>agent plan</code> before writing. The contract keeps account discovery, planning, validation, draft creation, and live-publish approvals explicit.
      </p>
      <DocsTable
        columns={["Primitive", "Contract"]}
        rows={[
          ["Capability catalog", "Returns supported intent names, input schemas, safety levels, canonical actions, and `catalog_version`."],
          ["Context grounding", "Returns real workspace, profile, account, defaults, and setup-readiness context."],
          ["Agent guide", "Returns client-specific prompt guidance for safe validate/dry-run-before-publish workflows."],
          ["Agent plan", "Returns structured actions for draft or publish intent, including missing inputs and required user confirmations."],
          ["MCP bridge", "`examples mcp.claude-code`, `agent mcp-config`, and `agent mcp-test` bootstrap the hosted MCP endpoint without replacing MCP."],
          ["Instruction packages", "`agent install --client codex|claude-code` points agents at first-party instructions for safe UniPost operation."],
          ["Execute beta", "`agent execute --plan plan.json` accepts only current `agent plan --json` envelopes, uses the registry safety level instead of plan-provided `safety_level`, and rejects live publish plans."],
          ["Async waits", "`connect wait`, `posts wait`, and `media wait` let agents observe terminal state instead of polling blindly."],
          ["Status enums", "CLI JSON normalizes backend aliases such as `cancelled` to canonical `canceled`."],
        ]}
      />

      <h2 id="json-output">JSON and exit codes</h2>
      <p>
        Every agent-relevant Phase 5 command supports a stable envelope. Machine fields such as <code>code</code>, <code>normalized_code</code>, <code>catalog_version</code>, and <code>status</code> stay stable English identifiers even when human messages are localized.
      </p>
      <DocsCodeTabs snippets={JSON_ENVELOPE} />
      <DocsTable
        columns={["Exit code", "Meaning"]}
        rows={[
          ["0", "success"],
          ["1", "generic error"],
          ["2", "invalid arguments"],
          ["3", "missing required input"],
          ["4", "authentication failure"],
          ["5", "authorization failure"],
          ["6", "validation failure"],
          ["7", "upstream UniPost API failure"],
          ["8", "network failure"],
          ["9", "unsafe action blocked"],
          ["10", "timeout"],
        ]}
      />
    </DocsPage>
  );
}

const styles = `
.cli-use-cases{display:grid;grid-template-columns:minmax(240px,.82fr) minmax(0,1.18fr);gap:22px;align-items:start;margin:10px 0 28px;padding:18px 0 24px;border-bottom:1px solid var(--docs-border)}
.cli-use-cases-copy h2{margin:6px 0 0;font-size:28px;line-height:1.16;color:var(--docs-text);font-weight:760;letter-spacing:0}
.cli-use-cases-copy p{margin:12px 0 0;color:var(--docs-text-soft);font-size:15px;line-height:1.65;max-width:58ch}
.cli-use-case-grid{display:grid;gap:10px;min-width:0}
.cli-use-case{border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated);padding:14px 16px;min-width:0}
.cli-use-case-label{font-family:var(--docs-mono);font-size:11px;font-weight:800;letter-spacing:.04em;text-transform:uppercase;color:var(--docs-text-faint)}
.cli-use-case h3{margin:6px 0 0;font-size:17px;line-height:1.3;color:var(--docs-text);font-weight:720;letter-spacing:0}
.cli-use-case p{margin:8px 0 0;color:var(--docs-text-soft);font-size:14px;line-height:1.6}
.cli-use-case code{display:block;margin-top:10px;color:var(--docs-text);font-family:var(--docs-mono);font-size:12px;line-height:1.55;overflow-wrap:anywhere}
.cli-status{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:18px;align-items:start;margin:8px 0 26px;padding:18px 20px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated)}
.cli-status p{margin:6px 0 0;color:var(--docs-text-soft);line-height:1.65}
.cli-status-label{font-size:12px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint);font-family:var(--docs-mono)}
.cli-phase-pill{display:inline-flex;align-items:center;justify-content:center;min-height:34px;padding:0 12px;border:1px solid var(--docs-border);border-radius:999px;background:var(--docs-bg-muted);color:var(--docs-text);font-family:var(--docs-mono);font-size:12px;font-weight:700;white-space:nowrap}
@media (max-width:920px){.cli-use-cases{grid-template-columns:1fr;gap:14px}.cli-use-cases-copy h2{font-size:24px}}
@media (max-width:720px){.cli-status{grid-template-columns:1fr}.cli-phase-pill{justify-content:flex-start;width:max-content;max-width:100%}}
`;
