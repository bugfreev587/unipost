"use client";

import { ApiReferencePage, CodeTabs, MethodBadge, ParamTable } from "../_components/doc-components";

const QUERY_PARAMS = [
  { name: "from", type: "RFC3339", required: false, description: "Start timestamp. Defaults to 7 days before now." },
  { name: "to", type: "RFC3339", required: false, description: "End timestamp. Defaults to now. The maximum raw range is 90 days." },
  { name: "interval", type: "hour | day", required: false, description: "Trend bucket size. Defaults to hour for short ranges and day for longer ranges." },
  { name: "method", type: "GET | POST | PUT | PATCH | DELETE", required: false, description: "Filter by HTTP method." },
  { name: "path", type: "string", required: false, description: "Filter by normalized endpoint path such as /v1/posts/:id/publish." },
  { name: "status_class", type: "2xx | 3xx | 4xx | 5xx", required: false, description: "Filter by HTTP status class." },
  { name: "sort", type: "string", required: false, description: "For summary endpoints: total_calls_desc, p95_ms_desc, p99_ms_desc, server_errors_desc, or rate_limited_desc." },
  { name: "limit", type: "integer", required: false, description: "Maximum rows to return. Defaults to 50, maximum 200." },
];

const METRICS = [
  ["total_calls", "Number of matching API requests."],
  ["success_count", "Responses with status below 400."],
  ["client_error_count", "Responses from 400 through 499, including 429."],
  ["server_error_count", "Responses with status 500 or higher."],
  ["rate_limited_count", "Responses with status 429, also included in client_error_count."],
  ["error_rate_pct", "All 4xx and 5xx responses divided by total calls, including 429 rate-limit responses."],
  ["server_failure_rate_pct", "5xx responses divided by total calls."],
  ["p50_ms / p95_ms / p99_ms", "Latency percentiles measured at the UniPost API layer."],
  ["avg_ms", "Average latency in milliseconds."],
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
    label: "Status Codes",
    code: `curl "https://api.unipost.dev/v1/api-metrics/status-codes?status_class=4xx" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
];

const RESPONSE = [
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
];

export default function APIMetricsDocsPage() {
  return (
    <ApiReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "API Metrics" }]}
      section="api"
      title="API Metrics"
      description="Query workspace-scoped latency, volume, and error metrics for API-key calls to the UniPost Developer API."
    >
      <div style={{ display: "grid", gap: 28 }}>
        <section>
          <h2 style={{ fontSize: 18, margin: "0 0 12px", color: "var(--docs-text)" }}>Endpoints</h2>
          <div style={{ display: "grid", gap: 8 }}>
            {[
              ["/v1/api-metrics/overall", "Aggregate calls, errors, reliability, and latency."],
              ["/v1/api-metrics/summary", "Per-method and per-normalized-path metrics."],
              ["/v1/api-metrics/trend", "Hourly or daily buckets for calls, errors, and latency."],
              ["/v1/api-metrics/status-codes", "Exact HTTP status-code distribution by endpoint."],
            ].map(([path, description]) => (
              <div key={path} style={{ display: "grid", gridTemplateColumns: "88px minmax(0, 1fr)", gap: 12, alignItems: "center", padding: 12, border: "1px solid var(--docs-border)", borderRadius: 8 }}>
                <MethodBadge method="GET" />
                <div>
                  <code style={{ fontFamily: "var(--docs-mono)", color: "var(--docs-text)" }}>{path}</code>
                  <div style={{ fontSize: 13, color: "var(--docs-text-soft)", marginTop: 4 }}>{description}</div>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section>
          <h2 style={{ fontSize: 18, margin: "0 0 12px", color: "var(--docs-text)" }}>Scope</h2>
          <p style={{ color: "var(--docs-text-soft)", fontSize: 14, lineHeight: 1.7 }}>
            Metrics include API-key-authenticated Developer API traffic only. Dashboard session traffic, admin routes, public hosted flows, OAuth callbacks, inbound provider webhooks, health checks, and the API metrics endpoints themselves are excluded.
          </p>
          <p style={{ color: "var(--docs-text-soft)", fontSize: 14, lineHeight: 1.7, marginTop: 10 }}>
            Trend <code style={{ fontFamily: "var(--docs-mono)" }}>error_count</code> uses the same 4xx-plus-5xx definition as <code style={{ fontFamily: "var(--docs-mono)" }}>error_rate_pct</code>; 429 responses are also exposed separately through <code style={{ fontFamily: "var(--docs-mono)" }}>rate_limited_count</code>.
          </p>
        </section>

        <ParamTable title="Query parameters" params={QUERY_PARAMS} />

        <section>
          <h2 style={{ fontSize: 18, margin: "0 0 12px", color: "var(--docs-text)" }}>Metric fields</h2>
          <div style={{ borderTop: "1px solid var(--docs-border)" }}>
            {METRICS.map(([name, description]) => (
              <div key={name} style={{ display: "grid", gridTemplateColumns: "210px minmax(0, 1fr)", gap: 14, padding: "10px 0", borderBottom: "1px solid var(--docs-border)" }}>
                <code style={{ fontFamily: "var(--docs-mono)", color: "var(--docs-text)" }}>{name}</code>
                <span style={{ color: "var(--docs-text-soft)", fontSize: 13.5 }}>{description}</span>
              </div>
            ))}
          </div>
        </section>

        <section>
          <h2 style={{ fontSize: 18, margin: "0 0 12px", color: "var(--docs-text)" }}>Examples</h2>
          <CodeTabs snippets={SNIPPETS} />
        </section>

        <section>
          <h2 style={{ fontSize: 18, margin: "0 0 12px", color: "var(--docs-text)" }}>Response</h2>
          <CodeTabs snippets={RESPONSE} />
        </section>
      </div>
    </ApiReferencePage>
  );
}
