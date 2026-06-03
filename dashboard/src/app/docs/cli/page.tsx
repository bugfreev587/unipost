import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../_components/docs-shell";

const QUICKSTART_SNIPPETS = [
  {
    label: "Phase 3 source run",
    lang: "bash",
    code: `cd cli
npm test
export UNIPOST_API_KEY=up_live_...
node bin/unipost.js init --json
node bin/unipost.js quickstart --name "Brand" --json
node bin/unipost.js agent bootstrap --client codex --json
node bin/unipost.js agent plan --intent plan_publish_post --from-file post.json --json
node bin/unipost.js posts create --from-file post.json --dry-run --json`,
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
unipost agent plan-publish --from-file post.json --json
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
    label: "Native fetch example",
    lang: "bash",
    code: `unipost examples posts.create \\
  --lang node \\
  --account sa_... \\
  --caption "Shipping with UniPost CLI." \\
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
  ["Auth and config", "Phase 3: `auth status`, `auth list`, `auth use`; later: browser/device login, logout, and config inspection."],
  ["Quickstart", "Phase 3: `init`, `doctor`, `quickstart`, `profiles list/create/get/use`, `connect create/get/wait`."],
  ["Accounts", "Phase 3: `accounts list`, `accounts get`; later: health, capability, and metric views."],
  ["Posts", "Phase 3: `posts list`, `posts get`, `posts analytics`, `posts validate`, `posts draft`, `posts create --dry-run`, `posts create`, `posts schedule`, `posts publish-draft`, `posts wait`, `posts cancel`, `posts retry`."],
  ["Media", "Phase 3: `media get`; later: local upload and media wait."],
  ["Analytics", "Phase 3: `analytics summary`, `analytics posts`, `analytics platforms`, `analytics platform`."],
  ["Examples", "Phase 3: `examples posts.create` with cURL and native Node fetch; later: connect and MCP examples."],
  ["Agent", "Phase 3: `agent bootstrap`, `agent capabilities`, `agent guide`, `agent context`, `agent mcp-config`, `agent plan`, `agent plan-publish`."],
] as const;

export default function CliPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Phase 3"
      title="CLI"
      lead="The UniPost CLI is the first-party terminal interface for developer quickstarts, CI checks, support diagnostics, and AI-agent operator workflows."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cli-status">
        <div>
          <div className="cli-status-label">Current status</div>
          <p>
            The Phase 3 source package now supports <code>init</code>, <code>quickstart</code>, profile setup, connect-session helpers, account discovery, stable JSON envelopes, post dry-runs, scheduled publish, post waits, cancel/retry workflows, media and analytics reads, and structured agent planning. Public npm release and browser/device auth remain later phases; for direct production integrations, use the <Link href="/docs/api">REST API</Link>, <Link href="/docs/sdk">SDKs</Link>, or <Link href="/docs/mcp">MCP</Link>.
          </p>
        </div>
        <div className="cli-phase-pill">Phase 3 operator beta</div>
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
        Phase 3 supports API-key fallback with <code>UNIPOST_API_KEY</code>. Browser/device auth and Dashboard setup tokens are planned for the full agent-assisted onboarding flow.
      </p>
      <DocsCodeTabs snippets={QUICKSTART_SNIPPETS} />

      <h2 id="command-groups">Command groups</h2>
      <DocsTable columns={["Group", "Commands"]} rows={COMMAND_ROWS} />

      <h2 id="safe-publishing">Safe publishing model</h2>
      <p>
        Phase 3 exposes publish-capable commands, but blocks accidental live writes. Validation, draft creation, and dry-runs stay safe without <code>--yes</code>; live and scheduled publishing require explicit approval plus a stable idempotency key.
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
        Codex, Claude Code, and other agents should use <code>agent bootstrap</code>, <code>agent capabilities</code>, <code>agent guide</code>, <code>agent context</code>, <code>agent mcp-config</code>, and <code>agent plan</code> before writing. Plans return missing inputs, required confirmations, canonical actions, and safe dry-run steps so agents do not infer command syntax from prose.
      </p>
      <DocsTable
        columns={["Primitive", "Contract"]}
        rows={[
          ["Capability catalog", "Returns supported intent names, input schemas, safety levels, canonical actions, and `catalog_version`."],
          ["Context grounding", "Returns real workspace, profile, account, defaults, and setup-readiness context."],
          ["Agent guide", "Returns client-specific prompt guidance for safe validate/dry-run-before-publish workflows."],
          ["Agent plan", "Returns structured actions for draft or publish intent, including missing inputs and required user confirmations."],
          ["Async waits", "`connect wait` and `posts wait` let agents observe terminal state instead of polling blindly."],
          ["Status enums", "CLI JSON normalizes backend aliases such as `cancelled` to canonical `canceled`."],
        ]}
      />

      <h2 id="json-output">JSON and exit codes</h2>
      <p>
        Every agent-relevant Phase 3 command supports a stable envelope. Machine fields such as <code>code</code>, <code>normalized_code</code>, <code>catalog_version</code>, and <code>status</code> stay stable English identifiers even when human messages are localized.
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
          ["Credentials", "Phase 3 uses `UNIPOST_API_KEY` or `--api-key`; local config stores defaults only, not secrets. OS keychain and browser/device auth are later."],
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
