import type { Metadata } from "next";
import Link from "next/link";
import { DocsPage } from "./_components/docs-shell";

export const metadata: Metadata = {
  alternates: { canonical: "https://unipost.dev/docs" },
};

export default function DocsHomePage() {
  return (
    <DocsPage
      eyebrow="Overview"
      title="Choose the fastest path into UniPost"
      lead="UniPost supports dashboard publishing, programmatic publishing, and hosted account connection flows. Pick the path that matches how you want to work first."
    >
      <div className="docs-grid">
        <Link href="/docs/dashboard-quickstart" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Dashboard Quickstart</div>
          <p>Connect accounts, open the compose flow, and publish your first post from the UniPost UI.</p>
        </Link>
        <Link href="/docs/quickstart" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Quickstart Mode</div>
          <p>Create an API key, connect accounts, and publish programmatically with the REST API or SDKs.</p>
        </Link>
        <Link href="/docs/connect-sessions" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Connect Sessions</div>
          <p>Let end users connect customer-owned accounts with hosted OAuth, using UniPost&apos;s shared app or workspace Platform Credentials.</p>
        </Link>
        <Link href="/docs/local-connect-test" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Local Connect testing</div>
          <p>Download the helper script, create a Connect Session from your terminal, and open the returned OAuth URL in a browser.</p>
        </Link>
        <Link href="/docs/guides/analytics" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Analytics Guides</div>
          <p>Answer task questions such as which UniPost API returns TikTok followers, post analytics, and export rows.</p>
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
        <li>Publish your first post with the REST API or an SDK.</li>
      </ul>
      <p>
        Start here: <Link href="/docs/quickstart">Quickstart Mode</Link>, then use the{" "}
        <Link href="/docs/publishing">Publishing guide</Link> for media upload and publish status flow.
      </p>

      <p>
        <strong>Customer-owned accounts:</strong> if you are embedding UniPost into your own product, start with{" "}
        <Link href="/docs/connect-sessions">Connect Sessions</Link>. Use{" "}
        <Link href="/docs/local-connect-test">Local Connect testing</Link> to prove the flow from your terminal, use <Link href="/docs/white-label">Hosted Connect</Link>{" "}
        when you need white-label branding, and use <Link href="/docs/platform-credentials">Platform Credentials</Link> when you need your own OAuth app and platform quota.
      </p>
    </DocsPage>
  );
}
