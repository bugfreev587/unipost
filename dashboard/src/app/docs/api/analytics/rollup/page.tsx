"use client";

import { EnumValues, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from", type: "string", description: "Required RFC3339 timestamp." },
  { name: "to", type: "string", description: "Required RFC3339 timestamp. Range is capped at 366 days." },
  { name: "granularity?", type: "string", description: <>Bucket size.<EnumValues values={["day", "week", "month"]} /></> },
  { name: "group_by?", type: "string", description: <>Comma-separated dimensions.<EnumValues values={["platform", "social_account_id", "external_user_id", "status"]} /></> },
  { name: "profile_id?", type: "string", description: "Limit the rollup to one UniPost profile." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "granularity", type: "string", description: "Applied time bucket." },
  { name: "group_by", type: "array", description: "Applied group dimensions." },
  { name: "series[]", type: "array", description: "Time buckets ordered newest first." },
  { name: "series[].groups[]", type: "array", description: "Grouped counts and normalized metrics." },
  { name: "series[].groups[].published_count", type: "number", description: "Published delivery count." },
  { name: "series[].groups[].impressions", type: "number", description: "Summed impressions." },
  { name: "series[].groups[].saves", type: "number", description: "Summed saves / bookmarks." },
  { name: "series[].groups[].clicks", type: "number", description: "Summed clicks." },
  { name: "series[].groups[].engagement_rate", type: "number", description: "Computed from normalized engagement over impressions." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/analytics/rollup?from=2026-05-01T00:00:00Z&to=2026-06-01T00:00:00Z&granularity=day&group_by=platform" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "granularity": "day",
    "group_by": ["platform"],
    "series": [
      {
        "bucket": "2026-05-26T00:00:00Z",
        "groups": [
          {
            "platform": "tiktok",
            "published_count": 6,
            "failed_count": 0,
            "partial_count": 0,
            "video_views": 42000,
            "likes": 2140,
            "engagement_rate": 0
          }
        ]
      }
    ]
  }
}`,
  },
];

export default function AnalyticsRollupPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Analytics rollup"
      description="Returns time-bucketed analytics grouped by platform, account, external user, or delivery status. Use it for charts and reporting summaries."
      method="GET"
      path="/v1/analytics/rollup"
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
