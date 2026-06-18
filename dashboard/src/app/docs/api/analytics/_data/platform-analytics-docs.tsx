"use client";

import type { ApiFieldItem } from "../../_components/doc-components";

export type PlatformAnalyticsEndpointId =
  | "profile"
  | "accountMetrics"
  | "media"
  | "posts"
  | "boards"
  | "pageAnalytics"
  | "pageInsights"
  | "postAnalytics";

type Method = "GET" | "POST" | "PATCH" | "DELETE";

export type PlatformAnalyticsEndpointDoc = {
  id: PlatformAnalyticsEndpointId;
  label: string;
  href: string;
  method: Method;
  path: string;
  description: string;
  scopeNote: string;
  requestSections: Array<{ title: string; items: ApiFieldItem[] }>;
  responses: Array<{ code: string; fields: ApiFieldItem[] }>;
  snippets: Array<{ lang: string; label: string; code: string }>;
  responseSnippets: Array<{ lang: string; label: string; code: string }>;
};

export type PlatformAnalyticsDoc = {
  slug: "instagram" | "threads" | "pinterest" | "facebook";
  label: string;
  platformName: string;
  title: string;
  description: string;
  productionReadiness: string;
  scopes: string[];
  endpoints: PlatformAnalyticsEndpointDoc[];
};

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const ACCOUNT_PATH_FIELDS = (platform: string): ApiFieldItem[] => [
  { name: "account_id", type: "string", description: `Connected ${platform} social account ID.` },
];

const POST_PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "UniPost post ID." },
];

const LIMIT_QUERY_FIELDS: ApiFieldItem[] = [
  { name: "limit?", type: "number", description: "Maximum rows to return. Defaults to 20 and caps at 50." },
];

const DAYS_QUERY_FIELDS: ApiFieldItem[] = [
  { name: "days?", type: "number", description: "Lookback window in days. Defaults to 28 and caps at 92." },
];

const DAYS_AND_LIMIT_QUERY_FIELDS: ApiFieldItem[] = [
  { name: "days?", type: "number", description: "Lookback window in days. Defaults to 28 and caps at 92." },
  { name: "limit?", type: "number", description: "Maximum Page posts to return. Defaults to 12 and caps at 50." },
];

const ACCOUNT_METRICS_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "social_account_id", type: "string", description: "UniPost account ID." },
  { name: "platform", type: "string", description: "Normalized platform name." },
  { name: "follower_count", type: "number", description: "Followers reported by the upstream platform." },
  { name: "following_count", type: "number", description: "Accounts this account is following, when the platform returns it." },
  { name: "post_count", type: "number", description: "Lifetime post or media count returned by the platform." },
  { name: "platform_specific", type: "object", description: "Platform-native fields that do not fit the normalized counters." },
  { name: "fetched_at", type: "string", description: "UTC timestamp when UniPost fetched the metrics." },
];

const POST_ANALYTICS_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "UniPost post ID." },
  { name: "results[]", type: "array", description: "Per-destination publish results and analytics snapshots." },
  { name: "results[].platform", type: "string", description: "Destination platform." },
  { name: "results[].analytics", type: "object", description: "Normalized impressions, reach, likes, comments, shares, saves, clicks, video views, and engagement rate where available." },
  { name: "results[].platform_specific", type: "object", description: "Native fields preserved for platform-specific reporting." },
  { name: "fetched_at", type: "string", description: "UTC timestamp when UniPost returned the snapshot." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "UNAUTHORIZED, NOT_FOUND, WRONG_PLATFORM, NEEDS_RECONNECT, VALIDATION_ERROR, NOT_SUPPORTED, or UPSTREAM_ERROR." },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

function authAndAccount(platform: string) {
  return [
    { title: "Authorization", items: AUTH_FIELDS },
    { title: "Path Params", items: ACCOUNT_PATH_FIELDS(platform) },
  ];
}

function curl(path: string) {
  return `curl "https://api.unipost.dev${path}" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`;
}

function fetchSnippet(path: string, resultName: string) {
  return `const res = await fetch("https://api.unipost.dev${path}", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const ${resultName} = await res.json();`;
}

