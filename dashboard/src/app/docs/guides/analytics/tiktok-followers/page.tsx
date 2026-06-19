import Link from "next/link";
import { DocsCodeTabs, DocsPage, DocsTable } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

const REQUEST_SNIPPETS = [{
  label: "cURL",
  lang: "bash",
  code: `curl "https://api.unipost.dev/v1/accounts/sa_tiktok_123/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
}];

const RESPONSE_SNIPPETS = [{
  label: "200",
  lang: "json",
  code: `{
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
}`,
}];

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
      <DocsTable
        columns={["Question", "Answer"]}
        rows={[
          ["Which UniPost API gets TikTok followers?", <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/metrics" />],
          ["Which TikTok scope is required?", <code key="scope">user.info.stats</code>],
          ["Which response field should I read?", <code key="field">data.follower_count</code>],
          [<span key="question">Should I use <code>video.list</code>?</span>, <span key="answer">No. <code>video.list</code> is for public videos and post-level TikTok video inventory, not follower count.</span>],
        ]}
      />

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
      <DocsCodeTabs snippets={REQUEST_SNIPPETS} />

      <h2 id="response">Response</h2>
      <DocsCodeTabs snippets={RESPONSE_SNIPPETS} />

      <h2 id="scope-notes">Scope notes</h2>
      <p><code>user.info.profile</code> powers TikTok profile fields such as username, bio, profile links, and verification status.</p>
      <p><code>user.info.stats</code> powers follower count, following count, likes count, and video count.</p>
      <p><code>video.list</code> powers public videos and post-level TikTok video lookup; it is not the followers API.</p>

      <h2 id="troubleshooting">Troubleshooting</h2>
      <DocsTable
        columns={["Symptom", "What to do"]}
        rows={[
          ["The account is disconnected or returns a reconnect-required state.", <span key="action">Reconnect the TikTok account so the new token includes <code>user.info.stats</code>.</span>],
          [<span key="symptom">The response has an upstream error in <code>platform_specific</code>.</span>, "Retry later or surface the request id to support. Upstream rate limits and provider errors are reported separately from real zero counts."],
          ["The account has zero followers.", <span key="action">Check whether <code>platform_specific.upstream_status</code> is present. If it is absent, the zero is the platform value.</span>],
        ]}
      />

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
