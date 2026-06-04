import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const AGENT_WORKFLOWS = [
  {
    title: "Read workspace context",
    label: "Profiles, accounts, and recent posts",
    intent: "Give the agent enough real UniPost context before it explains, plans, or writes code.",
    prompt: `请使用 UniPost CLI 查询我的 profiles 和 connected accounts。
先运行 agent bootstrap，然后列出 profiles、accounts 和最近的 posts。
只做读取和总结，不要创建、修改或发布任何内容。`,
    commands: `unipost agent bootstrap --client claude-code --json
unipost profiles list --json
unipost accounts list --json
unipost posts list --limit 10 --json`,
    summary: "The agent should summarize profile IDs, account IDs, platforms, connection status, and recent post status in normal language.",
  },
  {
    title: "Inspect account readiness",
    label: "Health, capabilities, and metrics",
    intent: "Ask the agent which connected account is ready for a specific social workflow.",
    prompt: `请检查我的 connected accounts 是否可以正常发帖。
对每个 account 查询 health、capabilities 和 metrics。
告诉我哪个账号适合发布，哪里有风险。
只读，不要创建 post。`,
    commands: `unipost accounts list --json
unipost accounts health --account sa_... --json
unipost accounts capabilities --account sa_... --json
unipost accounts metrics --account sa_... --json`,
    summary: "The agent should call out disconnected accounts, missing platform permissions, unsupported media types, and useful account-level metrics.",
  },
  {
    title: "Prepare posts safely",
    label: "Validate, dry-run, and draft",
    intent: "Let the agent turn a plain-language idea into a safe draft without publishing externally.",
    prompt: `请帮我为 LinkedIn 起草一条 post。
先找到合适的 account，然后 validate 和 draft。
不要 live publish，不要 schedule。需要发布时先把计划给我确认。`,
    commands: `unipost accounts list --json
unipost posts validate --account sa_... --caption "..." --json
unipost posts draft --account sa_... --caption "..." --json`,
    summary: "The agent should return the draft ID, validation result, suggested caption changes, and any missing inputs.",
  },
  {
    title: "Publish only after approval",
    label: "Explicit write confirmation",
    intent: "Keep live writes deliberate when the user has already reviewed the plan.",
    prompt: `我确认发布这个 draft。
请先复述将要发布的 account、caption 和目标平台。
确认无误后再执行 publish-draft。
使用 --yes 和稳定的 --idempotency-key。`,
    commands: `unipost posts get post_... --json
unipost posts publish-draft post_... \\
  --yes \\
  --idempotency-key user-approved-post-... \\
  --json
unipost posts wait post_... --timeout 120 --json`,
    summary: "The agent should restate the target before publishing, then report final status and any platform result IDs.",
  },
] as const;

const SETUP_SNIPPETS = [
  {
    label: "Claude Code",
    lang: "bash",
    code: `unipost agent install --client claude-code --json
unipost agent bootstrap --client claude-code --json
unipost agent capabilities --client claude-code --json
unipost agent context --json`,
  },
  {
    label: "Codex",
    lang: "bash",
    code: `unipost agent install --client codex --json
unipost agent bootstrap --client codex --json
unipost agent capabilities --client codex --json
unipost agent context --json`,
  },
];

const SAFETY_ROWS = [
  ["Read profiles, accounts, posts, analytics", "Safe. Ask in plain language and tell the agent to summarize."],
  ["Validate, dry-run, draft", "Safe preparation. These actions should not publish externally."],
  ["Create live post, schedule, publish draft", "Requires explicit user approval, `--yes`, and `--idempotency-key`."],
  ["Cancel or retry", "Requires explicit user approval, target IDs, and `--yes`."],
  ["Default instruction", "Start with read-only; do not publish, schedule, cancel, retry, or mutate data unless the user explicitly confirms."],
] as const;

const PROMPT_TEMPLATE = `请使用 UniPost CLI 完成这个任务：<你的目标>。

开始前先运行 agent bootstrap / capabilities / context。
优先使用 --json，并把结果用白话总结给我。
默认只读；不要 publish、schedule、cancel、retry 或修改数据。
如果任务需要写操作，先给我计划和将要运行的命令，等我确认。`;

