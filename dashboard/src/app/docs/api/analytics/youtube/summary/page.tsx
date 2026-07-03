"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected YouTube account ID." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "YYYY-MM-DD", description: "Start date. Aliases: start_date, startDate. Defaults to the first day of the last 28 complete days." },
  { name: "to?", type: "YYYY-MM-DD", description: "End date. Aliases: end_date, endDate. Defaults to yesterday in UTC." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "social_account_id", type: "string", description: "UniPost account ID." },
  { name: "platform", type: "youtube", description: "Always youtube." },
  { name: "start_date", type: "string", description: "Applied report start date." },
  { name: "end_date", type: "string", description: "Applied report end date." },
  { name: "metrics", type: "object", description: "Non-monetary YouTube Analytics metrics for the range." },
  { name: "metrics.views", type: "number", description: "Views in the report range." },
  { name: "metrics.likes", type: "number", description: "Likes in the report range." },
  { name: "metrics.comments", type: "number", description: "Comments in the report range." },
  { name: "metrics.shares", type: "number", description: "Shares in the report range." },
  { name: "metrics.estimated_minutes_watched", type: "number", description: "Estimated watch time in minutes." },
  { name: "metrics.average_view_duration", type: "number", description: "Average view duration in seconds." },
  { name: "metrics.average_view_percentage", type: "number", description: "Average percentage watched." },
  { name: "metrics.subscribers_gained", type: "number", description: "Subscribers gained in the report range." },
  { name: "metrics.subscribers_lost", type: "number", description: "Subscribers lost in the report range." },
  { name: "required_scopes[]", type: "string[]", description: "Required provider scopes, including yt-analytics.readonly." },
  { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "UNAUTHORIZED, NOT_FOUND, WRONG_PLATFORM, ACCOUNT_DISCONNECTED, NEEDS_RECONNECT, VALIDATION_ERROR, or UPSTREAM_ERROR." },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_youtube_123/youtube/analytics/summary?from=2026-07-01&to=2026-07-28" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "social_account_id": "sa_youtube_123",
    "platform": "youtube",
    "start_date": "2026-07-01",
    "end_date": "2026-07-28",
    "metrics": {
      "views": 1200,
      "likes": 88,
      "comments": 17,
      "shares": 9,
      "estimated_minutes_watched": 5400,
      "average_view_duration": 84,
      "average_view_percentage": 62.5,
      "subscribers_gained": 31,
      "subscribers_lost": 4
    },
    "required_scopes": ["https://www.googleapis.com/auth/yt-analytics.readonly"],
    "fetched_at": "2026-07-29T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "NEEDS_RECONNECT",
    "normalized_code": "needs_reconnect",
    "message": "Reconnect YouTube to enable analytics."
  },
  "request_id": "req_123"
}`,
  },
];

export default function YouTubeAnalyticsSummaryPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Get YouTube analytics summary"
      description="Returns non-monetary YouTube Analytics channel totals for one connected YouTube account. Defaults to the last 28 complete UTC days when no date range is provided."
      method="GET"
      path="/v1/accounts/:account_id/youtube/analytics/summary"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
