"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Period start in ISO-8601 format." },
  { name: "to?", type: "string", description: "Period end in ISO-8601 format." },
  { name: "granularity?", type: "string", description: 'Time bucket such as "day" or "week".' },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "totals", type: "object", description: "Workspace-wide totals across the selected period." },
  { name: "totals.impressions", type: "number", description: "Normalized impressions total." },
  { name: "trend", type: "array", description: "Time-series points for charting." },
  { name: "by_platform", type: "array", description: "Breakdown by destination network." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/analytics/summary?from=2026-04-01T00:00:00Z&to=2026-04-30T00:00:00Z&granularity=day" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const rollup = await client.analytics.rollup({
  from: "2026-04-01T00:00:00Z",
  to: "2026-04-30T00:00:00Z",
  granularity: "day",
});`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "totals": {
      "impressions": 128400,
      "reach": 92400,
      "likes": 3200
    },
    "trend": [
      {
        "date": "2026-04-01",
        "impressions": 4200
      }
    ],
    "by_platform": [
      {
        "platform": "instagram",
        "impressions": 68000
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
}`,
  },
];

export default function AnalyticsSummaryPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Workspace summary"
      description="Returns workspace-wide analytics totals and trend breakdowns for a selected period. Use it to power overview dashboards and reporting views."
      method="GET"
      path="/v1/analytics/summary"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
