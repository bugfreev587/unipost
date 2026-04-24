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
import { JsonMonacoTabs } from "../../_components/json-monaco-viewer";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform?",
    type: "string",
    description: "Only return accounts for one platform.",
  },
  {
    name: "external_user_id?",
    type: "string",
    description: "Only return accounts for one Connect user.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "data[]",
    type: "array",
    description: "Connected social accounts in the current workspace.",
  },
  {
    name: "data[].id",
    type: "string",
    description: "Publishable UniPost account ID.",
  },
  {
    name: "data[].platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "data[].account_name",
    type: "string | null",
    description: "Handle or display name.",
  },
  {
    name: "data[].status",
    type: "string",
    description: 'Connection state such as "active" or "reconnect_required".',
  },
  {
    name: "data[].connection_type",
    type: "string",
    description: '"byo" or "managed".',
  },
  {
    name: "data[].connected_at",
    type: "string",
    description: "Connection timestamp.",
  },
  {
    name: "data[].external_user_id",
    type: "string | null",
    description: "Your Connect user ID, if present.",
  },
  {
    name: "meta.total",
    type: "number",
    description: "Total number of returned accounts.",
  },
  {
    name: "meta.limit",
    type: "number",
    description: "Applied list size for this response.",
  },
  {
    name: "request_id",
    type: "string",
    description: "Request identifier for debugging and support.",
  },
];

const RESPONSE_401_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED".',
  },
  {
    name: "error.normalized_code",
    type: "string",
    description: 'Lowercase compatibility alias such as "unauthorized".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable auth error.",
  },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts?platform=instagram" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: accounts } = await client.accounts.list({
  platform: "instagram",
});

console.log(accounts);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

accounts = client.accounts.list(platform="instagram")
print(accounts["data"])`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  accounts, err := client.Accounts.List(context.Background(), &unipost.ListAccountsParams{
    Platform: "instagram",
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = accounts
}`,
  },
];

const RESPONSE_200 = `{
  "data": [
    {
      "id": "sa_instagram_123",
      "platform": "instagram",
      "account_name": "studio.alex",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "byo",
      "connected_at": "2026-04-02T10:00:00Z",
      "external_user_id": null
    }
  ],
  "meta": {
    "total": 1,
    "limit": 1
  },
  "request_id": "req_123"
}`;

const RESPONSE_401 = `{
  "error": {
    "code": "UNAUTHORIZED",
    "normalized_code": "unauthorized",
    "message": "Missing or invalid API key."
  },
  "request_id": "req_123"
}`;

const RESPONSE_TABS = [
  { code: "200", body: RESPONSE_200, fields: RESPONSE_200_FIELDS },
  { code: "401", body: RESPONSE_401, fields: RESPONSE_401_FIELDS },
];

const RESPONSE_SNIPPETS = RESPONSE_TABS.map((tab) => ({
  lang: "json",
  label: tab.code,
  code: tab.body,
}));

export function ListAccountsContent() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{
          __html: JSON.stringify({
            "@context": "https://schema.org",
            "@type": "TechArticle",
            name: "UniPost API — GET /v1/accounts",
            description: "List connected social media accounts",
            url: "https://unipost.dev/docs/api/accounts/list",
            author: { "@type": "Organization", name: "UniPost" },
            dateModified: "2026-04-22",
          }),
        }}
      />

      <ApiReferencePage
        section="accounts"
        title="List accounts"
        description={<>Returns connected social accounts in the current workspace. Use it to discover publishable <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>account_id</code> values.</>}
      >
        <ApiReferenceGrid
          left={
            <>
              <div style={{ display: "grid", gap: 16 }}>
                <ApiRequestConfigCard
                  method="GET"
                  path="/v1/accounts"
                  requestPathTemplate="/v1/accounts?{platform}&{external_user_id}"
                  baseUrl="https://api.unipost.dev"
                  authFields={AUTH_FIELDS}
                  queryFields={QUERY_FIELDS}
                  useMonacoForJsonResponse
                />

                <ApiEndpointCard method="GET" path="/v1/accounts">
                  <div style={{ padding: "16px 18px" }}>
                    <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>GET</span>
                    <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>/v1/accounts</code>
                  </div>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/accounts">
                  <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                    <ApiFieldList items={AUTH_FIELDS} />
                  </div>
                  <div style={{ padding: "18px" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Query Params</div>
                    <ApiFieldList items={QUERY_FIELDS} />
                  </div>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/accounts">
                  <div style={{ padding: "18px 18px 4px" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
                  </div>
                  <ApiAccordion title="200">
                    <ApiFieldList items={RESPONSE_200_FIELDS} />
                  </ApiAccordion>
                  <ApiAccordion title="401">
                    <ApiFieldList items={RESPONSE_401_FIELDS} />
                  </ApiAccordion>
                </ApiEndpointCard>
              </div>
            </>
          }
          right={
            <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
              <CodeTabs snippets={SNIPPETS} />
              <JsonMonacoTabs snippets={RESPONSE_SNIPPETS} />
            </div>
          }
        />
      </ApiReferencePage>
    </>
  );
}
