import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

export default function ExportPostAnalyticsGuidePage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Export post analytics rows"
      lead="Use the Analytics export endpoint when you need normalized post-level analytics as CSV for reporting, BI imports, or scheduled jobs."
      className="docs-page-wide"
    >
      <h2 id="when-to-use">When to use this guide</h2>
      <p>
        Use <ApiInlineLink endpoint="GET /v1/analytics/posts/export" /> when your app needs analytics rows across multiple UniPost-published
        posts instead of a single post response. The export accepts date, platform, profile, account, post, status, and sort filters.
      </p>

      <h2 id="steps">Steps</h2>
      <ol className="docs-step-list">
        <li>Choose the reporting window with <code>from</code> and <code>to</code>.</li>
        <li>Add optional filters such as <code>platform</code>, <code>account_id</code>, <code>profile_id</code>, or <code>status</code>.</li>
        <li>Call <ApiInlineLink endpoint="GET /v1/analytics/posts/export" /> and save the CSV response.</li>
        <li>Load the CSV into your warehouse, spreadsheet, or reporting job.</li>
      </ol>

      <h2 id="request">Request</h2>
      <DocsCodeTabs
        snippets={[{
          label: "cURL",
          lang: "bash",
          code: `curl "https://api.unipost.dev/v1/analytics/posts/export?platform=tiktok&from=2026-06-01&to=2026-06-30" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -o unipost-analytics-posts.csv`,
        }]}
      />

      <h2 id="columns">Useful columns</h2>
      <DocsTable
        columns={["Column", "Meaning"]}
        rows={[
          [<code key="column">post_id</code>, "UniPost post id."],
          [<code key="column">platform</code>, "Destination platform for the row."],
          [<code key="column">social_account_id</code>, "Connected social account used for the post."],
          [
            <span key="columns">
              <code>impressions</code>, <code>reach</code>, <code>likes</code>, <code>comments</code>, <code>shares</code>
            </span>,
            "Normalized performance metrics when available.",
          ],
          [<code key="column">last_failure_reason</code>, "Most recent cached upstream analytics failure, when present."],
        ]}
      />

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/analytics/posts/export" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Export analytics posts</div>
          <div className="docs-next-body">Exact query filters and CSV columns.</div>
        </Link>
        <Link href="/docs/guides/analytics/post-analytics" className="docs-next-card">
          <div className="docs-next-kicker">Guide</div>
          <div className="docs-next-title">Get post analytics</div>
          <div className="docs-next-body">Use a single-post response instead of CSV export.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