export default function CliAgentGuidePage() {
  return (
    <DocsPage
      className="docs-page-wide cli-agent-guide-page"
      eyebrow="Developer tools"
      title="AI Agent Guide"
      lead="Use plain-language prompts to let Codex, Claude Code, and other local agents operate UniPost through the CLI for safe read and write workflows."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <section className="agent-hero" aria-labelledby="agent-guide-purpose">
        <div className="agent-hero-copy">
          <div className="agent-kicker">Plain-language agent operation</div>
          <h2 id="agent-guide-purpose">Tell the agent what you want. Let the CLI do the precise work.</h2>
          <p>
            After agent setup, you do not need to memorize every UniPost command. Ask Claude Code or Codex in normal language, tell it to use UniPost CLI, and set the boundary: read-only by default, safe drafts before live publishing.
          </p>
        </div>
        <div className="agent-rule-card">
          <div className="agent-rule-title">Default rule</div>
          <p>
            Start with <strong>read and summarize</strong>. For writes, ask for the plan first. Live publish, schedule, cancel, and retry need explicit approval.
          </p>
          <code>--yes + --idempotency-key</code>
        </div>
      </section>

      <h2 id="setup">Set up the agent context</h2>
      <p>
        Finish CLI auth first from <code>CLI Overview</code>, then install the client-specific agent instructions. The bootstrap commands ground the agent in the active workspace, profiles, accounts, and safe next actions.
      </p>
      <DocsCodeTabs snippets={SETUP_SNIPPETS} />

      <h2 id="prompt-pattern">A reusable prompt pattern</h2>
      <DocsCodeTabs snippets={[{ label: "Prompt", lang: "text", code: PROMPT_TEMPLATE }]} />

      <div className="agent-workflows">
        {AGENT_WORKFLOWS.map((workflow) => (
          <section className="agent-workflow" key={workflow.title} aria-labelledby={slugify(workflow.title)}>
            <div className="agent-workflow-copy">
              <div className="agent-kicker">{workflow.label}</div>
              <h2 id={slugify(workflow.title)}>{workflow.title}</h2>
              <p>{workflow.intent}</p>
            </div>
            <div className="agent-workflow-steps">
              <div className="agent-block">
                <div className="agent-block-label">Tell the agent</div>
                <DocsCodeTabs snippets={[{ label: "Prompt", lang: "text", code: workflow.prompt }]} />
              </div>
              <div className="agent-block">
                <div className="agent-block-label">Expected CLI</div>
                <DocsCodeTabs snippets={[{ label: "CLI", lang: "bash", code: workflow.commands }]} />
              </div>
              <p className="agent-summary">{workflow.summary}</p>
            </div>
          </section>
        ))}
      </div>

      <h2 id="safety-boundary">Safety boundary</h2>
      <DocsTable columns={["Operation", "Agent rule"]} rows={SAFETY_ROWS} />
    </DocsPage>
  );
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

const styles = `
.agent-hero{display:grid;grid-template-columns:minmax(0,1fr) minmax(260px,.46fr);gap:22px;align-items:stretch;margin:8px 0 30px;padding:18px 0 26px;border-bottom:1px solid var(--docs-border)}
.agent-hero-copy h2{margin:8px 0 0;color:var(--docs-text);font-size:30px;line-height:1.15;font-weight:760;letter-spacing:0;max-width:760px}
.agent-hero-copy p{margin:12px 0 0;color:var(--docs-text-soft);font-size:15px;line-height:1.7;max-width:68ch}
.agent-kicker{font-family:var(--docs-mono);font-size:11px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint)}
.agent-rule-card{border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated);padding:16px 18px;align-self:start}
.agent-rule-title{font-size:14px;font-weight:760;color:var(--docs-text)}
.agent-rule-card p{margin:8px 0 0;color:var(--docs-text-soft);font-size:14px;line-height:1.6}
.agent-rule-card code{display:inline-flex;margin-top:12px;max-width:100%;overflow-wrap:anywhere;color:var(--docs-text);font-family:var(--docs-mono);font-size:12px}
.agent-workflows{display:grid;gap:28px;margin-top:12px}
.agent-workflow{display:grid;grid-template-columns:minmax(220px,.34fr) minmax(0,1fr);gap:22px;align-items:start;padding-top:24px;border-top:1px solid var(--docs-border)}
.agent-workflow-copy h2{margin:6px 0 0;color:var(--docs-text);font-size:24px;line-height:1.22;font-weight:760;letter-spacing:0}
.agent-workflow-copy p{margin:10px 0 0;color:var(--docs-text-soft);font-size:14px;line-height:1.65}
.agent-workflow-steps{display:grid;gap:12px;min-width:0}
.agent-block{min-width:0}
.agent-block-label{margin:0 0 7px;font-family:var(--docs-mono);font-size:11px;font-weight:800;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint)}
.agent-summary{margin:0;color:var(--docs-text-soft);font-size:14px;line-height:1.65}
@media (max-width:920px){.agent-hero,.agent-workflow{grid-template-columns:1fr}.agent-hero-copy h2{font-size:25px}}
`;
