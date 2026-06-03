import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const SETUP_SNIPPETS = [
  {
    label: "Dashboard setup token",
    lang: "bash",
    code: `# Dashboard: Project -> API Keys -> Connect with Claude Code / Codex
npx -y @unipost/cli agent bootstrap --client codex --setup-token ust_... --base-url https://api.unipost.dev --json
npx -y @unipost/cli auth status --json
npx -y @unipost/cli agent bootstrap --client codex --json`,
  },
  {
    label: "API key fallback",
    lang: "bash",
    code: `export UNIPOST_API_KEY=up_live_...
npx -y @unipost/cli auth login --api-key "$UNIPOST_API_KEY" --json
npx -y @unipost/cli auth status --json`,
  },
  {
    label: "Optional global install",
    lang: "bash",
    code: `npm install -g @unipost/cli
unipost auth status --json
unipost agent bootstrap --client codex --json`,
  },
];

const CONFIG_SNIPPETS = [
  {
    label: "Check current config",
    lang: "bash",
    code: `npx -y @unipost/cli config path --json
npx -y @unipost/cli config show --json
npx -y @unipost/cli auth status --json`,
  },
  {
    label: "Use dev API",
    lang: "bash",
    code: `npx -y @unipost/cli config set base_url https://dev-api.unipost.dev --json
npx -y @unipost/cli config show --json`,
  },
  {
    label: "Set defaults",
    lang: "bash",
    code: `npx -y @unipost/cli profiles list --json
npx -y @unipost/cli config set default_profile_id pr_... --json
npx -y @unipost/cli accounts list --json
npx -y @unipost/cli accounts health --account sa_... --json`,
  },
];

const SAFE_POST_SNIPPETS = [
  {
    label: "Validate and draft",
    lang: "bash",
    code: `npx -y @unipost/cli posts validate --account sa_... --caption "Shipping with UniPost CLI."
npx -y @unipost/cli posts draft --account sa_... --caption "Shipping with UniPost CLI."`,
  },
  {
    label: "Dry-run before approval",
    lang: "bash",
    code: `npx -y @unipost/cli posts create \\
  --from-file post.json \\
  --dry-run \\
  --json`,
  },
  {
    label: "Approved scheduled publish",
    lang: "bash",
    code: `npx -y @unipost/cli posts schedule \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI." \\
  --at 2026-06-10T09:00:00Z \\
  --yes \\
  --idempotency-key user-approved-2026-06-03-001 \\
  --json

npx -y @unipost/cli posts wait post_... --timeout 120 --json`,
  },
  {
    label: "Cancel or retry",
    lang: "bash",
    code: `npx -y @unipost/cli posts cancel post_... --yes --json
npx -y @unipost/cli posts retry post_... --result result_... --yes --json`,
  },
  {
    label: "Media upload",
    lang: "bash",
    code: `npx -y @unipost/cli media upload ./video.mp4 --json
npx -y @unipost/cli media wait med_... --timeout 120 --json

npx -y @unipost/cli posts create \\
  --from-file post-with-media.json \\
  --dry-run \\
  --json`,
  },
  {
    label: "Native fetch example",
    lang: "bash",
    code: `npx -y @unipost/cli examples posts.create \\
  --lang node \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI." \\
  --json`,
  },
  {
    label: "Structured agent execute",
    lang: "bash",
    code: `npx -y @unipost/cli agent plan \\
  --intent create_draft_post \\
  --from-file post.json \\
  --json > safe-plan.json

npx -y @unipost/cli agent execute \\
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
    "cli_version": "0.1.1",
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
    "cli_version": "0.1.1",
    "command": "auth status",
    "source": "cli"
  }
}`,
  },
];

