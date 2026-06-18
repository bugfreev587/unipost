import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const INSTALL_SNIPPETS = [
  {
    label: "Claude Code",
    lang: "bash",
    code: `unipost agent install --client claude-code --json
# copy the printed debug skill into .claude/skills/unipost-debug/
unipost agent guide --client claude-code --json`,
  },
  {
    label: "Codex",
    lang: "bash",
    code: `unipost agent install --client codex --json
# copy the printed debug skill into .codex/skills/unipost-debug/
unipost agent guide --client codex --json`,
  },
];

const LOOP_SNIPPET = `unipost doctor diagnose --json          # find root causes
unipost doctor explain --request-id req_... --json
unipost logs list --status error --since 2h --json
# patch local code / config, then verify
unipost doctor verify --json
# only if still stuck:
unipost doctor support-bundle --json
# with user approval, upload the redacted bundle for UniPost admin review
unipost doctor support-bundle --upload --json`;

const MCP_SNIPPET = `# After MCP setup, agents can call these read-only tools:
unipost_debug_recent_logs({ "status": "error", "limit": 20 })
unipost_debug_explain_request({ "request_id": "req_..." })
unipost_debug_stream_info({ "status": "error", "after_id": "110000" })`;

const DIAGNOSE_RESPONSE = `{
  "ok": true,
  "data": {
    "schema_version": "doctor.v1",
    "command": "doctor.diagnose",
    "status": "failed",
    "local_project": {
      "frameworks": ["next"],
      "sdk": { "detected": true, "packages": [{ "name": "@unipost/sdk", "version": "0.4.0" }] },
      "code_hints": [
        {
          "kind": "auth_header_missing_bearer",
          "file": "src/unipost.ts",
          "line": 12,
          "message": "Authorization header appears to use a raw API key..."
        }
      ]
    },
    "findings": [
      {
        "id": "finding_local_auth_header_missing_bearer",
        "severity": "error",
        "category": "auth",
        "confidence": 0.88,
        "summary": "Local integration code appears to send the UniPost API key without the Bearer prefix.",
        "recommended_actions": [
          {
            "type": "code_patch",
            "safety": "safe_to_execute_without_user",
            "instruction": "Patch the request headers to send Authorization: Bearer <UNIPOST_API_KEY>.",
            "target_files": ["src/unipost.ts"]
          }
        ],
        "verify_command": "unipost doctor verify --json"
      }
    ]
  }
}`;

const STATUS_ROWS = [
  ["passed", "No blocking issues. The integration looks healthy."],
  ["failed", "Blocking issues with safe or assisted fixes. Patch and re-verify."],
  ["input_required", "A manual product step is needed (e.g. connect an account)."],
  ["needs_support", "Could not resolve safely. Generate a redacted support bundle."],
] as const;

const SAFETY_ROWS = [
  ["read_only", "The agent may run it freely."],
  ["safe_to_execute_without_user", "The agent may apply the code/config patch directly."],
  ["needs_user_approval", "Explain first, then ask before changing it (e.g. real `.env`)."],
  ["manual_only", "A real dashboard/product step the user must do themselves."],
] as const;

export default function CliAgentDebugPage() {
  return (
    <DocsPage
      className="docs-page-wide"
      eyebrow="Developer tools"
      title="AI-assisted debugging"
      lead="Let Claude Code, Codex, or another local agent diagnose and repair a broken UniPost integration with the unipost CLI — diagnose, explain, patch, verify, escalate."
    >
      <h2 id="how-it-works">How it works</h2>
      <p>
        The <code>unipost doctor</code> commands emit a stable, machine-readable
        <code> doctor.v1</code> contract under the normal CLI envelope. The agent reads
        the findings, applies the recommended actions according to their safety level,
        verifies the fix, and only escalates a redacted support bundle when it cannot
        resolve the problem. Code edits stay with the agent, which already understands the
        repository; the CLI provides trusted evidence and deterministic verification.
      </p>

      <h2 id="install">Install the debug skill</h2>
      <p>
        Install the first-party UniPost debug skill for your agent, then ask it in plain
        language: <em>&ldquo;Use UniPost debug to fix my integration.&rdquo;</em>
      </p>
      <DocsCodeTabs snippets={INSTALL_SNIPPETS} />

      <h2 id="loop">The debugging loop</h2>
      <DocsCodeTabs snippets={[{ label: "CLI", lang: "bash", code: LOOP_SNIPPET }]} />

      <h2 id="contract">Reading doctor findings</h2>
      <p>
        Every <code>unipost doctor *--json</code> command returns the doctor payload under
        <code> data</code>. Branch on <code>data.status</code> and iterate
        <code> data.findings</code>; never scrape the human-readable text.
      </p>
      <DocsCodeTabs snippets={[{ label: "doctor diagnose --json", lang: "json", code: DIAGNOSE_RESPONSE }]} />
      <DocsTable columns={["data.status", "Meaning"]} rows={STATUS_ROWS} />

      <h2 id="local-project">Local project hints</h2>
      <p>
        <code>doctor diagnose</code> also inspects the current project for safe repair
        hints: detected frameworks, installed UniPost SDK packages, environment variable
        names from example files, and relative file/line hints for common issues like a
        missing <code>Bearer</code> prefix, singular <code>account_id</code> post payloads,
        or local file paths passed as media URLs. It does not read real <code>.env</code>
        contents; those files are reported only as secret-bearing files.
      </p>

      <h2 id="safety">Action safety levels</h2>
      <p>
        Each recommended action carries a <code>safety</code> level that tells the agent
        how far it may go without you.
      </p>
      <DocsTable columns={["safety", "Agent rule"]} rows={SAFETY_ROWS} />

      <h2 id="secrets">Secrets and non-destructive verification</h2>
      <p>
        The CLI masks API keys (for example <code>up_live_9BWr...vhzHk</code>) and never
        prints OAuth tokens, cookies, or webhook secrets. <code>unipost doctor verify</code>
        runs only non-destructive checks (auth probe, workspace read, account/health read,
        logs read) and never live-publishes. Live publishing requires the explicit
        <code> --allow-live-publish</code> opt-in.
      </p>

      <h2 id="escalate">Escalating to support</h2>
      <p>
        When the agent cannot resolve the issue, <code>unipost doctor support-bundle --json</code>
        writes a redacted <code>unipost-debug-report.md</code> with request ids, log ids,
        findings, and environment metadata — no secrets and no source code. With explicit user
        approval, add <code>--upload</code> to store the redacted report in the super-admin
        support bundle viewer; the CLI still writes the local report first and returns the
        uploaded bundle id under <code>data.support.upload</code>.
      </p>

      <h2 id="mcp">MCP debug tools</h2>
      <p>
        The UniPost MCP server exposes read-only debug tools for agents that already have a
        workspace API key. Agents can list recent workspace logs, explain one request or log id,
        and obtain the authenticated SSE stream URL plus reconnect rules. The stream tool returns
        instructions instead of opening an indefinite MCP call.
      </p>
      <DocsCodeTabs snippets={[{ label: "MCP", lang: "js", code: MCP_SNIPPET }]} />
    </DocsPage>
  );
}
