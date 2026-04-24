import Link from "next/link";
import { DocsPage, DocsTable } from "./_components/docs-shell";

export default function DocsHomePage() {
  return (
    <DocsPage
      eyebrow="Documentation"
      title="Start with the task you need to ship."
      lead="UniPost docs are organized around integration jobs, not product marketing. Pick the workflow you are trying to complete, follow the shortest path, and then drop into the API reference when you need exact request details for authentication, accounts, publishing, analytics, operations, and billing."
    >
      <section id="start-here" className="docs-home-section">
        <div className="docs-kicker">Start by task</div>
        <div className="docs-task-list">
          <Link href="/docs/api/api-keys/list" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Authenticate your integration</div>
              <div className="docs-task-body">Start with Bearer auth, understand live vs test API keys, and see the common auth failures before wiring any client code.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">API Keys</span>
            </div>
          </Link>

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

          <Link href="/docs/api/analytics" className="docs-task-item">
            <div className="docs-task-copy">
              <div className="docs-task-title">Measure performance and delivery health</div>
              <div className="docs-task-body">Use analytics, notifications, and account health endpoints when you need reporting, alerting, retries, or human review loops around publishing.</div>
            </div>
            <div className="docs-task-links">
              <span className="docs-task-link">Analytics + Operations</span>
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
            <div className="docs-mini-title">API-key setup</div>
            <p>Start with authentication when you need to wire a server, script, SDK client, or internal tool against UniPost.</p>
            <p><Link href="/docs/api/api-keys/list">Open API Keys</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Team-owned accounts</div>
            <p>Use direct account connection flows when your own workspace is managing publishing.</p>
            <p><Link href="/docs/quickstart">Open Quickstart</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Customer-owned accounts</div>
            <p>Use Connect sessions and managed users when your SaaS needs each customer or end user to authorize their own accounts.</p>
            <p><Link href="/docs/api/connect/sessions">Read Connect Sessions</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Media-first publishing</div>
            <p>Use the media API when your workflow starts from local files, uploaded assets, or reusable media IDs instead of public URLs.</p>
            <p><Link href="/docs/api/media">Open Media API</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Per-platform publishing rules</div>
            <p>Check platform guides when the question is about caption limits, media support, or thread behavior on one network.</p>
            <p><Link href="/docs/platforms">Browse Platforms</Link></p>
          </div>
          <div className="docs-mini-card">
            <div className="docs-mini-title">Operational feedback loops</div>
            <p>Use analytics, notifications, account health, and webhooks when you need monitoring, retries, or downstream decisioning.</p>
            <p><Link href="/docs/api/analytics">Open Analytics</Link></p>
          </div>
        </div>
      </section>

      <h2 id="why-unipost">What UniPost Covers</h2>
      <DocsTable
        columns={["Layer", "What UniPost does", "Why it matters"]}
        rows={[
          ["API Keys", "Manage workspace keys and authenticate every server-side request", "Ship integrations with clear live vs test environments and predictable auth boundaries"],
          ["Accounts", "Connect workspace-owned or end-user-owned social accounts and group them under managed users", "Build for your team, your customers, or both without losing account ownership context"],
          ["Publishing", "Publish one request across platforms with platform-specific captions, drafts, validation, and media uploads", "One integration, without flattening each platform's quirks or skipping review steps"],
          ["Operations", "Track analytics, account health, notifications, webhooks, errors, and billing usage", "Run social publishing as infrastructure instead of treating it like a single fire-and-forget API call"],
          ["Inbox", "Reserve a future surface for unified conversations and moderation APIs", "Plan for reply and support workflows without assuming the public Inbox API is already available today"],
        ]}
      />

      <h2 id="reference-map">Reference Map</h2>
      <ul className="docs-list">
        <li>Start in <Link href="/docs/quickstart">Quickstart</Link> if you want the shortest path from API key to first successful publish.</li>
        <li>Open <Link href="/docs/api/api-keys/list">API Keys</Link> first when you are wiring a new backend, SDK client, or environment setup.</li>
        <li>Read <Link href="/docs/api/connect/sessions">Connect Sessions</Link> and <Link href="/docs/api/users">Managed Users</Link> when your product needs customer-owned account onboarding.</li>
        <li>Use <Link href="/docs/api/media">Media</Link>, <Link href="/docs/api/posts/validate">Validate</Link>, and <Link href="/docs/api/posts/drafts">Drafts</Link> for safer media-heavy or human-reviewed publish flows.</li>
        <li>Open <Link href="/docs/api/analytics">Analytics</Link>, <Link href="/docs/api/notifications">Notifications</Link>, and <Link href="/docs/api/billing">Billing</Link> when you are building operational tooling around the core publish path.</li>
        <li>Open <Link href="/docs/platforms">Platforms</Link> when you need per-network constraints, media rules, or content examples.</li>
        <li>Use <Link href="/docs/api">API Reference</Link> when you already know the capability you want and need exact request and response details.</li>
        <li>Read <Link href="/docs/sdk">SDK</Link> and <Link href="/docs/mcp">MCP</Link> when UniPost is being consumed by agents, scripts, or internal developer tooling.</li>
      </ul>
    </DocsPage>
  );
}
