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

const LIST_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "external_user_id",
    type: "string",
    description: "Your own stable end-user identifier.",
  },
  {
    name: "external_user_email",
    type: "string | null",
    description: "Optional email captured during onboarding.",
  },
  {
    name: "account_count",
    type: "number",
    description: "Total connected accounts for that managed user.",
  },
  {
    name: "platform_counts",
    type: "object",
    description: "Breakdown of connected accounts by platform.",
  },
  {
    name: "reconnect_count",
    type: "number",
    description: "How many accounts need reconnect attention.",
  },
];

const DETAIL_PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "external_user_id",
    type: "string",
    description: "Managed user identifier in your own product.",
  },
];

const DETAIL_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "external_user_id",
    type: "string",
    description: "Your stable user identifier.",
  },
  {
    name: "accounts",
    type: "array",
    description: "Connected social accounts for that user.",
  },
  {
    name: "accounts[].id",
    type: "string",
    description: "Publishable account ID.",
  },
  {
    name: "accounts[].platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "accounts[].status",
    type: "string",
    description: "Connection health for that account.",
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

const LIST_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/users" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: users } = await client.users.list();
console.log(users.length);`,
  },
];

const DETAIL_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/users/user_abc" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const user = await client.users.get("user_abc");
console.log(user.external_user_id);`,
  },
];

const LIST_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "external_user_id": "user_abc",
      "external_user_email": "alex@example.com",
      "account_count": 3,
      "platform_counts": {
        "instagram": 1,
        "linkedin": 2
      },
      "reconnect_count": 0
    }
  ]
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

const DETAIL_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "external_user_id": "user_abc",
    "accounts": [
      {
        "id": "sa_instagram_123",
        "platform": "instagram",
        "status": "active"
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Managed user not found."
  }
}`,
  },
];

function ReferenceBlock({
  label,
  method,
  path,
  requestTitle,
  requestFields,
  response200Fields,
  responseExtra,
  snippets,
  responseSnippets,
}: {
  label: string;
  method: string;
  path: string;
  requestTitle?: string;
  requestFields?: ApiFieldItem[];
  response200Fields: ApiFieldItem[];
  responseExtra?: Array<{ code: string; fields: ApiFieldItem[] }>;
  snippets: { lang: string; label: string; code: string }[];
  responseSnippets: { lang: string; label: string; code: string }[];
}) {
  const methodColor = method === "GET" ? "#10b981" : "#3b82f6";

  return (
    <section style={{ display: "grid", gap: 18 }}>
      <div style={{ fontSize: 18, fontWeight: 700, color: "var(--docs-text)" }}>{label}</div>
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: methodColor, marginRight: 12 }}>{method}</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{path}</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: requestFields?.length ? "18px" : "18px 18px 6px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                <ApiFieldList items={AUTH_FIELDS} />
              </div>
              {requestFields?.length ? (
                <div style={{ padding: "18px", borderTop: "1px solid var(--docs-border)" }}>
                  <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>{requestTitle}</div>
                  <ApiFieldList items={requestFields} />
                </div>
              ) : null}
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              <ApiAccordion title="200">
                <ApiFieldList items={response200Fields} />
              </ApiAccordion>
              {(responseExtra || []).map((item) => (
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

export default function ManagedUsersPage() {
  return (
    <ApiReferencePage
      section="core"
      title="Managed users"
      description="Groups customer-owned social accounts by your own external_user_id. Use these endpoints to power a connected-accounts view inside your own product."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <ReferenceBlock
          label="List users"
          method="GET"
          path="/v1/users"
          response200Fields={LIST_RESPONSE_FIELDS}
          responseExtra={[{ code: "401", fields: ERROR_FIELDS }]}
          snippets={LIST_SNIPPETS}
          responseSnippets={LIST_RESPONSE_SNIPPETS}
        />

        <ReferenceBlock
          label="Get user"
          method="GET"
          path="/v1/users/:external_user_id"
          requestTitle="Path Params"
          requestFields={DETAIL_PATH_FIELDS}
          response200Fields={DETAIL_RESPONSE_FIELDS}
          responseExtra={[
            { code: "401", fields: ERROR_FIELDS },
            { code: "404", fields: ERROR_FIELDS },
          ]}
          snippets={DETAIL_SNIPPETS}
          responseSnippets={DETAIL_RESPONSE_SNIPPETS}
        />
      </div>
    </ApiReferencePage>
  );
}
