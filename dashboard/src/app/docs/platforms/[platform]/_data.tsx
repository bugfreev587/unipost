import type { ReactNode } from "react";

export type SetupMode = "quickstart" | "whitelabel" | "native" | "credentials";

export type PlatformSummary = {
  publishing: "full" | "limited" | "none";
  scheduling: "full" | "limited" | "none";
  analytics: "full" | "limited" | "none";
  inbox: "full" | "limited" | "none";
  connection: string;
};

export type PlatformDoc = {
  title: string;
  brandColor: string;
  icon: ReactNode;
  tagline: string;
  lead: string;
  badges: readonly string[];
  summary: PlatformSummary;
  capabilities: readonly (readonly string[])[];
  requirements: readonly (readonly string[])[];
  options?: readonly (readonly string[])[];
  analytics: readonly (readonly string[])[];
  inbox?: {
    note?: string;
    rows: readonly (readonly string[])[];
  };
  setup: readonly (readonly ReactNode[])[];
  examples: ReadonlyArray<{ title: string; body: string; note?: string }>;
  errors: readonly (readonly string[])[];
  limitations: readonly (readonly string[])[];
};

// Platform brand glyphs — kept inline so the docs bundle does not depend on
// an icon font. Colors match each platform's public brand color; we do not
// resize these, we just drop them into the hero at 28px.
const icons = {
  twitter: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
    </svg>
  ),
  linkedin: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433a2.062 2.062 0 01-2.063-2.065 2.064 2.064 0 112.063 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z" />
    </svg>
  ),
  instagram: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 100 12.324 6.162 6.162 0 000-12.324zM12 16a4 4 0 110-8 4 4 0 010 8zm6.406-11.845a1.44 1.44 0 100 2.881 1.44 1.44 0 000-2.881z" />
    </svg>
  ),
  threads: (
    <svg viewBox="0 0 192 192" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M141.537 88.988a66.667 66.667 0 0 0-2.518-1.143c-1.482-27.307-16.403-42.94-41.457-43.1h-.34c-14.986 0-27.449 6.396-35.12 18.036l13.779 9.452c5.73-8.695 14.724-10.548 21.348-10.548h.229c8.249.053 14.474 2.452 18.503 7.129 2.932 3.405 4.893 8.111 5.864 14.05-7.314-1.243-15.224-1.626-23.68-1.14-23.82 1.371-39.134 15.326-38.092 34.7.528 9.818 5.235 18.28 13.256 23.808 6.768 4.666 15.471 6.98 24.49 6.52 11.918-.607 21.27-5.003 27.79-13.066 4.947-6.116 8.1-13.908 9.532-23.619 5.708 3.45 9.953 8.063 12.37 13.676 4.106 9.533 4.349 25.194-7.865 37.315-10.724 10.64-23.618 15.254-38.399 15.358-16.388-.115-28.796-5.382-36.877-15.66-7.515-9.56-11.416-23.12-11.594-40.322.178-17.202 4.079-30.762 11.594-40.322 8.081-10.278 20.489-15.545 36.877-15.66 16.506.116 29.148 5.42 37.567 15.76 4.108 5.048 7.21 11.467 9.312 19.023l14.854-3.982c-2.605-9.463-6.641-17.573-12.159-24.356C152.088 14.14 136.308 7.353 116.379 7.2h-.069c-19.874.142-35.468 6.947-46.333 20.25C60.4 39.452 55.545 55.77 55.33 75.94l-.002.162.002.16c.215 20.17 5.07 36.488 14.645 48.49 10.865 13.303 26.459 20.108 46.333 20.25h.069c18.134-.119 33.577-5.86 45.916-17.068 16.456-14.938 17.617-36.986 12.28-49.39-3.835-8.908-11.151-16.063-21.036-20.544zm-36.844 51.014c-9.985.508-20.361-3.928-21.025-13.278-.477-6.732 4.746-14.243 24.298-15.368 2.132-.123 4.22-.183 6.263-.183 6.26 0 12.12.616 17.39 1.812-1.98 22.459-14.948 26.513-26.926 27.017z" />
    </svg>
  ),
  tiktok: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M19.59 6.69a4.83 4.83 0 01-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 01-2.88 2.5 2.89 2.89 0 01-2.89-2.89 2.89 2.89 0 012.89-2.89c.28 0 .54.04.79.1v-3.5a6.37 6.37 0 00-.79-.05A6.34 6.34 0 003.15 15.2a6.34 6.34 0 0010.86 4.48 6.3 6.3 0 001.86-4.48V8.73a8.26 8.26 0 004.84 1.56V6.84a4.85 4.85 0 01-1.12-.15z" />
    </svg>
  ),
  youtube: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M23.498 6.186a3.016 3.016 0 00-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 00.502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 002.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 002.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z" />
    </svg>
  ),
  bluesky: (
    <svg viewBox="0 0 600 530" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M135.7 44.3C202.3 94.8 273.6 197.2 300 249.6c26.4-52.4 97.7-154.8 164.3-205.3C520.4 1.5 588 -22.1 588 68.2c0 18 -10.4 151.2-16.5 172.8-21.2 75-98.6 94.1-167.9 82.6 121.1 20.7 151.8 89.2 85.3 157.8C390.5 584.2 310.2 500 300 481.4c-10.2 18.6-90.5 102.8-188.9 0C44.6 413.8 75.3 345.3 196.4 324.6c-69.3 11.5-146.7-7.6-167.9-82.6C22.4 220.4 12 87.2 12 69.2c0-90.3 67.6-66.7 123.7-24.9z" />
    </svg>
  ),
  facebook: (
    <svg viewBox="0 0 24 24" width="26" height="26" fill="currentColor" aria-hidden>
      <path d="M24 12.073c0-6.627-5.373-12-12-12s-12 5.373-12 12c0 5.99 4.388 10.954 10.125 11.854v-8.385H7.078v-3.47h3.047V9.43c0-3.007 1.792-4.669 4.533-4.669 1.312 0 2.686.235 2.686.235v2.953H15.83c-1.491 0-1.956.925-1.956 1.874v2.25h3.328l-.532 3.47h-2.796v8.385C19.612 23.027 24 18.062 24 12.073z" />
    </svg>
  ),
};

