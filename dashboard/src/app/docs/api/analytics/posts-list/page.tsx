"use client";

import { EnumValues, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start date as YYYY-MM-DD. Defaults to the last 30 days." },
  { name: "to?", type: "string", description: "End date as YYYY-MM-DD. Inclusive by day." },
  { name: "platform?", type: "string", description: <>Destination platform filter.<EnumValues values={["instagram", "threads", "pinterest", "tiktok"]} /></> },
  { name: "profile_id?", type: "string", description: "Limit results to one UniPost profile." },
  { name: "account_id?", type: "string", description: "Limit results to one connected social account." },
  { name: "post_id?", type: "string", description: "Limit results to one UniPost post." },
  { name: "status?", type: "string", description: <>Delivery result status.<EnumValues values={["published", "failed", "partial"]} /></> },
  { name: "sort?", type: "string", description: <>Sort key.<EnumValues values={["published_at", "published_at_asc", "impressions", "likes", "engagement_rate"]} /></> },
  { name: "limit?", type: "number", description: "Page size. Defaults to 50 and caps at 100." },
  { name: "cursor?", type: "string", description: "Cursor from meta.next_cursor." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "One row per social platform delivery result." },
  { name: "data[].post_id", type: "string", description: "UniPost post ID." },
  { name: "data[].social_post_result_id", type: "string", description: "Platform delivery result ID." },
  { name: "data[].platform", type: "string", description: "Destination platform." },
  { name: "data[].impressions", type: "number", description: "Normalized impressions." },
  { name: "data[].likes", type: "number", description: "Normalized likes." },
  { name: "data[].saves", type: "number", description: "Normalized saves / bookmarks." },
  { name: "data[].clicks", type: "number", description: "Normalized link or outbound clicks." },
  { name: "data[].engagement_rate", type: "number", description: "Computed engagement rate from normalized metrics." },
  { name: "data[].platform_specific", type: "object", description: "Native fields preserved from the source platform." },
  { name: "meta.next_cursor", type: "string", description: "Cursor for the next page when has_more is true." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/analytics/posts?platform=tiktok&sort=engagement_rate&limit=25" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/analytics/posts?platform=instagram", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const posts = await res.json();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "post_id": "post_abc123",
      "social_post_result_id": "spr_123",
      "platform": "pinterest",
      "result_status": "published",
      "impressions": 18420,
      "saves": 81,
      "clicks": 244,
      "engagement_rate": 0.0182,
      "fetched_at": "2026-05-26T06:00:00Z"
    }
  ],
  "meta": {
    "limit": 25,
    "has_more": true,
    "next_cursor": "25"
  }
}`,
  },
];

export default function AnalyticsPostsListPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="List analytics posts"
      description="Returns normalized post-level analytics rows across UniPost-published content. Use it for reporting tables, top-post lists, exports, and custom dashboards."
      method="GET"
      path="/v1/analytics/posts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