function accountMetricsEndpoint(platform: string, href: string, label: string, scopeNote: string, samplePlatform: string): PlatformAnalyticsEndpointDoc {
  const path = "/v1/accounts/:account_id/metrics";
  return {
    id: "accountMetrics",
    label,
    href,
    method: "GET",
    path,
    description: `Returns live account-level metrics for one connected ${platform} account.`,
    scopeNote,
    requestSections: authAndAccount(platform),
    responses: [
      { code: "200", fields: ACCOUNT_METRICS_RESPONSE_FIELDS },
      { code: "401", fields: ERROR_FIELDS },
      { code: "404", fields: ERROR_FIELDS },
      { code: "409", fields: ERROR_FIELDS },
      { code: "501", fields: ERROR_FIELDS },
      { code: "502", fields: ERROR_FIELDS },
    ],
    snippets: [
      { lang: "curl", label: "cURL", code: curl(`/v1/accounts/sa_${samplePlatform}_123/metrics`) },
      { lang: "js", label: "Node.js", code: fetchSnippet(`/v1/accounts/sa_${samplePlatform}_123/metrics`, "metrics") },
    ],
    responseSnippets: [
      {
        lang: "json",
        label: "200",
        code: `{
  "data": {
    "social_account_id": "sa_${samplePlatform}_123",
    "platform": "${samplePlatform}",
    "follower_count": 48620,
    "following_count": 318,
    "post_count": 328,
    "platform_specific": {},
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`,
      },
    ],
  };
}

function postAnalyticsEndpoint(platform: string, href: string, label: string, samplePlatform: string): PlatformAnalyticsEndpointDoc {
  const path = "/v1/posts/:post_id/analytics";
  return {
    id: "postAnalytics",
    label,
    href,
    method: "GET",
    path,
    description: `Returns normalized analytics for ${platform} destinations published through UniPost.`,
    scopeNote: "Uses the post analytics pipeline for UniPost-published content.",
    requestSections: [
      { title: "Authorization", items: AUTH_FIELDS },
      { title: "Path Params", items: POST_PATH_FIELDS },
    ],
    responses: [
      { code: "200", fields: POST_ANALYTICS_RESPONSE_FIELDS },
      { code: "401", fields: ERROR_FIELDS },
      { code: "404", fields: ERROR_FIELDS },
      { code: "502", fields: ERROR_FIELDS },
    ],
    snippets: [
      { lang: "curl", label: "cURL", code: curl("/v1/posts/post_abc123/analytics") },
      { lang: "js", label: "Node.js", code: fetchSnippet("/v1/posts/post_abc123/analytics", "analytics") },
    ],
    responseSnippets: [
      {
        lang: "json",
        label: "200",
        code: `{
  "data": {
    "post_id": "post_abc123",
    "results": [
      {
        "platform": "${samplePlatform}",
        "analytics": {
          "likes": 612,
          "comments": 38,
          "shares": 91,
          "engagement_rate": 0.0641
        },
        "platform_specific": {}
      }
    ],
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`,
      },
    ],
  };
}

