import Link from "next/link";
import { DocsPage, DocsTable } from "./_components/docs-shell";

export default function DocsHomePage() {
  return (
    <DocsPage
      eyebrow="Documentation"
      title="Start with the task you need to ship."
      lead="UniPost docs are organized around integration jobs, not product marketing. Pick the workflow you are trying to complete, follow the shortest path, and then drop into the API reference when you need exact request details."
    >
      <section id="start-here" className="docs-home-section">
        <div className="docs-kicker">Start by task</div>
        <div className="docs-task-list">
          <Link href="/docs/quickstart" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Publish your first post</div>
              <div className="docs-task-body">Create an API key, connect an account, fetch its account ID, publish with `platform_posts[]`, and validate before you automate anything.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">Quickstart</span>
            </div>
          </Link>

          <Link href="/docs/api/connect/sessions" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Let customers connect their own accounts</div>
              <div className="docs-task-body">Use hosted Connect sessions when you are onboarding end-user or customer-owned social accounts instead of team-owned workspace accounts.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">Connect Sessions</span>
            </div>
          </Link>

          <Link href="/docs/api/posts/validate" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Validate content before publish</div>
              <div className="docs-task-body">Run the same request body through preflight validation to catch caption limits, media conflicts, and per-platform issues before anything is posted.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">Validate API</span>
            </div>
          </Link>

          <Link href="/docs/api/webhooks" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Receive delivery and account events</div>
              <div className="docs-task-body">Subscribe to webhook events when you need asynchronous status updates for publishes, account state changes, and downstream automation.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">Webhooks</span>
            </div>
          </Link>

          <Link href="/docs/mcp" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Use UniPost from agents or MCP clients</div>
              <div className="docs-task-body">Connect the hosted MCP server when the integration surface is an AI agent, a tool-using assistant, or an internal automation environment.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">MCP Guide</span>
            </div>
          </Link>
        </div>
      </section>

      <section id="common-workflows" className="docs-home-section">
        <div className="docs-kicker">Common workflows</div>
        <div className="docs-mini-grid">
          <div className="docs-mini-card">
            <div className="docs-mini-title">Team-owned accounts</div>
            <p>Use direct account connection flows when your own workspace is managing publishing.</p>
            <p><Link href="/docs/quickstart">Open Quickstart</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Customer-owned accounts</div>
            <p>Use Connect sessions when your SaaS needs each customer or user to authorize their own accounts.</p>
            <p><Link href="/docs/api/connect/sessions">Read Connect Sessions</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Per-platform publishing rules</div>
            <p>Check platform guides when the question is about caption limits, media support, or thread behavior on one network.</p>
            <p><Link href="/docs/platforms">Browse Platforms</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Operational feedback loops</div>
            <p>Use analytics, notifications, and webhooks when you need monitoring, retries, or downstream decisioning.</p>
            <p><Link href="/docs/api">Open API Reference</Link></p>
          </div>
        </div>
      </section>

      <h2 id="why-unipost">What UniPost Covers</h2>
      <DocsTable
        columns={["Layer", "What UniPost does", "Why it matters"]}
        rows={[
          ["Accounts", "Connect workspace-owned or end-user-owned social accounts", "Build for your team, your customers, or both"],
          ["Publishing", "Publish one request across platforms with platform-specific captions", "One integration, without flattening each platform's quirks"],
          ["Safety", "Validate drafts before publish and generate preview links", "Catch platform-specific failures before they go live"],
          ["Operations", "Track analytics, account health, usage, and webhooks", "Run social publishing as infrastructure, not just as a button"],
        ]}
      />

      <h2 id="reference-map">Reference Map</h2>
      <ul className="docs-list">
        <li>Start in <Link href="/docs/quickstart">Quickstart</Link> if you want the shortest path from API key to first successful publish.</li>
        <li>Open <Link href="/docs/platforms">Platforms</Link> when you need per-network constraints, media rules, or content examples.</li>
        <li>Use <Link href="/docs/api">API Reference</Link> when you already know the capability you want and need exact request and response details.</li>
        <li>Read <Link href="/docs/sdk">SDK</Link> and <Link href="/docs/mcp">MCP</Link> when UniPost is being consumed by agents, scripts, or internal developer tooling.</li>
      </ul>
    </DocsPage>
  );
}
