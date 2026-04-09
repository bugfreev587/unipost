// Platform landing page configs — single source of truth for all 7 pages.
// Analytics metrics validated against dashboard/src/lib/platform-capabilities.ts.

export interface PlatformConfig {
  name: string;
  slug: string;
  icon: string;
  brandColor: string;
  heroTitle: string;
  heroSub: string;
  contentTypes: string[];

  capabilities: {
    icon: string;
    title: string;
    desc: string;
  }[];

  codeExample: {
    js: string;
    python: string;
    curl: string;
  };

  alternatingFeatures: {
    num: string;
    title: string;
    desc: string;
    placeholderIcon: string;
    placeholderLabel: string;
  }[];

  whyNot: {
    without: string[];
    with: string[];
  };

  // Bluesky uses appPassword instead of quickstart/native
  modes:
    | { type: "dual"; quickstartDesc: string; nativeDesc: string; quickstartFeats: string[]; nativeFeats: string[] }
    | { type: "appPassword"; desc: string; features: string[] };

  metrics: { label: string; sampleValue: string }[];

  faq: { q: string; a: string }[];

  seo: {
    title: string;
    description: string;
    keywords: string[];
  };

  // Instagram-specific: waitlist mode
  waitlist?: boolean;
}

// ────────────────────────────────────────────────────────────────────
// Instagram
// ────────────────────────────────────────────────────────────────────
export const instagram: PlatformConfig = {
  name: "Instagram",
  slug: "instagram",
  icon: "📸",
  brandColor: "#E1306C",
  heroTitle: "Instagram API\nfor Developers",
  heroSub:
    "Post photos, videos, and Reels to Instagram programmatically. UniPost handles Meta OAuth, long-lived token refresh, and business account verification automatically.",
  contentTypes: ["Photos", "Videos", "Reels"],
  waitlist: true,

  capabilities: [
    { icon: "🖼️", title: "Photos & Videos", desc: "Post single images, videos, and Reels to Instagram Business and Creator accounts." },
    { icon: "🔄", title: "Token Auto-refresh", desc: "Meta long-lived tokens are refreshed automatically in the background. No manual intervention needed." },
    { icon: "📊", title: "Post Analytics", desc: "Read reach, likes, comments, shares, and saves from every published post." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Hello from UniPost! 🚀',
      account_ids: ['sa_instagram_123'],
      media_urls: ['https://example.com/photo.jpg']
    })
  }
);

