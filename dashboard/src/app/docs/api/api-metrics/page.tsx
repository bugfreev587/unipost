"use client";

import { ApiInlineLink, EnumValues, type ApiFieldItem } from "../_components/doc-components";
import { SingleEndpointReferencePage } from "../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start timestamp in RFC3339 format. Defaults to 7 days before now." },
  { name: "to?", type: "string", description: "End timestamp in RFC3339 format. Defaults to now. The maximum raw range is 90 days." },
  { name: "interval?", type: "string", description: <>Trend bucket size. Defaults to hour for short ranges and day for longer ranges.<EnumValues values={["hour", "day"]} /></> },
  { name: "method?", type: "string", description: <>Filter by HTTP method.<EnumValues values={["GET", "POST", "PUT", "PATCH", "DELETE"]} /></> },
  { name: "path?", type: "string", description: <>Filter by normalized endpoint path, such as <code>/v1/posts/:id/publish</code>.</> },
  { name: "status_class?", type: "string", description: <>Filter by HTTP status class.<EnumValues values={["2xx", "3xx", "4xx", "5xx"]} /></> },
  { name: "sort?", type: "string", description: <>Summary sort key.<EnumValues values={["total_calls_desc", "p95_ms_desc", "p99_ms_desc", "server_errors_desc", "rate_limited_desc"]} /></> },
  { name: "limit?", type: "integer", description: "Maximum rows for list endpoints. Defaults to 50, maximum 200." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "total_calls", type: "number", description: "Number of matching API requests." },
  { name: "success_count", type: "number", description: "Responses with status below 400." },
  { name: "client_error_count", type: "number", description: "Responses from 400 through 499, including 429." },
  { name: "server_error_count", type: "number", description: "Responses with status 500 or higher." },
  { name: "rate_limited_count", type: "number", description: "Responses with status 429, also included in client_error_count." },
  { name: "error_rate_pct", type: "number", description: "All 4xx and 5xx responses divided by total calls, including 429 rate-limit responses." },
  { name: "server_failure_rate_pct", type: "number", description: "5xx responses divided by total calls." },
  { name: "reliability_pct", type: "number", description: "Requests that did not return 5xx, divided by total calls." },
  { name: "p50_ms", type: "number", description: "Median latency measured at the UniPost API layer." },
  { name: "p95_ms", type: "number", description: "95th percentile latency measured at the UniPost API layer." },
  { name: "p99_ms", type: "number", description: "99th percentile latency measured at the UniPost API layer." },
  { name: "avg_ms", type: "number", description: "Average latency in milliseconds." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "Overall",
    code: `curl "https://api.unipost.dev/v1/api-metrics/overall?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Summary",
    code: `curl "https://api.unipost.dev/v1/api-metrics/summary?sort=p95_ms_desc&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Trend",
    code: `curl "https://api.unipost.dev/v1/api-metrics/trend?interval=day" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "curl",
    label: "Status Codes",
    code: `curl "https://api.unipost.dev/v1/api-metrics/status-codes?status_class=4xx" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "total_calls": 18423,
    "success_count": 17980,
    "client_error_count": 391,
    "server_error_count": 52,
    "rate_limited_count": 17,
    "error_rate_pct": 2.41,
    "server_failure_rate_pct": 0.28,
    "reliability_pct": 99.72,
    "p50_ms": 118,
    "p95_ms": 642,
    "p99_ms": 1484,
    "avg_ms": 184
  }
}`,
  },
  {
    lang: "json",
    label: "400",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "time range cannot exceed 90 days"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "401",
    code: `{
  "error": {
    "code": "UNAUTHORIZED",
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
  },
  "request_id": "req_123"
}`,
  },
];

export default function APIMetricsDocsPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="API Metrics"
      description={
        <>
          Query workspace-scoped latency, volume, and error metrics for API-key calls to the UniPost Developer API. Metrics include API-key-authenticated Developer API traffic only; dashboard sessions, admin routes, hosted public flows, OAuth callbacks, inbound provider webhooks, health checks, and the metrics endpoints themselves are excluded.
        </>
      }
      method="GET"
      path="/v1/api-metrics/overall"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <section className="api-field-section">
        <h2 className="api-field-section-title">Related Endpoints</h2>
        <div style={{ display: "grid", gap: 10 }}>
          <p style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
            Use <ApiInlineLink endpoint="GET /v1/api-metrics/overall" /> for aggregate totals, <ApiInlineLink endpoint="GET /v1/api-metrics/summary" /> for per-endpoint latency and error rows, <ApiInlineLink endpoint="GET /v1/api-metrics/trend" /> for hourly or daily chart buckets, and <ApiInlineLink endpoint="GET /v1/api-metrics/status-codes" /> for exact status-code distribution.
          </p>
          <p style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
            Trend <code>error_count</code> uses the same 4xx-plus-5xx definition as <code>error_rate_pct</code>; 429 responses are also exposed separately through <code>rate_limited_count</code>.
          </p>
        </div>
      </section>
    </SingleEndpointReferencePage>
  );
}
