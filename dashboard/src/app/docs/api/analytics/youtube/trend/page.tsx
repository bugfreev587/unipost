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
  { name: "rows[]", type: "array", description: "Daily rows sorted by day." },
  { name: "rows[].date", type: "string", description: "Day in YYYY-MM-DD format." },
  { name: "rows[].metrics", type: "object", description: "Non-monetary metrics for that day." },
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
    code: `curl "https://api.unipost.dev/v1/accounts/sa_youtube_123/youtube/analytics/trend?from=2026-07-01&to=2026-07-28" \\
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
    "rows": [
      {
        "date": "2026-07-01",
        "metrics": {
          "views": 120,
          "likes": 8,
          "comments": 2,
          "shares": 1,
          "estimated_minutes_watched": 540,
          "average_view_duration": 84,
          "average_view_percentage": 62.5,
          "subscribers_gained": 3,
          "subscribers_lost": 0
        }
      }
    ],
    "fetched_at": "2026-07-29T18:30:00Z",
    "required_scopes": ["https://www.googleapis.com/auth/yt-analytics.readonly"]
  }
}`,
  },
];

export default function YouTubeAnalyticsTrendPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Get YouTube analytics trend"
      description="Returns daily non-monetary YouTube Analytics rows for one connected YouTube account."
      method="GET"
      path="/v1/accounts/:account_id/youtube/analytics/trend"
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
