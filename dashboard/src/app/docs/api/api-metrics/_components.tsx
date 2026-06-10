"use client";

import type { ReactNode } from "react";
import { ApiInlineLink, EnumValues, type ApiFieldItem } from "../_components/doc-components";
import { SingleEndpointReferencePage } from "../_components/single-endpoint-page";

type ApiMetricsEndpoint = "overall" | "summary" | "trend" | "status-codes";

type ApiMetricsEndpointConfig = {
  title: string;
  description: ReactNode;
  path: string;
  queryFields: ApiFieldItem[];
  responseFields: ApiFieldItem[];
  snippets: Array<{ lang: string; label: string; code: string }>;
  responseSnippets: Array<{ lang: string; label: string; code: string }>;
};

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const TIME_RANGE_FIELDS: ApiFieldItem[] = [
  { name: "from?", type: "string", description: "Start timestamp in RFC3339 format. Defaults to 7 days before now." },
  { name: "to?", type: "string", description: "End timestamp in RFC3339 format. Defaults to now. The maximum raw range is 90 days." },
];

const FILTER_FIELDS: ApiFieldItem[] = [
  { name: "method?", type: "string", description: <>Filter by HTTP method.<EnumValues values={["GET", "POST", "PUT", "PATCH", "DELETE"]} /></> },
  { name: "path?", type: "string", description: <>Filter by normalized endpoint path, such as <code>/v1/posts/:id/publish</code>.</> },
  { name: "status_class?", type: "string", description: <>Filter by HTTP status class.<EnumValues values={["2xx", "3xx", "4xx", "5xx"]} /></> },
];

const TREND_FIELDS: ApiFieldItem[] = [
  { name: "interval?", type: "string", description: <>Trend bucket size. Defaults to hour for ranges of 7 days or less and day for longer ranges.<EnumValues values={["hour", "day"]} /></> },
];

const SUMMARY_FIELDS: ApiFieldItem[] = [
  { name: "sort?", type: "string", description: <>Sort key for returned endpoint rows.<EnumValues values={["total_calls_desc", "p95_ms_desc", "p99_ms_desc", "server_errors_desc", "rate_limited_desc"]} /></> },
  { name: "limit?", type: "integer", description: "Maximum rows to return. Defaults to 50, maximum 200." },
];

const STATUS_CODE_FIELDS: ApiFieldItem[] = [
  { name: "limit?", type: "integer", description: "Maximum status-code rows to return. Defaults to 50, maximum 200." },
];

const LATENCY_FIELDS: ApiFieldItem[] = [
  { name: "p50_ms", type: "number", description: "Median latency measured at the UniPost API layer." },
  { name: "p95_ms", type: "number", description: "95th percentile latency measured at the UniPost API layer." },
  { name: "p99_ms", type: "number", description: "99th percentile latency measured at the UniPost API layer." },
  { name: "avg_ms", type: "number", description: "Average latency in milliseconds." },
];

const AGGREGATE_METRIC_FIELDS: ApiFieldItem[] = [
  { name: "total_calls", type: "number", description: "Number of matching API requests." },
  { name: "success_count", type: "number", description: "Responses with status below 400." },
  { name: "client_error_count", type: "number", description: "Responses from 400 through 499, including 429." },
  { name: "server_error_count", type: "number", description: "Responses with status 500 or higher." },
  { name: "rate_limited_count", type: "number", description: "Responses with status 429, also included in client_error_count." },
  { name: "error_rate_pct", type: "number", description: "All 4xx and 5xx responses divided by total calls, including 429 rate-limit responses." },
  { name: "server_failure_rate_pct", type: "number", description: "5xx responses divided by total calls." },
];

const OVERALL_RESPONSE_FIELDS: ApiFieldItem[] = [
  ...AGGREGATE_METRIC_FIELDS.map((field) => ({ ...field, name: `data.${field.name}` })),
  { name: "data.reliability_pct", type: "number", description: "Requests that did not return 5xx, divided by total calls." },
  ...LATENCY_FIELDS.map((field) => ({ ...field, name: `data.${field.name}` })),
];