// Response
{
  "data": {
    "id": "post_abc123",
    "status": "published",
    "results": [{
      "platform": "instagram",
      "status": "published"
    }]
  }
}`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Hello from UniPost! 🚀',
        'account_ids': ['sa_instagram_123'],
        'media_urls': ['https://example.com/photo.jpg'],
    }
)

data = response.json()['data']
print(data['id'])  # post_abc123`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello from UniPost! 🚀",
    "account_ids": ["sa_instagram_123"],
    "media_urls": ["https://example.com/photo.jpg"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "Media Upload — Any URL",
      desc: "Pass any publicly accessible image or video URL. UniPost downloads the file and uploads it to Meta's media servers automatically. No need to host files on a verified domain.",
      placeholderIcon: "🖼️",
      placeholderLabel: "API response showing successful Instagram image post",
    },
    {
      num: "02",
      title: "60-Day Token Auto-refresh",
      desc: "Meta's long-lived tokens expire every 60 days. UniPost's background worker refreshes them automatically before expiry. Your posts keep working — forever.",
      placeholderIcon: "🔄",
      placeholderLabel: "Dashboard showing Instagram account Active status",
    },
  ],

  whyNot: {
    without: [
      "Apply for Meta developer access (weeks)",
      "Implement Meta OAuth 2.0 from scratch",
      "Handle 60-day token refresh manually",
      "Learn Meta's media upload quirks",
      "Build error handling for Meta's error codes",
      "Maintain when Meta changes their API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth handled completely automatically",
      "Token refresh runs in background forever",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's Meta developer credentials. No approval process, no waiting. Start posting immediately.",
    nativeDesc: "Use your own Meta developer app. Users see your app name during OAuth. Full brand ownership.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated against platform-capabilities.ts: reach, likes, comments, shares, saves (no impressions, no clicks)
  metrics: [
    { label: "Reach", sampleValue: "38.4k" },
    { label: "Likes", sampleValue: "2.9k" },
    { label: "Comments", sampleValue: "456" },
    { label: "Shares", sampleValue: "789" },
    { label: "Saves", sampleValue: "1.2k" },
  ],

  faq: [
    { q: "Do I need to apply for Meta developer access?", a: "In Quickstart mode, no. UniPost's approved Meta app handles everything. In Native mode, you'll need your own Meta app with instagram_content_publish scope." },
    { q: "Does UniPost support Instagram personal accounts?", a: "No. Instagram's API only supports Business and Creator accounts. You can convert a personal account to a Business account in the Instagram app for free." },
    { q: "How long does setup take?", a: "In Quickstart mode: about 5 minutes. Connect your Instagram Business account via OAuth and you're ready to post via API." },
    { q: "What happens when my token expires?", a: "UniPost's background worker automatically refreshes Meta tokens before they expire (every 60 days). You don't need to do anything." },
    { q: "Can I post Instagram Stories?", a: "Not yet. Currently supports feed posts, videos, and Reels. Story support is on the roadmap." },
    { q: "Is Instagram included in the free plan?", a: "Yes. The free plan includes 100 posts/month to all 7 platforms including Instagram. No credit card required." },
  ],

  seo: {
    title: "Instagram API for Developers — Post Photos & Videos | UniPost",
    description: "Post photos, videos, and Reels to Instagram programmatically. UniPost handles Meta OAuth, token refresh, and business account setup. Free plan available.",
    keywords: ["instagram api for developers", "post to instagram api", "instagram content publish api", "instagram business api python", "instagram api nodejs"],
  },
};

// ────────────────────────────────────────────────────────────────────
// LinkedIn
// ────────────────────────────────────────────────────────────────────
export const linkedin: PlatformConfig = {
  name: "LinkedIn",
  slug: "linkedin",
  icon: "💼",
  brandColor: "#0A66C2",
  heroTitle: "LinkedIn API\nfor Developers",
  heroSub:
    "Publish text posts, images, and articles to LinkedIn programmatically. Perfect for SaaS products, content tools, and B2B marketing automation.",
  contentTypes: ["Text Posts", "Images", "Articles"],

  capabilities: [
    { icon: "📝", title: "Text & Image Posts", desc: "Post text with images to personal profiles and company pages via a single API call." },
    { icon: "🏢", title: "Organization Pages", desc: "Post to personal profiles and company pages. Just include the right account_id." },
    { icon: "🔄", title: "Token Auto-refresh", desc: "LinkedIn tokens are refreshed automatically. Connect once, post forever." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Excited to announce our new API! 🚀',
      account_ids: ['sa_linkedin_456'],
      media_urls: ['https://example.com/banner.jpg']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Excited to announce our new API! 🚀',
        'account_ids': ['sa_linkedin_456'],
        'media_urls': ['https://example.com/banner.jpg'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Excited to announce our new API! 🚀",
    "account_ids": ["sa_linkedin_456"],
    "media_urls": ["https://example.com/banner.jpg"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "Rich Text Posts",
      desc: "LinkedIn supports long-form text content. Post professional updates, thought leadership pieces, and company announcements with full formatting support.",
      placeholderIcon: "📝",
      placeholderLabel: "LinkedIn post published via UniPost API",
    },
    {
      num: "02",
      title: "Image Attachments",
      desc: "Attach single images to your LinkedIn posts. Pass any public URL — UniPost handles the upload to LinkedIn's media servers automatically.",
      placeholderIcon: "🖼️",
      placeholderLabel: "LinkedIn image post API response",
    },
  ],

  whyNot: {
    without: [
      "Apply for LinkedIn developer access",
      "Implement LinkedIn's OAuth 2.0 flow",
      "Handle token refresh on your own",
      "Learn LinkedIn's media upload process",
      "Parse LinkedIn-specific error responses",
      "Maintain when LinkedIn updates their API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth handled completely automatically",
      "Token refresh runs in background forever",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's LinkedIn developer credentials. No approval process, no waiting.",
    nativeDesc: "Use your own LinkedIn developer app. Users see your app name during OAuth.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated: impressions, reach, likes, comments, shares, clicks
  metrics: [
    { label: "Impressions", sampleValue: "24.1k" },
    { label: "Reach", sampleValue: "18.7k" },
    { label: "Likes", sampleValue: "1.4k" },
    { label: "Comments", sampleValue: "312" },
    { label: "Shares", sampleValue: "189" },
    { label: "Clicks", sampleValue: "2.3k" },
  ],

  faq: [
    { q: "Can I post to LinkedIn Company Pages?", a: "Yes, if your account has admin access to the page. Connect the page via OAuth and use its account_id in API calls." },
    { q: "Does LinkedIn support video posts?", a: "Not yet in the current version. Text and image posts are fully supported. Video support is coming in a future update." },
    { q: "Do I need LinkedIn developer access?", a: "In Quickstart mode, no — UniPost's credentials handle everything. In Native mode, you'll need your own LinkedIn app." },
    { q: "How long does setup take?", a: "About 5 minutes in Quickstart mode. Connect your LinkedIn account via OAuth and start posting immediately." },
    { q: "What happens when my token expires?", a: "UniPost automatically refreshes LinkedIn tokens in the background. You never need to re-authenticate." },
    { q: "Is LinkedIn included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including LinkedIn." },
  ],

  seo: {
    title: "LinkedIn API for Developers — Post Text & Images | UniPost",
    description: "Publish text posts and images to LinkedIn programmatically. Perfect for B2B marketing automation. UniPost handles OAuth and token refresh. Free plan available.",
    keywords: ["linkedin api for developers", "post to linkedin api", "linkedin content api", "linkedin api python", "linkedin api nodejs"],
  },
};

// ────────────────────────────────────────────────────────────────────
// X / Twitter
// ────────────────────────────────────────────────────────────────────
export const twitter: PlatformConfig = {
  name: "X / Twitter",
  slug: "twitter",
  icon: "𝕏",
  brandColor: "#1D9BF0",
  heroTitle: "Twitter API\nfor Developers",
  heroSub:
    "Post tweets, threads, and images to X/Twitter programmatically. UniPost handles OAuth 2.0 PKCE, token refresh, and rate limits automatically.",
  contentTypes: ["Tweets", "Threads", "Images"],

  capabilities: [
    { icon: "💬", title: "Tweets & Threads", desc: "Post single tweets or multi-tweet threads using the thread_position field." },
    { icon: "🖼️", title: "Image & Video", desc: "Attach images and videos to tweets. UniPost handles media upload to Twitter automatically." },
    { icon: "⚡", title: "Rate Limit Handling", desc: "UniPost manages Twitter's rate limits. Your requests are queued and retried automatically." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Shipping a new feature today! 🚀',
      account_ids: ['sa_twitter_789'],
      media_urls: ['https://example.com/screenshot.png']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Shipping a new feature today! 🚀',
        'account_ids': ['sa_twitter_789'],
        'media_urls': ['https://example.com/screenshot.png'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Shipping a new feature today! 🚀",
    "account_ids": ["sa_twitter_789"],
    "media_urls": ["https://example.com/screenshot.png"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "Twitter Threads",
      desc: "Publish multi-tweet threads with the thread_position field. Each tweet in the thread is posted in order and linked automatically. Build narratives across multiple tweets.",
      placeholderIcon: "🧵",
      placeholderLabel: "Multi-tweet thread published via UniPost API",
    },
    {
      num: "02",
      title: "Media Support",
      desc: "Attach images and videos to tweets. Pass any public URL — UniPost downloads, processes, and uploads to Twitter's media endpoint for you.",
      placeholderIcon: "🖼️",
      placeholderLabel: "Tweet with image attachment published via API",
    },
  ],

  whyNot: {
    without: [
      "Implement OAuth 2.0 PKCE flow from scratch",
      "Handle token refresh and rotation",
      "Manage Twitter's complex rate limits",
      "Build media upload to Twitter's endpoints",
      "Parse Twitter's unique error format",
      "Maintain when Twitter changes their API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth 2.0 PKCE handled automatically",
      "Rate limits managed in background",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's Twitter developer credentials. No approval process, no waiting.",
    nativeDesc: "Use your own Twitter developer app. Users see your app name during OAuth.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated: impressions, likes, comments, shares
  metrics: [
    { label: "Impressions", sampleValue: "52.8k" },
    { label: "Likes", sampleValue: "3.1k" },
    { label: "Comments", sampleValue: "892" },
    { label: "Shares", sampleValue: "1.4k" },
  ],

  faq: [
    { q: "What Twitter API access does UniPost use?", a: "Twitter API access is included in your UniPost subscription. No separate Twitter developer plan required." },
    { q: "Can I post Twitter threads with UniPost?", a: "Yes. Use thread_position 1, 2, 3... to create threaded tweets. Each tweet is linked automatically." },
    { q: "Do I need my own Twitter developer account?", a: "In Quickstart mode, no. UniPost handles everything. In Native mode, you'll need your own Twitter app credentials." },
    { q: "How does UniPost handle rate limits?", a: "UniPost queues requests and retries automatically when rate limits are hit. You don't need to build any retry logic." },
    { q: "What happens when my token expires?", a: "UniPost refreshes Twitter tokens automatically in the background. No action needed on your end." },
    { q: "Is Twitter included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including Twitter." },
  ],

  seo: {
    title: "Twitter API for Developers — Post Tweets & Threads | UniPost",
    description: "Post tweets, threads, and images to X/Twitter programmatically. UniPost handles OAuth 2.0 PKCE, token refresh, and rate limits. Free plan available.",
    keywords: ["twitter api for developers", "post to twitter api", "tweet api", "twitter api python", "twitter api nodejs", "x api"],
  },
};

// ────────────────────────────────────────────────────────────────────
// TikTok
// ────────────────────────────────────────────────────────────────────
export const tiktok: PlatformConfig = {
  name: "TikTok",
  slug: "tiktok",
  icon: "🎵",
  brandColor: "#00F2EA",
  heroTitle: "TikTok API\nfor Developers",
  heroSub:
    "Upload and publish videos to TikTok programmatically. UniPost handles video file upload, direct posting, and creator account OAuth.",
  contentTypes: ["Videos"],

  capabilities: [
    { icon: "📹", title: "Video Upload", desc: "File-based upload — no need for publicly accessible video URLs or domain verification." },
    { icon: "📤", title: "Direct Post", desc: "Videos are published directly to the user's TikTok profile. No inbox or review step." },
    { icon: "🔐", title: "Creator Accounts", desc: "Supports authenticated TikTok creator accounts via OAuth." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Check out this new feature! #dev',
      account_ids: ['sa_tiktok_321'],
      media_urls: ['https://example.com/demo.mp4']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Check out this new feature! #dev',
        'account_ids': ['sa_tiktok_321'],
        'media_urls': ['https://example.com/demo.mp4'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Check out this new feature! #dev",
    "account_ids": ["sa_tiktok_321"],
    "media_urls": ["https://example.com/demo.mp4"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "File-based Upload",
      desc: "TikTok uses FILE_UPLOAD — no domain verification required. Pass a video URL and UniPost downloads, processes, and uploads the file directly to TikTok's servers.",
      placeholderIcon: "📹",
      placeholderLabel: "TikTok video upload API response",
    },
    {
      num: "02",
      title: "Direct Publishing",
      desc: "Videos are published directly to the user's TikTok profile — no inbox review step. Your users see the video live immediately after the API call.",
      placeholderIcon: "📤",
      placeholderLabel: "TikTok post published successfully",
    },
  ],

  whyNot: {
    without: [
      "Apply for TikTok developer access",
      "Implement TikTok's OAuth flow",
      "Handle chunked video upload protocol",
      "Manage publish status polling",
      "Build retry logic for upload failures",
      "Maintain when TikTok changes their API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth handled completely automatically",
      "Video upload managed end-to-end",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's TikTok developer credentials. No approval process, no waiting.",
    nativeDesc: "Use your own TikTok developer app. Users see your app name during OAuth.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated: likes, comments, shares, video_views
  metrics: [
    { label: "Video Views", sampleValue: "127k" },
    { label: "Likes", sampleValue: "8.4k" },
    { label: "Comments", sampleValue: "1.2k" },
    { label: "Shares", sampleValue: "3.6k" },
  ],

  faq: [
    { q: "Does UniPost support TikTok personal accounts?", a: "TikTok API requires creator accounts. Personal accounts are not supported by TikTok's API." },
    { q: "What video formats are supported?", a: "MP4 is recommended. Maximum file size is 500MB." },
    { q: "Do I need TikTok developer access?", a: "In Quickstart mode, no. UniPost handles everything. In Native mode, you'll need your own TikTok app." },
    { q: "How long does setup take?", a: "About 5 minutes in Quickstart mode. Connect a TikTok creator account via OAuth and start uploading." },
    { q: "What happens when my token expires?", a: "UniPost refreshes TikTok tokens automatically in the background." },
    { q: "Is TikTok included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including TikTok." },
  ],

  seo: {
    title: "TikTok API for Developers — Upload & Publish Videos | UniPost",
    description: "Upload and publish videos to TikTok programmatically. UniPost handles OAuth, video upload, and direct posting. Free plan available.",
    keywords: ["tiktok api for developers", "post to tiktok api", "tiktok video upload api", "tiktok api python", "tiktok api nodejs"],
  },
};

// ────────────────────────────────────────────────────────────────────
// YouTube
// ────────────────────────────────────────────────────────────────────
export const youtube: PlatformConfig = {
  name: "YouTube",
  slug: "youtube",
  icon: "▶️",
  brandColor: "#FF0000",
  heroTitle: "YouTube API\nfor Developers",
  heroSub:
    "Upload and publish videos to YouTube programmatically. Set titles, descriptions, tags, and privacy settings through a single API call.",
  contentTypes: ["Videos", "Shorts"],

  capabilities: [
    { icon: "📹", title: "Video Upload", desc: "Upload HD videos with full metadata — title, description, tags, and category." },
    { icon: "🏷️", title: "Metadata Control", desc: "Set title, description, tags, thumbnail, and category in a single API call." },
    { icon: "🔒", title: "Privacy Settings", desc: "Control visibility: public, private, or unlisted. Schedule for future publishing." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'How to build a social API',
      account_ids: ['sa_youtube_555'],
      media_urls: ['https://example.com/tutorial.mp4']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'How to build a social API',
        'account_ids': ['sa_youtube_555'],
        'media_urls': ['https://example.com/tutorial.mp4'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "How to build a social API",
    "account_ids": ["sa_youtube_555"],
    "media_urls": ["https://example.com/tutorial.mp4"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "Full Metadata Control",
      desc: "Set title, description, tags, and category in a single API call. UniPost maps your fields to YouTube's Data API v3 automatically.",
      placeholderIcon: "🏷️",
      placeholderLabel: "YouTube video with full metadata uploaded via API",
    },
    {
      num: "02",
      title: "Privacy & Scheduling",
      desc: "Control video visibility — public, private, or unlisted. Use the scheduled_at field to publish at a specific time.",
      placeholderIcon: "🔒",
      placeholderLabel: "YouTube video with scheduled publish time",
    },
  ],

  whyNot: {
    without: [
      "Implement Google OAuth 2.0 from scratch",
      "Handle resumable upload protocol",
      "Manage YouTube quota limits",
      "Build metadata mapping for Data API v3",
      "Handle video processing status polling",
      "Maintain when YouTube changes their API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth handled completely automatically",
      "Video upload managed end-to-end",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's Google developer credentials. No approval process, no waiting.",
    nativeDesc: "Use your own Google developer app. Users see your app name during OAuth.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated: likes, comments, video_views
  metrics: [
    { label: "Video Views", sampleValue: "89.2k" },
    { label: "Likes", sampleValue: "4.7k" },
    { label: "Comments", sampleValue: "623" },
  ],

  faq: [
    { q: "Can I upload videos longer than 15 minutes?", a: "Yes, if your YouTube account is verified for long-form content. UniPost does not impose additional length limits." },
    { q: "Does UniPost support YouTube Shorts?", a: "Yes. Shorts are vertical videos under 60 seconds uploaded via the same API endpoint." },
    { q: "Do I need Google developer access?", a: "In Quickstart mode, no. UniPost handles everything. In Native mode, you'll need your own Google OAuth app." },
    { q: "How long does setup take?", a: "About 5 minutes in Quickstart mode. Connect your YouTube channel via OAuth and start uploading." },
    { q: "What happens when my token expires?", a: "UniPost refreshes Google tokens automatically in the background." },
    { q: "Is YouTube included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including YouTube." },
  ],

  seo: {
    title: "YouTube API for Developers — Upload & Publish Videos | UniPost",
    description: "Upload and publish videos to YouTube programmatically. Set titles, descriptions, tags, and privacy settings. Free plan available.",
    keywords: ["youtube api for developers", "upload to youtube api", "youtube video upload api", "youtube api python", "youtube api nodejs"],
  },
};

// ────────────────────────────────────────────────────────────────────
// Bluesky
// ────────────────────────────────────────────────────────────────────
export const bluesky: PlatformConfig = {
  name: "Bluesky",
  slug: "bluesky",
  icon: "🦋",
  brandColor: "#0085FF",
  heroTitle: "Bluesky API\nfor Developers",
  heroSub:
    "Post text and images to Bluesky via the AT Protocol. Connect with App Passwords — no OAuth approval process, no developer account needed.",
  contentTypes: ["Text", "Images", "Threads"],

  capabilities: [
    { icon: "⚡", title: "Instant Setup", desc: "Connect with an App Password — 5 minutes, no approval process, no developer account needed." },
    { icon: "🧵", title: "Thread Support", desc: "Publish multi-post threads using the thread_position field. Build narratives across posts." },
    { icon: "🖼️", title: "Image Posts", desc: "Attach images to Bluesky posts. UniPost handles blob upload to the AT Protocol automatically." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Hello Bluesky from UniPost! 🦋',
      account_ids: ['sa_bluesky_101']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Hello Bluesky from UniPost! 🦋',
        'account_ids': ['sa_bluesky_101'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello Bluesky from UniPost! 🦋",
    "account_ids": ["sa_bluesky_101"]
  }'`,
  },

  alternatingFeatures: [
    {
      num: "01",
      title: "Thread Publishing",
      desc: "Use thread_position to publish multi-post threads. Each post is linked automatically via AT Protocol reply references. Build narratives, tutorials, and announcements.",
      placeholderIcon: "🧵",
      placeholderLabel: "Multi-post Bluesky thread via UniPost API",
    },
    {
      num: "02",
      title: "No Approval Needed",
      desc: "Bluesky is fully open. Generate an App Password at bsky.app, paste it into UniPost, and start posting in under 5 minutes. No developer account, no review process.",
      placeholderIcon: "⚡",
      placeholderLabel: "Dashboard showing Bluesky account connected instantly",
    },
  ],

  whyNot: {
    without: [
      "Learn the AT Protocol record format",
      "Implement blob upload for images",
      "Build facet detection for links and mentions",
      "Handle reply references for threads",
      "Manage session tokens and refresh",
      "Maintain when AT Protocol evolves",
    ],
    with: [
      "Connect in 5 minutes with App Password",
      "Text, images, and threads via one API",
      "Facets and links handled automatically",
      "Thread replies linked automatically",
      "Session management in background",
      "We handle protocol updates",
    ],
  },

  // Bluesky uses App Password — no Quickstart/Native concept
  modes: {
    type: "appPassword",
    desc: "Bluesky connects via App Passwords — a simple token you generate at bsky.app/settings/app-passwords. No OAuth app, no developer account, no approval process.",
    features: [
      "Generate App Password at bsky.app",
      "Paste into UniPost Dashboard",
      "Start posting immediately",
      "No developer account needed",
      "No approval or review process",
      "Works on Free plan",
    ],
  },

  // Validated: likes, comments, shares
  metrics: [
    { label: "Likes", sampleValue: "1.8k" },
    { label: "Replies", sampleValue: "342" },
    { label: "Reposts", sampleValue: "567" },
  ],

  faq: [
    { q: "Do I need a Bluesky developer account?", a: "No. Just an App Password from bsky.app/settings/app-passwords. No developer account or approval needed." },
    { q: "Does UniPost support Bluesky threads?", a: "Yes. Use the thread_position field to publish multi-post threads. Replies are linked automatically." },
    { q: "How long does setup take?", a: "About 2 minutes. Generate an App Password at bsky.app, paste it into the UniPost Dashboard, and start posting." },
    { q: "Can I attach images to Bluesky posts?", a: "Yes. Pass image URLs and UniPost handles blob upload to the AT Protocol automatically." },
    { q: "Does Bluesky have rate limits?", a: "AT Protocol has generous limits. UniPost manages them automatically so you don't need to worry." },
    { q: "Is Bluesky included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including Bluesky." },
  ],

  seo: {
    title: "Bluesky API for Developers — Post via AT Protocol | UniPost",
    description: "Post text, images, and threads to Bluesky via the AT Protocol. No developer account needed. Connect with App Passwords in 2 minutes. Free plan available.",
    keywords: ["bluesky api for developers", "post to bluesky api", "at protocol api", "bluesky api python", "bluesky api nodejs"],
  },
};

// ────────────────────────────────────────────────────────────────────
// Threads
// ────────────────────────────────────────────────────────────────────
export const threads: PlatformConfig = {
  name: "Threads",
  slug: "threads",
  icon: "🧵",
  brandColor: "#000000",
  heroTitle: "Threads API\nfor Developers",
  // TODO(threads-unified-oauth): update when unified Instagram+Threads connect ships
  heroSub:
    "Publish text posts and images to Threads programmatically. Connect Threads separately via its own OAuth flow. Uses the same Meta developer app as Instagram.",
  contentTypes: ["Text", "Images"],

  capabilities: [
    { icon: "📝", title: "Text & Image Posts", desc: "Post text and images to Threads. Supports up to 500 characters per post." },
    { icon: "🔗", title: "Meta OAuth", desc: "Connect via Threads' own OAuth flow. Uses the same Meta developer app as Instagram." },
    { icon: "🔄", title: "Unified API", desc: "Same API endpoint and format as every other platform. No Threads-specific code needed." },
  ],

  codeExample: {
    js: `const response = await fetch(
  'https://api.unipost.dev/v1/social-posts',
  {
    method: 'POST',
    headers: {
      'Authorization': 'Bearer up_live_xxx',
      'Content-Type': 'application/json'
    },
    body: JSON.stringify({
      caption: 'Hello Threads from UniPost! 🧵',
      account_ids: ['sa_threads_202']
    })
  }
);`,
    python: `import requests

response = requests.post(
    'https://api.unipost.dev/v1/social-posts',
    headers={
        'Authorization': 'Bearer up_live_xxx',
        'Content-Type': 'application/json',
    },
    json={
        'caption': 'Hello Threads from UniPost! 🧵',
        'account_ids': ['sa_threads_202'],
    }
)`,
    curl: `curl -X POST https://api.unipost.dev/v1/social-posts \\
  -H "Authorization: Bearer up_live_xxx" \\
  -H "Content-Type: application/json" \\
  -d '{
    "caption": "Hello Threads from UniPost! 🧵",
    "account_ids": ["sa_threads_202"]
  }'`,
  },

  alternatingFeatures: [
    {
      // TODO(threads-unified-oauth): update when unified connect ships
      num: "01",
      title: "Same Meta Developer App",
      desc: "Threads uses the same Meta developer app as Instagram. Connect Threads separately via its own OAuth flow — one app, two platforms.",
      placeholderIcon: "🔗",
      placeholderLabel: "Dashboard showing both Threads and Instagram connected",
    },
    {
      num: "02",
      title: "500-Character Posts",
      desc: "Threads supports up to 500 characters per post. Post text updates, announcements, and quick thoughts through the same API you use for every other platform.",
      placeholderIcon: "📝",
      placeholderLabel: "Threads post published via UniPost API",
    },
  ],

  whyNot: {
    without: [
      "Apply for Meta developer access",
      "Implement Threads OAuth flow",
      "Handle Meta token refresh",
      "Learn Threads-specific media format",
      "Build error handling for Meta API",
      "Maintain when Meta updates Threads API",
    ],
    with: [
      "Connect in 5 minutes (Quickstart mode)",
      "OAuth handled completely automatically",
      "Token refresh runs in background forever",
      "Unified media upload across all platforms",
      "Consistent error format across 7 platforms",
      "We handle breaking API changes",
    ],
  },

  modes: {
    type: "dual",
    quickstartDesc: "Use UniPost's Meta developer credentials. Connect Threads instantly, no approval needed.",
    nativeDesc: "Use your own Meta developer app. Users see your app name during the Threads OAuth flow.",
    quickstartFeats: ["Instant access, no approval needed", "OAuth shows \"UniPost\" branding", "Available on Free plan"],
    nativeFeats: ["OAuth shows your app name", "Complete credential ownership", "Paid plans only"],
  },

  // Validated: impressions, likes, comments, shares
  metrics: [
    { label: "Impressions", sampleValue: "15.3k" },
    { label: "Likes", sampleValue: "987" },
    { label: "Replies", sampleValue: "213" },
    { label: "Reposts", sampleValue: "156" },
  ],

  faq: [
    { q: "Do I need a separate developer app for Threads?", a: "No. Threads uses the same Meta developer app as Instagram. One app covers both platforms." },
    { q: "Can I cross-post to Threads and Instagram?", a: "Yes. Include both account IDs in a single API call to post the same content to both platforms simultaneously." },
    { q: "How long does setup take?", a: "About 5 minutes in Quickstart mode. Connect your Threads account via OAuth and start posting." },
    { q: "What's the character limit for Threads?", a: "500 characters per post. UniPost validates content length before posting." },
    { q: "What happens when my token expires?", a: "UniPost refreshes Meta tokens automatically in the background." },
    { q: "Is Threads included in the free plan?", a: "Yes. The free plan includes 100 posts/month across all 7 platforms including Threads." },
  ],

  seo: {
    title: "Threads API for Developers — Post Text & Images | UniPost",
    description: "Publish text posts and images to Threads programmatically. Uses the same Meta developer app as Instagram. Free plan available.",
    keywords: ["threads api for developers", "post to threads api", "threads content api", "threads api python", "meta threads api"],
  },
};

// ────────────────────────────────────────────────────────────────────
// All platforms (ordered for display / launch priority)
// ────────────────────────────────────────────────────────────────────
export const ALL_PLATFORMS: PlatformConfig[] = [
  instagram, linkedin, twitter, tiktok, youtube, bluesky, threads,
];

export function getPlatformBySlug(slug: string): PlatformConfig | undefined {
  return ALL_PLATFORMS.find((p) => p.slug === slug);
}
