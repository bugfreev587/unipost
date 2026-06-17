"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected TikTok account ID such as sa_tiktok_123." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "social_account_id", type: "string", description: "UniPost account ID." },
  { name: "platform", type: "string", description: 'Always "tiktok" for this guide.' },
  { name: "follower_count", type: "number", description: "Follower count from user.info.stats." },
  { name: "following_count", type: "number", description: "Following count from user.info.stats." },
  { name: "post_count", type: "number", description: "Public video count from user.info.stats." },
  { name: "platform_specific.likes_count", type: "number", description: "Total profile likes returned by TikTok." },
  { name: "platform_specific.video_count", type: "number", description: "TikTok's native video count alias." },
  { name: "fetched_at", type: "string", description: "UTC timestamp when UniPost fetched the metrics." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'UNAUTHORIZED, FEATURE_DISABLED, ACCOUNT_DISCONNECTED, NOT_FOUND, NEEDS_RECONNECT, NOT_SUPPORTED, or UPSTREAM_ERROR.' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_tiktok_123/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/accounts/sa_tiktok_123/metrics", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const metrics = await res.json();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "social_account_id": "sa_tiktok_123",
    "platform": "tiktok",
    "follower_count": 12400,
    "following_count": 328,
    "post_count": 146,
    "platform_specific": {
      "likes_count": 86700,
      "video_count": 146
    },
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

export default function TikTokAccountMetricsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Analytics", href: "/docs/api/analytics/summary" },
        { label: "TikTok Analytics", href: "/docs/api/analytics/tiktok" },
        { label: "Account metrics" },
      ]}
      section="analytics"
      title="Get TikTok account metrics"
      description="Returns account-level TikTok statistics for one connected account. TikTok has approved user.info.stats for production use; enable tiktok.analytics_scopes in the target environment to serve this public-ready endpoint."
      method="GET"
      path="/v1/accounts/:account_id/metrics"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "501", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
