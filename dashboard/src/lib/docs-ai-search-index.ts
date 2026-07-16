import { PLATFORM_METRICS } from "@/lib/platform-capabilities";

export type DocsAiConfidence = "high" | "medium" | "low" | "none";
export type DocsAiIntent = "analytics" | "auth" | "connect" | "credentials" | "posting" | "reference" | "unknown";
export type DocsAiProductArea =
  | "accounts"
  | "analytics"
  | "auth"
  | "billing"
  | "connect"
  | "credentials"
  | "inbox"
  | "platforms"
  | "posting"
  | "publishing"
  | "resources";

export type DocsAiChunk = {
  id: string;
  title: string;
  path: string;
  section_id: string;
  primary_nav: "Guides" | "API Reference" | "Platforms" | "Resources" | "Overview";
  section_title: string;
  content: string;
  product_area: DocsAiProductArea;
  tags: string[];
  intent_tags: DocsAiIntent[];
  endpoint_aliases: string[];
  platforms: string[];
  last_indexed_at: string;
};

export type DocsAiSearchHit = {
  chunk: DocsAiChunk;
  score: number;
  matchedTerms: string[];
};

export type DocsAiSearchResult = {
  hits: DocsAiSearchHit[];
  confidence: DocsAiConfidence;
  intent: DocsAiIntent;
  coverage_reason?: string;
};

export type DocsAiSource = {
  id: string;
  title: string;
  path: string;
  section_title: string;
  primary_nav: string;
  excerpt: string;
};

export type GroundedDocsAnswer = {
  answer: string;
  steps: string[];
  confidence: DocsAiConfidence;
  sources: DocsAiSource[];
  related: DocsAiSource[];
  generated_by: "ai" | "extractive" | "fallback";
  coverage_reason?: string;
};

const LAST_INDEXED_AT = "2026-07-04T00:00:00.000Z";

function chunk(input: Omit<DocsAiChunk, "last_indexed_at">): DocsAiChunk {
  return { ...input, last_indexed_at: LAST_INDEXED_AT };
}

const ALL_PUBLISH_PLATFORMS = [
  "instagram",
  "threads",
  "pinterest",
  "tiktok",
  "facebook",
  "linkedin",
  "twitter",
  "youtube",
  "bluesky",
];

const OAUTH_PLATFORMS = ["instagram", "threads", "pinterest", "tiktok", "facebook", "linkedin", "twitter", "youtube"];

const platformCapabilitySummary = Object.entries(PLATFORM_METRICS)
  .map(([platform, metrics]) => {
    const supported = Object.entries(metrics)
      .filter(([, supportedMetric]) => supportedMetric)
      .map(([metric]) => metric)
      .join(", ");
    const unsupported = Object.entries(metrics)
      .filter(([, supportedMetric]) => !supportedMetric)
      .map(([metric]) => metric)
      .join(", ");

    return `${platform}: supported ${supported || "none"}; unsupported ${unsupported || "none"}.`;
  })
  .join("\n");