const yes = "Yes" as const;
const no = "No" as const;
const partial = "Partial" as const;

// Connection-mode cell snippets reused across platforms.
const modeQuickstart = ["Quickstart", "Fast setup — UniPost handles OAuth", "UniPost-managed app", "Free / paid quota"] as const;
const modeWhitelabel = ["White-label", "Your customers connect their own accounts", "Your OAuth app", "Paid plans only"] as const;
const modeBlueskyCreds = ["Credentials", "Paste handle + app password (no OAuth)", "Bluesky app password", "Free"] as const;
const modeTwitterNative = ["Native credentials", "Upload X API keys per account", "Your X dev tier", "Requires X paid tier"] as const;

export const PLATFORMS: Record<string, PlatformDoc> = {
  twitter: {
    title: "Twitter / X",
    brandColor: "#0f172a",
    icon: icons.twitter,
    tagline: "Short-form text, media, threads, and first-comment replies.",
    lead: "Publish, schedule, and thread posts on X from UniPost. Strongest support today for text, media, and reply chains.",
    badges: ["Publishing", "Scheduling", "Analytics", "Threads", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "full",
      inbox: "none",
      connection: "Native credentials today — requires your X paid tier",
    },
    capabilities: [
      ["Text posts", yes, "Up to 280 characters"],
      ["Image posts", yes, "Up to 4 images"],
      ["Video posts", yes, "Exactly 1 video"],
      ["GIF posts", yes, "Exactly 1 GIF"],
      ["Threads", yes, "Use `thread_position`"],
      ["First comment", yes, "Posted as a reply after publish"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Inbox / DMs", no, "Not part of the UniPost inbox today"],
    ],
    requirements: [
      ["caption", "Optional", "280 chars", "Required unless media-only flow is valid"],
      ["media_urls or media_ids", "Optional", "1-4 images OR 1 video OR 1 GIF", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Do not mix media types."],
      ["thread_position", "Optional", "1-indexed", "Use on each thread entry"],
      ["first_comment", "Optional", "text", "Supported as a self-reply"],
    ],
    analytics: [
      ["Impressions", yes, "Supported"],
      ["Likes", yes, "Supported"],
      ["Comments / replies", yes, "Supported"],
      ["Shares / reposts", yes, "Supported"],
      ["Reach", no, "Not exposed by the X API"],
      ["Saves / bookmarks", no, "Not exposed by the X API"],
      ["Video views", no, "Not exposed for org accounts today"],
    ],
    setup: [
      modeTwitterNative,
      ["Quickstart", "Not available — X removed the shared app path", "—", "—"],
      modeWhitelabel,
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
    { "account_id": "sa_twitter_1", "caption": "1/ Here is what changed", "thread_position": 1 },
    { "account_id": "sa_twitter_1", "caption": "2/ Here is why it matters", "thread_position": 2 }
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
    limitations: [
      ["No shared Quickstart app", "X removed the managed developer path — every workspace connects with its own keys"],
      ["Inbox is not supported", "UniPost inbox covers Meta and Threads today"],
      ["Rate limits follow your X tier", "Free tier is not enough for production publish volume"],
    ],
  },

  bluesky: {
    title: "Bluesky",
    brandColor: "#0085ff",
    icon: icons.bluesky,
    tagline: "Short-form text with images, video, and native threads.",
    lead: "One of the cleanest fits for UniPost: short-form text, image and video support, and multi-post threading all map cleanly into the standard publish model.",
    badges: ["Publishing", "Scheduling", "Threads", "Free to connect"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "limited",
      inbox: "none",
      connection: "Handle + app password — no OAuth",
    },
    capabilities: [
      ["Text posts", yes, "Up to 300 graphemes"],
      ["Image posts", yes, "Up to 4 images"],
      ["Video posts", yes, "Exactly 1 video"],
      ["Threads", yes, "Use `thread_position`"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Text-only posts", yes, "Media is optional"],
      ["First comment", no, "Use threads instead"],
      ["Inbox / DMs", no, "Not part of the UniPost inbox today"],
    ],
    requirements: [
      ["caption", "Optional", "300 graphemes", "Validate catches overages before publish"],
      ["media_urls or media_ids", "Optional", "1-4 images OR 1 video", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Do not mix image and video."],
      ["thread_position", "Optional", "1-indexed", "Preferred way to create multi-post flows"],
      ["first_comment", "Rejected", "n/a", "Use thread_position instead"],
    ],
    analytics: [
      ["Likes", yes, "Supported"],
      ["Comments / replies", yes, "Supported"],
      ["Shares / reposts", yes, "Supported"],
      ["Impressions", no, "Not exposed by Bluesky"],
      ["Reach", no, "Not exposed by Bluesky"],
      ["Saves", no, "Not exposed by Bluesky"],
      ["Video views", no, "Not exposed by Bluesky"],
    ],
    setup: [
      ["Quickstart", "Paste Bluesky handle + app password — no OAuth", "None required", "Free"],
      modeBlueskyCreds,
      ["White-label", "Not applicable — Bluesky does not use OAuth apps", "—", "—"],
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
    limitations: [
      ["No OAuth", "Authentication is via app password — generate one in Bluesky settings"],
      ["Engagement only in analytics", "Bluesky API does not expose impressions, reach, or views"],
      ["No inbox surface", "Replies are not synced into UniPost inbox today"],
    ],
  },

  linkedin: {
    title: "LinkedIn",
    brandColor: "#0a66c2",
    icon: icons.linkedin,
    tagline: "Long-form text, multi-image, and first-comment posts with audience controls.",
    lead: "LinkedIn is where you usually want cleaner formatting, longer copy, and explicit audience controls. UniPost keeps that complexity under `platform_options.linkedin` while preserving one core publish shape.",
    badges: ["Publishing", "Scheduling", "Analytics", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "full",
      inbox: "none",
      connection: "OAuth — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Text posts", yes, "Up to 3,000 characters"],
      ["Image posts", yes, "Up to 9 images"],
      ["Video posts", yes, "Exactly 1 video"],
      ["First comment", yes, "Supported"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Visibility options", yes, "Use `platform_options.linkedin`"],
      ["Threads", no, "Publish as separate posts instead"],
      ["Inbox / DMs", no, "Not part of the UniPost inbox today"],
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
    analytics: [
      ["Impressions", yes, "Supported"],
      ["Reach", yes, "Supported"],
      ["Likes", yes, "Supported"],
      ["Comments", yes, "Supported"],
      ["Shares", yes, "Supported"],
      ["Clicks", yes, "Supported"],
      ["Saves", no, "Not exposed by LinkedIn"],
      ["Video views", no, "Not exposed per-post today"],
    ],
    setup: [
      modeQuickstart,
      modeWhitelabel,
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
    limitations: [
      ["No thread-style posting", "LinkedIn is a single-post surface — use longer copy instead"],
      ["Mixed media not allowed", "Either images OR video, never both in one share"],
      ["No inbox surface", "DMs and comment moderation are not in UniPost inbox today"],
    ],
  },

  instagram: {
    title: "Instagram",
    brandColor: "#e1306c",
    icon: icons.instagram,
    tagline: "Feed, Reels, Stories, and carousels for connected business or creator accounts.",
    lead: "Instagram is media-first. The main question is not whether you can send a caption, but whether the media combination, count, and publish surface are valid for the connected business or creator account.",
    badges: ["Publishing", "Scheduling", "Analytics", "Inbox", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "full",
      inbox: "full",
      connection: "OAuth via Meta — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Feed posts", yes, "Single image, single video, or 2-10 carousel"],
      ["Reels", yes, "Exactly 1 video"],
      ["Stories", yes, "Exactly 1 image or video"],
      ["Carousel", yes, "2-10 items, image + video mix allowed"],
      ["First comment", yes, "Supported"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Inbox (comments + DMs)", yes, "Routed into UniPost inbox"],
      ["Text-only posts", no, "Instagram is media-first"],
      ["Threads", no, "Not a Twitter-style thread platform"],
    ],
    requirements: [
      ["media_urls or media_ids", "Required", "1 image, 1 video, or 2-10 carousel items", "Media is required. Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`."],
      ["caption", "Optional", "2,200 chars", "Commonly sent with media"],
      ["platform_options.instagram.mediaType", "Optional", "feed / reels / story", "Defaults to `feed`. Use it to force Reels or Story behavior and trigger Instagram-specific preflight validation."],
      ["first_comment", "Optional", "text", "Supported after publish"],
      ["reels", "Exactly 1 video", "Required", "Reels do not accept images or carousels"],
      ["story", "Exactly 1 media item", "Required", "Stories accept one image or one video"],
    ],
    options: [
      ["platform_options.instagram.mediaType", "feed / reels / story", "Selects which Instagram publish surface UniPost should target."],
    ],
    analytics: [
      ["Reach", yes, "Supported"],
      ["Likes", yes, "Supported"],
      ["Comments", yes, "Supported"],
      ["Shares", yes, "Supported"],
      ["Saves", yes, "Supported"],
      ["Impressions", no, "Removed from Graph API v22 (April 2024)"],
      ["Clicks", no, "Not exposed per-post"],
    ],
    inbox: {
      note: "Instagram routes comments and DMs into UniPost inbox once the connected account is a Business or Creator account linked to a Facebook Page.",
      rows: [
        ["Comments on feed / Reels", yes, "Source `ig_comment`"],
        ["Direct messages (DMs)", yes, "Source `ig_dm`"],
        ["Story replies", no, "Not supported by Graph webhook today"],
        ["Reply from UniPost", yes, "One reply per item supported"],
      ],
    },
    setup: [
      modeQuickstart,
      modeWhitelabel,
      ["Requirement", "Connected IG must be Business or Creator linked to a Facebook Page", "Meta app review required for public use", "—"],
    ],
    examples: [
      {
        title: "Single image",
        body: `{
  "caption": "Sunset 🌅",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/sunset.jpg"],
  "platform_options": {
    "instagram": { "mediaType": "feed" }
  }
}`,
      },
      {
        title: "Reel",
        body: `{
  "caption": "30-second intro 🎬",
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/intro.mp4"],
  "platform_options": {
    "instagram": { "mediaType": "reels" }
  }
}`,
      },
      {
        title: "Story",
        body: `{
  "account_ids": ["sa_instagram_1"],
  "media_urls": ["https://cdn.example.com/story.jpg"],
  "platform_options": {
    "instagram": { "mediaType": "story" }
  }
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
      ["max_images_exceeded / max_videos_exceeded", "More than 10 carousel items or more than 1 video supplied"],
      ["invalid_instagram_media_type", "Instagram mediaType must be `feed`, `reels`, or `story`"],
      ["instagram_reels_require_video", "Reels require exactly one video"],
      ["instagram_story_single_media_only", "Stories require exactly one image or video"],
      ["mixed_media_unsupported", "Mixed media outside a valid Instagram carousel flow"],
    ],
    limitations: [
      ["Text-only posts not supported", "Instagram requires media on every post"],
      ["Impressions removed", "Graph API v22 no longer exposes impressions for image or carousel"],
      ["Requires Business or Creator account", "Personal IG accounts cannot publish via the Graph API"],
    ],
  },

  threads: {
    title: "Threads",
    brandColor: "#000000",
    icon: icons.threads,
    tagline: "Short-form text, media, and native threaded conversations.",
    lead: "Threads works well for short conversational copy and thread-like multi-post flows. UniPost treats it like a text-first platform with optional media and native thread behavior.",
    badges: ["Publishing", "Scheduling", "Analytics", "Inbox", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "full",
      inbox: "limited",
      connection: "OAuth via Meta — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Text posts", yes, "Up to 500 characters"],
      ["Image posts", yes, "Single image or carousel"],
      ["Video posts", yes, "Single video or carousel"],
      ["Carousel", yes, "2-20 items, mixed media allowed"],
      ["Threads", yes, "Use `thread_position`"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Inbox (replies)", yes, "Routed as `threads_reply`"],
      ["First comment", no, "Use threads instead"],
    ],
    requirements: [
      ["caption", "Optional", "500 chars", "Short-form conversational copy"],
      ["media_urls or media_ids", "Optional", "single asset or 2-20 carousel", "Use `media_urls` for hosted assets or `media_ids` for local files uploaded via `POST /v1/media`. Mixed media allowed only in carousel flow."],
      ["thread_position", "Optional", "1-indexed", "Preferred over first_comment"],
      ["first_comment", "Rejected", "n/a", "Validate will catch this before publish"],
    ],
    analytics: [
      ["Impressions", yes, "Supported"],
      ["Likes", yes, "Supported"],
      ["Comments / replies", yes, "Supported"],
      ["Shares / reposts", yes, "Supported"],
      ["Reach", no, "Not exposed per-post"],
      ["Saves", no, "Not exposed by Threads"],
      ["Video views", no, "Not exposed per-post today"],
    ],
    inbox: {
      note: "Threads surfaces replies to your posts. DMs are not part of the Threads API today.",
      rows: [
        ["Replies to your posts", yes, "Source `threads_reply`"],
        ["Direct messages (DMs)", no, "Not supported by the Threads API"],
        ["Reply from UniPost", yes, "One reply per item supported"],
      ],
    },
    setup: [
      modeQuickstart,
      modeWhitelabel,
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
    limitations: [
      ["No first-comment flow", "Use `thread_position` for follow-up content"],
      ["No DMs", "Threads API does not expose direct messages today"],
    ],
  },

  tiktok: {
    title: "TikTok",
    brandColor: "#000000",
    icon: icons.tiktok,
    tagline: "Single video or photo carousel publishing with privacy controls.",
    lead: "TikTok is a video-led publishing surface with a small number of important platform-specific controls. UniPost keeps those under `platform_options.tiktok` while preserving a consistent top-level request shape.",
    badges: ["Publishing", "Scheduling", "Analytics", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "limited",
      inbox: "none",
      connection: "OAuth — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Video posts", yes, "Single video"],
      ["Photo carousel", yes, "Up to 35 images"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Privacy options", yes, "Use `platform_options.tiktok`"],
      ["Analytics", partial, "Engagement + view count; reach/impressions not exposed"],
      ["Text-only posts", no, "TikTok is media-first"],
      ["Threads", no, "Not applicable"],
      ["Inbox / comments", no, "Not part of the UniPost inbox today"],
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
    analytics: [
      ["Video views", yes, "TikTok `view_count` (video plays)"],
      ["Likes", yes, "Supported"],
      ["Comments", yes, "Supported"],
      ["Shares", yes, "Supported"],
      ["Impressions", no, "TikTok exposes views, not display impressions"],
      ["Reach", no, "Not exposed by TikTok"],
      ["Saves", no, "Not exposed by TikTok"],
    ],
    setup: [
      modeQuickstart,
      modeWhitelabel,
      ["Requirement", "TikTok app must pass audit for public use", "Sandbox apps limit posting to allowlisted accounts", "—"],
    ],
    examples: [
      {
        title: "Video post",
        body: `{
  "caption": "How we built it",
  "account_ids": ["sa_tiktok_1"],
  "media_urls": ["https://cdn.example.com/build.mp4"],
  "platform_options": {
    "tiktok": { "privacy_level": "PUBLIC_TO_EVERYONE" }
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
    limitations: [
      ["Audit required for public use", "TikTok Content Posting API apps must pass audit before posting to non-allowlisted accounts"],
      ["No text-only posts", "TikTok is strictly media-led"],
      ["Inbox is not supported", "Comments and DMs are not in UniPost inbox today"],
    ],
  },

  youtube: {
    title: "YouTube",
    brandColor: "#ff0000",
    icon: icons.youtube,
    tagline: "Single-video publishing with rich metadata, Shorts, and playlist insertion.",
    lead: "YouTube usually needs more metadata than short-form networks. UniPost exposes those controls in `platform_options.youtube` while keeping the publish flow consistent with the rest of the platform set.",
    badges: ["Publishing", "Scheduling", "Analytics", "Shorts", "Playlists", "White-label"],
    summary: {
      publishing: "full",
      scheduling: "full",
      analytics: "limited",
      inbox: "none",
      connection: "OAuth via Google — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Video posts", yes, "Exactly 1 video"],
      ["Shorts", yes, "Use `platform_options.youtube.shorts`"],
      ["Scheduling", yes, "Use `scheduled_at` or `platform_options.youtube.publish_at`"],
      ["Playlist insertion", yes, "Use `platform_options.youtube.playlist_id`"],
      ["Analytics", partial, "Likes, comments, and view count today"],
      ["Text-only posts", no, "Video-first platform"],
      ["Image posts", no, "Not a native publish target"],
      ["Inbox / comments", no, "Not part of the UniPost inbox today"],
    ],
    requirements: [
      ["media_urls or media_ids", "Required", "Exactly 1 video", "Prefer `media_ids` when starting from a local file. Create the media ID with `POST /v1/media`, upload the file to the returned `upload_url`, then publish."],
      ["caption", "Optional", "5,000 chars", "Used as YouTube description text."],
      ["platform_options.youtube.title", "Required", "max 100 chars", "Required video title. This does not fall back to `caption`."],
      ["platform_options.youtube.made_for_kids", "Required", "boolean", "Explicit audience selection required before publish."],
      ["platform_options.youtube.privacy_status", "Optional", "private / public / unlisted", "Dashboard defaults to `public`, but YouTube may still force private for unverified API projects."],
      ["platform_options.youtube.shorts", "Optional", "boolean", "Routes the upload toward Shorts behavior"],
    ],
    options: [
      ["platform_options.youtube.category_id", "string", "YouTube category ID"],
      ["platform_options.youtube.tags", "string[]", "Tag list for snippet metadata"],
      ["platform_options.youtube.default_language", "string", "BCP-47 style language tag such as `en` or `en-US`."],
      ["platform_options.youtube.recording_date", "string", "Recording date as `YYYY-MM-DD` or RFC3339 datetime."],
      ["platform_options.youtube.publish_at", "string", "RFC3339 datetime. Requires `privacy_status: private`."],
      ["platform_options.youtube.notify_subscribers", "boolean", "Defaults to `true` when omitted."],
      ["platform_options.youtube.embeddable", "boolean", "Whether the video can be embedded off YouTube."],
      ["platform_options.youtube.license", "youtube / creativeCommon", "YouTube license selection."],
      ["platform_options.youtube.public_stats_viewable", "boolean", "Controls extended public stats visibility."],
      ["platform_options.youtube.contains_synthetic_media", "boolean", "Disclosure flag for realistic altered/synthetic media."],
      ["platform_options.youtube.playlist_id", "string", "If set, UniPost calls `playlistItems.insert` after the upload succeeds."],
    ],
    analytics: [
      ["Video views", yes, "Supported"],
      ["Likes", yes, "Supported"],
      ["Comments", yes, "Supported"],
      ["Impressions", no, "YouTube Data API does not expose impressions per video"],
      ["Reach", no, "Not exposed by YouTube Data API"],
      ["Shares", no, "Not exposed by YouTube Data API"],
      ["Saves", no, "Not exposed by YouTube Data API"],
    ],
    setup: [
      modeQuickstart,
      modeWhitelabel,
      ["Verification", "Unverified Google projects cap uploads and may force `private`", "Complete YouTube API verification for public uploads", "—"],
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
      "title": "Quarterly product update",
      "made_for_kids": false,
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
      "title": "Quarterly product update",
      "made_for_kids": false,
      "privacy_status": "public"
    }
  }
}`,
        note: "`med_uploaded_video_1` is a placeholder for the media ID returned by `POST /v1/media` after you reserve the upload and PUT the video bytes to UniPost storage.",
      },
      {
        title: "Scheduled private upload with playlist insertion",
        body: `{
  "caption": "Weekly briefing",
  "account_ids": ["sa_youtube_1"],
  "media_urls": ["https://cdn.example.com/weekly-briefing.mp4"],
  "platform_options": {
    "youtube": {
      "title": "Weekly briefing",
      "made_for_kids": false,
      "privacy_status": "private",
      "publish_at": "2026-05-01T09:00:00Z",
      "playlist_id": "PL1234567890"
    }
  }
}`,
      },
    ],
    errors: [
      ["media_required", "YouTube requires exactly one video"],
      ["too_many_media", "More than one media asset supplied"],
      ["invalid_privacy_status", "YouTube privacy value is not recognized"],
      ["youtube_made_for_kids_required", "YouTube requires an explicit made_for_kids value"],
      ["youtube_publish_at_requires_private", "YouTube `publish_at` only works with private visibility"],
      ["invalid_license", "YouTube license value is not recognized"],
    ],
    limitations: [
      ["Title is required and does not fall back to caption", "`platform_options.youtube.title` must be set explicitly"],
      ["`made_for_kids` must be explicit", "YouTube refuses uploads without an audience choice"],
      ["No inbox surface", "Comment moderation is not in UniPost inbox today"],
    ],
  },

  facebook: {
    title: "Facebook Page",
    brandColor: "#1877f2",
    icon: icons.facebook,
    tagline: "Page-owned posting with photo or video, plus inbox comments and DMs.",
    lead: "Facebook integrates at the Page level. Connect the Meta OAuth flow once and pick which Pages to link; each Page becomes its own UniPost account. Publishing is currently one photo or one video per post.",
    badges: ["Publishing", "Reels", "Scheduling", "Inbox", "White-label", "Beta"],
    summary: {
      publishing: "limited",
      scheduling: "full",
      analytics: "limited",
      inbox: "full",
      connection: "OAuth via Meta — Quickstart and White-label both supported",
    },
    capabilities: [
      ["Text posts", yes, "Up to 63,206 characters"],
      ["Image posts", partial, "Exactly 1 image (v1 scope — no carousel yet)"],
      ["Video posts", partial, "Exactly 1 video (non-resumable, ≤ 1 GB in v1)"],
      ["Reels", yes, "Vertical video via `platform_options.facebook.mediaType=\"reel\"` (requires `FEATURE_FACEBOOK_REELS`)"],
      ["Link posts", yes, "Provide the URL in caption"],
      ["Scheduling", yes, "Use `scheduled_at`"],
      ["Inbox (comments + DMs)", yes, "Routed into UniPost inbox"],
      ["Carousel / album", no, "v1 does not support multi-item Page posts"],
      ["First comment", no, "Not supported on Pages today"],
      ["Threads", no, "Not applicable"],
    ],
    requirements: [
      ["caption or media", "At least one required", "Text up to 63,206 chars", "UniPost rejects posts with neither text nor media"],
      ["media_urls or media_ids", "Optional", "1 image OR 1 video", "No mixed media. Use `media_urls` for hosted assets or `media_ids` for uploaded files"],
      ["link", "Optional", "URL in caption", "Link and media cannot be combined in the same post"],
      ["platform_options.mediaType", "Optional", "feed / reel", "`feed` (default) routes to `/{page_id}/videos`; `reel` routes to the 3-phase `/{page_id}/video_reels` flow"],
      ["platform_options.title", "Optional", "Reels only", "Video title surfaced alongside the Reel"],
      ["platform_options.thumb_offset_ms", "Optional", "0–60,000 ms", "Reels only — which frame Meta picks as the thumbnail"],
    ],
    analytics: [
      ["Likes", partial, "Roadmapped — Phase 2"],
      ["Comments", partial, "Roadmapped — Phase 2"],
      ["Impressions", partial, "Roadmapped — Phase 2"],
      ["Reach", partial, "Roadmapped — Phase 2"],
      ["Shares", no, "Not yet wired through analytics"],
    ],
    inbox: {
      note: "Once the Page is connected, comments on Page posts and Messenger DMs are routed into UniPost inbox.",
      rows: [
        ["Comments on Page posts", yes, "Source `fb_comment`"],
        ["Messenger DMs", yes, "Source `fb_dm`"],
        ["Reply from UniPost", yes, "One reply per item supported"],
      ],
    },
    setup: [
      modeQuickstart,
      modeWhitelabel,
      ["Requirement", "Connected account must be a Facebook Page you manage", "Meta app review required for public use", "—"],
    ],
    examples: [
      {
        title: "Text-only post",
        body: `{
  "caption": "Shipping a product update today — more details inside.",
  "account_ids": ["sa_facebook_1"]
}`,
      },
      {
        title: "Photo post",
        body: `{
  "caption": "Launch day 🎉",
  "account_ids": ["sa_facebook_1"],
  "media_urls": ["https://cdn.example.com/launch.jpg"]
}`,
      },
      {
        title: "Video post (Feed)",
        body: `{
  "caption": "Highlights from the conference",
  "account_ids": ["sa_facebook_1"],
  "media_urls": ["https://cdn.example.com/highlights.mp4"]
}`,
      },
      {
        title: "Reel",
        body: `{
  "caption": "Teaser for tomorrow's drop.",
  "account_ids": ["sa_facebook_1"],
  "media_urls": ["https://cdn.example.com/reel-vertical.mp4"],
  "platform_options": {
    "facebook": {
      "mediaType": "reel",
      "title": "Launch teaser",
      "thumb_offset_ms": 2500
    }
  }
}`,
        note: "Reels run through `/{page_id}/video_reels` (3 phases: start → transfer → finish). Vertical video required; link attachments are not supported. Requires `FEATURE_FACEBOOK_REELS=true` on the API.",
      },
    ],
    errors: [
      ["post_body_required", "Post must include text, link, or media"],
      ["mixed_media_unsupported", "Facebook v1 accepts one photo or one video per post"],
      ["link_with_media_unsupported", "Link and media cannot be combined in the same post"],
      ["invalid_facebook_media_type", "`mediaType` must be `feed` or `reel`"],
      ["facebook_reels_unsupported", "Reels publishing requires `FEATURE_FACEBOOK_REELS` to be enabled"],
    ],
    limitations: [
      ["No carousels in v1", "Facebook `batch_publish` is on the roadmap"],
      ["No resumable uploads yet", "Videos must be ≤ 1 GB until Phase 2.5"],
      ["Analytics ship in Phase 2", "Engagement and reach metrics are roadmapped — not yet surfaced"],
      ["Reels are feature-flagged", "Set `FEATURE_FACEBOOK_REELS=true` to enable the `/video_reels` publish path"],
    ],
  },
};

export function platformSlugs(): string[] {
  return Object.keys(PLATFORMS);
}
