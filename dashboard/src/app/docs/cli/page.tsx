import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const QUICKSTART_SNIPPETS = [
  {
    label: "Phase 2 source run",
    lang: "bash",
    code: `cd cli
npm test
export UNIPOST_API_KEY=up_live_...
node bin/unipost.js init --json
node bin/unipost.js quickstart --name "Brand" --json
node bin/unipost.js agent bootstrap --client codex --json
node bin/unipost.js posts validate --account sa_... --caption "Shipping with UniPost CLI." --json`,
  },
  {
    label: "Planned npm install",
    lang: "bash",
    code: `npm install -g @unipost/cli
export UNIPOST_API_KEY=up_live_...
unipost init
unipost quickstart --name "Brand"
unipost agent guide --client codex`,
  },
  {
    label: "Agent setup",
    lang: "bash",
    code: `unipost agent capabilities --json
unipost agent bootstrap --client codex --json
unipost agent guide --client claude-code --json
unipost agent mcp-config --client codex`,
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
    label: "Native fetch example",
    lang: "bash",
    code: `unipost examples posts.create \\
  --lang node \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI."`,
  },
  {
    label: "Later live-write guardrail",
    lang: "bash",
    code: `unipost posts create \\
  --from-file post.json \\
  --yes \\
  --idempotency-key user-approved-2026-06-02-001 \\
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
    "hint": "Set UNIPOST_API_KEY or pass --api-key.",
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
  ["Auth and config", "Phase 2: `auth status`, `auth list`, `auth use`; later: browser/device login, logout, and config inspection."],
  ["Quickstart", "Phase 2: `init`, `doctor`, `quickstart`, `profiles list/create/get/use`, `connect create/get/wait`."],
  ["Accounts", "Phase 2: `accounts list`, `accounts get`; later: health, capability, and metric views."],
  ["Posts", "Phase 2: `posts validate`, `posts draft`; later: publish, schedule, wait, cancel, retry, list, and get."],
  ["Media", "Later phase: upload, inspect, and wait for media processing."],
  ["Analytics", "Later phase: workspace, post, platform, and account-level reporting."],
  ["Examples", "Phase 2: `examples posts.create` with cURL and native Node fetch; later: connect and MCP examples."],
  ["Agent", "Phase 2: `agent bootstrap`, `agent capabilities`, `agent guide`, `agent context`, `agent mcp-config`; later: planning and execution helpers."],
] as const;

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Phase 2"
      title="CLI"
      lead="The UniPost CLI is the first-party terminal interface for developer quickstarts, CI checks, support diagnostics, and AI-agent draft workflows."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cli-status">
        <div>
          <div className="cli-status-label">Current status</div>
          <p>
            The Phase 2 source package now supports <code>init</code>, <code>quickstart</code>, profile setup, connect-session helpers, account discovery, post validation and draft creation, stable JSON envelopes, and agent bootstrap/context commands. Public npm release, browser/device auth, and live publishing commands remain later phases; for direct production integrations, use the <Link href="/docs/api">REST API</Link>, <Link href="/docs/sdk">SDKs</Link>, or <Link href="/docs/mcp">MCP</Link>.
          </p>
        </div>
        <div className="cli-phase-pill">Phase 2 quickstart</div>
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

      <h2 id="planned-install">Install and first run</h2>
      <p>
        Phase 2 supports API-key fallback with <code>UNIPOST_API_KEY</code>. Browser/device auth and Dashboard setup tokens are planned for the full agent-assisted onboarding flow.
      </p>
      <DocsCodeTabs snippets={QUICKSTART_SNIPPETS} />

      <h2 id="command-groups">Command groups</h2>
      <DocsTable columns={["Group", "Commands"]} rows={COMMAND_ROWS} />

      <h2 id="safe-publishing">Safe publishing model</h2>
      <p>
        Phase 2 stops at validation and draft creation. Immediate or scheduled publishing is treated as a later live-write surface and will require explicit confirmation in non-interactive usage.
      </p>
      <DocsTable
        columns={["Action", "Non-interactive rule"]}
        rows={[
          ["Validate", "Allowed without `--yes`."],
          ["Draft", "Allowed without `--yes`; it does not publish externally."],
          ["Dry-run", "Later phase; allowed without `--yes` because it validates and previews only."],
          ["Live publish", "Later phase; requires `--yes` and `--idempotency-key`."],
          ["Scheduled publish", "Later phase; requires `--yes` and `--idempotency-key` because it eventually posts externally."],
          ["Cancel or retry", "Later phase; requires explicit resource IDs and `--yes`."],
        ]}
      />
      <DocsCodeTabs snippets={SAFE_POST_SNIPPETS} />

      <h2 id="agent-contract">Agent contract</h2>
      <p>
        Codex, Claude Code, and other agents should use <code>agent bootstrap</code>, <code>agent capabilities</code>, <code>agent guide</code>, <code>agent context</code>, and <code>agent mcp-config</code> before writing. Planning and execution helpers are deferred until later security review.
      </p>
      <DocsTable
        columns={["Primitive", "Contract"]}
        rows={[
          ["Capability catalog", "Returns supported intent names, input schemas, safety levels, canonical actions, and `catalog_version`."],
          ["Context grounding", "Returns real workspace, profile, account, defaults, and setup-readiness context."],
          ["Agent guide", "Returns client-specific prompt guidance for safe validate-before-draft workflows."],
          ["Async waits", "`connect wait` lets agents observe terminal connection state instead of polling blindly; media and post waits are later."],
          ["Status enums", "CLI JSON normalizes backend aliases such as `cancelled` to canonical `canceled`."],
        ]}
      />

      <h2 id="json-output">JSON and exit codes</h2>
      <p>
        Every agent-relevant Phase 2 command supports a stable envelope. Machine fields such as <code>code</code>, <code>normalized_code</code>, <code>catalog_version</code>, and <code>status</code> stay stable English identifiers even when human messages are localized.
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
          ["Credentials", "Phase 2 uses `UNIPOST_API_KEY` or `--api-key`; local config stores defaults only, not secrets. OS keychain and browser/device auth are later."],
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
