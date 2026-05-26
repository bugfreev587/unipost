"use client";

import { EnumValues, type ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start date as YYYY-MM-DD. Defaults to the last 30 days." },
  { name: "to?", type: "string", description: "End date as YYYY-MM-DD. Inclusive by day." },
  { name: "platform?", type: "string", description: <>Destination platform filter.<EnumValues values={["instagram", "threads", "pinterest", "tiktok"]} /></> },
  { name: "profile_id?", type: "string", description: "Limit exported rows to one UniPost profile." },
  { name: "account_id?", type: "string", description: "Limit exported rows to one connected social account." },
  { name: "post_id?", type: "string", description: "Limit exported rows to one UniPost post." },
  { name: "status?", type: "string", description: <>Delivery result status.<EnumValues values={["published", "failed", "partial"]} /></> },
  { name: "sort?", type: "string", description: <>Sort key.<EnumValues values={["published_at", "published_at_asc", "impressions", "likes", "engagement_rate"]} /></> },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "Content-Type", type: "text/csv", meta: "Header", description: "CSV response encoded as UTF-8." },
  { name: "Content-Disposition", type: "attachment", meta: "Header", description: 'Downloads as "unipost-analytics-posts.csv".' },
  { name: "post_id", type: "string", description: "UniPost post ID column." },
  { name: "social_post_result_id", type: "string", description: "Platform delivery result ID column." },
  { name: "platform", type: "string", description: "Destination platform column." },
  { name: "impressions", type: "number", description: "Normalized impressions column." },
  { name: "saves", type: "number", description: "Normalized saves / bookmarks column." },
  { name: "clicks", type: "number", description: "Normalized link or outbound clicks column." },
  { name: "engagement_rate", type: "number", description: "Computed engagement rate column." },
  { name: "last_failure_reason", type: "string", description: "Most recent cached upstream failure, when present." },
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
    code: `curl "https://api.unipost.dev/v1/analytics/posts/export?platform=pinterest&from=2026-05-01&to=2026-05-31" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -o unipost-analytics-posts.csv`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/analytics/posts/export?platform=tiktok", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const csv = await res.text();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "csv",
    label: "200",
    code: `post_id,social_post_result_id,platform,social_account_id,profile_id,result_status,post_status,external_id,url,created_at,published_at,impressions,reach,likes,comments,shares,saves,clicks,video_views,engagement_rate,fetched_at,last_failure_reason
post_abc123,spr_123,pinterest,acct_123,prof_123,published,published,1107111520928568449,https://pinterest.com/pin/1107111520928568449,2026-05-02T18:12:00Z,2026-05-02T18:15:00Z,18420,17102,215,17,8,81,244,0,0.0307,2026-05-26T06:00:00Z,`,
  },
];

export default function AnalyticsPostsExportPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Export analytics posts"
      description="Downloads normalized post-level analytics as CSV across UniPost-published content. Use it for scheduled reports, BI imports, and one-off workspace exports."
      method="GET"
      path="/v1/analytics/posts/export"
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
