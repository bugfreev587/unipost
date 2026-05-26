"use client";

import { type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "platform?", type: "string", description: "Optional platform filter such as instagram, threads, pinterest, or tiktok." },
  { name: "profile_id?", type: "string", description: "Optional profile filter." },
  { name: "account_id?", type: "string", description: "Optional connected social account filter." },
  { name: "post_id?", type: "string", description: "Optional UniPost post filter." },
  { name: "from?", type: "string", description: "Start date as YYYY-MM-DD. Defaults to the last 30 days." },
  { name: "to?", type: "string", description: "End date as YYYY-MM-DD. Inclusive by day." },
  { name: "limit?", type: "number", description: "Maximum rows to mark for refresh. Caps at 500." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "string", description: 'Always "queued" when the refresh request is accepted.' },
  { name: "matched_count", type: "number", description: "Published delivery rows that matched the filters." },
  { name: "requested_count", type: "number", description: "Rows marked stale so the analytics refresh worker picks them up." },
  { name: "processed_by", type: "string", description: 'Currently "analytics_refresh_worker".' },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "PLAN_FEATURE_NOT_AVAILABLE", or "VALIDATION_ERROR".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/analytics/refresh" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "tiktok",
    "from": "2026-05-01",
    "to": "2026-05-31",
    "limit": 200
  }'`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "202",
    code: `{
  "data": {
    "status": "queued",
    "matched_count": 42,
    "requested_count": 42,
    "limit": 200,
    "processed_by": "analytics_refresh_worker"
  }
}`,
  },
];

export default function AnalyticsRefreshPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Request analytics refresh"
      description="Marks matching published post analytics rows stale so the background analytics refresh worker fetches fresh platform metrics."
      method="POST"
      path="/v1/analytics/refresh"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "JSON Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