const SUMMARY_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "One row per normalized API method and path." },
  { name: "data[].path", type: "string", description: <>Normalized endpoint path, such as <code>/v1/posts/:id/publish</code>.</> },
  { name: "data[].method", type: "string", description: "HTTP method for the row." },
  ...AGGREGATE_METRIC_FIELDS.map((field) => ({ ...field, name: `data[].${field.name}` })),
  ...LATENCY_FIELDS.map((field) => ({ ...field, name: `data[].${field.name}` })),
];

const TREND_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Hourly or daily buckets ordered by time." },
  { name: "data[].bucket", type: "string", description: "Bucket start timestamp in RFC3339 format." },
  { name: "data[].total_calls", type: "number", description: "Number of matching API requests in the bucket." },
  { name: "data[].success_count", type: "number", description: "Responses with status below 400 in the bucket." },
  { name: "data[].error_count", type: "number", description: "All 4xx and 5xx responses in the bucket, including 429." },
  { name: "data[].client_error_count", type: "number", description: "Responses from 400 through 499, including 429." },
  { name: "data[].server_error_count", type: "number", description: "Responses with status 500 or higher." },
  { name: "data[].rate_limited_count", type: "number", description: "Responses with status 429." },
  ...LATENCY_FIELDS.map((field) => ({ ...field, name: `data[].${field.name}` })),
];

const STATUS_CODES_RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Status-code distribution rows ordered by call count." },
  { name: "data[].status_code", type: "number", description: "Exact HTTP status code." },
  { name: "data[].method", type: "string", description: "HTTP method for the status-code row." },
  { name: "data[].path", type: "string", description: <>Normalized endpoint path, such as <code>/v1/media</code>.</> },
  { name: "data[].total_calls", type: "number", description: "Number of matching API requests with this status code." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "VALIDATION_ERROR", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const endpointConfigs: Record<ApiMetricsEndpoint, ApiMetricsEndpointConfig> = {
  overall: {
    title: "Overall metrics",
    description: (
      <>
        Returns aggregate latency, volume, and error totals for API-key-authenticated Developer API calls in the workspace. Use this endpoint for dashboard headline cards and service health summaries.
      </>
    ),
    path: "/v1/api-metrics/overall",
    queryFields: [...TIME_RANGE_FIELDS, ...FILTER_FIELDS],
    responseFields: OVERALL_RESPONSE_FIELDS,
    snippets: [
      {
        lang: "curl",
        label: "cURL",
        code: `curl "https://api.unipost.dev/v1/api-metrics/overall?from=2026-06-01T00:00:00Z&to=2026-06-09T00:00:00Z" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
      },
    ],
    responseSnippets: [
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
    ],
  },
  summary: {
    title: "Endpoint summary",
    description: (
      <>
        Returns per-endpoint metrics grouped by normalized API path and method. Use this endpoint to find the slowest, busiest, or most error-prone API surfaces for a workspace.
      </>
    ),
    path: "/v1/api-metrics/summary",
    queryFields: [...TIME_RANGE_FIELDS, ...FILTER_FIELDS, ...SUMMARY_FIELDS],
    responseFields: SUMMARY_RESPONSE_FIELDS,
    snippets: [
      {
        lang: "curl",
        label: "cURL",
        code: `curl "https://api.unipost.dev/v1/api-metrics/summary?sort=p95_ms_desc&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
      },
    ],
    responseSnippets: [
      {
        lang: "json",
        label: "200",
        code: `{
  "data": [
    {
      "path": "/v1/media",
      "method": "POST",
      "total_calls": 842,
      "success_count": 817,
      "client_error_count": 18,
      "server_error_count": 7,
      "rate_limited_count": 3,
      "error_rate_pct": 2.97,
      "server_failure_rate_pct": 0.83,
      "p50_ms": 344,
      "p95_ms": 1860,
      "p99_ms": 4214,
      "avg_ms": 522
    }
  ]
}`,
      },
    ],
  },
  trend: {
    title: "Metrics trend",
    description: (
      <>
        Returns latency and outcome metrics bucketed by hour or day. Use this endpoint to chart traffic, reliability, and latency over time.
      </>
    ),
    path: "/v1/api-metrics/trend",
    queryFields: [...TIME_RANGE_FIELDS, ...TREND_FIELDS, ...FILTER_FIELDS],
    responseFields: TREND_RESPONSE_FIELDS,
    snippets: [
      {
        lang: "curl",
        label: "cURL",
        code: `curl "https://api.unipost.dev/v1/api-metrics/trend?interval=day&status_class=5xx" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
      },
    ],
    responseSnippets: [
      {
        lang: "json",
        label: "200",
        code: `{
  "data": [
    {
      "bucket": "2026-06-08T00:00:00Z",
      "total_calls": 2368,
      "success_count": 2317,
      "error_count": 51,
      "client_error_count": 44,
      "server_error_count": 7,
      "rate_limited_count": 2,
      "p50_ms": 121,
      "p95_ms": 688,
      "p99_ms": 1524,
      "avg_ms": 196
    }
  ]
}`,
      },
    ],
  },
  "status-codes": {
    title: "Status-code distribution",
    description: (
      <>
        Returns exact status-code counts by normalized API path and method. Use this endpoint when a status class needs to be broken down into specific HTTP responses.
      </>
    ),
    path: "/v1/api-metrics/status-codes",
    queryFields: [...TIME_RANGE_FIELDS, ...FILTER_FIELDS, ...STATUS_CODE_FIELDS],
    responseFields: STATUS_CODES_RESPONSE_FIELDS,
    snippets: [
      {
        lang: "curl",
        label: "cURL",
        code: `curl "https://api.unipost.dev/v1/api-metrics/status-codes?status_class=4xx&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
      },
    ],
    responseSnippets: [
      {
        lang: "json",
        label: "200",
        code: `{
  "data": [
    {
      "status_code": 429,
      "method": "POST",
      "path": "/v1/posts",
      "total_calls": 17
    },
    {
      "status_code": 400,
      "method": "POST",
      "path": "/v1/media",
      "total_calls": 9
    }
  ]
}`,
      },
    ],
  },
};

export function ApiMetricsEndpointPage({ endpoint }: { endpoint: ApiMetricsEndpoint }) {
  const config = endpointConfigs[endpoint];

  return (
    <SingleEndpointReferencePage
      section="analytics"
      title={config.title}
      description={config.description}
      method="GET"
      path={config.path}
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: config.queryFields },
      ]}
      responses={[
        { code: "200", fields: config.responseFields },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={config.snippets}
      responseSnippets={config.responseSnippets}
    >
      <div style={{ display: "grid", gap: 34 }}>
        <MetricSemantics />
        <RelatedMetricsEndpoints />
      </div>
    </SingleEndpointReferencePage>
  );
}

