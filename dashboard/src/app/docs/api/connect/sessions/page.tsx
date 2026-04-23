"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "../../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const CREATE_BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform",
    type: "string",
    description: "Destination platform for the hosted onboarding flow.",
  },
  {
    name: "external_user_id",
    type: "string",
    description: "Your stable end-user identifier.",
  },
  {
    name: "external_user_email?",
    type: "string",
    description: "Optional email for reconciliation and support.",
  },
  {
    name: "return_url?",
    type: "string",
    description: "Where UniPost redirects the user after completion.",
  },
];

const CREATE_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Connect session ID.",
  },
  {
    name: "url",
    type: "string",
    description: "Hosted onboarding URL to redirect the user to.",
  },
  {
    name: "status",
    type: "string",
    description: 'Initial status, usually "pending".',
  },
  {
    name: "expires_at",
    type: "string | null",
    description: "Expiration timestamp for the hosted session.",
  },
];

const GET_PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "session_id",
    type: "string",
    description: "Connect session ID such as cs_abc123.",
  },
];

const GET_RESPONSE_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "Connect session ID.",
  },
  {
    name: "status",
    type: "string",
    description: 'Session state such as "pending", "completed", or "expired".',
  },
  {
    name: "managed_account_id",
    type: "string | null",
    description: "Resulting UniPost account when the flow completes.",
  },
  {
    name: "external_user_id",
    type: "string",
    description: "Your user identifier associated with the flow.",
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

const CREATE_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/connect/sessions" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform": "twitter",
    "external_user_id": "user_123",
    "external_user_email": "alex@acme.com",
    "return_url": "https://app.acme.com/integrations/done"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const session = await client.connect.createSession({
  platform: "twitter",
  externalUserId: "user_123",
  externalUserEmail: "alex@acme.com",
  returnUrl: "https://app.acme.com/integrations/done",
});

console.log(session.url);`,
  },
];

const GET_SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/connect/sessions/cs_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const session = await client.connect.getSession("cs_abc123");
console.log(session.status);`,
  },
];

const CREATE_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "cs_abc123",
    "url": "https://connect.unipost.dev/session/cs_abc123",
    "status": "pending",
    "expires_at": "2026-04-22T18:00:00Z"
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

const GET_RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "cs_abc123",
    "status": "completed",
    "managed_account_id": "sa_twitter_123",
    "external_user_id": "user_123"
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Connect session not found."
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

export default function ConnectSessionsPage() {
  return (
    <ApiReferencePage
      section="accounts"
      title="Connect sessions"
      description="Hosted onboarding sessions for customer-owned social accounts. Use them when your product needs to connect user-owned destinations without building OAuth flows yourself."
    >
      <div style={{ display: "grid", gap: 34 }}>
        <EndpointBlock
          label="Create session"
          method="POST"
          path="/v1/connect/sessions"
          requestTitle="Request Body"
          requestFields={CREATE_BODY_FIELDS}
          responseFields={CREATE_RESPONSE_FIELDS}
          extraResponses={[{ code: "401", fields: ERROR_FIELDS }]}
          snippets={CREATE_SNIPPETS}
          responseSnippets={CREATE_RESPONSE_SNIPPETS}
        />

        <EndpointBlock
          label="Get session"
          method="GET"
          path="/v1/connect/sessions/:session_id"
          requestTitle="Path Params"
          requestFields={GET_PATH_FIELDS}
          responseFields={GET_RESPONSE_FIELDS}
          extraResponses={[
            { code: "401", fields: ERROR_FIELDS },
            { code: "404", fields: ERROR_FIELDS },
          ]}
          snippets={GET_SNIPPETS}
          responseSnippets={GET_RESPONSE_SNIPPETS}
        />
      </div>
    </ApiReferencePage>
  );
}
