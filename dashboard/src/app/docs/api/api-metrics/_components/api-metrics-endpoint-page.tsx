"use client";

import { ApiInlineLink, EnumValues, type ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

type EndpointKey = "overall" | "summary" | "trend" | "status-codes";

type Snippet = {
  lang: string;
  label: string;
  code: string;
};

type EndpointDefinition = {
  title: string;
  description: React.ReactNode;
  path: string;
  queryFields: ApiFieldItem[];
  responseFields: ApiFieldItem[];
  snippet: Snippet;
  responseSnippet: Snippet;
};

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const COMMON_QUERY_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start timestamp in RFC3339 format. Defaults to 7 days before now." },
  { name: "to?", type: "string", description: "End timestamp in RFC3339 format. Defaults to now. The maximum raw range is 90 days." },
  { name: "method?", type: "string", description: <>Filter by HTTP method.<EnumValues values={["GET", "POST", "PUT", "PATCH", "DELETE"]} /></> },
  { name: "path?", type: "string", description: <>Filter by normalized endpoint path, such as <code>/v1/posts/:id/publish</code>.</> },
  { name: "status_class?", type: "string", description: <>Filter by HTTP status class.<EnumValues values={["2xx", "3xx", "4xx", "5xx"]} /></> },
];

const SORT_FIELD: ApiFieldItem = {
  name: "sort?",
  type: "string",
  description: <>Summary sort key.<EnumValues values={["total_calls_desc", "p95_ms_desc", "p99_ms_desc", "server_errors_desc", "rate_limited_desc"]} /></>,
};

const LIMIT_FIELD: ApiFieldItem = {
  name: "limit?",
  type: "integer",
  description: "Maximum rows for list endpoints. Defaults to 50, maximum 200.",
};

const INTERVAL_FIELD: ApiFieldItem = {
  name: "interval?",
  type: "string",
  description: <>Trend bucket size. Defaults to hour for ranges up to 7 days and day for longer ranges.<EnumValues values={["hour", "day"]} /></>,
};

const OVERALL_RESPONSE_FIELDS: ApiFieldItem[] = [
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

const SUMMARY_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Per-endpoint metric rows ordered by the requested sort key." },
  { name: "data[].path", type: "string", description: "Normalized API route path." },
  { name: "data[].method", type: "string", description: "HTTP method for the route." },
  { name: "data[].total_calls", type: "number", description: "Number of matching API requests for the endpoint." },
  { name: "data[].success_count", type: "number", description: "Responses with status below 400." },
  { name: "data[].client_error_count", type: "number", description: "Responses from 400 through 499, including 429." },
  { name: "data[].server_error_count", type: "number", description: "Responses with status 500 or higher." },
  { name: "data[].rate_limited_count", type: "number", description: "Responses with status 429." },
  { name: "data[].error_rate_pct", type: "number", description: "All 4xx and 5xx responses divided by total calls." },
  { name: "data[].server_failure_rate_pct", type: "number", description: "5xx responses divided by total calls." },
  { name: "data[].p50_ms", type: "number", description: "Median endpoint latency in milliseconds." },
  { name: "data[].p95_ms", type: "number", description: "95th percentile endpoint latency in milliseconds." },
  { name: "data[].p99_ms", type: "number", description: "99th percentile endpoint latency in milliseconds." },
  { name: "data[].avg_ms", type: "number", description: "Average endpoint latency in milliseconds." },
];

const TREND_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Hourly or daily buckets ordered oldest to newest." },
  { name: "data[].bucket", type: "string", description: "Bucket start timestamp." },
  { name: "data[].total_calls", type: "number", description: "Total requests in the bucket." },
  { name: "data[].success_count", type: "number", description: "Responses with status below 400." },
  { name: "data[].error_count", type: "number", description: "All 4xx and 5xx responses in the bucket." },
  { name: "data[].client_error_count", type: "number", description: "Responses from 400 through 499, including 429." },
  { name: "data[].server_error_count", type: "number", description: "Responses with status 500 or higher." },
  { name: "data[].rate_limited_count", type: "number", description: "Responses with status 429." },
  { name: "data[].p50_ms", type: "number", description: "Median latency in the bucket." },
  { name: "data[].p95_ms", type: "number", description: "95th percentile latency in the bucket." },
  { name: "data[].p99_ms", type: "number", description: "99th percentile latency in the bucket." },
  { name: "data[].avg_ms", type: "number", description: "Average latency in the bucket." },
];