export const DOCS_AI_INDEX: DocsAiChunk[] = [
  chunk({
    id: "api-inbox-list-x",
    title: "List Inbox items",
    path: "/docs/api/inbox/list",
    section_id: "response",
    primary_nav: "API Reference",
    section_title: "GET Inbox items",
    product_area: "inbox",
    tags: ["inbox list", "x replies", "x direct messages", "x_reply", "x_dm"],
    intent_tags: ["reference"],
    endpoint_aliases: ["GET /v1/inbox", "/v1/inbox"],
    platforms: ["twitter", "instagram", "threads", "facebook"],
    content: "GET /v1/inbox lists normalized Inbox items. Use source x_reply for eligible public X replies and x_dm for legacy X direct messages. Inbox requires the Basic plan or higher. Responses include thread keys, message ownership, timestamps, and X billing metadata when present.",
  }),
  chunk({
    id: "api-inbox-reply-x",
    title: "Reply to an Inbox item",
    path: "/docs/api/inbox/reply",
    section_id: "request",
    primary_nav: "API Reference",
    section_title: "POST Inbox reply",
    product_area: "inbox",
    tags: ["inbox reply", "x comment reply", "x dm send", "idempotency"],
    intent_tags: ["reference", "posting"],
    endpoint_aliases: ["POST /v1/inbox/{id}/reply", "POST /v1/inbox/:id/reply"],
    platforms: ["twitter", "instagram", "threads", "facebook"],
    content: "POST /v1/inbox/:id/reply sends a supported response. X replies and DMs require Idempotency-Key. Managed X writes consume the monthly allowance; workspace X app writes bypass UniPost X Credits. Reuse the same key after uncertain outcomes and never blindly resend.",
  }),
  chunk({
    id: "api-inbox-sync-x",
    title: "Sync and backfill Inbox",
    path: "/docs/api/inbox/sync",
    section_id: "x-backfill",
    primary_nav: "API Reference",
    section_title: "POST Inbox sync",
    product_area: "inbox",
    tags: ["inbox sync", "x backfill", "confirmation token", "daily inbound cap"],
    intent_tags: ["reference"],
    endpoint_aliases: ["POST /v1/inbox/sync", "/v1/inbox/sync"],
    platforms: ["twitter", "instagram", "threads", "facebook"],
    content: "POST /v1/inbox/sync runs existing polling or a bounded X backfill. Managed X reads reserve monthly allowance and inbound daily capacity before a paid read. Larger estimates return a short-lived confirmation token bound to the exact accounts and request.",
  }),
  chunk({
    id: "guide-x-comments",
    title: "Receive and reply to X comments",
    path: "/docs/guides/x/comments",
    section_id: "prerequisites",
    primary_nav: "Guides",
    section_title: "X comments workflow",
    product_area: "inbox",
    tags: ["x comments", "twitter replies", "x_reply", "summoned reply"],
    intent_tags: ["reference", "posting"],
    endpoint_aliases: ["GET /v1/inbox", "POST /v1/inbox/:id/reply", "POST /v1/inbox/sync"],
    platforms: ["twitter"],
    content: "Use source x_reply for eligible public replies that summon the connected X account. Verify x_inbox.comments_enabled, list the item, reply with an idempotency key, and use a bounded seven-day backfill when needed.",
  }),
  chunk({
    id: "guide-x-direct-messages",
    title: "Receive and reply to X direct messages",
    path: "/docs/guides/x/direct-messages",
    section_id: "prerequisites",
    primary_nav: "Guides",
    section_title: "X direct-message workflow",
    product_area: "inbox",
    tags: ["x direct messages", "twitter dm", "x_dm", "dm permissions"],
    intent_tags: ["reference", "posting"],
    endpoint_aliases: ["GET /v1/inbox", "POST /v1/inbox/:id/reply", "POST /v1/inbox/sync"],
    platforms: ["twitter"],
    content: "Use source x_dm for legacy X direct-message events. Verify x_inbox.dms_enabled and the dm.read and dm.write scopes, protect private message content, send with an idempotency key, and backfill at most 30 days.",
  }),
  chunk({
    id: "guide-x-reconnect-permissions",
    title: "Reconnect X Inbox permissions",
    path: "/docs/guides/x/reconnect-permissions",
    section_id: "inspect",
    primary_nav: "Guides",
    section_title: "Restore X Inbox capability",
    product_area: "inbox",
    tags: ["x reconnect", "missing scopes", "dm.read", "workspace x app credentials"],
    intent_tags: ["auth", "connect"],
    endpoint_aliases: ["GET /v1/accounts/{id}/capabilities", "GET /v1/accounts/:id/capabilities"],
    platforms: ["twitter"],
    content: "Read x_inbox capability state first. Workspace X apps require Client ID, Client Secret, app Bearer Token, and Consumer Secret. Reconnect OAuth and approve tweet.read, tweet.write, users.read, offline.access, dm.read, and dm.write, then confirm the missing scopes list is empty.",
  }),
  chunk({
    id: "guide-x-credits",
    title: "Plan and monitor X Credits",
    path: "/docs/guides/x/credits",
    section_id: "estimate",
    primary_nav: "Guides",
    section_title: "Estimate managed-X usage",
    product_area: "billing",
    tags: ["x credits", "twitter credits", "allowance", "usage", "hard limit", "billing period", "managed x", "byo"],
    intent_tags: ["posting", "reference"],
    endpoint_aliases: ["GET /v1/billing/x-credits", "/v1/billing/x-credits"],
    platforms: ["twitter"],
    content:
      "X Credits are a weighted managed-X allowance separate from posts/month. The allowance resets each billing period. Use GET /v1/billing/x-credits to read monthly_allowance, monthly_used, monthly_remaining, billing_period_end, inbound_daily_usage, and inbound_daily_limit. Managed X operations stop at the hard limit. BYO X API connections do not consume UniPost X Credits. Validation does not consume X Credits, and the independent 20 X posts per account per UTC day safety cap still applies.",
  }),
  chunk({
    id: "api-reference-x-credits",
    title: "X Credits allowance",
    path: "/docs/api/x-credits",
    section_id: "response",
    primary_nav: "API Reference",
    section_title: "GET X Credits allowance",
    product_area: "billing",
    tags: ["x credits api", "twitter usage api", "monthly allowance", "remaining credits", "catalog version"],
    intent_tags: ["reference", "posting"],
    endpoint_aliases: ["GET /v1/billing/x-credits", "/v1/billing/x-credits"],
    platforms: ["twitter"],
    content:
      "GET /v1/billing/x-credits returns mode monthly_allowance, plan_id, monthly_allowance, monthly_used, monthly_remaining, billing_period_start, billing_period_end, catalog_version, inbound_daily_usage, inbound_daily_limit, and a managed-versus-BYO note. Enterprise limits are null because they are contract-defined. x_monthly_usage_limit_exceeded means managed-X work reached the billing-period hard limit.",
  }),
  chunk({
    id: "guide-connect-sessions-overview",
    title: "Connect Sessions",
    path: "/docs/connect-sessions",
    section_id: "when-to-use",
    primary_nav: "Guides",
    section_title: "When to use Connect Sessions",
    product_area: "connect",
    tags: ["connect sessions", "customer accounts", "hosted oauth", "managed accounts", "account connection", "api"],
    intent_tags: ["connect"],
    endpoint_aliases: [
      "POST /v1/connect/sessions",
      "GET /v1/connect/sessions/{session_id}",
      "GET /v1/connect/sessions/:session_id",
      "/v1/connect/sessions",
    ],
    platforms: OAUTH_PLATFORMS,
    content:
      "Use Connect Sessions when your product needs to send an end user through account authorization and then publish on behalf of the account they connected. The primary API is POST /v1/connect/sessions. The hosted flow authorizes your end user inside UniPost's Connect flow. Store external_user_id plus the completed managed_account_id. The managed_account_id is the account_id used later in platform_posts.",
  }),
  chunk({
    id: "guide-connect-sessions-create",
    title: "Create a Connect Session",
    path: "/docs/connect-sessions",
    section_id: "quickstart-session",
    primary_nav: "Guides",
    section_title: "Shared-app fallback session",
    product_area: "connect",
    tags: ["connect sessions", "create session", "hosted oauth url", "allow_quickstart_creds", "tiktok"],
    intent_tags: ["connect"],
    endpoint_aliases: [
      "POST /v1/connect/sessions",
      "client.connect.createSession",
      "client.connect.create_session",
      "/v1/connect/sessions",
    ],
    platforms: OAUTH_PLATFORMS,
    content:
      "Create a hosted customer account connection by calling POST /v1/connect/sessions with platform, profile_id when required, external_user_id, optional external_user_email, optional return_url, and allow_quickstart_creds when shared-app fallback is acceptable. For TikTok, set platform to tiktok. The response includes data.id, data.url, data.status, data.expires_at, and data.allow_quickstart_creds. Redirect or send the end user to data.url so they can authorize the account.",
  }),
  chunk({
    id: "guide-connect-sessions-completion",
    title: "Handle Connect Session completion",
    path: "/docs/connect-sessions",
    section_id: "completion",
    primary_nav: "Guides",
    section_title: "Handle completion",
    product_area: "connect",
    tags: ["connect sessions", "webhook", "account.connected", "managed_account_id", "polling"],
    intent_tags: ["connect"],
    endpoint_aliases: [
      "GET /v1/connect/sessions/{session_id}",
      "GET /v1/connect/sessions/:session_id",
      "account.connected",
    ],
    platforms: OAUTH_PLATFORMS,
    content:
      "After the hosted URL completes, subscribe to the account.connected webhook and store the returned social_account_id as the account id for future publishing. Poll GET /v1/connect/sessions/{session_id} only as a fallback for local development, CLI demos, or integrations that cannot receive webhooks. Terminal statuses are completed, expired, and cancelled.",
  }),
  chunk({
    id: "api-reference-connect-session-create",
    title: "Create connect session",
    path: "/docs/api/connect/sessions/create",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "POST connect session",
    product_area: "connect",
    tags: ["api reference", "connect sessions", "create session", "hosted onboarding", "oauth"],
    intent_tags: ["connect", "reference"],
    endpoint_aliases: [
      "POST /v1/connect/sessions",
      "/v1/connect/sessions",
      "connect.createSession",
      "connect.create_session",
    ],
    platforms: OAUTH_PLATFORMS,
    content:
      "POST /v1/connect/sessions creates a hosted onboarding session for a customer-owned social account. The body includes platform, profile_id when needed, external_user_id, optional external_user_email, optional return_url, and optional allow_quickstart_creds. The 201 response returns data.id, data.platform, data.url, data.allow_quickstart_creds, data.status, data.expires_at, and later managed_account_id after completion.",
  }),
  chunk({
    id: "api-reference-connect-session-get",
    title: "Get connect session",
    path: "/docs/api/connect/sessions/get",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET connect session",
    product_area: "connect",
    tags: ["api reference", "connect sessions", "polling", "session status", "managed_account_id"],
    intent_tags: ["connect", "reference"],
    endpoint_aliases: [
      "GET /v1/connect/sessions/{session_id}",
      "GET /v1/connect/sessions/:session_id",
      "/v1/connect/sessions/{session_id}",
      "/v1/connect/sessions/:session_id",
    ],
    platforms: OAUTH_PLATFORMS,
    content:
      "GET /v1/connect/sessions/{session_id} returns a Connect Session by id. Use it as a polling fallback to inspect status. The response includes status and managed_account_id when the hosted connection completes.",
  }),
  chunk({
    id: "guide-tiktok-platform-credentials",
    title: "TikTok Platform Credential Setup",
    path: "/docs/platform-credentials/tiktok",
    section_id: "api-workflow",
    primary_nav: "Guides",
    section_title: "Connect a TikTok account through your app",
    product_area: "credentials",
    tags: ["tiktok", "platform credentials", "client key", "client secret", "callback", "connect session"],
    intent_tags: ["credentials", "connect"],
    endpoint_aliases: [
      "POST /v1/platform-credentials",
      "GET /v1/platform-credentials",
      "POST /v1/connect/sessions",
    ],
    platforms: ["tiktok"],
    content:
      "For customer TikTok connections with your own app, save TikTok Platform Credentials first. TikTok calls the public identifier Client Key; UniPost stores Client Key and Client Secret. Add the exact callback URL https://api.unipost.dev/v1/connect/callback/tiktok in TikTok for Developers. After credentials are saved, create a TikTok Connect Session with platform set to tiktok, a profile_id, external_user_id, and return_url, then send the returned hosted OAuth URL to the end user.",
  }),
  chunk({
    id: "platform-guide-tiktok-connect",
    title: "TikTok platform guide",
    path: "/docs/platforms/tiktok",
    section_id: "setup",
    primary_nav: "Platforms",
    section_title: "Setup",
    product_area: "platforms",
    tags: ["tiktok", "quickstart", "white-label", "oauth", "connect", "publishing", "analytics"],
    intent_tags: ["connect", "posting", "analytics"],
    endpoint_aliases: [
      "POST /v1/connect/sessions",
      "POST /v1/posts",
      "GET /v1/accounts/{account_id}/metrics",
    ],
    platforms: ["tiktok"],
    content:
      "TikTok supports OAuth connection through Quickstart and White-label / Hosted Connect. TikTok publishing supports single video posts and photo carousels with privacy controls under platform_options.tiktok. TikTok analytics are limited to approved scopes and expose views, likes, comments, and shares for supported video inventory; follower count is read through unified account metrics.",
  }),
  chunk({
    id: "api-reference-api-keys",
    title: "Create API key",
    path: "/docs/api/api-keys/create",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "POST API key",
    product_area: "auth",
    tags: ["api keys", "authentication", "authorization", "bearer token", "sdk"],
    intent_tags: ["auth", "reference"],
    endpoint_aliases: [
      "POST /v1/api-keys",
      "GET /v1/api-keys",
      "/v1/api-keys",
    ],
    platforms: [],
    content:
      "UniPost API calls use Authorization: Bearer <token> with a workspace API key. POST /v1/api-keys creates a new API key for the authenticated workspace. The plaintext key is returned only once at creation time; store it before navigating away. The first key must be created in the dashboard because no API key exists yet.",
  }),
  chunk({
    id: "api-reference-list-profiles",
    title: "List profiles",
    path: "/docs/api/profiles/list",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET profiles",
    product_area: "accounts",
    tags: ["profiles", "profile_id", "branding", "connect sessions"],
    intent_tags: ["connect", "reference"],
    endpoint_aliases: [
      "GET /v1/profiles",
      "/v1/profiles",
      "client.profiles.list",
    ],
    platforms: [],
    content:
      "GET /v1/profiles lists profiles that belong to the workspace behind the API key. Use a profile id when creating Connect Sessions in workspaces with multiple profiles, and when choosing the branded hosted Connect surface.",
  }),
  chunk({
    id: "api-reference-list-accounts",
    title: "List accounts",
    path: "/docs/api/accounts/list",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET accounts",
    product_area: "accounts",
    tags: ["accounts", "social accounts", "account_id", "platform", "analytics"],
    intent_tags: ["analytics", "posting", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts",
      "/v1/accounts",
    ],
    platforms: ALL_PUBLISH_PLATFORMS,
    content:
      "GET /v1/accounts lists connected social accounts. Use it to choose the connected account id for publishing and account metrics. The returned account id is also the managed_account_id from completed Connect Sessions.",
  }),
  chunk({
    id: "guide-publishing-overview",
    title: "Publishing guide",
    path: "/docs/publishing",
    section_id: "overview",
    primary_nav: "Guides",
    section_title: "Overview",
    product_area: "publishing",
    tags: ["publish", "posting", "posts", "platform_posts", "media", "account_id"],
    intent_tags: ["posting"],
    endpoint_aliases: [
      "POST /v1/posts",
      "POST /v1/media",
      "/v1/posts",
      "/v1/media",
    ],
    platforms: ALL_PUBLISH_PLATFORMS,
    content:
      "After an account is connected, publish by creating posts with connected account ids in platform_posts or account_ids. For local files, upload media with POST /v1/media, then publish with media_ids. TikTok publishing requires video or image carousel media and uses platform_options.tiktok for privacy and upload controls.",
  }),
  chunk({
    id: "guide-publish-gifs",
    title: "Publish GIFs to X and Facebook",
    path: "/docs/guides/publish-gifs",
    section_id: "platform-support",
    primary_nav: "Guides",
    section_title: "Platform support",
    product_area: "publishing",
    tags: [
      "publish GIF",
      "GIF post",
      "animated GIF",
      "X GIF",
      "Twitter GIF",
      "Facebook GIF",
      "image/gif",
      "GIF to MP4",
      "local GIF upload",
    ],
    intent_tags: ["posting"],
    endpoint_aliases: [
      "GET /v1/accounts",
      "POST /v1/media",
      "GET /v1/media/{media_id}",
      "POST /v1/posts/validate",
      "POST /v1/posts",
      "GET /v1/posts/{post_id}",
    ],
    platforms: ["twitter", "facebook", "linkedin", "threads", "instagram", "tiktok", "pinterest", "youtube", "bluesky"],
    content:
      "UniPost directly supports GIF publishing to X and Facebook. Publish a hosted GIF with platform_posts[].media_urls, or reserve a local image/gif upload with POST /v1/media, PUT bytes to upload_url, poll GET /v1/media/{media_id}, then validate and publish with media_ids. A GIF sent to both X and Facebook must be 5 MB or smaller. LinkedIn and Threads native GIF integration is coming soon. For Instagram, TikTok, Pinterest, YouTube, and Bluesky, a UniPost GIF-to-MP4 conversion option is coming soon.",
  }),
  chunk({
    id: "guide-platform-options",
    title: "Platform options examples",
    path: "/docs/guides/platform-options",
    section_id: "request-shape",
    primary_nav: "Guides",
    section_title: "Use flat platform options",
    product_area: "publishing",
    tags: [
      "platform_options",
      "platform_posts",
      "YouTube Shorts visibility",
      "Instagram Stories",
      "TikTok privacy",
      "Facebook Reels",
      "Pinterest board_id",
    ],
    intent_tags: ["posting"],
    endpoint_aliases: ["POST /v1/posts", "POST /v1/posts/validate", "/v1/posts", "/v1/posts/validate"],
    platforms: ["youtube", "instagram", "tiktok", "facebook", "pinterest"],
    content:
      "Use platform_posts[].platform_options as a flat object. Do not nest a platform name inside platform_posts. YouTube API requests default to private when privacy_status is omitted, so set privacy_status to public for a public video or Short. The guide includes Instagram mediaType, TikTok privacy and interaction controls, Facebook feed and Reel options, and Pinterest board_id examples.",
  }),
  chunk({
    id: "guide-video-audio-overlay",
    title: "Overlay user audio onto a video",
    path: "/docs/guides/video-audio-overlay",
    section_id: "steps",
    primary_nav: "Guides",
    section_title: "API steps",
    product_area: "publishing",
    tags: [
      "audio overlay",
      "custom audio",
      "combine video and audio",
      "video audio",
      "media processing",
      "upload audio",
      "voiceover",
      "music",
      "sdk 0.5.0",
      "size_bytes",
      "sizeBytes",
      "tiktok",
      "instagram",
    ],
    intent_tags: ["posting"],
    endpoint_aliases: [
      "POST /v1/media",
      "GET /v1/media/{media_id}",
      "GET /v1/media/:media_id",
      "POST /v1/media/audio-overlays",
      "GET /v1/media/audio-overlays/{id}",
      "GET /v1/media/audio-overlays/:id",
      "POST /v1/posts",
      "/v1/media/audio-overlays",
    ],
    platforms: ALL_PUBLISH_PLATFORMS,
    content:
      "Use the Video + audio overlay guide when a user uploads one video file and one separate audio file, such as narration, music, or voiceover. Use UniPost SDK version 0.5.0 or later because official SDKs starting in 0.5.0 no longer force callers to provide sizeBytes or size_bytes when reserving uploads with POST /v1/media. File size is optional: send it when the app already knows the byte length, or omit it when the app does not know it yet. Step 1: upload the video input with POST /v1/media, upload bytes to upload_url, then poll GET /v1/media/{media_id}; upload_url is a presigned file upload destination, not another UniPost JSON endpoint. Step 2: upload the audio input with the same flow. Step 3: generate the overlay video by calling POST /v1/media/audio-overlays with video_media_id, audio_media_id, mode mix or replace, optional volumes, and fit trim_to_video or loop_to_video, then poll GET /v1/media/audio-overlays/{id} until status is succeeded. Step 4: publish output_media_id as a normal video media_id with POST /v1/posts. This is not platform music-library attachment for image or carousel posts; UniPost creates a processed video output before publishing.",
  }),
  chunk({
    id: "guide-instagram-stories",
    title: "Publish Instagram Stories",
    path: "/docs/guides/instagram-stories",
    section_id: "request-shape",
    primary_nav: "Guides",
    section_title: "Choose one request shape",
    product_area: "publishing",
    tags: [
      "instagram stories",
      "instagram story",
      "story publishing",
      "story posted as feed",
      "normal feed",
      "mediaType story",
      "platform_options",
      "platform_posts",
      "validation error",
      "422",
    ],
    intent_tags: ["posting"],
    endpoint_aliases: [
      "POST /v1/posts",
      "POST /v1/posts/validate",
      "/v1/posts",
      "/v1/posts/validate",
    ],
    platforms: ["instagram"],
    content:
      "Use the Instagram Stories guide when an Instagram publish should be an ephemeral Story rather than a normal feed post or Reel. Recommended request shape: platform_posts[] with platform_posts[].platform_options.mediaType set to story as a flat destination option. Do not put platform_posts[].platform_options.instagram.mediaType in the new shape. That is legacy account_ids syntax and UniPost returns 422 VALIDATION_ERROR with docs_url when it is mixed into platform_posts. Legacy account_ids requests may still use top-level platform_options.instagram.mediaType. Stories require exactly one image or video. Validate with POST /v1/posts/validate before publishing: valid request shapes return 200 with data.valid true or false; mixed request-shape errors return 422 and should be fixed before retrying.",
  }),
  chunk({
    id: "analytics-guide-tiktok-followers",
    title: "Get TikTok followers",
    path: "/docs/guides/analytics/tiktok-followers",
    section_id: "answer",
    primary_nav: "Guides",
    section_title: "Direct answer",
    product_area: "analytics",
    tags: ["analytics", "tiktok", "followers", "account metrics", "scopes"],
    intent_tags: ["analytics"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
      "/v1/accounts/{account_id}/metrics",
      "/v1/accounts/:account_id/metrics",
    ],
    platforms: ["tiktok"],
    content:
      "TikTok followers use the unified UniPost account metrics API: GET /v1/accounts/{account_id}/metrics. The approved TikTok scope is user.info.stats. Read data.follower_count from the response. video.list is for public videos and post-level TikTok video inventory, not follower count. user.info.profile powers profile fields.",
  }),
  chunk({
    id: "analytics-guide-account-metrics",
    title: "Get account metrics across platforms",
    path: "/docs/guides/analytics/account-metrics",
    section_id: "fields",
    primary_nav: "Guides",
    section_title: "Fields to read",
    product_area: "analytics",
    tags: ["analytics", "accounts", "followers", "subscribers", "following", "post count", "metrics", "youtube"],
    intent_tags: ["analytics"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
    ],
    platforms: ["instagram", "threads", "tiktok", "twitter", "youtube"],
    content:
      "Use GET /v1/accounts/{account_id}/metrics for account-level metrics such as data.follower_count, data.following_count, data.post_count, and data.platform_specific. Supported platforms include X, Instagram, Threads, TikTok, and YouTube. YouTube V1 account metrics use youtube.readonly and map channel subscriberCount to follower_count, videoCount to post_count, and viewCount to platform_specific.view_count. Unsupported platforms return NOT_SUPPORTED instead of an empty success response.",
  }),
  chunk({
    id: "api-reference-account-metrics",
    title: "Get account metrics",
    path: "/docs/api/accounts/metrics",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET account metrics",
    product_area: "accounts",
    tags: ["api reference", "account metrics", "followers", "subscribers", "account_id", "youtube"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/metrics",
      "GET /v1/accounts/{id}/metrics",
      "GET /v1/accounts/:account_id/metrics",
      "GET /v1/accounts/:id/metrics",
    ],
    platforms: ["instagram", "threads", "tiktok", "twitter", "youtube"],
    content:
      "GET /v1/accounts/{account_id}/metrics returns normalized account metrics for one connected social account. The response includes data.social_account_id, data.platform, data.follower_count, data.following_count, data.post_count, data.platform_specific, and data.fetched_at. For YouTube, follower_count comes from channel subscriberCount, post_count comes from videoCount, following_count is 0 with following_count_supported=false, and platform_specific includes view_count and hidden_subscriber_count.",
  }),
  chunk({
    id: "analytics-youtube-v2-overview",
    title: "YouTube Analytics",
    path: "/docs/api/analytics/youtube",
    section_id: "overview",
    primary_nav: "API Reference",
    section_title: "Overview",
    product_area: "analytics",
    tags: ["analytics", "youtube", "reports", "yt-analytics.readonly", "summary", "trend", "videos"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/youtube/analytics/summary",
      "GET /v1/accounts/{account_id}/youtube/analytics/trend",
      "GET /v1/accounts/{account_id}/youtube/analytics/videos",
    ],
    platforms: ["youtube"],
    content:
      "YouTube Analytics V2 uses the YouTube Analytics API for owner-authorized, non-monetary reports. Endpoints include GET /v1/accounts/{account_id}/youtube/analytics/summary, GET /v1/accounts/{account_id}/youtube/analytics/trend, and GET /v1/accounts/{account_id}/youtube/analytics/videos. Required provider scope: https://www.googleapis.com/auth/yt-analytics.readonly. Monetary reports are not included and yt-analytics-monetary.readonly is not required.",
  }),
  chunk({
    id: "analytics-youtube-v2-summary",
    title: "Get YouTube analytics summary",
    path: "/docs/api/analytics/youtube/summary",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET YouTube analytics summary",
    product_area: "analytics",
    tags: ["analytics", "youtube", "summary", "views", "subscribers", "watch time"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/youtube/analytics/summary",
      "GET /v1/accounts/{id}/youtube/analytics/summary",
    ],
    platforms: ["youtube"],
    content:
      "GET /v1/accounts/{account_id}/youtube/analytics/summary returns channel-level YouTube Analytics totals for a date range. Metrics include views, likes, comments, shares, estimated_minutes_watched, average_view_duration, average_view_percentage, subscribers_gained, and subscribers_lost. Defaults to the last 28 complete UTC days.",
  }),
  chunk({
    id: "analytics-youtube-v2-trend-videos",
    title: "Get YouTube analytics trend and videos",
    path: "/docs/api/analytics/youtube/trend",
    section_id: "endpoint",
    primary_nav: "API Reference",
    section_title: "GET YouTube analytics trend and videos",
    product_area: "analytics",
    tags: ["analytics", "youtube", "trend", "daily", "top videos"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/youtube/analytics/trend",
      "GET /v1/accounts/{account_id}/youtube/analytics/videos",
      "GET /v1/accounts/{id}/youtube/analytics/trend",
      "GET /v1/accounts/{id}/youtube/analytics/videos",
    ],
    platforms: ["youtube"],
    content:
      "GET /v1/accounts/{account_id}/youtube/analytics/trend returns daily YouTube Analytics rows. GET /v1/accounts/{account_id}/youtube/analytics/videos returns top video rows sorted by views, with limit default 25 and cap 200.",
  }),
  chunk({
    id: "analytics-guide-export-posts",
    title: "Export post analytics rows",
    path: "/docs/guides/analytics/export-post-analytics",
    section_id: "steps",
    primary_nav: "Guides",
    section_title: "Steps",
    product_area: "analytics",
    tags: ["analytics", "export", "csv", "posts", "reporting", "bi"],
    intent_tags: ["analytics"],
    endpoint_aliases: [
      "GET /v1/analytics/posts/export",
      "/v1/analytics/posts/export",
    ],
    platforms: ALL_PUBLISH_PLATFORMS,
    content:
      "Use GET /v1/analytics/posts/export when your app needs normalized post-level analytics rows as CSV across multiple UniPost-published posts. Choose from and to, add optional platform, account_id, profile_id, post_id, status, and sort filters, then save the CSV response.",
  }),
  chunk({
    id: "analytics-guide-post-analytics",
    title: "Get post analytics",
    path: "/docs/guides/analytics/post-analytics",
    section_id: "overview",
    primary_nav: "Guides",
    section_title: "Overview",
    product_area: "analytics",
    tags: ["analytics", "posts", "post analytics", "likes", "comments", "shares", "views"],
    intent_tags: ["analytics"],
    endpoint_aliases: [
      "GET /v1/posts/{post_id}/analytics",
      "GET /v1/posts/:post_id/analytics",
      "/v1/posts/{post_id}/analytics",
    ],
    platforms: ALL_PUBLISH_PLATFORMS,
    content:
      "Use GET /v1/posts/{post_id}/analytics for normalized post-level analytics on a single UniPost-published post. The response includes destination results, normalized analytics fields such as likes, comments, shares, saves, clicks, video views, engagement rate where available, platform_specific data, and fetched_at.",
  }),
  chunk({
    id: "analytics-platform-capabilities",
    title: "Platform analytics capabilities",
    path: "/docs/api/analytics/platforms",
    section_id: "capabilities",
    primary_nav: "API Reference",
    section_title: "Supported metrics by platform",
    product_area: "platforms",
    tags: ["analytics", "platforms", "capabilities", "metrics", "support matrix"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/analytics/platforms",
      "/v1/analytics/platforms",
    ],
    platforms: Object.keys(PLATFORM_METRICS),
    content:
      "Platform analytics capabilities come from PLATFORM_METRICS, the shared product source of truth for supported post metrics. Current snapshot:\n" +
      platformCapabilitySummary,
  }),
  chunk({
    id: "analytics-guide-reconnect-scopes",
    title: "Reconnect accounts for analytics scopes",
    path: "/docs/guides/analytics/reconnect-analytics-scopes",
    section_id: "steps",
    primary_nav: "Guides",
    section_title: "Steps",
    product_area: "analytics",
    tags: ["analytics", "scopes", "reconnect", "oauth", "permissions"],
    intent_tags: ["analytics", "connect"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/health",
      "GET /v1/accounts/:account_id/health",
      "GET /v1/accounts",
    ],
    platforms: ["instagram", "threads", "pinterest", "tiktok", "youtube", "facebook"],
    content:
      "If an account was connected before analytics scopes were granted, reconnect it so the token includes the new platform scopes. TikTok analytics scopes include user.info.profile, user.info.stats, and video.list. YouTube V1 account metrics use youtube.readonly. YouTube Analytics V2 reports use yt-analytics.readonly through the YouTube Analytics API. Use account health or account listing state to identify accounts that need reconnect before relying on live analytics metrics.",
  }),
  chunk({
    id: "analytics-tiktok-native-drilldown",
    title: "TikTok native analytics drilldowns",
    path: "/docs/api/analytics/tiktok",
    section_id: "overview",
    primary_nav: "API Reference",
    section_title: "Overview",
    product_area: "analytics",
    tags: ["analytics", "tiktok", "profile", "videos", "native drilldown"],
    intent_tags: ["analytics", "reference"],
    endpoint_aliases: [
      "GET /v1/accounts/{account_id}/tiktok/profile",
      "GET /v1/accounts/{account_id}/tiktok/videos",
      "GET /v1/accounts/{account_id}/metrics",
    ],
    platforms: ["tiktok"],
    content:
      "TikTok native drilldowns are optional when a product needs profile details or public video inventory. user.info.profile powers TikTok profile fields, user.info.stats powers account stats, and video.list powers public videos. For follower count, prefer the unified GET /v1/accounts/{account_id}/metrics endpoint.",
  }),
];

