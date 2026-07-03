import Link from "next/link";
import { DocsPage, DocsTable } from "../../../_components/docs-shell";
import { ApiInlineLink } from "../../../api/_components/doc-components";

export default function ReconnectAnalyticsScopesGuidePage() {
  return (
    <DocsPage
      eyebrow="Analytics Guides"
      title="Reconnect accounts for Analytics scopes"
      lead="If an account was connected before a platform granted analytics scopes, reconnect the account so UniPost receives a token with the new permissions."
      className="docs-page-wide"
    >
      <h2 id="when-reconnect-is-needed">When reconnect is needed</h2>
      <p>
        UniPost cannot silently add provider permissions to an existing OAuth token. If a platform account was connected before
        analytics scopes were available, the user must reconnect that account through the same connection mode they used originally.
      </p>

      <h2 id="workflow">Workflow</h2>
      <ol className="docs-step-list">
        <li>Check the account status with <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/health" /> or list accounts with <ApiInlineLink endpoint="GET /v1/accounts" />.</li>
        <li>If the account is disconnected or needs updated permissions, send the user through OAuth again.</li>
        <li>For workspace-owned accounts, start OAuth with <ApiInlineLink endpoint="POST /v1/oauth/connect" />.</li>
        <li>For customer-owned accounts, create a hosted <Link href="/docs/connect-sessions">Connect Session</Link>.</li>
        <li>After reconnect, call the same Analytics API again.</li>
      </ol>

      <h2 id="tiktok-scopes">TikTok analytics scopes</h2>
      <p>
        TikTok Analytics uses the approved <code>user.info.profile</code>, <code>user.info.stats</code>, and <code>video.list</code> scopes.
        For follower count, the required scope is <code>user.info.stats</code>, and the UniPost API remains{" "}
        <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />.
      </p>

      <h2 id="youtube-scopes">YouTube account metrics scope</h2>
      <p>
        YouTube V1 account metrics use the YouTube Data API channel statistics endpoint and the existing{" "}
        <code>youtube.readonly</code> OAuth scope. You do not need a YouTube Analytics API scope or a new UniPost API key scope for{" "}
        <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/metrics" />. Reconnect only when the stored Google token is invalid,
        lacks <code>youtube.readonly</code>, or no longer resolves to the expected channel.
      </p>
      <p>
        YouTube Analytics V2 reports use <code>yt-analytics.readonly</code> and are exposed through{" "}
        <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/youtube/analytics/summary" />,{" "}
        <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/youtube/analytics/trend" />, and{" "}
        <ApiInlineLink endpoint="GET /v1/accounts/{account_id}/youtube/analytics/videos" />. Existing YouTube accounts connected before
        that scope was granted must reconnect before V2 reports are available.
      </p>

      <h2 id="platform-scope-map">Common scope map</h2>
      <DocsTable
        columns={["Platform", "Analytics scopes", "Common UniPost API"]}
        rows={[
          ["Instagram", <span key="scopes"><code>instagram_business_basic</code>, <code>instagram_business_manage_insights</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/metrics" />],
          ["Threads", <span key="scopes"><code>threads_basic</code>, <code>threads_manage_insights</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/metrics" />],
          ["Pinterest", <span key="scopes"><code>pins:read</code>, <code>boards:read</code>, <code>user_accounts:read</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/posts/{post_id}/analytics" />],
          ["TikTok", <span key="scopes"><code>user.info.profile</code>, <code>user.info.stats</code>, <code>video.list</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/metrics" />],
          ["YouTube V1", <span key="scopes"><code>youtube.readonly</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/metrics" />],
          ["YouTube V2", <span key="scopes"><code>yt-analytics.readonly</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/accounts/{account_id}/youtube/analytics/summary" />],
          ["Facebook Page", <span key="scopes"><code>pages_read_engagement</code>, <code>read_insights</code></span>, <ApiInlineLink key="api" endpoint="GET /v1/posts/{post_id}/analytics" />],
        ]}
      />

      <h2 id="reference">Reference</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/accounts/health" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Check account health</div>
          <div className="docs-next-body">Inspect connection state before retrying Analytics calls.</div>
        </Link>
        <Link href="/docs/api/accounts/oauth-connect" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Connect account with OAuth</div>
          <div className="docs-next-body">Start workspace-owned account OAuth.</div>
        </Link>
        <Link href="/docs/connect-sessions" className="docs-next-card">
          <div className="docs-next-kicker">Guide</div>
          <div className="docs-next-title">Connect Sessions</div>
          <div className="docs-next-body">Reconnect customer-owned accounts with hosted OAuth.</div>
        </Link>
        <Link href="/docs/api/analytics/platforms" className="docs-next-card">
          <div className="docs-next-kicker">Reference</div>
          <div className="docs-next-title">Platform capabilities</div>
          <div className="docs-next-body">Supported Analytics capabilities and scopes.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