const STATUS_CODE_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Status-code distribution rows ordered by total call count." },
  { name: "data[].status_code", type: "number", description: "Exact HTTP response status code." },
  { name: "data[].method", type: "string", description: "HTTP method for the endpoint." },
  { name: "data[].path", type: "string", description: "Normalized API route path." },
  { name: "data[].total_calls", type: "number", description: "Number of matching responses for this status, method, and path." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "VALIDATION_ERROR", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ENDPOINTS: Record<EndpointKey, EndpointDefinition> = {
  overall: {
    title: "Overall",
    description: "Returns aggregate API-key traffic totals, latency percentiles, and error counters for a workspace.",
    path: "/v1/api-metrics/overall",
    queryFields: COMMON_QUERY_FIELDS,
    responseFields: OVERALL_RESPONSE_FIELDS,
    snippet: {
      lang: "curl",
      label: "cURL",
      code: `curl "https://api.unipost.dev/v1/api-metrics/overall?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
    },
    responseSnippet: {
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
  },
  summary: {
    title: "Summary",
    description: "Returns per-endpoint API metrics rows so dashboards can rank slow, high-volume, or error-heavy routes.",
    path: "/v1/api-metrics/summary",
    queryFields: [...COMMON_QUERY_FIELDS, SORT_FIELD, LIMIT_FIELD],
    responseFields: SUMMARY_RESPONSE_FIELDS,
    snippet: {
      lang: "curl",
      label: "cURL",
      code: `curl "https://api.unipost.dev/v1/api-metrics/summary?sort=p95_ms_desc&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
    },
    responseSnippet: {
      lang: "json",
      label: "200",
      code: `{
  "data": [
    {
      "path": "/v1/posts/:id/publish",
      "method": "POST",
      "total_calls": 1268,
      "success_count": 1210,
      "client_error_count": 42,
      "server_error_count": 16,
      "rate_limited_count": 9,
      "error_rate_pct": 4.57,
      "server_failure_rate_pct": 1.26,
      "p50_ms": 184,
      "p95_ms": 860,
      "p99_ms": 1730,
      "avg_ms": 242
    }
  ]
}`,
    },
  },
  trend: {
    title: "Trend",
    description: "Returns hourly or daily API metrics buckets for charts, alerting, and operational reviews.",
    path: "/v1/api-metrics/trend",
    queryFields: [...COMMON_QUERY_FIELDS, INTERVAL_FIELD],
    responseFields: TREND_RESPONSE_FIELDS,
    snippet: {
      lang: "curl",
      label: "cURL",
      code: `curl "https://api.unipost.dev/v1/api-metrics/trend?interval=day" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
    },
    responseSnippet: {
      lang: "json",
      label: "200",
      code: `{
  "data": [
    {
      "bucket": "2026-06-01T00:00:00Z",
      "total_calls": 2480,
      "success_count": 2398,
      "error_count": 82,
      "client_error_count": 71,
      "server_error_count": 11,
      "rate_limited_count": 6,
      "p50_ms": 116,
      "p95_ms": 604,
      "p99_ms": 1392,
      "avg_ms": 178
    }
  ]
}`,
    },
  },
  "status-codes": {
    title: "Status-Code",
    description: "Returns exact status-code distribution by endpoint for debugging client errors, rate limits, and server failures.",
    path: "/v1/api-metrics/status-codes",
    queryFields: [...COMMON_QUERY_FIELDS, LIMIT_FIELD],
    responseFields: STATUS_CODE_RESPONSE_FIELDS,
    snippet: {
      lang: "curl",
      label: "cURL",
      code: `curl "https://api.unipost.dev/v1/api-metrics/status-codes?status_class=4xx" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
    },
    responseSnippet: {
      lang: "json",
      label: "200",
      code: `{
  "data": [
    {
      "status_code": 429,
      "method": "POST",
      "path": "/v1/posts",
      "total_calls": 19
    }
  ]
}`,
    },
  },
};

const RELATED_ENDPOINTS: Array<{ key: EndpointKey; endpoint: string; purpose: string }> = [
  { key: "overall", endpoint: "GET /v1/api-metrics/overall", purpose: "aggregate totals" },
  { key: "summary", endpoint: "GET /v1/api-metrics/summary", purpose: "per-endpoint latency and error rows" },
  { key: "trend", endpoint: "GET /v1/api-metrics/trend", purpose: "hourly or daily chart buckets" },
  { key: "status-codes", endpoint: "GET /v1/api-metrics/status-codes", purpose: "exact status-code distribution" },
];

export function APIMetricsEndpointPage({ endpoint }: { endpoint: EndpointKey }) {
  const definition = ENDPOINTS[endpoint];

  return (
    <SingleEndpointReferencePage
      section="api-metrics"
      title={definition.title}
      description={
        <>
          {definition.description} Metrics include API-key-authenticated Developer API traffic only; dashboard sessions, admin routes, hosted public flows, OAuth callbacks, inbound provider webhooks, health checks, and the metrics endpoints themselves are excluded.
        </>
      }
      method="GET"
      path={definition.path}
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: definition.queryFields },
      ]}
      responses={[
        { code: "200", fields: definition.responseFields },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={[definition.snippet]}
      responseSnippets={[definition.responseSnippet]}
    >
      <section className="api-field-section">
        <h2 className="api-field-section-title">Related Endpoints</h2>
        <div style={{ display: "grid", gap: 10 }}>
          {RELATED_ENDPOINTS.map((related) => (
            <p key={related.key} style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
              <ApiInlineLink endpoint={related.endpoint} /> for {related.purpose}.
            </p>
          ))}
          <p style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
            Trend <code>error_count</code> uses the same 4xx-plus-5xx definition as <code>error_rate_pct</code>; 429 responses are also exposed separately through <code>rate_limited_count</code>.
          </p>
        </div>
      </section>
    </SingleEndpointReferencePage>
  );
}