const STOP_WORDS = new Set([
  "a",
  "about",
  "an",
  "and",
  "api",
  "are",
  "can",
  "do",
  "does",
  "for",
  "from",
  "how",
  "i",
  "is",
  "it",
  "my",
  "of",
  "on",
  "or",
  "the",
  "to",
  "unipost",
  "use",
  "what",
  "which",
  "with",
]);

function normalize(value: string) {
  return value.toLowerCase().replace(/[\u2018\u2019]/g, "'").replace(/[\u201c\u201d]/g, "\"");
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function normalizeEndpoint(value: string) {
  return normalize(value)
    .replace(/\{[a-z0-9_]+\}/g, "{param}")
    .replace(/:[a-z0-9_]+/g, "{param}")
    .replace(/\s+/g, " ")
    .trim();
}

function tokenize(query: string) {
  return Array.from(new Set(
    normalize(query)
      .split(/[^a-z0-9_./:{}-]+/)
      .map((token) => token.trim())
      .filter((token) => token.length > 1 && !STOP_WORDS.has(token)),
  ));
}

function includesTerm(haystack: string, term: string) {
  if (/[./:{}-]/.test(term)) return haystack.includes(term);
  return new RegExp(`\\b${escapeRegExp(term)}\\b`, "i").test(haystack);
}

function hasAny(value: string, patterns: RegExp[]) {
  return patterns.some((pattern) => pattern.test(value));
}

function detectIntent(query: string): DocsAiIntent {
  const normalizedQuery = normalize(query);
  const hasEndpoint = /\b(GET|POST|PUT|PATCH|DELETE)\s+\/v1\//i.test(query) || /\/v1\//.test(query);
  const isBareEndpoint = /^(GET|POST|PUT|PATCH|DELETE)\s+\/v1\/[a-z0-9_/:{}.-]+$/i.test(query.trim());
  if (isBareEndpoint) return "reference";

  const connectIntent = hasAny(normalizedQuery, [
    /\bconnect\b/,
    /\bconnection\b/,
    /\bconnect session\b/,
    /\boauth\b/,
    /\bauthori[sz]e\b/,
    /\bonboarding\b/,
    /\bhosted\b/,
    /\bcallback\b/,
    /\breturn_url\b/,
    /\ballow_quickstart_creds\b/,
  ]);
  const credentialsIntent = hasAny(normalizedQuery, [
    /\bcredential/,
    /\bclient key\b/,
    /\bclient secret\b/,
    /\bapp id\b/,
    /\bdeveloper app\b/,
    /\bredirect uri\b/,
    /\bcallback url\b/,
  ]);
  const analyticsIntent = hasAny(normalizedQuery, [
    /\banalytics?\b/,
    /\bmetrics?\b/,
    /\bfollowers?\b/,
    /\bfans?\b/,
    /\bsubscribers?\b/,
    /\bfollower_count\b/,
    /\bviews?\b/,
    /\blikes?\b/,
    /\bcomments?\b/,
    /\bshares?\b/,
    /\breports?\b/,
    /\btrend\b/,
    /\btop videos?\b/,
    /\bwatch time\b/,
    /\bexport\b/,
    /\bcsv\b/,
    /\bvideo\.list\b/,
    /\buser\.info\b/,
  ]);
  const authIntent = hasAny(normalizedQuery, [
    /\bapi keys?\b/,
    /\bauth\b/,
    /\bauthentication\b/,
    /\bauthorization\b/,
    /\bbearer\b/,
    /\btoken\b/,
  ]);
  const postingIntent = hasAny(normalizedQuery, [
    /\bpublish\b/,
    /\bposting\b/,
    /\bposts?\b/,
    /\bmedia\b/,
    /\bschedule\b/,
    /\bcaption\b/,
    /\bphoto\b/,
    /\bvideo\b/,
    /\bcarousel\b/,
  ]);

  if (hasEndpoint && !connectIntent && !analyticsIntent && !credentialsIntent && !postingIntent && !authIntent) return "reference";
  if (connectIntent) return "connect";
  if (credentialsIntent) return "credentials";
  if (analyticsIntent) return "analytics";
  if (authIntent) return "auth";
  if (postingIntent) return "posting";
  if (hasEndpoint) return "reference";
  return "unknown";
}

function chunkCoversIntent(chunkToScore: DocsAiChunk, intent: DocsAiIntent) {
  if (intent === "unknown") return true;
  if (intent === "reference") return chunkToScore.primary_nav === "API Reference" || chunkToScore.intent_tags.includes("reference");
  if (chunkToScore.intent_tags.includes(intent)) return true;

  switch (intent) {
    case "analytics":
      return ["analytics", "accounts", "platforms"].includes(chunkToScore.product_area);
    case "connect":
      return ["connect"].includes(chunkToScore.product_area);
    case "credentials":
      return ["credentials"].includes(chunkToScore.product_area);
    case "auth":
      return ["auth"].includes(chunkToScore.product_area);
    case "posting":
      return ["posting", "publishing"].includes(chunkToScore.product_area);
    default:
      return false;
  }
}

function isTaskQuery(query: string) {
  return /\b(how|which|what|where|connect|get|export|reconnect|field|fields|followers?|publish|create)\b/i.test(query);
}

function scoreChunk(chunkToScore: DocsAiChunk, query: string, terms: string[], intent: DocsAiIntent) {
  const normalizedQuery = normalize(query);
  const normalizedEndpointQuery = normalizeEndpoint(query);
  const title = normalize(chunkToScore.title);
  const content = normalize(chunkToScore.content);
  const tags = chunkToScore.tags.map(normalize).join(" ");
  const intentTags = chunkToScore.intent_tags.map(normalize).join(" ");
  const aliases = chunkToScore.endpoint_aliases.map(normalize).join(" ");
  const normalizedAliases = chunkToScore.endpoint_aliases.map(normalizeEndpoint).join(" ");
  const path = normalize(chunkToScore.path);
  const platforms = chunkToScore.platforms.map(normalize).join(" ");
  const haystack = `${title} ${content} ${tags} ${intentTags} ${aliases} ${path} ${platforms}`;
  const matchedTerms: string[] = [];
  let score = 0;

  for (const term of terms) {
    if (!includesTerm(haystack, term)) continue;

    matchedTerms.push(term);
    score += 4;
    if (includesTerm(title, term)) score += 13;
    if (includesTerm(tags, term)) score += 10;
    if (includesTerm(intentTags, term)) score += 14;
    if (includesTerm(aliases, term)) score += 18;
    if (includesTerm(platforms, term)) score += 7;
    if (includesTerm(path, term)) score += 6;
    if (includesTerm(content, term)) score += 4;
  }

  if (normalizedQuery && title.includes(normalizedQuery)) score += 42;
  if (normalizedQuery && aliases.includes(normalizedQuery)) score += 70;
  if (normalizedEndpointQuery && normalizedAliases.includes(normalizedEndpointQuery)) score += intent === "reference" ? 150 : 95;

  if (normalizedQuery.includes("connect session") && chunkToScore.tags.some((tag) => normalize(tag).includes("connect session"))) score += 40;
  if (normalizedQuery.includes("allow_quickstart_creds") && content.includes("allow_quickstart_creds")) score += 32;
  if (normalizedQuery.includes("tiktok") && chunkToScore.platforms.includes("tiktok")) score += 12;
  if (normalizedQuery.includes("follower") && chunkToScore.id.includes("followers") && intent !== "connect") score += 34;
  if (normalizedQuery.includes("video.list") && content.includes("video.list")) score += 28;
  if ((normalizedQuery.includes("export") || normalizedQuery.includes("csv")) && path.includes("export")) score += 30;
  if (normalizedQuery.includes("client key") && content.includes("client key")) score += 34;
  if (normalizedQuery.includes("callback") && content.includes("callback")) score += 20;

  if (chunkCoversIntent(chunkToScore, intent)) score += intent === "unknown" ? 0 : 42;
  if (!chunkCoversIntent(chunkToScore, intent) && intent !== "unknown") score -= 42;

  if (isTaskQuery(query) && intent !== "reference" && chunkToScore.primary_nav === "Guides") score += 16;
  if (intent === "reference" && chunkToScore.primary_nav === "API Reference") score += 42;
  if (intent === "reference" && chunkToScore.primary_nav === "Guides") score -= 12;

  return { score: Math.max(0, score), matchedTerms };
}

function confidenceForScore(score: number): DocsAiConfidence {
  if (score >= 118) return "high";
  if (score >= 74) return "medium";
  if (score >= 42) return "low";
  return "none";
}

function coverageReasonFor(hits: DocsAiSearchHit[], intent: DocsAiIntent, confidence: DocsAiConfidence) {
  if (hits.length === 0 || confidence === "none") return "not enough source coverage";
  const top = hits[0]?.chunk;
  if (!top) return "not enough source coverage";
  if (!chunkCoversIntent(top, intent)) return `top source does not cover ${intent} intent`;
  return undefined;
}

export function searchDocsIndex(query: string, options: { limit?: number } = {}): DocsAiSearchResult {
  const trimmed = query.trim();
  if (!trimmed) {
    return { hits: [], confidence: "none", intent: "unknown", coverage_reason: "empty query" };
  }

  const intent = detectIntent(trimmed);
  const terms = tokenize(trimmed);
  const hits = DOCS_AI_INDEX
    .map((chunkItem) => {
      const scored = scoreChunk(chunkItem, trimmed, terms, intent);
      return { chunk: chunkItem, score: scored.score, matchedTerms: scored.matchedTerms };
    })
    .filter((hit) => hit.score > 0)
    .sort((a, b) => b.score - a.score || a.chunk.title.localeCompare(b.chunk.title))
    .slice(0, options.limit ?? 5);
  const confidence = confidenceForScore(hits[0]?.score ?? 0);
  const coverage_reason = coverageReasonFor(hits, intent, confidence);

  return {
    hits,
    confidence: coverage_reason ? "none" : confidence,
    intent,
    coverage_reason,
  };
}

function sourceFromHit(hit: DocsAiSearchHit): DocsAiSource {
  const excerpt = hit.chunk.content.length > 240
    ? `${hit.chunk.content.slice(0, 237).trimEnd()}...`
    : hit.chunk.content;

  return {
    id: hit.chunk.id,
    title: hit.chunk.title,
    path: hit.chunk.path,
    section_title: hit.chunk.section_title,
    primary_nav: hit.chunk.primary_nav,
    excerpt,
  };
}

function uniqueHitsByPath(hits: DocsAiSearchHit[]) {
  const seen = new Set<string>();
  return hits.filter((hit) => {
    if (seen.has(hit.chunk.path)) return false;
    seen.add(hit.chunk.path);
    return true;
  });
}

function hitById(search: DocsAiSearchResult, id: string) {
  const rankedHit = search.hits.find((hit) => hit.chunk.id === id);
  if (rankedHit) return rankedHit;

  const chunkItem = DOCS_AI_INDEX.find((item) => item.id === id);
  if (!chunkItem) return undefined;

  return {
    chunk: chunkItem,
    score: 0,
    matchedTerms: [],
  };
}

function orderedHitsForAnswer(query: string, search: DocsAiSearchResult) {
  if (search.intent === "connect") {
    const isExactCreateSession = normalizeEndpoint(query).includes("post /v1/connect/sessions");
    const mentionsTikTok = normalize(query).includes("tiktok");
    const primaryGuide = hitById(search, "guide-connect-sessions-create")
      ?? search.hits.find((hit) => hit.chunk.product_area === "connect" && hit.chunk.primary_nav === "Guides");
    const createReference = hitById(search, "api-reference-connect-session-create");
    const completionGuide = hitById(search, "guide-connect-sessions-completion");
    const tiktokCredentials = mentionsTikTok ? hitById(search, "guide-tiktok-platform-credentials") : undefined;

    if (isExactCreateSession) {
      return uniqueHitsByPath([
        createReference,
        primaryGuide,
        completionGuide,
        ...search.hits,
      ].filter((hit): hit is DocsAiSearchHit => Boolean(hit)));
    }

    return uniqueHitsByPath([
      primaryGuide,
      createReference,
      tiktokCredentials,
      completionGuide,
      ...search.hits,
    ].filter((hit): hit is DocsAiSearchHit => Boolean(hit)));
  }

  return uniqueHitsByPath(search.hits);
}

function isInsufficientCoverage(search: DocsAiSearchResult) {
  return search.confidence === "none" || search.hits.length === 0 || Boolean(search.coverage_reason);
}

function answerForConnectTask(query: string, search: DocsAiSearchResult) {
  const normalizedQuery = normalize(query);
  const isExactCreateSession = normalizeEndpoint(query).includes("post /v1/connect/sessions");
  const mentionsTikTok = normalizedQuery.includes("tiktok");

  if (isExactCreateSession) {
    return {
      answer:
        "POST /v1/connect/sessions creates a hosted onboarding session for a customer-owned social account. Send platform, external_user_id, profile_id when needed, optional return_url, and optional allow_quickstart_creds; the response starts pending and includes data.url for the hosted authorization flow.",
      steps: [
        "Create the session from your backend with Authorization: Bearer <UNIPOST_API_KEY>.",
        "Send the returned data.url to the end user so they can authorize the account.",
        "Handle completion from the account.connected webhook, or poll GET /v1/connect/sessions/{session_id} as a fallback.",
      ],
    };
  }

  if (mentionsTikTok || search.hits.some((hit) => hit.chunk.platforms.includes("tiktok"))) {
    return {
      answer:
        "Use Connect Sessions for customer-owned TikTok account connection. Call POST /v1/connect/sessions with platform set to tiktok, your external_user_id, profile_id when needed, and return_url when you want the browser sent back to your app; then send the returned data.url to the user for TikTok authorization.",
      steps: [
        "Create or choose the API key that your backend will use as Authorization: Bearer <UNIPOST_API_KEY>.",
        "If you need your own TikTok app on the consent screen, save TikTok Platform Credentials with the Client Key, Client Secret, and UniPost callback URL first.",
        "Call POST /v1/connect/sessions with platform=tiktok, profile_id when required, external_user_id, optional external_user_email, optional return_url, and allow_quickstart_creds only when shared-app fallback is acceptable.",
        "Redirect or send the end user to data.url.",
        "Store the connected account id from the account.connected webhook, or poll GET /v1/connect/sessions/{session_id} as a fallback until status is completed.",
      ],
    };
  }

  return {
    answer:
      "Use Connect Sessions when your product needs an end user to authorize a customer-owned account. Call POST /v1/connect/sessions, send the returned data.url to the user, then store the managed_account_id from completion.",
    steps: [
      "Choose the target platform and profile_id if your workspace has multiple profiles.",
      "Call POST /v1/connect/sessions with external_user_id and optional return_url.",
      "Send data.url to the user.",
      "Use account.connected webhooks for completion, with GET /v1/connect/sessions/{session_id} as a polling fallback.",
    ],
  };
}

function answerForCredentialsTask(query: string) {
  const normalizedQuery = normalize(query);

  if (normalizedQuery.includes("tiktok")) {
    return {
      answer:
        "For TikTok Platform Credentials, create or use a TikTok developer app, add UniPost's callback URL https://api.unipost.dev/v1/connect/callback/tiktok, then save the TikTok Client Key and Client Secret in UniPost. After that, create a TikTok Connect Session with POST /v1/connect/sessions and send the returned data.url to your end user.",
      steps: [
        "Create the TikTok app from a company-owned developer account.",
        "Add https://api.unipost.dev/v1/connect/callback/tiktok to TikTok's callback or redirect allowlist.",
        "Save the Client Key and Client Secret in UniPost Platform Credentials.",
        "Create a TikTok Connect Session with platform=tiktok to validate the end-user OAuth path.",
      ],
    };
  }

  return null;
}

function answerForAnalyticsTask(query: string) {
  const normalizedQuery = normalize(query);

  if (normalizedQuery.includes("video.list") && /follower|fans?/.test(normalizedQuery)) {
    return {
      answer:
        "No. video.list is for TikTok public video inventory and post-level video metrics, not follower count. To get TikTok followers through UniPost, call GET /v1/accounts/{account_id}/metrics and read data.follower_count; the TikTok scope behind that account metric is user.info.stats.",
      steps: [
        "Use GET /v1/accounts to find the connected TikTok account id.",
        "Call GET /v1/accounts/{account_id}/metrics.",
        "Read data.follower_count from the response.",
        "Use video.list-backed TikTok drilldowns only when you need public videos or video engagement counters.",
      ],
    };
  }

  if (normalizedQuery.includes("tiktok") && /follower|fans?/.test(normalizedQuery)) {
    return {
      answer:
        "Use the unified UniPost account metrics endpoint, not a TikTok-native followers endpoint. Call GET /v1/accounts/{account_id}/metrics for the TikTok account and read data.follower_count. The TikTok scope behind that field is user.info.stats; video.list is for public videos and post-level inventory.",
      steps: [
        "List accounts with GET /v1/accounts and choose the TikTok account id.",
        "If the account predates analytics scope approval, reconnect it so user.info.stats is on the token.",
        "Call GET /v1/accounts/{account_id}/metrics.",
        "Read data.follower_count from the response.",
      ],
    };
  }

  if (normalizedQuery.includes("youtube") && /analytics api|yt-analytics|reports?|trend|top videos?|watch time|summary/.test(normalizedQuery)) {
    return {
      answer:
        "Use the YouTube Analytics V2 endpoints for owner-authorized YouTube reports: GET /v1/accounts/{account_id}/youtube/analytics/summary for channel totals, GET /v1/accounts/{account_id}/youtube/analytics/trend for daily rows, and GET /v1/accounts/{account_id}/youtube/analytics/videos for top videos. These endpoints require https://www.googleapis.com/auth/yt-analytics.readonly; monetary reports are not included.",
      steps: [
        "List accounts with GET /v1/accounts and choose the connected YouTube account id.",
        "Use summary for date-ranged channel totals, trend for daily rows, or videos for top video rows.",
        "Pass from and to as YYYY-MM-DD dates, or omit them to use the last 28 complete UTC days.",
        "If the API returns NEEDS_RECONNECT, reconnect the YouTube account so the token includes yt-analytics.readonly.",
      ],
    };
  }

  if (normalizedQuery.includes("youtube") && /metric|subscriber|follower|scope|analytics?/.test(normalizedQuery)) {
    return {
      answer:
        "Use GET /v1/accounts/{account_id}/metrics for YouTube V1 account metrics. This uses the existing youtube.readonly OAuth scope and does not require a new UniPost API key scope. Read data.follower_count for subscribers, data.post_count for public video count, and data.platform_specific.view_count for channel views. Use the separate YouTube Analytics V2 endpoints when you need date-ranged reports.",
      steps: [
        "List accounts with GET /v1/accounts and choose the connected YouTube account id.",
        "Call GET /v1/accounts/{account_id}/metrics.",
        "Read data.follower_count, data.post_count, and data.platform_specific.view_count.",
        "If you need summary, daily trend, or top-video reports, use /youtube/analytics/summary, /youtube/analytics/trend, or /youtube/analytics/videos with yt-analytics.readonly.",
      ],
    };
  }

  if (normalizedQuery.includes("export") || normalizedQuery.includes("csv")) {
    return {
      answer:
        "Use GET /v1/analytics/posts/export when you need normalized post analytics rows as CSV across multiple UniPost-published posts. Add date filters, then optionally filter by platform, account_id, profile_id, post_id, status, or sort.",
      steps: [
        "Choose the reporting window with from and to.",
        "Add optional filters such as platform, account_id, profile_id, or status.",
        "Call GET /v1/analytics/posts/export and save the CSV response.",
      ],
    };
  }

  if (normalizedQuery.includes("account") && normalizedQuery.includes("field")) {
    return {
      answer:
        "The account metrics response is normalized around data.social_account_id, data.platform, data.follower_count, data.following_count, data.post_count, data.platform_specific, and data.fetched_at.",
      steps: [
        "Call GET /v1/accounts/{account_id}/metrics for the connected account.",
        "Read normalized fields first.",
        "Use data.platform_specific only for provider-native additions or upstream diagnostics.",
      ],
    };
  }

  return null;
}

function answerForKnownTask(query: string, search: DocsAiSearchResult) {
  if (search.intent === "connect" || search.intent === "reference") {
    const connectAnswer = answerForConnectTask(query, search);
    if (connectAnswer && search.hits.some((hit) => hit.chunk.intent_tags.includes("connect"))) return connectAnswer;
  }

  if (search.intent === "credentials") {
    const credentialsAnswer = answerForCredentialsTask(query);
    if (credentialsAnswer) return credentialsAnswer;
  }

  if (search.intent === "analytics" || search.intent === "reference") {
    const analyticsAnswer = answerForAnalyticsTask(query);
    if (analyticsAnswer) return analyticsAnswer;
  }

  return null;
}

export function buildGroundedDocsAnswer(query: string, search = searchDocsIndex(query)): GroundedDocsAnswer {
  const answerHits = orderedHitsForAnswer(query, search);
  const candidateSources = answerHits.slice(0, 4).map(sourceFromHit);
  const related = answerHits.slice(4, 7).map(sourceFromHit);

  if (isInsufficientCoverage(search)) {
    return {
      answer:
        "I could not find enough source coverage in the docs to answer that confidently. Try the related docs below or file a missing-docs report from this search panel.",
      steps: [],
      confidence: "none",
      sources: [],
      related: candidateSources,
      generated_by: "fallback",
      coverage_reason: search.coverage_reason ?? "not enough source coverage",
    };
  }

  const taskAnswer = answerForKnownTask(query, search);

  return {
    answer: taskAnswer?.answer ?? "The docs contain relevant source coverage, but no deterministic task answer matched this query. Use the cited sources below.",
    steps: taskAnswer?.steps ?? [],
    confidence: search.confidence,
    sources: candidateSources.slice(0, 3),
    related,
    generated_by: "extractive",
    coverage_reason: search.coverage_reason,
  };
}
