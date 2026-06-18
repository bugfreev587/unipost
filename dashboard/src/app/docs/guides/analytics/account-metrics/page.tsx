import Link from "next/link";
import { CodeBlock } from "../../../_components/code-block";
import { DocsPage } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

export default function AccountMetricsGuidePage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Get account metrics across platforms"
      lead="Use one UniPost account metrics endpoint for follower, following, and post counts across supported connected accounts."
      className="docs-page-wide"
    >
      <h2 id="when-to-use">When to use this guide</h2>
      <p>
        Use <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" /> when your product needs account-level metrics such as
        followers, following, lifetime post count, or platform-specific account stats. This is the normalized UniPost path for X,
        Instagram, Threads, and TikTok when the connected account has the required platform scopes.
      </p>

      <h2 id="steps">Steps</h2>
      <ol className="docs-step-list">
        <li>List connected accounts with <ApiInlineLink endpoint="GET /v1/accounts" />.</li>
        <li>Pick the account id for the platform you want to display.</li>
        <li>Call <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />.</li>
        <li>Read the normalized fields first, then inspect <code>platform_specific</code> for platform-native additions.</li>
      </ol>

      <h2 id="request">Request</h2>
      <CodeBlock
        language="bash"
        title="cURL"
        code={`curl "https://api.unipost.dev/v1/accounts/sa_instagram_123/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`}
      />

      <h2 id="fields">Fields to read</h2>
      <div className="docs-table-wrap">
        <table className="docs-table">
          <thead>
            <tr>
              <th>Field</th>
              <th>Meaning</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td><code>data.follower_count</code></td>
              <td>Followers reported by the platform.</td>
            </tr>
            <tr>
              <td><code>data.following_count</code></td>
              <td>Accounts this account follows, when the platform exposes it.</td>
            </tr>
            <tr>
              <td><code>data.post_count</code></td>
              <td>Lifetime post, video, or media count exposed by the platform.</td>
            </tr>
            <tr>
              <td><code>data.platform_specific</code></td>
              <td>Provider-native additions or upstream failure details that do not fit the normalized shape.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2 id="platform-notes">Platform notes</h2>
      <ul className="docs-step-list">
        <li>TikTok followers require <code>user.info.stats</code>; likes count appears in <code>platform_specific.likes_count</code>.</li>
        <li>Instagram and Threads require the approved account insight scopes documented in platform capabilities.</li>
        <li>X account metrics depend on X API availability and rate limits for the connected account.</li>
        <li>Unsupported platforms return <code>NOT_SUPPORTED</code> instead of an empty success response.</li>
      </ul>

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/accounts/metrics" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Get account metrics</div>
          <div className="docs-next-body">Endpoint contract and response fields.</div>
        </Link>
        <Link href="/docs/api/analytics/platforms" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Platform capabilities</div>
          <div className="docs-next-body">Supported Analytics capabilities by platform.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
