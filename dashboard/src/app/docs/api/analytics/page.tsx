"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const SUMMARY_QUERY_FIELDS: ApiFieldItem[] = [
  {
    name: "from?",
    type: "string",
    description: "Period start in ISO-8601 format.",
  },
  {
    name: "to?",
    type: "string",
    description: "Period end in ISO-8601 format.",
  },
  {
    name: "granularity?",
    type: "string",
    description: 'Time bucket such as "day" or "week".',
  },
];

const SUMMARY_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "totals",
    type: "object",
    description: "Workspace-wide totals across the selected period.",
  },
  {
    name: "totals.impressions",
    type: "number",
    description: "Normalized impressions total.",
  },
  {
    name: "trend",
    type: "array",
    description: "Time-series points for charting.",
  },
  {
    name: "by_platform",
    type: "array",
    description: "Breakdown by destination network.",
  },
];

const POST_PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "post_id",
    type: "string",
    description: "Social post ID such as post_abc123.",
  },
];

const POST_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "post_id",
    type: "string",
    description: "Requested social post ID.",
  },
  {
    name: "metrics",
    type: "object",
    description: "Normalized engagement and reach metrics.",
  },
  {
    name: "metrics.likes",
    type: "number",
    description: "Like count for the post.",
  },
  {
    name: "metrics.comments",
    type: "number",
    description: "Comment count for the post.",
  },
  {
    name: "metrics.reach",
    type: "number",
    description: "Reach value when the platform exposes it.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable error message.",
  },
];

const SUMMARY_SNIPPETS = [
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

const POST_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-posts/post_abc123/analytics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const postAnalytics = await client.posts.analytics("post_abc123");`,
  },
];

const SUMMARY_RESPONSE_SNIPPETS = [
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

const POST_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "post_id": "post_abc123",
    "metrics": {
      "likes": 214,
      "comments": 19,
      "reach": 4210
    }
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Post not found."
  }
}`,
  },
];

function EndpointBlock({
  label,
  method,
  path,
  requestTitle,
  requestFields,
  responseFields,
  extraResponses,
  snippets,
  responseSnippets,
}: {
  label: string;
  method: string;
  path: string;
  requestTitle: string;
  requestFields: ApiFieldItem[];
  responseFields: ApiFieldItem[];
  extraResponses: Array<{ code: string; fields: ApiFieldItem[] }>;
  snippets: { lang: string; label: string; code: string }[];
  responseSnippets: { lang: string; label: string; code: string }[];
}) {
  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ fontSize: 18, fontWeight: 700, color: "var(--docs-text)" }}>{label}</div>
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>{method}</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{path}</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                <ApiFieldList items={AUTH_FIELDS} />
              </div>
              <div style={{ padding: "18px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>{requestTitle}</div>
                <ApiFieldList items={requestFields} />
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              <ApiAccordion title="200">
                <ApiFieldList items={responseFields} />
              </ApiAccordion>
              {extraResponses.map((item) => (
                <ApiAccordion key={item.code} title={item.code}>
                  <ApiFieldList items={item.fields} />
                </ApiAccordion>
              ))}
            </ApiEndpointCard>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={snippets} />
            <CodeTabs snippets={responseSnippets} />
          </div>
        }
      />
    </section>
  );
}

export default function AnalyticsPage() {
  return (
    <ApiReferencePage
      section="analytics"
      title="Analytics"
      description="Unified analytics endpoints for workspace rollups and per-post performance. Use them to power dashboards, reports, and agent feedback loops without stitching together platform-specific APIs."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <EndpointBlock
          label="Workspace summary"
          method="GET"
          path="/v1/analytics/summary"
          requestTitle="Query Params"
          requestFields={SUMMARY_QUERY_FIELDS}
          responseFields={SUMMARY_RESPONSE_FIELDS}
          extraResponses={[{ code: "401", fields: ERROR_FIELDS }]}
          snippets={SUMMARY_SNIPPETS}
          responseSnippets={SUMMARY_RESPONSE_SNIPPETS}
        />

        <EndpointBlock
          label="Post analytics"
          method="GET"
          path="/v1/social-posts/:post_id/analytics"
          requestTitle="Path Params"
          requestFields={POST_PATH_FIELDS}
          responseFields={POST_RESPONSE_FIELDS}
          extraResponses={[
            { code: "401", fields: ERROR_FIELDS },
            { code: "404", fields: ERROR_FIELDS },
          ]}
          snippets={POST_SNIPPETS}
          responseSnippets={POST_RESPONSE_SNIPPETS}
        />
      </div>
    </ApiReferencePage>
  );
}
