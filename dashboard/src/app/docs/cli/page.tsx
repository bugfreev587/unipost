import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const QUICKSTART_SNIPPETS = [
  {
    label: "Planned install",
    lang: "bash",
    code: `npm install -g @unipost/cli
unipost auth status
unipost doctor
unipost quickstart --platform linkedin`,
  },
  {
    label: "Agent bootstrap",
    lang: "bash",
    code: `npx -y @unipost/cli agent bootstrap --client codex --json
unipost agent capabilities --client codex --json
unipost agent context --json`,
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
    label: "Approved publish",
    lang: "bash",
    code: `unipost posts create \\
  --from-file post.json \\
  --yes \\
  --idempotency-key user-approved-2026-06-02-001 \\
  --json \\
  --non-interactive`,
  },
  {
    label: "Schedule and wait",
    lang: "bash",
    code: `unipost posts schedule \\
  --account sa_... \\
  --caption "Launching tomorrow." \\
  --at 2026-06-10T09:00:00Z \\
  --yes \\
  --idempotency-key schedule-2026-06-10-001 \\
  --json

unipost posts wait post_... --timeout 120 --json`,
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
    "cli_version": "0.1.0",
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
    "hint": "Set UNIPOST_API_KEY or run unipost auth login.",
    "docs_url": "https://unipost.dev/docs/quickstart"
  },
  "warnings": [],
  "meta": {
    "request_id": "req_...",
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.1.0",
    "command": "auth status",
    "source": "cli"
  }
}`,
  },
];

const COMMAND_ROWS = [
  ["Auth and config", "`auth status`, `auth login`, `auth list`, `auth use`, `auth logout`, `config show`, `config path`"],
  ["Quickstart", "`init`, `doctor`, `quickstart`, `profiles list/create/use`, `connect create/get/wait`"],
  ["Accounts", "`accounts list`, `accounts get`, `accounts health`, `accounts capabilities`, `accounts metrics`"],
  ["Posts", "`posts validate`, `posts draft`, `posts create`, `posts schedule`, `posts publish-draft`, `posts wait`, `posts cancel`, `posts retry`, `posts list`, `posts get`"],
  ["Media", "`media upload`, `media get`, `media wait`"],
  ["Analytics", "`analytics summary`, `analytics posts`, `analytics platforms`, `analytics platform`"],
  ["Examples", "`examples posts.create`, `examples connect.create`, `examples mcp.claude-code`"],
  ["Agent", "`agent bootstrap`, `agent capabilities`, `agent guide`, `agent context`, `agent plan`, `agent plan-publish`, `agent mcp-config`"],
] as const;

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Planned"
      title="CLI"
      lead="The UniPost CLI is the planned first-party terminal interface for developer quickstarts, CI checks, support diagnostics, and AI-agent operations."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cli-status">
        <div>
          <div className="cli-status-label">Current status</div>
          <p>
            The command contract is being implemented in phases. The npm package is planned as <code>@unipost/cli</code>; until it is released, use the <Link href="/docs/api">REST API</Link>, <Link href="/docs/sdk">SDKs</Link>, or <Link href="/docs/mcp">MCP</Link>.
          </p>
        </div>
        <div className="cli-phase-pill">Phase 0 contract</div>
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

      <h2 id="planned-install">Planned install and first run</h2>
      <p>
        The first public release will support API-key fallback with <code>UNIPOST_API_KEY</code>. Browser/device auth and Dashboard setup tokens are planned for the full agent-assisted onboarding flow.
      </p>
      <DocsCodeTabs snippets={QUICKSTART_SNIPPETS} />

      <h2 id="command-groups">Command groups</h2>
      <DocsTable columns={["Group", "Planned commands"]} rows={COMMAND_ROWS} />

      <h2 id="safe-publishing">Safe publishing model</h2>
      <p>
        The CLI defaults to validation and draft creation. Immediate or scheduled publishing is treated as a live write and requires explicit confirmation in non-interactive usage.
      </p>
      <DocsTable
        columns={["Action", "Non-interactive rule"]}
        rows={[
          ["Validate", "Allowed without `--yes`."],
          ["Draft", "Allowed without `--yes`; it does not publish externally."],
          ["Dry-run", "Allowed without `--yes`; it validates and previews only."],
          ["Live publish", "Requires `--yes` and `--idempotency-key`."],
          ["Scheduled publish", "Requires `--yes` and `--idempotency-key` because it eventually posts externally."],
          ["Cancel or retry", "Requires explicit resource IDs and `--yes`."],
        ]}
      />
      <DocsCodeTabs snippets={SAFE_POST_SNIPPETS} />

      <h2 id="agent-contract">Agent contract</h2>
      <p>
        Codex, Claude Code, and other agents should use <code>agent capabilities</code>, <code>agent guide</code>, <code>agent context</code>, and <code>agent plan</code> before writing. The optional <code>agent execute</code> runner is deferred until a later security review.
      </p>
      <DocsTable
        columns={["Primitive", "Contract"]}
        rows={[
          ["Capability catalog", "Returns supported intent names, input schemas, safety levels, canonical actions, and `catalog_version`."],
          ["Context grounding", "Returns real workspace, profile, account, recent post, and failed post context."],
          ["Intent planning", "Returns structured actions and args, missing inputs, required confirmations, and display-only commands."],
          ["Async waits", "`connect wait`, `media wait`, and `posts wait` let agents observe terminal state instead of polling blindly."],
          ["Status enums", "CLI JSON normalizes backend aliases such as `cancelled` to canonical `canceled`."],
        ]}
      />

      <h2 id="json-output">JSON and exit codes</h2>
      <p>
        Every agent-relevant command will support a stable envelope. Machine fields such as <code>code</code>, <code>normalized_code</code>, <code>intent</code>, <code>safety_level</code>, and <code>status</code> stay stable English identifiers even when human messages are localized.
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
        columns={["Area", "Planned behavior"]}
        rows={[
          ["Pagination", "`--limit`, `--cursor`, and `--all`; JSON metadata includes `next_cursor` when available."],
          ["Output", "`--json` or `--output json`; `--field` for scripts; `--no-color` and `NO_COLOR=1` for plain output."],
          ["Networking", "Bounded retries for reads and idempotent writes; respect `Retry-After`; no automatic retry for unsafe writes without idempotency."],
          ["Credentials", "Prefer OS keychain for local CLI-created keys; use `UNIPOST_API_KEY` for CI and fallback."],
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
