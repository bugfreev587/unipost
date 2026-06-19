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

const LOOP_SNIPPET = `unipost auth status --json              # classify local auth first
unipost doctor diagnose --json          # find root causes
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

const DEBUG_FLOW = [
  {
    title: "Classify auth",
    label: "Local readiness",
    body: "Start with the local CLI binding so the agent knows whether it can query the workspace or needs a setup step first.",
    command: "unipost auth status --json",
  },
  {
    title: "Find evidence",
    label: "Doctor payload",
    body: "Use doctor.v1 findings and workspace logs as the source of truth instead of guessing from error text.",
    command: "unipost doctor diagnose --json",
  },
  {
    title: "Patch safely",
    label: "Recommended actions",
    body: "Apply only actions whose safety level allows it. Ask before touching secrets, dashboards, or live product state.",
    command: "finding.recommended_actions[]",
  },
  {
    title: "Verify or escalate",
    label: "Closed loop",
    body: "Re-run verification after every patch. Generate a redacted bundle only when the issue cannot be resolved locally.",
    command: "unipost doctor verify --json",
  },
] as const;

const SETUP_RULES = [
  ["1. Finish CLI auth", "Run `unipost init`, the Dashboard setup command, or `auth login --api-key` until `unipost auth status --json` reports a usable credential."],
  ["2. Add agent instructions", "Run `unipost agent install --client codex|claude-code --json`, then copy the returned debug skill files into the agent-specific skills directory."],
  ["3. Start with diagnosis", "Ask the agent to use UniPost debug. It should run auth status, diagnose, explain/logs commands, patch local code when safe, and verify."],
  ["4. Escalate with redaction", "If local repair is not enough, create a support bundle. Uploading still requires explicit user approval."],
] as const;

export default function CliAgentDebugPage() {
  return (
    <DocsPage
      className="docs-page-wide cli-agent-debug-page"
      eyebrow="Developer tools"
      title="AI-assisted debugging"
      lead="Let Claude Code, Codex, or another local agent diagnose and repair a broken UniPost integration with the unipost CLI — diagnose, explain, patch, verify, escalate."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <section className="debug-hero" aria-labelledby="debug-purpose">
        <div className="debug-hero-copy">
          <div className="debug-kicker">Agent repair loop</div>
          <h2 id="debug-purpose">Give the agent evidence, boundaries, and a verification command.</h2>
          <p>
            The debug kit turns UniPost errors into a closed loop for local agents:
            classify auth, inspect logs, patch safe code/config issues, then prove the
            fix with non-destructive verification.
          </p>
        </div>
        <div className="debug-rule-card">
          <div className="debug-rule-title">Default boundary</div>
          <p>
            Read freely. Patch local code when the finding says it is safe. Ask before
            changing secrets, uploading bundles, or doing anything in the dashboard.
          </p>
          <code>doctor diagnose -&gt; patch -&gt; doctor verify</code>
        </div>
      </section>

      <h2 id="how-it-works">How it works</h2>
      <p>
        The <code>unipost doctor</code> commands emit a stable, machine-readable
        <code> doctor.v1</code> contract under the normal CLI envelope. The agent reads
        the findings, applies the recommended actions according to their safety level,
        verifies the fix, and only escalates a redacted support bundle when it cannot
        resolve the problem. Code edits stay with the agent, which already understands the
        repository; the CLI provides trusted evidence and deterministic verification.
        Agents should run <code>unipost auth status --json</code> first. If auth is
        <code> missing</code>, run <code>unipost init</code> or the Dashboard setup
        command. If auth is <code> metadata_only</code>, rebind with
        <code> auth login --api-key</code> or set <code>UNIPOST_API_KEY</code>.
      </p>

      <section className="debug-flow" aria-label="AI-assisted debugging flow">
        {DEBUG_FLOW.map((item) => (
          <article className="debug-flow-card" key={item.title}>
            <div className="debug-kicker">{item.label}</div>
            <h3>{item.title}</h3>
            <p>{item.body}</p>
            <code>{item.command}</code>
          </article>
        ))}
      </section>

      <h2 id="install">Install the debug skill</h2>
      <p>
        Install the first-party UniPost debug skill for your agent, then ask it in plain
        language: <em>&ldquo;Use UniPost debug to fix my integration.&rdquo;</em>
      </p>
      <DocsTable columns={["Step", "What to do"]} rows={SETUP_RULES} />
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

const styles = `
.debug-hero{display:grid;grid-template-columns:minmax(0,1fr) minmax(260px,.44fr);gap:22px;align-items:stretch;margin:8px 0 30px;padding:18px 0 26px;border-bottom:1px solid var(--docs-border)}
.debug-hero-copy h2{margin:8px 0 0;color:var(--docs-text);font-size:30px;line-height:1.15;font-weight:760;letter-spacing:0;max-width:760px}
.debug-hero-copy p{margin:12px 0 0;color:var(--docs-text-soft);font-size:15px;line-height:1.7;max-width:68ch}
.debug-kicker{font-family:var(--docs-mono);font-size:11px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint)}
.debug-rule-card{border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated);padding:16px 18px;align-self:start}
.debug-rule-title{font-size:14px;font-weight:760;color:var(--docs-text)}
.debug-rule-card p{margin:8px 0 0;color:var(--docs-text-soft);font-size:14px;line-height:1.6}
.debug-rule-card code{display:inline-flex;margin-top:12px;max-width:100%;overflow-wrap:anywhere;color:var(--docs-text);font-family:var(--docs-mono);font-size:12px}
.debug-flow{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:10px;margin:22px 0 8px}
.debug-flow-card{min-width:0;border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated);padding:14px 16px}
.debug-flow-card h3{margin:6px 0 0;color:var(--docs-text);font-size:17px;line-height:1.3;font-weight:720;letter-spacing:0}
.debug-flow-card p{margin:8px 0 0;color:var(--docs-text-soft);font-size:14px;line-height:1.6}
.debug-flow-card code{display:block;margin-top:10px;color:var(--docs-text);font-family:var(--docs-mono);font-size:12px;line-height:1.55;overflow-wrap:anywhere}
@media (max-width:1040px){.debug-flow{grid-template-columns:repeat(2,minmax(0,1fr))}}
@media (max-width:920px){.debug-hero{grid-template-columns:1fr}.debug-hero-copy h2{font-size:25px}}
@media (max-width:640px){.debug-flow{grid-template-columns:1fr}}
`;
