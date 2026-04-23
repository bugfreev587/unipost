"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  ApiRequestConfigCard,
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

const PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "account_id",
    type: "string",
    description: "Connected social account ID such as sa_twitter_1.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "status",
    type: "string",
    description: 'High-level account state such as "active" or "reconnect_required".',
  },
  {
    name: "token_refreshed_at",
    type: "string | null",
    description: "Last successful token refresh timestamp.",
  },
  {
    name: "last_publish_at",
    type: "string | null",
    description: "Most recent publish attempt timestamp.",
  },
  {
    name: "last_publish_status",
    type: "string | null",
    description: "Outcome of the most recent publish attempt.",
  },
  {
    name: "last_publish_error",
    type: "string | null",
    description: "Most recent downstream platform error, if one exists.",
  },
];

const RESPONSE_401_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable auth error.",
  },
];

const RESPONSE_404_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "NOT_FOUND".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Returned when the account is missing or outside the workspace.",
  },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts/sa_twitter_1/health" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const health = await client.accounts.health("sa_twitter_1");
console.log(health.status);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "status": "active",
    "token_refreshed_at": "2026-04-22T10:12:00Z",
    "last_publish_at": "2026-04-22T08:30:00Z",
    "last_publish_status": "published",
    "last_publish_error": null
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
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Account not found."
  }
}`,
  },
];

export default function AccountHealthPage() {
  return (
    <ApiReferencePage
      section="accounts"
      title="Account health"
      description="Returns the current operational health for one connected account. Use it to decide whether reconnect attention is needed before your app tries to publish."
    >
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiRequestConfigCard
              method="GET"
              path="/v1/social-accounts/:account_id/health"
              requestPathTemplate="/v1/social-accounts/:account_id/health"
              baseUrl="https://api.unipost.dev"
              authFields={AUTH_FIELDS}
              pathFields={PATH_FIELDS}
              useMonacoForJsonResponse
            />

            <ApiEndpointCard method="GET" path="/v1/social-accounts/:account_id/health">
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>GET</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>/v1/social-accounts/:account_id/health</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method="GET" path="/v1/social-accounts/:account_id/health">
              <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                <ApiFieldList items={AUTH_FIELDS} />
              </div>
              <div style={{ padding: "18px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Path Params</div>
                <ApiFieldList items={PATH_FIELDS} />
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method="GET" path="/v1/social-accounts/:account_id/health">
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              <ApiAccordion title="200">
                <ApiFieldList items={RESPONSE_200_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="401">
                <ApiFieldList items={RESPONSE_401_FIELDS} />
              </ApiAccordion>
              <ApiAccordion title="404">
                <ApiFieldList items={RESPONSE_404_FIELDS} />
              </ApiAccordion>
            </ApiEndpointCard>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={SNIPPETS} />
            <CodeTabs snippets={RESPONSE_SNIPPETS} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}