const COMMAND_ROWS = [
  ["Auth and config", "`config path`, `config show`, `config set base_url`, `config set default_profile_id`, `auth login --setup-token`, `auth login --api-key`, `auth logout`, `auth status`, `auth list`, and `auth use`."],
  ["Quickstart", "`init`, `doctor`, `quickstart`, `profiles list/create/get/use`, and `connect create/get/wait`."],
  ["Accounts", "`accounts list`, `accounts get`, `accounts health`, `accounts capabilities`, and `accounts metrics`."],
  ["Posts", "`posts list`, `posts get`, `posts analytics`, `posts validate`, `posts draft`, `posts create --dry-run`, `posts create`, `posts schedule`, `posts publish-draft`, `posts wait`, `posts cancel`, and `posts retry`."],
  ["Media", "`media upload`, `media get`, and `media wait`."],
  ["Analytics", "`analytics summary`, `analytics posts`, `analytics platforms`, and `analytics platform`."],
  ["Examples", "`examples posts.create` plus `examples mcp.claude-code` for hosted MCP setup."],
  ["Agent", "`agent bootstrap`, `agent capabilities`, `agent guide`, `agent context`, `agent mcp-config`, `agent mcp-test`, `agent install`, `agent plan`, `agent plan-publish`, and restricted `agent execute`."],
] as const;

