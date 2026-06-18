"use client";

import { EnumValues, type ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: <>Analytics-capable platform key.<EnumValues values={["instagram", "threads", "pinterest", "tiktok", "facebook"]} /></> },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start date as YYYY-MM-DD. Defaults to the last 30 days." },
  { name: "to?", type: "string", description: "End date as YYYY-MM-DD. Inclusive by day." },
  { name: "profile_id?", type: "string", description: "Limit platform detail to one UniPost profile." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: "Platform key requested in the path." },
  { name: "period.start", type: "string", description: "Start date included in the response." },
  { name: "period.end", type: "string", description: "End date included in the response." },
  { name: "availability.platform", type: "string", description: "Platform key for the availability row." },
  { name: "availability.supported_metrics", type: "array", description: "Normalized metrics that UniPost can return for this platform." },
  { name: "availability.health", type: "string", description: <>Current analytics state.<EnumValues values={["not_connected", "pending", "ready", "degraded", "needs_reconnect"]} /></> },
  { name: "summary.posts", type: "number", description: "Published post count in the period." },
  { name: "summary.impressions", type: "number", description: "Summed normalized impressions." },
  { name: "summary.video_views", type: "number", description: "Summed normalized video views." },
  { name: "summary.engagement_rate", type: "number", description: "Computed engagement rate for the period." },
  { name: "trend[]", type: "array", description: "Daily metric buckets for the platform." },
  { name: "accounts[]", type: "array", description: "Connected accounts with post counts and analytics health." },
  { name: "top_posts[]", type: "array", description: "Top post-level analytics rows sorted by engagement rate." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", "NOT_FOUND", or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/analytics/platforms/tiktok?from=2026-05-01&to=2026-05-31" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/analytics/platforms/pinterest", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const pinterestAnalytics = await res.json();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "platform": "tiktok",
    "period": {
      "start": "2026-05-01",
      "end": "2026-05-31"
    },
    "availability": {
      "platform": "tiktok",
      "supported_metrics": ["views", "likes", "comments", "shares", "video_views", "engagement_rate"],
      "health": "ready"
    },
    "summary": {
      "posts": 18,
      "impressions": 92000,
      "video_views": 88400,
      "engagement_rate": 0.0641
    },
    "trend": [],
    "accounts": [],
    "top_posts": []
  }
}`,
  },
];

export default function AnalyticsPlatformDetailPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Get analytics platform"
      description="Returns one platform's analytics availability, summary metrics, daily trend, connected-account health, and top posts."
      method="GET"
      path="/v1/analytics/platforms/{platform}"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
