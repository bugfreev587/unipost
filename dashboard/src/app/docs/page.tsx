import Link from "next/link";
import { DocsPage, DocsTable } from "./_components/docs-shell";

export default function DocsHomePage() {
  return (
    <DocsPage
      eyebrow="Documentation"
      title="Docs that get you from API key to production fast."
      lead="UniPost helps you onboard user accounts, validate content before publish, ship cross-platform posts, and track results. This new docs structure is organized around the way developers actually build: start fast, go platform by platform, then drop into the API reference."
    >
      <h2 id="start-here">Start Here</h2>
      <div className="docs-grid">
        <div className="docs-card">
          <div className="docs-card-title">Quickstart</div>
          <p>Connect an account, publish your first post, and learn the recommended request shape in under ten minutes.</p>
          <p><Link href="/docs/quickstart">Open Quickstart</Link></p>
        </div>
        <div className="docs-card">
          <div className="docs-card-title">Platform Guides</div>
          <p>See what each platform supports, what content rules apply, and how to publish text, image, video, or threaded posts.</p>
          <p><Link href="/docs/platforms">Browse Platforms</Link></p>
        </div>
        <div className="docs-card">
          <div className="docs-card-title">API Reference</div>
          <p>Read endpoint-specific request and response details for accounts, connect sessions, posts, analytics, webhooks, and more.</p>
          <p><Link href="/docs/api">Open API Reference</Link></p>
        </div>
        <div className="docs-card">
          <div className="docs-card-title">MCP</div>
          <p>Use UniPost from AI agents with a hosted MCP server, then layer preview, validate, and analytics into your workflows.</p>
          <p><Link href="/docs/mcp">Read MCP Guide</Link></p>
        </div>
      </div>

      <h2 id="why-unipost">Why UniPost</h2>
      <DocsTable
        columns={["Layer", "What UniPost does", "Why it matters"]}
        rows={[
          ["Accounts", "Connect workspace-owned or end-user-owned social accounts", "Build for your team, your customers, or both"],
          ["Publishing", "Publish one request across platforms with platform-specific captions", "One integration, without flattening each platform's quirks"],
          ["Safety", "Validate drafts before publish and generate preview links", "Catch platform-specific failures before they go live"],
          ["Operations", "Track analytics, account health, usage, and webhooks", "Run social publishing as infrastructure, not just as a button"],
        ]}
      />

      <h2 id="how-to-use">How To Use These Docs</h2>
      <ul className="docs-list">
        <li>Start in <Link href="/docs/quickstart">Quickstart</Link> if you are integrating UniPost for the first time.</li>
        <li>Open <Link href="/docs/platforms">Platforms</Link> when you need exact content rules and examples for a specific network.</li>
        <li>Use <Link href="/docs/api">API Reference</Link> when you already know what you want to do and need endpoint details.</li>
      </ul>
    </DocsPage>
  );
}
