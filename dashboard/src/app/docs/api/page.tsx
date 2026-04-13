import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";

export default function ApiReferenceLandingPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="API reference organized by workflow."
      lead="Instead of making you scan one giant long-form page, the API reference is split by what you are trying to do: connect accounts, publish posts, validate drafts, inspect analytics, or receive events."
    >
      <h2 id="authentication">Authentication</h2>
      <p>Every API request uses Bearer auth with your UniPost API key. Dashboard-authenticated routes exist separately for the product UI, but the public API reference focuses on API-key integrations.</p>

      <h2 id="endpoint-groups">Endpoint groups</h2>
      <DocsTable
        columns={["Group", "What it covers", "Start here"]}
        rows={[
          ["Accounts", "List, connect, disconnect, and inspect social accounts", "Social Accounts"],
          ["Connect sessions", "Hosted onboarding for end-user-owned accounts", "Connect Sessions"],
          ["Posts", "Create, validate, schedule, draft, and preview content", "Create Post"],
          ["Analytics", "Inspect performance and workspace-level summaries", "Analytics"],
          ["Webhooks", "Subscribe to publish and account events", "Webhooks"],
        ]}
      />

      <h2 id="connect-sessions">Connect sessions</h2>
      <p>Use Connect sessions when your customers need to connect their own social accounts inside your product. This is where UniPost stops being just a posting API and becomes account-onboarding infrastructure.</p>

      <h2 id="managed-users">Managed users</h2>
      <p>Managed Users groups connected accounts by your own `external_user_id` so you can model end users cleanly across platforms.</p>

      <h2 id="validate">Validate</h2>
      <p>Validate is the recommended preflight endpoint for automation and AI workflows. It catches content issues before anything is written or published.</p>

      <h2 id="drafts">Drafts</h2>
      <p>Drafts and preview links give you a review surface before content is published. This is especially useful when an LLM or automation is generating the first draft.</p>

      <h2 id="preview-links">Preview links</h2>
      <p>Preview links are designed for review and collaboration flows. They let a human inspect the platform-specific output before the final publish step.</p>

      <h2 id="media">Media</h2>
      <p>Use the media library when you want UniPost to manage upload references and resolve media IDs server-side instead of depending on public URLs.</p>

      <h2 id="analytics">Analytics</h2>
      <p>Analytics includes both per-post performance and workspace rollups, so you can power dashboards, summaries, and agent workflows from one layer.</p>

      <h2 id="account-health">Account health</h2>
      <p>Account health gives you a fast signal when a connected account is healthy, degraded, or disconnected.</p>

      <h2 id="billing">Billing</h2>
      <p>Billing and usage endpoints let you map product behavior back to plan limits and account usage.</p>

      <h2 id="errors">Errors</h2>
      <p>UniPost uses structured error responses so clients can distinguish validation issues, authorization problems, conflicts, and transient server failures.</p>

      <div className="docs-grid">
        <div className="docs-card">
          <div className="docs-card-title">Detailed endpoint pages</div>
          <p>Existing detailed endpoint pages already live under the new shell. Start with the core publish path.</p>
          <p><Link href="/docs/api/posts/create">Open Create Post</Link></p>
        </div>
        <div className="docs-card">
          <div className="docs-card-title">Accounts reference</div>
          <p>Use the social accounts reference when you need account IDs, filtering, or managed account lookups.</p>
          <p><Link href="/docs/api/accounts/list">Open Social Accounts</Link></p>
        </div>
        <div className="docs-card">
          <div className="docs-card-title">Events reference</div>
          <p>Webhooks are where most production integrations become observable and debuggable.</p>
          <p><Link href="/docs/api/webhooks">Open Webhooks</Link></p>
        </div>
      </div>
    </DocsPage>
  );
}