function MetricSemantics() {
  return (
    <section className="api-field-section">
      <h2 className="api-field-section-title">Metric Semantics</h2>
      <div style={{ display: "grid", gap: 10 }}>
        <p style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
          Metrics include API-key-authenticated Developer API traffic only. Dashboard sessions, admin routes, hosted public flows, OAuth callbacks, inbound provider webhooks, health checks, and the metrics endpoints themselves are excluded.
        </p>
        <p style={{ margin: 0, color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
          Trend <code>error_count</code> uses the same 4xx-plus-5xx definition as <code>error_rate_pct</code>; 429 responses are also exposed separately through <code>rate_limited_count</code>.
        </p>
      </div>
    </section>
  );
}

function RelatedMetricsEndpoints() {
  return (
    <section className="api-field-section">
      <h2 className="api-field-section-title">Related Endpoints</h2>
      <ul style={{ display: "grid", gap: 10, margin: 0, paddingLeft: 18, listStyleType: "disc", listStylePosition: "outside", color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.7 }}>
        <li><ApiInlineLink endpoint="GET /v1/api-metrics/overall" /> for aggregate totals.</li>
        <li><ApiInlineLink endpoint="GET /v1/api-metrics/summary" /> for per-endpoint latency and error rows.</li>
        <li><ApiInlineLink endpoint="GET /v1/api-metrics/trend" /> for hourly or daily chart buckets.</li>
        <li><ApiInlineLink endpoint="GET /v1/api-metrics/status-codes" /> for exact status-code distribution.</li>
      </ul>
    </section>
  );
}