export const platformAnalyticsDocs: Record<PlatformAnalyticsDoc["slug"], PlatformAnalyticsDoc> = {
  instagram: {
    slug: "instagram",
    label: "Instagram Analytics",
    platformName: "Instagram",
    title: "Instagram Analytics",
    description: "Optional native drilldown for Instagram Business profile details, live account metrics, recent media insight rows, and UniPost-published Instagram post performance.",
    productionReadiness: "Public-ready for connected Instagram Business accounts with instagram_business_basic and instagram_business_manage_insights granted.",
    scopes: ["instagram_business_basic", "instagram_business_manage_insights"],
    endpoints: [
      {
        id: "profile",
        label: "Get Instagram profile",
        href: "/docs/api/analytics/instagram/profile",
        method: "GET",
        path: "/v1/accounts/:account_id/instagram/profile",
        description: "Returns Instagram Business profile identity and profile-level counters.",
        scopeNote: "Requires instagram_business_basic and an active Instagram Business connection.",
        requestSections: authAndAccount("Instagram"),
        responses: [
          { code: "200", fields: [
            { name: "social_account_id", type: "string", description: "UniPost account ID." },
            { name: "platform", type: "string", description: "Always instagram." },
            { name: "id", type: "string", description: "Instagram user ID." },
            { name: "username", type: "string", description: "Instagram username." },
            { name: "profile_picture_url", type: "string", description: "Profile picture URL." },
            { name: "followers_count", type: "number", description: "Follower count." },
            { name: "follows_count", type: "number", description: "Following count." },
            { name: "media_count", type: "number", description: "Media count." },
            { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_instagram_123/instagram/profile") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_instagram_123/instagram/profile", "profile") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "social_account_id": "sa_instagram_123",
    "platform": "instagram",
    "id": "17841400000000000",
    "username": "studioalex",
    "profile_picture_url": "https://graph.instagram.com/profile.jpg",
    "followers_count": 48620,
    "follows_count": 318,
    "media_count": 328,
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`,
          },
        ],
      },
      accountMetricsEndpoint("Instagram", "/docs/api/analytics/instagram/account-metrics", "Get Instagram account metrics", "Requires instagram_business_basic; metrics are fetched live from Instagram.", "instagram"),
      {
        id: "media",
        label: "List Instagram media analytics",
        href: "/docs/api/analytics/instagram/media",
        method: "GET",
        path: "/v1/accounts/:account_id/instagram/media",
        description: "Returns recent Instagram media with reach, likes, comments, shares, saves, and media links.",
        scopeNote: "Requires instagram_business_manage_insights for native media metrics.",
        requestSections: [...authAndAccount("Instagram"), { title: "Query Params", items: LIMIT_QUERY_FIELDS }],
        responses: [
          { code: "200", fields: [
            { name: "media[]", type: "array", description: "Recent Instagram media rows." },
            { name: "media[].id", type: "string", description: "Instagram media ID." },
            { name: "media[].caption", type: "string", description: "Media caption." },
            { name: "media[].media_type", type: "string", description: "IMAGE, VIDEO, CAROUSEL_ALBUM, or REELS." },
            { name: "media[].permalink", type: "string", description: "Instagram permalink." },
            { name: "media[].reach", type: "number", description: "Reach from media insights." },
            { name: "media[].like_count", type: "number", description: "Like count." },
            { name: "media[].comments_count", type: "number", description: "Comment count." },
            { name: "media[].shares", type: "number", description: "Share count." },
            { name: "media[].saves", type: "number", description: "Save count." },
            { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
            { name: "limit", type: "number", description: "Limit applied to the request." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_instagram_123/instagram/media?limit=20") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_instagram_123/instagram/media?limit=20", "media") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "media": [
      {
        "id": "ig_1806218473",
        "caption": "Launch carousel",
        "media_type": "CAROUSEL_ALBUM",
        "permalink": "https://www.instagram.com/p/abc123/",
        "reach": 14800,
        "like_count": 1100,
        "comments_count": 86,
        "shares": 143,
        "saves": 392
      }
    ],
    "fetched_at": "2026-06-18T18:30:00Z",
    "limit": 20
  }
}`,
          },
        ],
      },
      postAnalyticsEndpoint("Instagram", "/docs/api/analytics/posts", "Instagram post analytics", "instagram"),
    ],
  },
  threads: {
    slug: "threads",
    label: "Threads Analytics",
    platformName: "Threads",
    title: "Threads Analytics",
    description: "Optional native drilldown for Threads profile details, live account metrics, recent post insights, and UniPost-published Threads post performance.",
    productionReadiness: "Public-ready for connected Threads profiles with threads_basic and threads_manage_insights granted.",
    scopes: ["threads_basic", "threads_manage_insights"],
    endpoints: [
      {
        id: "profile",
        label: "Get Threads profile",
        href: "/docs/api/analytics/threads/profile",
        method: "GET",
        path: "/v1/accounts/:account_id/threads/profile",
        description: "Returns Threads profile identity for one connected account.",
        scopeNote: "Requires threads_basic.",
        requestSections: authAndAccount("Threads"),
        responses: [
          { code: "200", fields: [
            { name: "social_account_id", type: "string", description: "UniPost account ID." },
            { name: "platform", type: "string", description: "Always threads." },
            { name: "id", type: "string", description: "Threads user ID." },
            { name: "username", type: "string", description: "Threads username." },
            { name: "threads_profile_picture_url", type: "string", description: "Profile picture URL." },
            { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_threads_123/threads/profile") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_threads_123/threads/profile", "profile") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "social_account_id": "sa_threads_123",
    "platform": "threads",
    "id": "17841400000000000",
    "username": "studioalex",
    "threads_profile_picture_url": "https://graph.threads.net/profile.jpg",
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`,
          },
        ],
      },
      accountMetricsEndpoint("Threads", "/docs/api/analytics/threads/account-metrics", "Get Threads account metrics", "Requires threads_manage_insights for account insights.", "threads"),
      {
        id: "posts",
        label: "List Threads post analytics",
        href: "/docs/api/analytics/threads/posts",
        method: "GET",
        path: "/v1/accounts/:account_id/threads/posts",
        description: "Returns recent Threads posts with views, likes, replies, reposts, quotes, and shares.",
        scopeNote: "Requires threads_manage_insights.",
        requestSections: [...authAndAccount("Threads"), { title: "Query Params", items: LIMIT_QUERY_FIELDS }],
        responses: [
          { code: "200", fields: [
            { name: "posts[]", type: "array", description: "Recent Threads post rows." },
            { name: "posts[].id", type: "string", description: "Threads post ID." },
            { name: "posts[].text", type: "string", description: "Post text." },
            { name: "posts[].permalink", type: "string", description: "Threads permalink." },
            { name: "posts[].views", type: "number", description: "View count." },
            { name: "posts[].likes", type: "number", description: "Like count." },
            { name: "posts[].replies", type: "number", description: "Reply count." },
            { name: "posts[].reposts", type: "number", description: "Repost count." },
            { name: "posts[].quotes", type: "number", description: "Quote count." },
            { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
            { name: "limit", type: "number", description: "Limit applied to the request." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_threads_123/threads/posts?limit=20") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_threads_123/threads/posts?limit=20", "posts") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "posts": [
      {
        "id": "threads_1805129771",
        "text": "Analytics API launch thread",
        "permalink": "https://www.threads.net/@studioalex/post/abc123",
        "views": 18600,
        "likes": 1400,
        "replies": 132,
        "reposts": 284,
        "quotes": 61
      }
    ],
    "fetched_at": "2026-06-18T18:30:00Z",
    "limit": 20
  }
}`,
          },
        ],
      },
      postAnalyticsEndpoint("Threads", "/docs/api/analytics/posts", "Threads post analytics", "threads"),
    ],
  },
  pinterest: {
    slug: "pinterest",
    label: "Pinterest Analytics",
    platformName: "Pinterest",
    title: "Pinterest Analytics",
    description: "Optional native drilldown for Pinterest board inventory and UniPost-published Pin performance, including impressions, saves, outbound clicks, and comments.",
    productionReadiness: "Public-ready for connected Pinterest accounts with pins:read, boards:read, and user_accounts:read granted.",
    scopes: ["pins:read", "boards:read", "user_accounts:read"],
    endpoints: [
      {
        id: "boards",
        label: "List Pinterest boards",
        href: "/docs/api/analytics/pinterest/boards",
        method: "GET",
        path: "/v1/accounts/:account_id/pinterest/boards",
        description: "Returns Pinterest boards available to the connected account.",
        scopeNote: "Requires boards:read and user_accounts:read.",
        requestSections: authAndAccount("Pinterest"),
        responses: [
          { code: "200", fields: [
            { name: "boards[]", type: "array", description: "Pinterest board rows." },
            { name: "boards[].id", type: "string", description: "Board ID." },
            { name: "boards[].name", type: "string", description: "Board name." },
            { name: "boards[].description", type: "string", description: "Board description." },
            { name: "sandbox_mode", type: "boolean", description: "Whether the Pinterest adapter is using sandbox mode." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_pinterest_123/pinterest/boards") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_pinterest_123/pinterest/boards", "boards") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "boards": [
      {
        "id": "1107111520928571145",
        "name": "Product Marketing",
        "description": "Launch assets and workflow diagrams"
      }
    ],
    "sandbox_mode": false
  }
}`,
          },
        ],
      },
      postAnalyticsEndpoint("Pinterest", "/docs/api/analytics/pinterest/post-analytics", "Pinterest post analytics", "pinterest"),
    ],
  },
  facebook: {
    slug: "facebook",
    label: "Facebook Page Analytics",
    platformName: "Facebook Page",
    title: "Facebook Page Analytics",
    description: "Optional native drilldown for Facebook Page profile data, Page Insights, recent Page posts, and UniPost-published Facebook post performance.",
    productionReadiness: "Public-ready for connected Facebook Pages with pages_read_engagement granted; read_insights unlocks Page-level insight fields.",
    scopes: ["pages_read_engagement", "read_insights"],
    endpoints: [
      {
        id: "pageAnalytics",
        label: "Get Facebook Page analytics",
        href: "/docs/api/analytics/facebook/page-analytics",
        method: "GET",
        path: "/v1/accounts/:account_id/facebook/page-analytics",
        description: "Returns Page profile, Page Insights when available, recent Page posts, and per-post engagement.",
        scopeNote: "Requires pages_read_engagement. read_insights is recommended for Page Insights.",
        requestSections: [...authAndAccount("Facebook Page"), { title: "Query Params", items: DAYS_AND_LIMIT_QUERY_FIELDS }],
        responses: [
          { code: "200", fields: [
            { name: "social_account_id", type: "string", description: "UniPost account ID." },
            { name: "platform", type: "string", description: "Always facebook." },
            { name: "page", type: "object", description: "Facebook Page profile." },
            { name: "insights", type: "object", description: "Page Insights for the requested window, when available." },
            { name: "posts[]", type: "array", description: "Recent Page posts with engagement metrics." },
            { name: "required_scopes[]", type: "array", description: "Required scopes, including pages_read_engagement." },
            { name: "recommended_scopes[]", type: "array", description: "Recommended scopes, including read_insights." },
            { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_facebook_123/facebook/page-analytics?days=28&limit=12") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_facebook_123/facebook/page-analytics?days=28&limit=12", "analytics") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "social_account_id": "sa_facebook_123",
    "platform": "facebook",
    "page": {
      "id": "1029384756",
      "name": "Studio Alex"
    },
    "insights": {
      "follows": 421,
      "impressions": 28420,
      "views": 7110,
      "post_engagements": 1852
    },
    "posts": [
      {
        "id": "1029384756_555",
        "message": "Launch recap",
        "likes": 312,
        "comments": 24,
        "shares": 19,
        "clicks": 81,
        "engagement_total": 436
      }
    ],
    "required_scopes": ["pages_read_engagement"],
    "recommended_scopes": ["read_insights"],
    "fetched_at": "2026-06-18T18:30:00Z"
  }
}`,
          },
        ],
      },
      {
        id: "pageInsights",
        label: "Get Facebook Page insights",
        href: "/docs/api/analytics/facebook/page-insights",
        method: "GET",
        path: "/v1/accounts/:account_id/facebook/page-insights",
        description: "Returns Page-level follows, impressions, views, and post engagements for a lookback window.",
        scopeNote: "Requires read_insights. Pages below Meta's 100-like insight threshold return zeroed metrics with below_100_likes_notice.",
        requestSections: [...authAndAccount("Facebook Page"), { title: "Query Params", items: DAYS_QUERY_FIELDS }],
        responses: [
          { code: "200", fields: [
            { name: "follows", type: "number", description: "Page follows in the window." },
            { name: "impressions", type: "number", description: "Page impressions in the window." },
            { name: "views", type: "number", description: "Page views in the window." },
            { name: "post_engagements", type: "number", description: "Page post engagements in the window." },
            { name: "below_100_likes_notice", type: "boolean", description: "True when Meta suppresses insights because the Page is below the threshold." },
            { name: "since", type: "string", description: "Window start timestamp." },
            { name: "until", type: "string", description: "Window end timestamp." },
          ] },
          { code: "401", fields: ERROR_FIELDS },
          { code: "404", fields: ERROR_FIELDS },
          { code: "409", fields: ERROR_FIELDS },
          { code: "502", fields: ERROR_FIELDS },
        ],
        snippets: [
          { lang: "curl", label: "cURL", code: curl("/v1/accounts/sa_facebook_123/facebook/page-insights?days=28") },
          { lang: "js", label: "Node.js", code: fetchSnippet("/v1/accounts/sa_facebook_123/facebook/page-insights?days=28", "insights") },
        ],
        responseSnippets: [
          {
            lang: "json",
            label: "200",
            code: `{
  "data": {
    "follows": 421,
    "impressions": 28420,
    "views": 7110,
    "post_engagements": 1852,
    "below_100_likes_notice": false,
    "since": "2026-05-21T18:30:00Z",
    "until": "2026-06-18T18:30:00Z"
  }
}`,
          },
        ],
      },
      postAnalyticsEndpoint("Facebook Page", "/docs/api/analytics/facebook/post-analytics", "Facebook Page post analytics", "facebook"),
    ],
  },
};
