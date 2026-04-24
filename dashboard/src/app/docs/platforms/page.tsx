import Link from "next/link";
import { DocsPage, DocsTable } from "../_components/docs-shell";
import { ApiInlineLink } from "../api/_components/doc-components";

const PLATFORM_QUICK_REFERENCE = [
  [
    "Twitter/X",
    "Text, images, videos, GIFs, threads, first-comment replies",
    "Yes",
    <Link key="twitter-guide" href="/docs/platforms/twitter">View guide</Link>,
  ],
  [
    "LinkedIn",
    "Text, multi-image posts, video, audience visibility, first comment",
    "Yes",
    <Link key="linkedin-guide" href="/docs/platforms/linkedin">View guide</Link>,
  ],
  [
    "Instagram",
    "Feed posts, reels, stories, carousels, first comment, mediaType validation",
    "Yes",
    <Link key="instagram-guide" href="/docs/platforms/instagram">View guide</Link>,
  ],
  [
    "Threads",
    "Text, images, videos, carousel-style posts, multi-post threads",
    "Yes",
    <Link key="threads-guide" href="/docs/platforms/threads">View guide</Link>,
  ],
  [
    "TikTok",
    "Single-video posts, photo carousels, privacy and upload-mode controls",
    "Partial",
    <Link key="tiktok-guide" href="/docs/platforms/tiktok">View guide</Link>,
  ],
  [
    "YouTube",
    "Long-form videos, Shorts, scheduling, playlist insertion, metadata controls",
    "Yes",
    <Link key="youtube-guide" href="/docs/platforms/youtube">View guide</Link>,
  ],
  [
    "Bluesky",
    "Text, images, videos, thread_position, direct credential connection",
    "Limited",
    <Link key="bluesky-guide" href="/docs/platforms/bluesky">View guide</Link>,
  ],
] as const;

const PLATFORM_FEATURES = [
  ["Twitter/X", "Best current fit for fast text publishing, thread chains, and first-comment style follow-ups."],
  ["LinkedIn", "Best fit for longer-form professional copy, audience visibility controls, and single-video posting."],
  ["Instagram", "Media-first surface with explicit feed / reels / story / carousel behavior under `platform_options.instagram.mediaType`."],
  ["Threads", "Strong fit for conversational posts and thread sequencing through `thread_position`, without first-comment support."],
  ["TikTok", "Video-led publishing with photo carousels and platform-specific privacy/upload settings."],
  ["YouTube", "Single-video workflow with support for Shorts, visibility, scheduling, tags, and playlist insertion."],
  ["Bluesky", "Simple publish model for text, images, videos, and threads, plus direct app-password account connection."],
] as const;

const ANALYTICS_COVERAGE = [
  ["Workspace summary", "Cross-platform rollups for totals, trends, and by-platform reporting.", <Link key="analytics-summary" href="/docs/api/analytics">Analytics summary</Link>],
  ["Post analytics", "Normalized per-post metrics such as likes, comments, reach, and platform-exposed engagement values.", <Link key="analytics-posts" href="/docs/api/analytics/posts">Post analytics</Link>],
  ["Best current coverage", "Instagram, Twitter/X, LinkedIn, Threads, and YouTube have the clearest documented analytics paths today.", "Supported now"],
  ["Partial / limited coverage", "TikTok is documented as partial, and Bluesky is currently more limited than the main OAuth-based networks.", "Current state"],
] as const;

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
        columns={["Platform", "Current UniPost support", "Analytics", "Guide"]}
        rows={PLATFORM_QUICK_REFERENCE}
      />

      <h2 id="getting-started">Getting Started</h2>

      <h3 id="connect-an-account">1. Connect an Account</h3>
      <p>Start by connecting a workspace-owned account with <ApiInlineLink endpoint="POST /v1/accounts/connect" /> or use <ApiInlineLink endpoint="POST /v1/connect/sessions" /> when your customers need to connect their own accounts through hosted Connect.</p>
      <p>The currently documented platform keys are <code>twitter</code>, <code>linkedin</code>, <code>instagram</code>, <code>threads</code>, <code>tiktok</code>, <code>youtube</code>, and <code>bluesky</code>.</p>

      <h3 id="create-a-post">2. Create a Post</h3>
      <p>The main publish surface is <ApiInlineLink endpoint="POST /v1/posts" />. UniPost&apos;s recommended request shape is <code>platform_posts[]</code>, which lets you send platform-specific captions, media, and options instead of flattening everything into one shared payload.</p>
      <p>Before any automated publish, especially LLM-driven content, run the same payload through <ApiInlineLink endpoint="POST /v1/posts/validate" /> so platform limits fail early.</p>

      <h3 id="cross-post-to-multiple-platforms">3. Cross-Post to Multiple Platforms</h3>
      <p>Cross-posting is a first-class UniPost workflow. One request can target multiple connected accounts across different networks, while still adapting copy and media per destination through <code>platform_posts[]</code>.</p>
      <p>This is also where UniPost becomes more than a wrapper around seven separate platform APIs: retries, validation, async delivery state, analytics, and webhooks all sit behind one publish contract.</p>

      <h2 id="platform-specific-features">Platform-Specific Features</h2>
      <p>Each network still has its own behavior, and UniPost exposes that difference where it matters instead of pretending every destination works the same.</p>
      <DocsTable
        columns={["Platform", "What stands out in UniPost today"]}
        rows={PLATFORM_FEATURES}
      />

      <h2 id="analytics-coverage">Analytics Coverage</h2>
      <p>UniPost already exposes analytics as a shared layer rather than making you integrate each platform&apos;s reporting stack separately. The current public docs focus on workspace rollups and post-level metrics.</p>
      <DocsTable
        columns={["Area", "What exists today", "Docs"]}
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
