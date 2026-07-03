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
  { name: "limit?", type: "number", description: "Maximum rows to return. Defaults to 25 and caps at 200." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "social_account_id", type: "string", description: "UniPost account ID." },
  { name: "platform", type: "youtube", description: "Always youtube." },
  { name: "start_date", type: "string", description: "Applied report start date." },
  { name: "end_date", type: "string", description: "Applied report end date." },
  { name: "videos[]", type: "array", description: "Top video rows sorted by views descending." },
  { name: "videos[].video_id", type: "string", description: "YouTube video ID." },
  { name: "videos[].metrics", type: "object", description: "Non-monetary metrics for that video." },
  { name: "limit", type: "number", description: "Limit applied to the request." },
  { name: "fetched_at", type: "string", description: "UTC fetch timestamp." },
  { name: "required_scopes[]", type: "string[]", description: "Required provider scopes, including yt-analytics.readonly." },
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
    code: `curl "https://api.unipost.dev/v1/accounts/sa_youtube_123/youtube/analytics/videos?from=2026-07-01&to=2026-07-28&limit=25" \\
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
    "videos": [
      {
        "video_id": "abc123",
        "metrics": {
          "views": 300,
          "likes": 24,
          "comments": 6,
          "shares": 4,
          "estimated_minutes_watched": 1200,
          "average_view_duration": 92,
          "average_view_percentage": 61.1,
          "subscribers_gained": 8,
          "subscribers_lost": 1
        }
      }
    ],
    "limit": 25,
    "fetched_at": "2026-07-29T18:30:00Z",
    "required_scopes": ["https://www.googleapis.com/auth/yt-analytics.readonly"]
  }
}`,
  },
];

export default function YouTubeAnalyticsVideosPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Get YouTube analytics top videos"
      description="Returns top YouTube video analytics rows for one connected YouTube account. YouTube requires maxResults of 200 or less for this report."
      method="GET"
      path="/v1/accounts/:account_id/youtube/analytics/videos"
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
