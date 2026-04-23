"use client";

import { useState } from "react";
import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  ResponseBlock,
  type ApiFieldItem,
} from "../../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Use your UniPost API key as a Bearer token.",
  },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform",
    type: "string",
    description: 'Filter returned accounts by platform such as "twitter", "linkedin", "instagram", or "bluesky".',
  },
  {
    name: "external_user_id",
    type: "string",
    description: "Filter accounts connected on behalf of a specific end user during a Connect flow.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "UniPost social account ID used in publish requests.",
  },
  {
    name: "platform",
    type: "string",
    description: "Normalized platform identifier returned by UniPost for this account.",
  },
  {
    name: "account_name",
    type: "string | null",
    description: "Human-readable handle or display name returned by the platform.",
  },
  {
    name: "status",
    type: "string",
    description: 'Connection state for the account, usually "active" or "reconnect_required".',
  },
  {
    name: "connection_type",
    type: "string",
    description: '"byo" means your own platform credentials. "managed" means the account was connected through UniPost Connect.',
  },
  {
    name: "connected_at",
    type: "string",
    description: "ISO 8601 timestamp for when the account was connected.",
  },
  {
    name: "external_user_id",
    type: "string | null",
    description: "Your own end-user identifier from Connect. Null for workspace-owned or BYO accounts.",
  },
];

const RESPONSE_401_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Returns "UNAUTHORIZED" when the API key is missing or invalid.',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable authentication failure message.",
  },
];

const SNIPPETS = [
  {
    lang: "js",
    label: "Node.js",
    code: `const response = await fetch(
  "https://api.unipost.dev/v1/social-accounts",
  {
    headers: {
      Authorization: "Bearer up_live_xxxx",
    },
  }
);

const { data } = await response.json();
console.log(data);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `import requests

response = requests.get(
    "https://api.unipost.dev/v1/social-accounts",
    headers={"Authorization": "Bearer up_live_xxxx"},
)

data = response.json()["data"]
print(data)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `req, _ := http.NewRequest(
    "GET",
    "https://api.unipost.dev/v1/social-accounts",
    nil,
)
req.Header.Set("Authorization", "Bearer up_live_xxxx")

resp, _ := http.DefaultClient.Do(req)`,
  },
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-accounts" \\
  -H "Authorization: Bearer up_live_xxxx"`,
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

function ResponseExampleTabs() {
  const [activeCode, setActiveCode] = useState("200");
  const active = RESPONSE_TABS.find((tab) => tab.code === activeCode) || RESPONSE_TABS[0];

  return (
    <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, overflow: "hidden", background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)" }}>
      <div style={{ display: "flex", gap: 18, padding: "14px 18px 0", borderBottom: "1px solid var(--docs-border)" }}>
        {RESPONSE_TABS.map((tab) => {
          const activeTab = tab.code === activeCode;
          return (
            <button
              key={tab.code}
              type="button"
              onClick={() => setActiveCode(tab.code)}
              style={{
                border: 0,
                borderBottom: activeTab ? "2px solid #f04d23" : "2px solid transparent",
                background: "transparent",
                color: activeTab ? "#f04d23" : "var(--docs-text-muted)",
                fontSize: 14,
                fontWeight: 700,
                padding: "0 0 12px",
                cursor: "pointer",
              }}
            >
              {tab.code}
            </button>
          );
        })}
      </div>
      <div style={{ padding: 16 }}>
        <ResponseBlock title={active.code} code={active.body} />
      </div>
    </div>
  );
}

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
              <ApiEndpointCard method="GET" path="/v1/social-accounts">
                <div style={{ padding: "16px 18px", borderBottom: "1px solid var(--docs-border)" }}>
                  <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: "#10b981", marginRight: 12 }}>GET</span>
                  <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>/v1/social-accounts</code>
                </div>
                <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                  <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Authorization</div>
                  <ApiFieldList items={AUTH_FIELDS} />
                </div>
                <div style={{ padding: "18px", borderBottom: "1px solid var(--docs-border)" }}>
                  <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Query Params</div>
                  <ApiFieldList items={QUERY_FIELDS} />
                </div>
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
            </>
          }
          right={
            <div style={{ display: "grid", gap: 20 }}>
              <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, overflow: "hidden", background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)" }}>
                <div style={{ padding: 16 }}>
                  <CodeTabs snippets={SNIPPETS} />
                </div>
              </div>

              <ResponseExampleTabs />
            </div>
          }
        />
      </ApiReferencePage>
    </>
  );
}
