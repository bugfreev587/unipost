import { notFound } from "next/navigation";
import { DocsCode, DocsPage, DocsRichText, DocsTable } from "../../_components/docs-shell";
import { ApiInlineLink } from "../../api/_components/doc-components";

type PlatformDoc = {
  title: string;
  lead: string;
  overview: string;
  capabilities: readonly (readonly string[])[];
  requirements: readonly (readonly string[])[];
  options?: readonly (readonly string[])[];
  examples: ReadonlyArray<{ title: string; body: string; note?: string }>;
  errors: readonly (readonly string[])[];
};

const PLATFORMS: Record<string, PlatformDoc> = {
  bluesky: {
    title: "Bluesky",
    lead: "Bluesky is one of the cleanest fits for UniPost: short-form text, image and video support, and multi-post threading all map cleanly into the standard publish model.",
    overview: "Use Bluesky when you want fast text publishing, direct account setup, and thread-based conversational posting. UniPost supports both direct Bluesky credential connection and normal multi-account publishing semantics here.",
    capabilities: [
      ["Text posts", "Yes", "Up to 300 graphemes"],
      ["Image posts", "Yes", "Up to 4 images"],
      ["Video posts", "Yes", "Exactly 1 video"],
      ["Threads", "Yes", "Use `thread_position`"],
      ["First comment", "No", "Use threads instead"],
      ["Text-only posts", "Yes", "Media is optional"],
    ],
    requirements: [
      ["caption", "Optional", "300 graphemes", "Validate catches overages before publish"],
      ["media_urls or media_ids", "Optional", "1-4 images OR 1 video", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Do not mix image and video."],
      ["thread_position", "Optional", "1-indexed", "Preferred way to create multi-post flows"],
      ["first_comment", "Rejected", "n/a", "Use thread_position instead"],
    ],
    examples: [
      {
        title: "Text post",
        body: `{
  "caption": "Shipping on Bluesky today.",
  "account_ids": ["sa_bluesky_1"]
}`,
      },
      {
        title: "Image post",
        body: `{
  "caption": "Photos from the trip ✈️",
  "account_ids": ["sa_bluesky_1"],
  "media_urls": [
    "https://cdn.example.com/photo1.jpg",
    "https://cdn.example.com/photo2.jpg"
  ]
}`,
      },
      {
        title: "Thread",
        body: `{
  "platform_posts": [
    { "account_id": "sa_bluesky_1", "caption": "1/ Why we changed the docs", "thread_position": 1 },
    { "account_id": "sa_bluesky_1", "caption": "2/ The new structure is easier to scan", "thread_position": 2 }
  ]
}`,
      },
    ],
    errors: [
      ["caption_too_long", "Caption exceeds 300 graphemes"],
      ["first_comment_unsupported", "Bluesky uses threads instead of first-comment publishing"],
      ["too_many_media", "More than 4 images or more than 1 video supplied"],
    ],
  },
  twitter: {
    title: "Twitter / X",
    lead: "Twitter/X is the strongest thread and first-comment platform in UniPost today. Use it when you need fast text publishing, reply chains, and media-aware posts with precise limits.",
    overview: "Twitter/X supports text-only posts, image posts, video posts, and multi-post threads. It is also one of the networks where first-comment style publishing works well because UniPost can create a self-reply after the main tweet lands.",
    capabilities: [
      ["Text posts", "Yes", "Up to 280 characters"],
      ["Image posts", "Yes", "Up to 4 images"],
      ["Video posts", "Yes", "Exactly 1 video"],
      ["GIF posts", "Yes", "Exactly 1 GIF"],
      ["Threads", "Yes", "Use `thread_position`"],
      ["First comment", "Yes", "Posted as a reply after publish"],
    ],
    requirements: [
      ["caption", "Optional", "280 chars", "Required unless media-only flow is valid"],
      ["media_urls or media_ids", "Optional", "1-4 images OR 1 video OR 1 GIF", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Do not mix media types."],
      ["thread_position", "Optional", "1-indexed", "Use on each thread entry"],
      ["first_comment", "Optional", "text", "Supported as a self-reply"],
    ],
    examples: [
      {
        title: "Image post",
        body: `{
  "caption": "Launching today 🚀",
  "account_ids": ["sa_twitter_1"],
  "media_urls": [
    "https://cdn.example.com/hero.jpg",
    "https://cdn.example.com/screenshot.png"
  ]
}`,
      },
      {
        title: "Video post",
        body: `{
  "caption": "Watch the demo 👇",
  "account_ids": ["sa_twitter_1"],
  "media_urls": ["https://cdn.example.com/demo.mp4"]
}`,
      },
      {
        title: "Thread with first comment",
        body: `{
  "platform_posts": [
    {
      "account_id": "sa_twitter_1",
      "caption": "1/ Here is what changed",
      "thread_position": 1
    },
    {
      "account_id": "sa_twitter_1",
      "caption": "2/ Here is why it matters",
      "thread_position": 2
    }
  ]
}`,
        note: "If you want a follow-up comment rather than a thread, send `first_comment` on a single post instead of using `thread_position`.",
      },
    ],
    errors: [
      ["caption_too_long", "Caption exceeds 280 characters"],
      ["too_many_media", "More than 4 images or unsupported media mix"],
      ["thread_unsupported", "Invalid thread shape or missing `thread_position` ordering"],
    ],
  },
  linkedin: {
    title: "LinkedIn",
    lead: "LinkedIn is where you usually want cleaner formatting, longer copy, and more explicit control over audience-facing metadata. UniPost keeps that complexity under `platform_options.linkedin` while preserving one core publish shape.",
    overview: "LinkedIn supports text-only posts, multi-image posts, video posts, and first comments. It does not behave like a Twitter-style thread platform, so longer narratives should usually be published as one longer caption.",
    capabilities: [
      ["Text posts", "Yes", "Up to 3,000 characters"],
      ["Image posts", "Yes", "Up to 9 images"],
      ["Video posts", "Yes", "Exactly 1 video"],
      ["Threads", "No", "Publish as separate posts instead"],
      ["First comment", "Yes", "Supported"],
      ["Visibility options", "Yes", "Use `platform_options.linkedin`"],
    ],
    requirements: [
      ["caption", "Optional", "3,000 chars", "Best for longer release notes and announcements"],
      ["media_urls or media_ids", "Optional", "1-9 images OR 1 video", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Do not mix images and video."],
      ["platform_options.linkedin", "Optional", "visibility", "Use for audience controls"],
      ["first_comment", "Optional", "text", "Posted after the main post lands"],
    ],
    options: [
      ["platform_options.linkedin.visibility", "anyone / connections", "Set post audience visibility"],
    ],
    examples: [
      {
        title: "Long-form text post",
        body: `{
  "caption": "We shipped a new release today with managed Connect, preview links, and post validation.",
  "account_ids": ["sa_linkedin_1"]
}`,
      },
      {
        title: "Multi-image post",
        body: `{
  "caption": "Recap of our launch event",
  "account_ids": ["sa_linkedin_1"],
  "media_urls": [
    "https://cdn.example.com/event-1.jpg",
    "https://cdn.example.com/event-2.jpg",
    "https://cdn.example.com/event-3.jpg"
  ]
}`,
      },
      {
        title: "Video post",
        body: `{
  "caption": "Customer story 🎬",
  "account_ids": ["sa_linkedin_1"],
  "media_urls": ["https://cdn.example.com/story.mp4"]
}`,
      },
    ],
    errors: [
      ["caption_too_long", "Caption exceeds 3,000 characters"],
      ["too_many_media", "More than 9 images supplied"],
      ["mixed_media_unsupported", "LinkedIn does not accept image and video in the same share"],
    ],
  },
  instagram: {
    title: "Instagram",
    lead: "Instagram is media-first. The main implementation question is not whether you can send a caption, but whether the media combination, count, and publish surface are valid for the connected business or creator account.",
    overview: "Use Instagram when you are publishing images, reels, or carousel content. Text-only posts are not supported. Mixed media is allowed only in carousel-style flows, not in simple single-asset posts.",
    capabilities: [
      ["Text-only posts", "No", "Instagram is media-first"],
      ["Single image", "Yes", "Supported"],
      ["Single video", "Yes", "Published as Reels/video"],
      ["Carousel", "Yes", "2-10 items, image and video mix allowed"],
      ["Threads", "No", "Not a Twitter-style thread platform"],
      ["First comment", "Yes", "Supported"],
    ],
    requirements: [
      ["media_urls or media_ids", "Required", "1 image, 1 video, or 2-10 carousel items", "Media is required. Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`."],
      ["caption", "Optional", "2,200 chars", "Commonly sent with media"],
      ["first_comment", "Optional", "text", "Supported after publish"],
      ["mixed media", "Allowed only in carousel", "2-10 items", "Single posts should not mix image and video"],
    ],
    examples: [
      {
        title: "Single image",
        body: `{
  "caption": "Sunset 🌅",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/sunset.jpg"]
}`,
      },
      {
        title: "Reels / single video",
        body: `{
  "caption": "30-second intro 🎬",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/intro.mp4"]
}`,
      },
      {
        title: "Carousel",
        body: `{
  "caption": "Product walkthrough",
  "account_ids": ["sa_instagram_1"],
  "media_urls": [
    "https://cdn.example.com/cover.jpg",
    "https://cdn.example.com/detail.jpg",
    "https://cdn.example.com/clip.mp4"
  ]
}`,
      },
    ],
    errors: [
      ["media_required", "Instagram requires media"],
      ["too_many_media", "More than 10 carousel items supplied"],
      ["mixed_media_unsupported", "Mixed media outside a carousel container"],
    ],
  },
  threads: {
    title: "Threads",
    lead: "Threads works well for short conversational copy and thread-like multi-post flows. UniPost treats it like a text-first platform with optional media and native thread behavior.",
    overview: "Threads supports text-only posts, single-image or single-video posts, and larger carousel-style containers. Use `thread_position` for multi-post sequencing. Do not use `first_comment` here.",
    capabilities: [
      ["Text posts", "Yes", "Up to 500 characters"],
      ["Image posts", "Yes", "Single image or carousel"],
      ["Video posts", "Yes", "Single video or carousel"],
      ["Carousel", "Yes", "2-20 items, mixed media allowed"],
      ["Threads", "Yes", "Use `thread_position`"],
      ["First comment", "No", "Use threads instead"],
    ],
    requirements: [
      ["caption", "Optional", "500 chars", "Short-form conversational copy"],
      ["media_urls or media_ids", "Optional", "single asset or 2-20 carousel", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Mixed media allowed only in carousel flow."],
      ["thread_position", "Optional", "1-indexed", "Preferred over first_comment"],
      ["first_comment", "Rejected", "n/a", "Validate will catch this before publish"],
    ],
    examples: [
      {
        title: "Text-only post",
        body: `{
  "caption": "Just shipped a new release ✨",
  "account_ids": ["sa_threads_1"]
}`,
      },
      {
        title: "Carousel",
        body: `{
  "caption": "Conference highlights",
  "account_ids": ["sa_threads_1"],
  "media_urls": [
    "https://cdn.example.com/talk-1.jpg",
    "https://cdn.example.com/talk-2.jpg",
    "https://cdn.example.com/keynote.mp4"
  ]
}`,
      },
      {
        title: "Thread",
        body: `{
  "platform_posts": [
    { "account_id": "sa_threads_1", "caption": "1/ A cleaner docs IA matters", "thread_position": 1 },
    { "account_id": "sa_threads_1", "caption": "2/ Less searching, more building", "thread_position": 2 }
  ]
}`,
      },
    ],
    errors: [
      ["caption_too_long", "Caption exceeds 500 characters"],
      ["first_comment_unsupported", "Threads should use thread_position instead"],
      ["too_many_media", "More than 20 carousel items supplied"],
    ],
  },
  tiktok: {
    title: "TikTok",
    lead: "TikTok is a video-led publishing surface with a small number of important platform-specific controls. UniPost keeps those controls under `platform_options.tiktok` while preserving a consistent top-level request shape.",
    overview: "TikTok supports a single video or a photo carousel depending on the upload mode. Text-only posts are not supported, and image/video mixing is not supported in one publish request.",
    capabilities: [
      ["Text-only posts", "No", "TikTok is media-first"],
      ["Photo carousel", "Yes", "Up to 35 images"],
      ["Video posts", "Yes", "Single video"],
      ["Threads", "No", "Not applicable"],
      ["Privacy options", "Yes", "Use `platform_options.tiktok`"],
      ["Analytics", "Partial", "Depends on connected account access"],
    ],
    requirements: [
      ["media_urls or media_ids", "Required", "1 video OR up to 35 images", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. This is the primary publish surface."],
      ["caption", "Optional", "2,200 chars", "Pair with media"],
      ["platform_options.tiktok.privacy_level", "Optional", "privacy enum", "Controls audience visibility"],
      ["platform_options.tiktok.upload_mode", "Optional", "pull_from_url / file_upload", "Use file_upload if CDN domain is not registered"],
    ],
    options: [
      ["platform_options.tiktok.privacy_level", "SELF_ONLY / PUBLIC_TO_EVERYONE / MUTUAL_FOLLOW_FRIENDS / FOLLOWER_OF_CREATOR", "Audience visibility"],
      ["platform_options.tiktok.photo_cover_index", "0-based number", "Which image becomes the carousel cover"],
    ],
    examples: [
      {
        title: "Video post",
        body: `{
  "caption": "How we built it",
  "account_ids": ["sa_tiktok_1"],
  "media_urls": ["https://cdn.example.com/build.mp4"],
  "platform_options": {
    "tiktok": {
      "privacy_level": "PUBLIC_TO_EVERYONE"
    }
  }
}`,
      },
      {
        title: "Photo carousel",
        body: `{
  "caption": "Lookbook 📸",
  "account_ids": ["sa_tiktok_1"],
  "media_urls": [
    "https://cdn.example.com/look-1.jpg",
    "https://cdn.example.com/look-2.jpg",
    "https://cdn.example.com/look-3.jpg"
  ],
  "platform_options": {
    "tiktok": {
      "privacy_level": "PUBLIC_TO_EVERYONE",
      "photo_cover_index": 0
    }
  }
}`,
      },
    ],
    errors: [
      ["media_required", "TikTok requires video or image carousel media"],
      ["mixed_media_unsupported", "Do not mix image and video in one publish body"],
      ["invalid_upload_mode", "TikTok upload mode is not recognized"],
    ],
  },
  youtube: {
    title: "YouTube",
    lead: "YouTube usually needs more metadata than short-form networks. UniPost exposes those controls in `platform_options.youtube` while keeping the publish flow consistent with the rest of the platform set.",
    overview: "YouTube is a single-video publish surface. Use it for long-form videos or Shorts. The most important platform-specific controls are privacy status, Shorts mode, category, and tags. For local video files, the most reliable UniPost workflow is to upload into the media library first and then publish with `media_ids`.",
    capabilities: [
      ["Video posts", "Yes", "Exactly 1 video"],
      ["Shorts", "Yes", "Use `platform_options.youtube.shorts`"],
      ["Scheduling", "Yes", "Use `scheduled_at`"],
      ["Text-only posts", "No", "Video-first platform"],
      ["Image posts", "No", "Not a native publish target"],
      ["Analytics", "Yes", "Supported"],
    ],
    requirements: [
      ["media_urls or media_ids", "Required", "Exactly 1 video", "Prefer `media_ids` when starting from a local file. Create the media ID with `POST /v1/media`, upload the file to the returned `upload_url`, then publish."],
      ["caption", "Optional", "5,000 chars", "Used as title/description body context"],
      ["platform_options.youtube.privacy_status", "Optional", "private / public / unlisted", "Default is often private"],
      ["platform_options.youtube.shorts", "Optional", "boolean", "Routes the upload toward Shorts behavior"],
    ],
    options: [
      ["platform_options.youtube.category_id", "string", "YouTube category ID"],
      ["platform_options.youtube.tags", "string[]", "Tag list for snippet metadata"],
    ],
    examples: [
      {
        title: "Long-form video from a hosted URL",
        body: `{
  "caption": "Quarterly product update",
  "account_ids": ["sa_youtube_1"],
  "media_urls": ["https://cdn.example.com/update.mp4"],
  "platform_options": {
    "youtube": {
      "privacy_status": "public",
      "category_id": "22",
      "tags": ["product", "quarterly", "update"]
    }
        }
}`,
      },
      {
        title: "Long-form video from UniPost media library",
        body: `{
  "caption": "Quarterly product update",
  "account_ids": ["sa_youtube_1"],
  "media_ids": ["med_uploaded_video_1"],
  "platform_options": {
    "youtube": {
      "privacy_status": "public",
      "category_id": "22",
      "tags": ["product", "quarterly", "update"]
    }
  }
}`,
        note: "`med_uploaded_video_1` is a placeholder for the media ID returned by `POST /v1/media` after you reserve the upload and PUT the video bytes to UniPost storage. See the Media API reference for the upload step before calling `POST /v1/social-posts`.",
      },
      {
        title: "Shorts",
        body: `{
  "caption": "30s feature demo",
  "account_ids": ["sa_youtube_1"],
  "media_urls": ["https://cdn.example.com/demo-vertical.mp4"],
  "platform_options": {
    "youtube": {
      "privacy_status": "public",
      "shorts": true
    }
  }
}`,
      },
    ],
    errors: [
      ["media_required", "YouTube requires exactly one video"],
      ["too_many_media", "More than one media asset supplied"],
      ["invalid_privacy_status", "YouTube privacy value is not recognized"],
    ],
  },
};

export default async function PlatformDetailPage({
  params,
}: {
  params: Promise<{ platform: string }>;
}) {
  const { platform } = await params;
  const data = PLATFORMS[platform];
  if (!data) notFound();
  const supportsManagedUploads = data.requirements.some((row) => row[0].includes("media_urls") || row[0].includes("media_ids"));

  return (
    <DocsPage eyebrow="Platform Guide" title={data.title} lead={data.lead}>
      <h2 id="overview">Overview</h2>
      <p>{data.overview}</p>

      <h2 id="capabilities">Supported capabilities</h2>
      <DocsTable columns={["Capability", "Supported", "Notes"]} rows={data.capabilities} />

      <h2 id="requirements">Requirements</h2>
      <DocsTable columns={["Field", "Required", "Limits", "Notes"]} rows={data.requirements} />

      {supportsManagedUploads ? (
        <>
          <h2 id="local-files">Hosted URLs vs Local Files</h2>
          <p>
            Every media-capable platform page here supports the same two input shapes. If your asset is already reachable on the public internet, send it in <code>media_urls</code>. If you are starting from a local image or video file, first call <ApiInlineLink endpoint="POST /v1/media" />, upload the bytes to the returned <code>upload_url</code>, and then publish with <code>media_ids</code>.
          </p>
          <p>
            A placeholder like <code>med_uploaded_video_1</code> or <code>med_uploaded_image_1</code> means “the media ID returned by the Media API after the upload was reserved.” The full upload flow is documented in <a href="/docs/api/media">Media API</a> and <a href="/docs/api/posts/create">Create Post</a>.
          </p>
        </>
      ) : null}

      {data.options ? (
        <>
          <h2 id="options">Platform-specific options</h2>
          <DocsTable columns={["Option", "Values", "Notes"]} rows={data.options} />
        </>
      ) : null}

      <h2 id="examples">Example requests</h2>
      <p>The examples below are intentionally small and copyable. They show the request body only, assuming a standard <ApiInlineLink endpoint="POST /v1/social-posts" /> call with Bearer auth.</p>
          {data.examples.map((example) => (
        <div key={example.title}>
          <h3 id={example.title.toLowerCase().replace(/[^a-z0-9]+/g, "-")}>{example.title}</h3>
          {example.note ? <p><DocsRichText text={example.note} /></p> : null}
          <DocsCode code={example.body} language="json" />
        </div>
      ))}

      <h2 id="common-errors">Common validation errors</h2>
      <DocsTable columns={["Code", "What it means"]} rows={data.errors} />

      <h2 id="related-reference">Related reference</h2>
      <p>Once you know the correct request shape for this platform, move to the API reference for the full endpoint contract, response schema, and validation payload shape.</p>
    </DocsPage>
  );
}