const TROUBLESHOOTING_ROWS = [
  ["API key is missing or invalid", "Use the Dashboard setup token flow first. If you are running in CI, set `UNIPOST_API_KEY`, then run `npx -y @unipost/cli auth status --json`."],
  ["`setup_token_invalid`, `setup_token_expired`, or `setup_token_used`", "Create a fresh Dashboard setup token. Setup tokens are short-lived and single-use, so copy the newest command from Dashboard before retrying."],
  ["`keychain_unavailable`", "The CLI could not store the named key in OS keychain. Retry from a normal logged-in desktop shell, or use `UNIPOST_API_KEY` as the fallback auth path."],
  ["Wrong API URL", "Copy the newest Dashboard setup command; it includes `--base-url` for the current environment. If you are configuring manually, run `npx -y @unipost/cli config set base_url https://dev-api.unipost.dev --json` for dev validation."],
  ["No profile or account IDs", "Run `npx -y @unipost/cli profiles list --json`, `npx -y @unipost/cli quickstart --name \"Brand\" --json`, or `npx -y @unipost/cli connect create --json`, then check `npx -y @unipost/cli accounts list --json`."],
  ["Live publish is blocked", "Start with `posts validate`, `posts draft`, or `posts create --dry-run`. Only add `--yes` and `--idempotency-key` after the user explicitly approves live or scheduled publishing."],
] as const;

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Developer tools"
      title="CLI"
      lead="Configure UniPost auth, account defaults, safe publish workflows, and AI-agent setup from your terminal."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cli-status">
        <div>
          <div className="cli-status-label">Beta status</div>
          <p>
            Dashboard setup tokens, npm-based <code>npx -y @unipost/cli</code> setup, and API-key fallback are available now. A setup token creates a named revocable CLI key and stores it in OS keychain; <code>UNIPOST_API_KEY</code> remains the CI-friendly fallback. Browser/device auth remains a later auth surface; for direct production integrations, use the <Link href="/docs/api">REST API</Link>, <Link href="/docs/sdk">SDKs</Link>, or <Link href="/docs/mcp">MCP</Link>.
          </p>
        </div>
        <div className="cli-phase-pill">Agent setup beta</div>
      </div>

      <h2 id="what-it-is">What the CLI is for</h2>
      <DocsTable
        columns={["Audience", "Job"]}
        rows={[
          ["Developers", "Verify auth, discover real account IDs, connect accounts, validate posts, create drafts, and generate cURL/native HTTP examples."],
          ["AI agents", "Use stable JSON, intent catalogs, safe planning, wait commands, and explicit publish guardrails instead of scraping docs or guessing command syntax."],
          ["Support and CI", "Run deterministic diagnostics, inspect request IDs, check account health, and branch on exit codes."],
        ]}
      />

      <h2 id="setup-steps">Set up the CLI in 3 steps</h2>
      <p>
        Use the Dashboard setup token flow for local development and agent setup. Use <code>UNIPOST_API_KEY</code> for CI, containers, or shells where keychain storage is unavailable.
      </p>
      <DocsTable
        columns={["Step", "What to do"]}
        rows={[
          ["1. Sign in", "In Dashboard, open Project -> API Keys -> Connect with Claude Code / Codex, copy the setup-token command, and run it in the terminal. The copied command includes the API URL for the current environment."],
          ["2. Verify auth", "Run `npx -y @unipost/cli auth status --json`. The result should show the active credential source and the configured API base URL."],
          ["3. Pick defaults", "Run `profiles list`, `accounts list`, and `accounts health`; set `default_profile_id` if you want shorter commands later."],
        ]}
      />
      <DocsCodeTabs snippets={SETUP_SNIPPETS} />

      <h2 id="configure">Configure the CLI</h2>
      <p>
        The CLI stores non-secret settings such as <code>base_url</code> and <code>default_profile_id</code> in its local config file. Production defaults to <code>https://api.unipost.dev</code>. When validating the development environment, set <code>base_url</code> to <code>https://dev-api.unipost.dev</code>.
      </p>
      <DocsCodeTabs snippets={CONFIG_SNIPPETS} />

      <h2 id="common-issues">Common issues</h2>
      <DocsTable columns={["Problem", "Fix"]} rows={TROUBLESHOOTING_ROWS} />

      <h2 id="command-groups">Command groups</h2>
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
        Codex, Claude Code, Cursor, and other agents should use <code>agent bootstrap</code>, <code>agent capabilities</code>, <code>agent guide</code>, <code>agent context</code>, <code>agent mcp-config</code>, <code>agent mcp-test</code>, <code>agent install</code>, and <code>agent plan</code> before writing. Phase 5 mirrors the same intent names into the MCP agent contract so clients can choose CLI commands, MCP tools, or client-specific instruction packages without guessing terminal syntax.
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

      <h2 id="status-enums">Canonical statuses</h2>
      <DocsTable
        columns={["Resource", "CLI-facing status values"]}
        rows={[
          ["Post", "`draft`, `scheduled`, `publishing`, `published`, `partial`, `failed`, `canceled`"],
          ["Connect session", "`pending`, `completed`, `expired`, `canceled`"],
          ["Media", "`pending`, `processing`, `ready`, `failed`"],
        ]}
      />

      <h2 id="runtime-contract">Runtime behavior</h2>
      <DocsTable
        columns={["Area", "Behavior"]}
        rows={[
          ["Pagination", "`--limit`, `--cursor`, and `--all`; JSON metadata includes `next_cursor` when available."],
          ["Output", "`--json` or `--output json`; `--field` for scripts; `--no-color` and `NO_COLOR=1` for plain output."],
          ["Networking", "Bounded retries for reads and idempotent writes; respect `Retry-After`; no automatic retry for unsafe writes without idempotency."],
          ["Credentials", "Setup-token login stores the named CLI key in OS keychain and keeps only locator/redacted metadata in config. `UNIPOST_API_KEY` and `--api-key` remain fallback auth paths; `auth logout` clears local credentials, and remote revoke happens from Dashboard."],
          ["Telemetry", "First-run notice, redaction, and opt-out through config, `--no-telemetry`, or `UNIPOST_TELEMETRY=0`."],
        ]}
      />
    </DocsPage>
  );
}

const styles = `
.cli-status{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:18px;align-items:start;margin:8px 0 26px;padding:18px 20px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated)}
.cli-status p{margin:6px 0 0;color:var(--docs-text-soft);line-height:1.65}
.cli-status-label{font-size:12px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint);font-family:var(--docs-mono)}
.cli-phase-pill{display:inline-flex;align-items:center;justify-content:center;min-height:34px;padding:0 12px;border:1px solid var(--docs-border);border-radius:999px;background:var(--docs-bg-muted);color:var(--docs-text);font-family:var(--docs-mono);font-size:12px;font-weight:700;white-space:nowrap}
@media (max-width:720px){.cli-status{grid-template-columns:1fr}.cli-phase-pill{justify-content:flex-start;width:max-content;max-width:100%}}
`;
