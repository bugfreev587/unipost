import Link from "next/link";
import { DocsPage } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

export default function AnalyticsGuidesPage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Choose the right Analytics API for the job"
      lead="UniPost Analytics is unified first: start with the shared account, post, and export APIs, then use platform-specific docs only when you need a native drilldown field."
      className="docs-page-wide"
    >
      <div className="docs-grid">
        <Link href="/docs/guides/analytics/tiktok-followers" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Get TikTok followers</div>
          <p>Use account metrics, confirm the TikTok `user.info.stats` scope, and read `data.follower_count`.</p>
        </Link>
        <Link href="/docs/guides/analytics/account-metrics" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Get account metrics</div>
          <p>Fetch follower, following, post, and platform-specific account stats across supported platforms.</p>
        </Link>
        <Link href="/docs/guides/analytics/post-analytics" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Get post analytics</div>
          <p>Read normalized analytics for UniPost-published posts, including likes, comments, shares, reach, and impressions.</p>
        </Link>
        <Link href="/docs/guides/analytics/export-post-analytics" className="docs-card" style={{ textDecoration: "none" }}>
          <div className="docs-card-title">Export analytics rows</div>
          <p>Download normalized post analytics as CSV for BI, reporting, or scheduled exports.</p>
        </Link>
      </div>

      <h2 id="unified-first">Unified-first Analytics</h2>
      <p>
        Most integrations should start with the shared UniPost surfaces: <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />{" "}
        for account-level counts, <ApiInlineLink endpoint="GET /v1/posts/{post_id}/analytics" /> for one UniPost-published post, and{" "}
        <ApiInlineLink endpoint="GET /v1/analytics/posts/export" /> for CSV exports.
      </p>
      <p>
        Platform-specific Analytics docs still matter for scopes, native fields, and drilldowns. They should not be the first path
        for a multi-platform integration when a normalized UniPost endpoint already answers the question.
      </p>

      <h2 id="common-tasks">Common tasks</h2>
      <div className="docs-table-wrap">
        <table className="docs-table">
          <thead>
            <tr>
              <th>Task</th>
              <th>UniPost API</th>
              <th>Start here</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>TikTok followers</td>
              <td><ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" /></td>
              <td><Link href="/docs/guides/analytics/tiktok-followers">Get TikTok followers</Link></td>
            </tr>
            <tr>
              <td>Account metrics across platforms</td>
              <td><ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" /></td>
              <td><Link href="/docs/guides/analytics/account-metrics">Get account metrics</Link></td>
            </tr>
            <tr>
              <td>One published post</td>
              <td><ApiInlineLink endpoint="GET /v1/posts/{post_id}/analytics" /></td>
              <td><Link href="/docs/guides/analytics/post-analytics">Get post analytics</Link></td>
            </tr>
            <tr>
              <td>CSV export</td>
              <td><ApiInlineLink endpoint="GET /v1/analytics/posts/export" /></td>
              <td><Link href="/docs/guides/analytics/export-post-analytics">Export analytics rows</Link></td>
            </tr>
            <tr>
              <td>Missing scopes</td>
              <td><ApiInlineLink endpoint="GET /v1/accounts/{account_id}/health" /></td>
              <td><Link href="/docs/guides/analytics/reconnect-analytics-scopes">Reconnect analytics scopes</Link></td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2 id="reference">Reference links</h2>
      <p>
        Use <Link href="/docs/api/analytics/platforms">Platform capabilities</Link> to see which Analytics capabilities are public,
        and use <Link href="/docs/api">API Reference</Link> for exact endpoint contracts.
      </p>
    </DocsPage>
  );
}
