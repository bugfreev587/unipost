import Link from "next/link";
import { DocsCode, DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const CHECK = "✓";
const DASH = "—";

const PLATFORM_QUICK_REFERENCE = [
  [
    "Twitter/X",
    CHECK,
    CHECK,
    CHECK,
    CHECK,
    CHECK,
    <Link key="twitter-guide" href="/docs/platforms/twitter">View</Link>,
  ],
  [
    "LinkedIn",
    CHECK,
    CHECK,
    CHECK,
    DASH,
    CHECK,
    <Link key="linkedin-guide" href="/docs/platforms/linkedin">View</Link>,
  ],
  [
    "Instagram",
    DASH,
    CHECK,
    CHECK,
    DASH,
    CHECK,
    <Link key="instagram-guide" href="/docs/platforms/instagram">View</Link>,
  ],
  [
    "Threads",
    CHECK,
    CHECK,
    CHECK,
    CHECK,
    CHECK,
    <Link key="threads-guide" href="/docs/platforms/threads">View</Link>,
  ],
  [
    "TikTok",
    DASH,
    CHECK,
    CHECK,
    DASH,
    DASH,
    <Link key="tiktok-guide" href="/docs/platforms/tiktok">View</Link>,
  ],
  [
    "YouTube",
    DASH,
    DASH,
    CHECK,
    DASH,
    CHECK,
    <Link key="youtube-guide" href="/docs/platforms/youtube">View</Link>,
  ],
  [
    "Bluesky",
    CHECK,
    CHECK,
    CHECK,
    CHECK,
    DASH,
    <Link key="bluesky-guide" href="/docs/platforms/bluesky">View</Link>,
  ],
] as const;

const PLATFORM_FEATURES = [
  ["Twitter/X", CHECK, CHECK, DASH, DASH, CHECK],
  ["LinkedIn", DASH, CHECK, DASH, CHECK, DASH],
  ["Instagram", CHECK, DASH, CHECK, DASH, CHECK],
  ["Threads", DASH, CHECK, DASH, DASH, DASH],
  ["TikTok", DASH, DASH, CHECK, DASH, DASH],
  ["YouTube", DASH, DASH, DASH, CHECK, DASH],
  ["Bluesky", DASH, CHECK, DASH, DASH, CHECK],
] as const;

const ANALYTICS_COVERAGE = [
  ["Twitter/X", CHECK, CHECK, <Link key="analytics-twitter" href="/docs/api/analytics">Analytics</Link>],
  ["LinkedIn", CHECK, CHECK, <Link key="analytics-linkedin" href="/docs/api/analytics">Analytics</Link>],
  ["Instagram", CHECK, CHECK, <Link key="analytics-instagram" href="/docs/api/analytics">Analytics</Link>],
  ["Threads", CHECK, CHECK, <Link key="analytics-threads" href="/docs/api/analytics">Analytics</Link>],
  ["TikTok", CHECK, DASH, <Link key="analytics-tiktok" href="/docs/api/analytics">Analytics</Link>],
  ["YouTube", CHECK, CHECK, <Link key="analytics-youtube" href="/docs/api/analytics">Analytics</Link>],
  ["Bluesky", CHECK, DASH, <Link key="analytics-bluesky" href="/docs/api/analytics">Analytics</Link>],
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
      lead="Use this page to understand what UniPost supports today across Twitter/X, LinkedIn, Instagram, Threads, TikTok, YouTube, and Bluesky. The goal here is macro-level implementation guidance: which destinations exist, how the publish model works across them, where analytics and webhooks fit, and when you need to drop into a platform-specific guide."
      className="docs-page-wide"
    >
      <h2 id="platform-quick-reference">Platform Quick Reference</h2>
      <p>UniPost currently documents seven publishing destinations in the public platform guides. Each guide expands on media rules, validation behavior, and example request bodies for that network.</p>
      <DocsTable
        columns={["Platform", "Text", "Images", "Video", "Threads", "Analytics", "Guide"]}
        rows={PLATFORM_QUICK_REFERENCE}
      />

      <h2 id="getting-started">Getting Started</h2>

      <h3 id="connect-an-account">1. Connect an Account</h3>
      <p>Start with <ApiInlineLink endpoint="POST /v1/accounts/connect" /> for workspace-owned accounts, or <ApiInlineLink endpoint="POST /v1/connect/sessions" /> for customer-owned account onboarding.</p>
      <DocsCode code={CONNECT_SNIPPET} language="bash" />

      <h3 id="create-a-post">2. Create a Post</h3>
      <p>Use <ApiInlineLink endpoint="POST /v1/posts" /> with the recommended <code>platform_posts[]</code> request shape.</p>
      <DocsCode code={CREATE_POST_SNIPPET} language="bash" />

      <h3 id="cross-post-to-multiple-platforms">3. Cross-Post to Multiple Platforms</h3>
      <p>Send multiple platform-specific payloads in one request when you need cross-posting with adapted copy.</p>
      <DocsCode code={CROSS_POST_SNIPPET} language="bash" />

      <h2 id="platform-specific-features">Platform-Specific Features</h2>
      <p>Each network still has its own behavior, and UniPost exposes that difference where it matters instead of pretending every destination works the same.</p>
      <DocsTable
        columns={["Platform", "First Comment", "Audience / Privacy", "Surface Controls", "Playlist / Tags", "Direct Credentials"]}
        rows={PLATFORM_FEATURES}
      />

      <h2 id="analytics-coverage">Analytics Coverage</h2>
      <p>UniPost already exposes analytics as a shared layer rather than making you integrate each platform&apos;s reporting stack separately. The current public docs focus on workspace rollups and post-level metrics.</p>
      <DocsTable
        columns={["Platform", "Workspace Summary", "Post Analytics", "Docs"]}
        rows={ANALYTICS_COVERAGE}
      />

      <h2 id="webhooks">Webhooks</h2>
      <p><Link href="/docs/api/webhooks">Developer webhooks</Link> are the current push-delivery mechanism across platforms. They cover async publish outcomes such as <code>post.published</code>, <code>post.partial</code>, and <code>post.failed</code>, plus account lifecycle events like <code>account.connected</code> and <code>account.disconnected</code>.</p>
      <p>That means the macro pattern is consistent even when platform behavior is not: publish once, then either poll the post resource or subscribe to webhooks for final outcome.</p>

      <h2 id="api-reference">API Reference</h2>
      <p>The fastest path through the current platform surface is usually:</p>
      <ul className="docs-list">
        <li><Link href="/docs/api/accounts/connect">Connect account</Link> for workspace-owned accounts.</li>
        <li><Link href="/docs/api/connect/sessions">Connect sessions</Link> for customer-owned account onboarding.</li>
        <li><Link href="/docs/api/posts/create">Create post</Link> for publish and scheduling.</li>
        <li><Link href="/docs/api/posts/validate">Validate post</Link> for preflight checks before publish.</li>
        <li><Link href="/docs/api/media">Media API</Link> when your workflow starts from local files instead of hosted URLs.</li>
        <li><Link href="/docs/api/analytics">Analytics</Link> and <Link href="/docs/api/webhooks">Developer webhooks</Link> for reporting and async delivery tracking.</li>
      </ul>
    </DocsPage>
  );
}
