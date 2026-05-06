import Link from "next/link";
import { DocsPage } from "./_components/docs-shell";

export default function DocsHomePage() {
  return (
    <DocsPage
      eyebrow="Overview"
      title="Choose the fastest path into UniPost"
      lead="UniPost supports two common starting points: publishing from the dashboard UI, or publishing programmatically through the API. Pick the path that matches how you want to work first."
    >
      <div className="docs-grid">
        <Link href="/docs/dashboard-quickstart" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Dashboard Quickstart</div>
          <p>Connect accounts, open the compose flow, and publish your first post from the UniPost UI.</p>
        </Link>
        <Link href="/docs/quickstart" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">API Quickstart</div>
          <p>Create an API key, connect accounts, and publish programmatically with SDKs, CLI, or MCP.</p>
        </Link>
      </div>

      <h2 id="dashboard-path">Using the Dashboard</h2>
      <ul className="docs-step-list">
        <li>Connect your first social account from the UniPost dashboard.</li>
        <li>Open the create-post flow and write your first caption.</li>
        <li>Customize per platform, add media, then publish now or schedule later.</li>
      </ul>
      <p>
        Start here: <Link href="/docs/dashboard-quickstart">Dashboard Quickstart</Link>
      </p>

      <h2 id="api-path">Using the API</h2>
      <ul className="docs-step-list">
        <li>Create an API key for your workspace.</li>
        <li>Connect at least one destination account.</li>
        <li>Publish your first post with the REST API, an SDK, CLI, or MCP.</li>
      </ul>
      <p>
        Start here: <Link href="/docs/quickstart">API Quickstart</Link>
      </p>

      <div className="docs-callout">
        <strong>Advanced setup:</strong> if you are embedding UniPost into your own product, see <Link href="/docs/white-label">White-label</Link>.
      </div>
    </DocsPage>
  );
}
