import Link from "next/link";
import { DocsCodeTabs, DocsPage } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

const REQUEST_SNIPPETS = [{
  label: "cURL",
  lang: "bash",
  code: `curl "https://api.unipost.dev/v1/posts/post_abc123/analytics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
}];

const RESPONSE_SNIPPETS = [{
  label: "200",
  lang: "json",
  code: `{
  "data": {
    "post_id": "post_abc123",
    "metrics": {
      "likes": 214,
      "comments": 19,
      "shares": 47,
      "saves": 12,
      "impressions": 18420,
      "reach": 4210,
      "video_views": 0,
      "engagement_rate": 0.0152,
      "platform_specific": {
        "retweet_count": 31,
        "quote_count": 16
      }
    }
  }
}`,
}];

export default function PostAnalyticsGuidePage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Get analytics for a UniPost-published post"
      lead="Use the shared post analytics endpoint when you already have a UniPost post id and need normalized performance metrics for that post."
      className="docs-page-wide"
    >
      <h2 id="when-to-use">When to use this guide</h2>
      <p>
        Use <ApiInlineLink endpoint="GET /v1/posts/{post_id}/analytics" /> for a post created through UniPost. It returns a normalized
        metrics object for likes, comments, shares, saves, impressions, reach, video views, engagement rate, and provider-specific additions.
      </p>
      <p>
        If you need account-level followers or following counts, use <Link href="/docs/guides/analytics/account-metrics">account metrics</Link> instead.
      </p>

      <h2 id="steps">Steps</h2>
      <ol className="docs-step-list">
        <li>Keep the UniPost post id returned by <ApiInlineLink endpoint="POST /v1/posts" /> or from <ApiInlineLink endpoint="GET /v1/posts" />.</li>
        <li>Call <ApiInlineLink endpoint="GET /v1/posts/{post_id}/analytics" />.</li>
        <li>Read the normalized <code>data.metrics</code> fields first.</li>
        <li>Use <code>data.metrics.platform_specific</code> for provider-native fields that do not fit the shared shape.</li>
      </ol>

      <h2 id="request">Request</h2>
      <DocsCodeTabs snippets={REQUEST_SNIPPETS} />

      <h2 id="response">Response</h2>
      <DocsCodeTabs snippets={RESPONSE_SNIPPETS} />

      <h2 id="scope-notes">Scope notes</h2>
      <p>Post analytics availability depends on the destination platform and the scopes granted to the connected account.</p>
      <p>TikTok post analytics uses <code>video.list</code> to look up public videos owned by the connected TikTok account.</p>
      <p>Pinterest Pin analytics uses the approved Pinterest read scopes for UniPost-published Pins.</p>
      <p>Instagram, Threads, and Facebook Page metrics depend on their respective insights scopes.</p>

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/analytics/posts" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Post analytics</div>
          <div className="docs-next-body">Exact endpoint contract and response fields.</div>
        </Link>
        <Link href="/docs/api/analytics/platforms" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Platform capabilities</div>
          <div className="docs-next-body">Supported analytics capabilities by platform.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
