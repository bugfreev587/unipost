"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected TikTok account ID such as sa_tiktok_123." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "cursor?", type: "number", description: "Cursor returned by the previous TikTok video.list response. Defaults to 0." },
  { name: "limit?", type: "number", description: "Maximum videos to return. Defaults to 20 and caps at 20." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "videos[]", type: "array", description: "Public videos returned by TikTok video.list." },
  { name: "videos[].id", type: "string", description: "TikTok video ID." },
  { name: "videos[].title", type: "string", description: "Video title when available." },
  { name: "videos[].video_description", type: "string", description: "Video description when available." },
  { name: "videos[].cover_image_url", type: "string", description: "Cover image URL." },
  { name: "videos[].share_url", type: "string", description: "Public share URL." },
  { name: "videos[].create_time", type: "number", description: "Unix timestamp in seconds." },
  { name: "videos[].view_count", type: "number", description: "View count returned by TikTok." },
  { name: "videos[].like_count", type: "number", description: "Like count returned by TikTok." },
  { name: "videos[].comment_count", type: "number", description: "Comment count returned by TikTok." },
  { name: "videos[].share_count", type: "number", description: "Share count returned by TikTok." },
  { name: "cursor", type: "number", description: "Cursor for the next request." },
  { name: "has_more", type: "boolean", description: "Whether TikTok has more videos after this page." },
  { name: "fetched_at", type: "string", description: "UTC timestamp when UniPost fetched the page." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'UNAUTHORIZED, FEATURE_DISABLED, NOT_FOUND, WRONG_PLATFORM, NEEDS_RECONNECT, or TIKTOK_ERROR.' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_tiktok_123/tiktok/videos?limit=20" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/accounts/sa_tiktok_123/tiktok/videos?limit=20", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const videos = await res.json();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "videos": [
      {
        "id": "7350123456789012345",
        "title": "Launch workflow in 30 seconds",
        "cover_image_url": "https://p16-sign-va.tiktokcdn.com/tos-maliva-cover.jpg",
        "share_url": "https://www.tiktok.com/@studioalex/video/7350123456789012345",
        "create_time": 1778544000,
        "view_count": 8200,
        "like_count": 612,
        "comment_count": 38,
        "share_count": 91,
        "duration": 28,
        "video_description": "Launch workflow in 30 seconds"
      }
    ],
    "cursor": 20,
    "has_more": true,
    "fetched_at": "2026-06-17T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "403",
    code: `{
  "error": {
    "code": "FEATURE_DISABLED",
    "normalized_code": "feature_disabled",
    "message": "TikTok analytics is not enabled in this environment."
  },
  "request_id": "req_123"
}`,
  },
];

export default function TikTokVideosAnalyticsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Analytics", href: "/docs/api/analytics/summary" },
        { label: "TikTok Analytics", href: "/docs/api/analytics/tiktok" },
        { label: "Public videos" },
      ]}
      section="analytics"
      title="List TikTok public videos"
      description="Returns owned public TikTok videos and engagement counters for one connected account. TikTok has approved video.list for production use; enable tiktok.analytics_scopes in the target environment to serve this public-ready endpoint."
      method="GET"
      path="/v1/accounts/:account_id/tiktok/videos"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
