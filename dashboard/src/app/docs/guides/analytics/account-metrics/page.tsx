import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";
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
        Instagram, Threads, TikTok, and YouTube when the connected account has the required platform scopes.
      </p>

      <h2 id="steps">Steps</h2>
      <ol className="docs-step-list">
        <li>List connected accounts with <ApiInlineLink endpoint="GET /v1/accounts" />.</li>
        <li>Pick the account id for the platform you want to display.</li>
        <li>Call <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />.</li>
        <li>Read the normalized fields first, then inspect <code>platform_specific</code> for platform-native additions.</li>
      </ol>

      <h2 id="request">Request</h2>
      <DocsCodeTabs
        snippets={[{
          label: "cURL",
          lang: "bash",
          code: `curl "https://api.unipost.dev/v1/accounts/sa_instagram_123/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
        }]}
      />

      <h2 id="fields">Fields to read</h2>
      <DocsTable
        columns={["Field", "Meaning"]}
        rows={[
          [<code key="field">data.follower_count</code>, "Followers reported by the platform."],
          [<code key="field">data.following_count</code>, "Accounts this account follows, when the platform exposes it."],
          [<code key="field">data.post_count</code>, "Lifetime post, video, or media count exposed by the platform."],
          [<code key="field">data.platform_specific</code>, "Provider-native additions or upstream failure details that do not fit the normalized shape."],
        ]}
      />

      <h2 id="platform-notes">Platform notes</h2>
      <p>TikTok followers require <code>user.info.stats</code>; likes count appears in <code>platform_specific.likes_count</code>.</p>
      <p>Instagram and Threads require the approved account insight scopes documented in platform capabilities.</p>
      <p>
        YouTube account metrics use the YouTube Data API channel statistics available through <code>youtube.readonly</code>. UniPost maps{" "}
        <code>subscriberCount</code> to <code>follower_count</code>, returns <code>following_count</code> as <code>0</code> with{" "}
        <code>platform_specific.following_count_supported</code> set to <code>false</code>, and maps <code>videoCount</code> to{" "}
        <code>post_count</code>. If subscribers are hidden, <code>follower_count</code> is <code>0</code> and{" "}
        <code>platform_specific.hidden_subscriber_count</code> is <code>true</code>.
      </p>
      <p>
        For richer YouTube reporting, use <Link href="/docs/api/analytics/youtube">YouTube Analytics V2</Link>. In the dashboard,
        Analytics - Platforms - YouTube combines this V1 channel snapshot with V2 summary, daily trend, and top video reports.
      </p>
      <p>X account metrics depend on X API availability and rate limits for the connected account.</p>
      <p>Unsupported platforms return <code>NOT_SUPPORTED</code> instead of an empty success response.</p>

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
