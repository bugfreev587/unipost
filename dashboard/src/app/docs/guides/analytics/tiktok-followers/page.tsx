import Link from "next/link";
import { CodeBlock } from "../../../_components/code-block";
import { DocsPage } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

export default function TikTokFollowersGuidePage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Get TikTok followers"
      lead="Use UniPost account metrics to read TikTok follower count from the normalized account response. You do not need to call a TikTok-native endpoint for this task."
      className="docs-page-wide"
    >
      <p className="docs-note">
        The API to use is <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />. TikTok followers come from the
        approved <code>user.info.stats</code> scope and are returned as <code>data.follower_count</code>.
      </p>

      <h2 id="answer">Direct answer</h2>
      <div className="docs-table-wrap">
        <table className="docs-table">
          <thead>
            <tr>
              <th>Question</th>
              <th>Answer</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Which UniPost API gets TikTok followers?</td>
              <td><ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" /></td>
            </tr>
            <tr>
              <td>Which TikTok scope is required?</td>
              <td><code>user.info.stats</code></td>
            </tr>
            <tr>
              <td>Which response field should I read?</td>
              <td><code>data.follower_count</code></td>
            </tr>
            <tr>
              <td>Should I use <code>video.list</code>?</td>
              <td>No. <code>video.list</code> is for public videos and post-level TikTok video inventory, not follower count.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2 id="steps">Steps</h2>
      <ol className="docs-step-list">
        <li>
          List accounts with <ApiInlineLink endpoint="GET /v1/accounts" /> and pick the TikTok account id. You can filter with
          <code> platform=tiktok</code> when listing accounts.
        </li>
        <li>
          Confirm the account is active. If the account was connected before TikTok analytics scopes were granted, reconnect it so
          the token includes <code>user.info.stats</code>.
        </li>
        <li>
          Call <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" /> with that account id.
        </li>
        <li>
          Read <code>data.follower_count</code> from the response.
        </li>
      </ol>

      <h2 id="request">Request</h2>
      <CodeBlock
        language="bash"
        title="cURL"
        code={`curl "https://api.unipost.dev/v1/accounts/sa_tiktok_123/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`}
      />

      <h2 id="response">Response</h2>
      <CodeBlock
        language="json"
        title="200"
        code={`{
  "data": {
    "social_account_id": "sa_tiktok_123",
    "platform": "tiktok",
    "follower_count": 12840,
    "following_count": 312,
    "post_count": 86,
    "platform_specific": {
      "likes_count": 54021
    },
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`}
      />

      <h2 id="scope-notes">Scope notes</h2>
      <ul className="docs-step-list">
        <li><code>user.info.profile</code> powers TikTok profile fields such as username, bio, profile links, and verification status.</li>
        <li><code>user.info.stats</code> powers follower count, following count, likes count, and video count.</li>
        <li><code>video.list</code> powers public videos and post-level TikTok video lookup; it is not the followers API.</li>
      </ul>

      <h2 id="troubleshooting">Troubleshooting</h2>
      <div className="docs-table-wrap">
        <table className="docs-table">
          <thead>
            <tr>
              <th>Symptom</th>
              <th>What to do</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>The account is disconnected or returns a reconnect-required state.</td>
              <td>Reconnect the TikTok account so the new token includes <code>user.info.stats</code>.</td>
            </tr>
            <tr>
              <td>The response has an upstream error in <code>platform_specific</code>.</td>
              <td>Retry later or surface the request id to support. Upstream rate limits and provider errors are reported separately from real zero counts.</td>
            </tr>
            <tr>
              <td>The account has zero followers.</td>
              <td>Check whether <code>platform_specific.upstream_status</code> is present. If it is absent, the zero is the platform value.</td>
            </tr>
          </tbody>
        </table>
      </div>

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/accounts/metrics" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Get account metrics</div>
          <div className="docs-next-body">Exact request, response fields, and error codes.</div>
        </Link>
        <Link href="/docs/api/accounts/list" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">List accounts</div>
          <div className="docs-next-body">Find the TikTok account id to pass into metrics.</div>
        </Link>
        <Link href="/docs/api/analytics/platforms" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Platform capabilities</div>
          <div className="docs-next-body">Confirm public Analytics capabilities by platform.</div>
        </Link>
        <Link href="/docs/guides/analytics/reconnect-analytics-scopes" className="docs-next-card">
          <div className="docs-next-kicker">Guide</div>
          <div className="docs-next-title">Reconnect analytics scopes</div>
          <div className="docs-next-body">Handle accounts connected before analytics scopes were available.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
