import Link from "next/link";
import { DocsCode, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

function checkCell() {
  return <span className="docs-matrix-check">✓</span>;
}

function dashCell() {
  return <span className="docs-matrix-dash">—</span>;
}

function partialCell() {
  return <span className="docs-matrix-partial">Limited</span>;
}

const PLATFORM_API_NAMES = [
  ["Twitter / X", <code key="twitter">twitter</code>],
  ["LinkedIn", <code key="linkedin">linkedin</code>],
  ["Instagram", <code key="instagram">instagram</code>],
  ["Threads", <code key="threads">threads</code>],
  ["TikTok", <code key="tiktok">tiktok</code>],
  ["YouTube", <code key="youtube">youtube</code>],
  ["Bluesky", <code key="bluesky">bluesky</code>],
  ["Facebook", <code key="facebook">facebook</code>],
  ["Pinterest", <code key="pinterest">pinterest</code>],
] as const;

const PLATFORM_QUICK_REFERENCE = [
  [
    "Twitter/X",
    checkCell(),
    checkCell(),
    checkCell(),
    checkCell(),
    checkCell(),
    <Link key="twitter-guide" href="/docs/platforms/twitter">View</Link>,
  ],
  [
    "LinkedIn",
    checkCell(),
    checkCell(),
    checkCell(),
    dashCell(),
    checkCell(),
    <Link key="linkedin-guide" href="/docs/platforms/linkedin">View</Link>,
  ],
  [
    "Instagram",
    dashCell(),
    checkCell(),
    checkCell(),
    dashCell(),
    checkCell(),
    <Link key="instagram-guide" href="/docs/platforms/instagram">View</Link>,
  ],
  [
    "Threads",
    checkCell(),
    checkCell(),
    checkCell(),
    checkCell(),
    checkCell(),
    <Link key="threads-guide" href="/docs/platforms/threads">View</Link>,
  ],
  [
    "TikTok",
    dashCell(),
    checkCell(),
    checkCell(),
    dashCell(),
    partialCell(),
    <Link key="tiktok-guide" href="/docs/platforms/tiktok">View</Link>,
  ],
  [
    "YouTube",
    dashCell(),
    dashCell(),
    checkCell(),
    dashCell(),
    partialCell(),
    <Link key="youtube-guide" href="/docs/platforms/youtube">View</Link>,
  ],
  [
    "Pinterest",
    dashCell(),
    checkCell(),
    checkCell(),
    dashCell(),
    partialCell(),
    <Link key="pinterest-guide" href="/docs/platforms/pinterest">View</Link>,
  ],
  [
    "Bluesky",
    checkCell(),
    checkCell(),
    checkCell(),
    checkCell(),
    partialCell(),
    <Link key="bluesky-guide" href="/docs/platforms/bluesky">View</Link>,
  ],
  [
    "Facebook (Beta)",
    checkCell(),
    checkCell(),
    checkCell(),
    dashCell(),
    partialCell(),
    <Link key="facebook-guide" href="/docs/platforms/facebook">View</Link>,
  ],
] as const;

const PLATFORM_FEATURES = [
  ["Twitter/X", checkCell(), checkCell(), dashCell(), dashCell(), checkCell()],
  ["LinkedIn", dashCell(), checkCell(), dashCell(), checkCell(), dashCell()],
  ["Instagram", checkCell(), dashCell(), checkCell(), dashCell(), checkCell()],
  ["Threads", dashCell(), checkCell(), dashCell(), dashCell(), dashCell()],
  ["TikTok", dashCell(), dashCell(), checkCell(), dashCell(), dashCell()],
  ["YouTube", dashCell(), dashCell(), dashCell(), checkCell(), dashCell()],
  ["Pinterest", dashCell(), checkCell(), checkCell(), dashCell(), dashCell()],
  ["Bluesky", dashCell(), checkCell(), dashCell(), dashCell(), checkCell()],
  ["Facebook (Beta)", dashCell(), dashCell(), checkCell(), dashCell(), dashCell()],
] as const;

const ANALYTICS_COVERAGE = [
  ["Twitter/X", checkCell(), dashCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-twitter" href="/docs/api/analytics">View</Link>],
  ["LinkedIn", dashCell(), dashCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-linkedin" href="/docs/api/analytics">View</Link>],
  ["Instagram", dashCell(), checkCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-instagram" href="/docs/api/analytics">View</Link>],
  ["Threads", checkCell(), dashCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-threads" href="/docs/api/analytics">View</Link>],
  ["TikTok", dashCell(), dashCell(), checkCell(), checkCell(), checkCell(), <Link key="analytics-tiktok" href="/docs/api/analytics">View</Link>],
  ["YouTube", dashCell(), dashCell(), checkCell(), checkCell(), checkCell(), <Link key="analytics-youtube" href="/docs/api/analytics">View</Link>],
  ["Pinterest", checkCell(), dashCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-pinterest" href="/docs/api/analytics">View</Link>],
  ["Bluesky", dashCell(), dashCell(), checkCell(), checkCell(), dashCell(), <Link key="analytics-bluesky" href="/docs/api/analytics">View</Link>],
  ["Facebook (Beta)", dashCell(), dashCell(), checkCell(), checkCell(), checkCell(), <Link key="analytics-facebook" href="/docs/api/analytics">View</Link>],
] as const;

const CONNECT_SNIPPET = `curl -X POST "https://api.unipost.dev/v1/accounts/connect" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile_id": "pr_brand_us",
    "platform": "bluesky",
    "credentials": {
      "identifier": "alex.bsky.social",
      "password": "app-password"
    }
  }'`;

const CREATE_POST_SNIPPET = `curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_123",
        "caption": "Hello from UniPost"
      }
    ]
  }'`;

const CROSS_POST_SNIPPET = `curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_123",
        "caption": "Short version for X"
      },
      {
        "account_id": "sa_linkedin_456",
        "caption": "Longer version for LinkedIn with more context."
      },
      {
        "account_id": "sa_bluesky_789",
        "caption": "Variant for Bluesky"
      }
    ],
    "idempotency_key": "launch-2026-04-24-001"
  }'`;

export default function PlatformsPage() {
  return (
    <DocsPage
      eyebrow="Platforms"
      title="Platform support across UniPost."
      lead="Use this page to understand what UniPost supports today across Twitter/X, LinkedIn, Instagram, Threads, TikTok, YouTube, Pinterest, Bluesky, and Facebook. The goal here is macro-level implementation guidance: which destinations exist, how the publish model works across them, where analytics and webhooks fit, and when you need to drop into a platform-specific guide."
      className="docs-page-wide"
    >
      <div className="docs-badge-row">
        <span className="docs-badge">Publishing</span>
        <span className="docs-badge">Media rules</span>
        <span className="docs-badge">Analytics</span>
        <span className="docs-badge">Webhooks</span>
      </div>

      <h2 id="platform-names">Platform names in the UniPost API</h2>
      <p>Wherever the API accepts a platform — as a query filter, a request body field, or a path segment — UniPost uses the lowercase, URL-safe identifier listed below. Use this exact value when calling endpoints like <ApiInlineLink endpoint="GET /v1/accounts" />, <ApiInlineLink endpoint="POST /v1/accounts/connect" />, or <ApiInlineLink endpoint="POST /v1/connect/sessions" />.</p>
      <DocsTable
        columns={["Network", "API platform value"]}
        rows={PLATFORM_API_NAMES}
      />

      <h2 id="platform-quick-reference">Platform Quick Reference</h2>
      <p>UniPost currently documents nine publishing destinations in the public platform guides. Each guide expands on media rules, validation behavior, and example request bodies for that network.</p>
      <DocsTable
        columns={["Platform", "Text", "Images", "Video", "Threads", "Analytics", "Guide"]}
        rows={PLATFORM_QUICK_REFERENCE}
      />

      <h2 id="getting-started">Getting Started</h2>
      <p>
        The platform pages below explain destination-specific limits and payload
        options. For the shared post creation, local media upload, and async result
        flow, use the <Link href="/docs/publishing">Publishing guide</Link>.
      </p>

      <div className="docs-step-flow">
        <div className="docs-step-row">
          <div className="docs-step-number">1</div>
          <div>
            <div className="docs-step-title">Connect an account</div>
            <div className="docs-step-copy">Start with <ApiInlineLink endpoint="POST /v1/accounts/connect" /> for workspace-owned accounts, or <ApiInlineLink endpoint="POST /v1/connect/sessions" /> for customer-owned account onboarding.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">2</div>
          <div>
            <div className="docs-step-title">Create a post</div>
            <div className="docs-step-copy">Use <ApiInlineLink endpoint="POST /v1/posts" /> with the recommended <code>platform_posts[]</code> request shape.</div>
          </div>
        </div>
        <div className="docs-step-row">
          <div className="docs-step-number">3</div>
          <div>
            <div className="docs-step-title">Cross-post to multiple platforms</div>
            <div className="docs-step-copy">Send multiple platform-specific payloads in one request when you need cross-posting with adapted copy.</div>
          </div>
        </div>
      </div>

      <h3 id="connect-an-account">Connect an Account</h3>
      <DocsCode code={CONNECT_SNIPPET} language="bash" />

      <h3 id="create-a-post">Create a Post</h3>
      <DocsCode code={CREATE_POST_SNIPPET} language="bash" />

      <h3 id="cross-post-to-multiple-platforms">Cross-Post to Multiple Platforms</h3>
      <DocsCode code={CROSS_POST_SNIPPET} language="bash" />

      <h2 id="platform-specific-features">Platform-Specific Features</h2>
      <p>Each network still has its own behavior, and UniPost exposes that difference where it matters instead of pretending every destination works the same.</p>
      <DocsTable
        columns={["Platform", "First Comment", "Audience / Privacy", "Surface Controls", "Playlist / Tags", "Direct Credentials"]}
        rows={PLATFORM_FEATURES}
      />

      <h2 id="analytics-coverage">Analytics Coverage</h2>
      <p>Which analytics KPIs are clearly represented in UniPost&apos;s current public analytics docs? This matrix only marks metrics that are explicitly documented today.</p>
      <DocsTable
        columns={["Platform", "Impressions", "Reach", "Likes", "Comments", "Views", "Docs"]}
        rows={ANALYTICS_COVERAGE}
      />

      <h2 id="webhooks">Webhooks</h2>
      <p><Link href="/docs/api/webhooks">Developer webhooks</Link> are the current push-delivery mechanism across platforms. They cover async publish outcomes such as <code>post.published</code>, <code>post.partial</code>, and <code>post.failed</code>, plus account lifecycle events like <code>account.connected</code> and <code>account.disconnected</code>.</p>
      <p>That means the macro pattern is consistent even when platform behavior is not: publish once, then either poll the post resource or subscribe to webhooks for final outcome.</p>

      <h2 id="api-reference">API Reference</h2>
      <p>The fastest path through the current platform surface is usually:</p>
      <ul className="docs-list">
        <li><Link href="/docs/api/accounts/oauth-connect">OAuth connect</Link> for OAuth platforms, or <Link href="/docs/api/accounts/connect">direct credentials</Link> for Bluesky.</li>
        <li><Link href="/docs/api/connect/sessions">Connect sessions</Link> for customer-owned account onboarding.</li>
        <li><Link href="/docs/api/posts/create">Create post</Link> for publish and scheduling.</li>
        <li><Link href="/docs/api/posts/validate">Validate post</Link> for preflight checks before publish.</li>
        <li><Link href="/docs/api/media">Media API</Link> when your workflow starts from local files instead of hosted URLs.</li>
        <li><Link href="/docs/api/analytics">Analytics</Link> and <Link href="/docs/api/webhooks">Developer webhooks</Link> for reporting and async delivery tracking.</li>
      </ul>
    </DocsPage>
  );
}
