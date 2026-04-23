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
    name: "id",
    type: "string",
    description: "Publishable UniPost account ID.",
  },
  {
    name: "platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "account_name",
    type: "string | null",
    description: "Handle or display name.",
  },
  {
    name: "status",
    type: "string",
    description: 'Connection state such as "active" or "reconnect_required".',
  },
  {
    name: "connection_type",
    type: "string",
    description: '"byo" or "managed".',
  },
  {
    name: "connected_at",
    type: "string",
    description: "Connection timestamp.",
  },
  {
    name: "external_user_id",
    type: "string | null",
    description: "Your Connect user ID, if present.",
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

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts?platform=instagram" \\
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
  ]
}`;

const RESPONSE_401 = `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
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
            name: "UniPost API — GET /v1/social-accounts",
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
                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <div style={{ padding: "16px 18px" }}>
                    <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>GET</span>
                    <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>/v1/social-accounts</code>
                  </div>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                    <ApiFieldList items={AUTH_FIELDS} />
                  </div>
                  <div style={{ padding: "18px" }}>
                    <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Query Params</div>
                    <ApiFieldList items={QUERY_FIELDS} />
                  </div>
                </ApiEndpointCard>

                <ApiEndpointCard method="GET" path="/v1/social-accounts">
                  <div style={{ padding: "18px" }}>
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
              <CodeTabs snippets={RESPONSE_SNIPPETS} />
            </div>
          }
        />
      </ApiReferencePage>
    </>
  );
}
