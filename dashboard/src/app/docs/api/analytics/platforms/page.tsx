"use client";

import { EnumValues, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start date as YYYY-MM-DD. Defaults to the last 30 days." },
  { name: "to?", type: "string", description: "End date as YYYY-MM-DD. Inclusive by day." },
  { name: "profile_id?", type: "string", description: "Limit availability and metrics to one profile." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "One platform availability row for every analytics-capable destination." },
  { name: "data[].platform", type: "string", description: "Platform key." },
  { name: "data[].supported_metrics", type: "array", description: "Normalized metrics that UniPost can return for this platform." },
  { name: "data[].health", type: "string", description: <>Current analytics state.<EnumValues values={["not_connected", "pending", "ready", "degraded", "needs_reconnect"]} /></> },
  { name: "data[].account_count", type: "number", description: "Connected account count." },
  { name: "data[].last_successful_fetch_at", type: "string", description: "Latest successful post analytics fetch, when present." },
  { name: "data[].last_failure_reason", type: "string", description: "Most recent cached upstream failure, when present." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", or "VALIDATION_ERROR".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/analytics/platforms?from=2026-05-01&to=2026-05-31" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Platform detail",
    code: `curl "https://api.unipost.dev/v1/analytics/platforms/pinterest?from=2026-05-01&to=2026-05-31" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "platform": "pinterest",
      "supported_metrics": ["impressions", "reach", "likes", "comments", "shares", "saves", "clicks", "video_views", "engagement_rate"],
      "refresh_supported": true,
      "account_count": 2,
      "active_account_count": 2,
      "needs_reconnect_count": 0,
      "health": "ready",
      "last_successful_fetch_at": "2026-05-26T06:00:00Z"
    }
  ]
}`,
  },
];

export default function AnalyticsPlatformsPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Analytics platforms"
      description="Returns analytics availability, supported metrics, account health, and platform detail entry points for connected destinations."
      method="GET"
      path="/v1/analytics/platforms"
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
