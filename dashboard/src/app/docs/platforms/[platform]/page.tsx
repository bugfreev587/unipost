import { notFound } from "next/navigation";
import { DocsCode, DocsPage, DocsTable } from "../../_components/docs-shell";

const PLATFORMS = {
  twitter: {
    title: "Twitter/X",
    lead: "Use UniPost to publish text, images, video, and multi-post threads to X. This page is organized around what developers need to know before they ship and before they let automation take over.",
    capabilities: [
      ["Text posts", "Yes", "Up to 280 characters"],
      ["Image posts", "Yes", "Use media URLs or media IDs"],
      ["Video posts", "Yes", "Single video per post"],
      ["Threads", "Yes", "Use thread_position"],
      ["First comment", "Yes", "Posted as a reply after publish"],
    ],
    requirements: [
      ["caption", "Optional", "280 chars", "Required if there is no media-only flow"],
      ["media_urls", "Optional", "images or single video", "Do not mix unsupported media types"],
      ["thread_position", "Optional", "1-indexed", "Use on each thread entry"],
    ],
    example: `{
  "platform_posts": [
    { "account_id": "sa_twitter_123", "caption": "1/ Why we changed our docs", "thread_position": 1 },
    { "account_id": "sa_twitter_123", "caption": "2/ We wanted shorter paths to value", "thread_position": 2 }
  ],
  "idempotency_key": "twitter-thread-001"
}`,
  },
  linkedin: {
    title: "LinkedIn",
    lead: "LinkedIn is usually where teams want longer copy, cleaner formatting, and more explicit control over visibility. UniPost keeps the request shape consistent while still exposing platform options where they matter.",
    capabilities: [
      ["Text posts", "Yes", "Long-form captions supported"],
      ["Image posts", "Yes", "Single or multiple images depending on asset flow"],
      ["Video posts", "Yes", "Supported"],
      ["Threads", "No", "Publish as individual posts instead"],
      ["First comment", "Yes", "Supported"],
    ],
    requirements: [
      ["caption", "Optional", "3,000 chars", "Best for longer release notes and announcements"],
      ["platform_options.linkedin", "Optional", "visibility", "Use for audience controls"],
      ["first_comment", "Optional", "text", "Posted after the main post lands"],
    ],
    example: `{
  "platform_posts": [
    {
      "account_id": "sa_linkedin_456",
      "caption": "We shipped a new release today with managed Connect, preview links, and post validation."
    }
  ],
  "idempotency_key": "linkedin-post-001"
}`,
  },
  instagram: {
    title: "Instagram",
    lead: "Instagram is media-first. The implementation question is usually not whether you can send a caption, but whether the media combination and publish surface are valid for the connected account.",
    capabilities: [
      ["Text-only posts", "Limited", "Usually pair with media"],
      ["Image posts", "Yes", "Primary flow"],
      ["Video posts", "Yes", "Primary flow"],
      ["Threads", "No", "Not supported as Twitter-style threads"],
      ["First comment", "Yes", "Supported"],
    ],
    requirements: [
      ["media_urls", "Usually required", "images or video", "Instagram is media-driven"],
      ["caption", "Optional", "2,200 chars", "Commonly sent with media"],
      ["first_comment", "Optional", "text", "Supported after publish"],
    ],
    example: `{
  "platform_posts": [
    {
      "account_id": "sa_instagram_789",
      "caption": "Launch day.",
      "media_urls": ["https://example.com/launch-image.jpg"]
    }
  ],
  "idempotency_key": "instagram-image-001"
}`,
  },
  threads: {
    title: "Threads",
    lead: "Threads works well with short conversational copy. Use thread_position for multi-post threads instead of first comments.",
    capabilities: [
      ["Text posts", "Yes", "Up to 500 characters"],
      ["Image posts", "Yes", "Supported"],
      ["Video posts", "Yes", "Supported"],
      ["Threads", "Yes", "Use thread_position"],
      ["First comment", "No", "Use threads instead"],
    ],
    requirements: [
      ["caption", "Optional", "500 chars", "Short-form conversational copy"],
      ["thread_position", "Optional", "1-indexed", "Preferred over first_comment"],
      ["first_comment", "Rejected", "n/a", "Validate will catch this before publish"],
    ],
    example: `{
  "platform_posts": [
    { "account_id": "sa_threads_321", "caption": "1/ A cleaner docs IA matters", "thread_position": 1 },
    { "account_id": "sa_threads_321", "caption": "2/ Less searching, more building", "thread_position": 2 }
  ]
}`,
  },
  tiktok: {
    title: "TikTok",
    lead: "TikTok is a video-led flow with platform-specific privacy and upload options. UniPost keeps these options in platform_options so the base request shape stays stable.",
    capabilities: [
      ["Text posts", "No", "Video-first platform"],
      ["Image posts", "Limited", "Depends on upload mode"],
      ["Video posts", "Yes", "Primary flow"],
      ["Threads", "No", "Not applicable"],
      ["Analytics", "Partial", "Depends on connected account access"],
    ],
    requirements: [
      ["media_urls", "Required", "video asset", "Primary publish surface"],
      ["platform_options.tiktok", "Optional", "privacy and upload settings", "Use for TikTok-specific controls"],
      ["caption", "Optional", "2,200 chars", "Pair with video"],
    ],
    example: `{
  "platform_posts": [
    {
      "account_id": "sa_tiktok_111",
      "caption": "Shipping better docs.",
      "media_urls": ["https://example.com/video.mp4"],
      "platform_options": {
        "tiktok": { "privacy_level": "PUBLIC_TO_EVERYONE" }
      }
    }
  ]
}`,
  },
  youtube: {
    title: "YouTube",
    lead: "YouTube publishing usually needs more platform metadata than short-form networks. UniPost exposes those controls in platform_options while keeping the publish flow consistent.",
    capabilities: [
      ["Video posts", "Yes", "Primary flow"],
      ["Text posts", "No", "Not a native publish target"],
      ["Image posts", "No", "Video-first surface"],
      ["Scheduling", "Yes", "Supported through scheduled_at"],
      ["Analytics", "Yes", "Supported"],
    ],
    requirements: [
      ["media_urls", "Required", "single video", "Primary requirement"],
      ["platform_options.youtube", "Optional", "privacy, shorts, category, tags", "Use for YouTube-specific metadata"],
      ["caption", "Optional", "5,000 chars", "Used as description/body"],
    ],
    example: `{
  "platform_posts": [
    {
      "account_id": "sa_youtube_222",
      "caption": "Release walkthrough and roadmap.",
      "media_urls": ["https://example.com/release-video.mp4"],
      "platform_options": {
        "youtube": { "privacy_status": "public", "shorts": false }
      }
    }
  ]
}`,
  },
  bluesky: {
    title: "Bluesky",
    lead: "Bluesky is a strong fit for short copy and threads. UniPost supports both direct account connections and multi-post threading with a stable request shape.",
    capabilities: [
      ["Text posts", "Yes", "Up to 300 graphemes"],
      ["Image posts", "Yes", "Supported"],
      ["Video posts", "Supported", "Depends on asset and account capability"],
      ["Threads", "Yes", "Use thread_position"],
      ["First comment", "No", "Use threads instead"],
    ],
    requirements: [
      ["caption", "Optional", "300 graphemes", "Validate catches overages before publish"],
      ["thread_position", "Optional", "1-indexed", "Best way to publish multi-post flows"],
      ["first_comment", "Rejected", "n/a", "Use thread_position instead"],
    ],
    example: `{
  "platform_posts": [
    { "account_id": "sa_bluesky_333", "caption": "1/ We rebuilt our docs IA", "thread_position": 1 },
    { "account_id": "sa_bluesky_333", "caption": "2/ It is much easier to scan now", "thread_position": 2 }
  ]
}`,
  },
} as const;

export default async function PlatformDetailPage({
  params,
}: {
  params: Promise<{ platform: keyof typeof PLATFORMS }>;
}) {
  const { platform } = await params;
  const data = PLATFORMS[platform];
  if (!data) notFound();

  return (
    <DocsPage eyebrow="Platform Guide" title={data.title} lead={data.lead}>
      <h2 id="overview">Overview</h2>
      <p>Every platform guide follows the same structure so teams can compare constraints quickly and then jump into a concrete request example.</p>

      <h2 id="capabilities">Supported capabilities</h2>
      <DocsTable columns={["Capability", "Supported", "Notes"]} rows={data.capabilities} />

      <h2 id="requirements">Requirements</h2>
      <DocsTable columns={["Field", "Required", "Limits", "Notes"]} rows={data.requirements} />

      <h2 id="examples">Example request</h2>
      <p>This example shows the request shape we want developers to copy first on this platform.</p>
      <DocsCode code={data.example} />

      <h2 id="related-reference">Related reference</h2>
      <p>After choosing the right request shape here, switch to the API reference for the full endpoint contract, validation responses, and response schema.</p>
    </DocsPage>
  );
}
