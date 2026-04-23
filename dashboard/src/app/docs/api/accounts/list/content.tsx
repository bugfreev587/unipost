"use client";

import {
  ApiInlineLink,
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  ResponseBlock,
  ErrorTable,
  type ErrorCodeRow,
  type ApiFieldItem,
} from "../../_components/doc-components";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Use your UniPost API key as a Bearer token. Every request to the public API uses workspace-scoped API key authentication.",
  },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform",
    type: "string",
    description: 'Filter returned accounts by destination platform such as "twitter", "linkedin", "instagram", "threads", "tiktok", "youtube", or "bluesky".',
  },
  {
    name: "external_user_id",
    type: "string",
    description: "Filter accounts connected on behalf of a specific end user during a Connect flow. This is the main lookup key when you embed account onboarding into your own product.",
  },
];

const RESPONSE_SUCCESS_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: <>UniPost social account ID. Use this as <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>account_id</code> when creating a post through <ApiInlineLink endpoint="POST /v1/social-posts" />.</>,
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
    description: 'Connection state for the account. In practice this is usually "active" or "reconnect_required".',
  },
  {
    name: "connection_type",
    type: "string",
    description: '"byo" means white-label / your own platform credentials. "managed" means the account was connected through UniPost Connect.',
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

const ERRORS: ErrorCodeRow[] = [
  { code: "UNAUTHORIZED", http: 401, description: "Missing API key, malformed Bearer token, or invalid key." },
  { code: "INTERNAL_ERROR", http: 500, description: "Unexpected server error while reading accounts. Retry the request." },
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
      "account_name": "magicxiaobo416",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "byo",
      "connected_at": "2026-04-02T10:00:00Z",
      "external_user_id": null
    },
    {
      "id": "sa_linkedin_456",
      "platform": "linkedin",
      "account_name": "Xiaobo Yu",
      "account_avatar_url": "https://...",
      "status": "active",
      "connection_type": "managed",
      "connected_at": "2026-04-05T14:30:00Z",
      "external_user_id": "user_abc"
    }
  ]
}`;

const RESPONSE_401 = `{
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Missing or invalid API key."
  }
}`;

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
        description={<>Returns connected social accounts in the current workspace. Use this endpoint to discover publishable <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>account_id</code> values, or to look up accounts created through your embedded Connect flow.</>}
      >
        <ApiReferenceGrid
          left={
            <>
              <ApiEndpointCard method="GET" path="/v1/social-accounts">
                <ApiAccordion title="Authorization" defaultOpen>
                  <ApiFieldList items={AUTH_FIELDS} />
                </ApiAccordion>
                <ApiAccordion title="Query Params" defaultOpen>
                  <ApiFieldList items={QUERY_FIELDS} />
                </ApiAccordion>
                <ApiAccordion title="Response Body" defaultOpen>
                  <ApiFieldList
                    title="200 response fields"
                    items={RESPONSE_SUCCESS_FIELDS}
                  />
                </ApiAccordion>
              </ApiEndpointCard>
            </>
          }
          right={
            <div style={{ display: "grid", gap: 20 }}>
              <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, overflow: "hidden", background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)" }}>
                <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--docs-border)", fontSize: 13, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: "var(--docs-text-faint)" }}>
                  Examples
                </div>
                <div style={{ padding: 16 }}>
                  <CodeTabs snippets={SNIPPETS} />
                </div>
              </div>

              <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, overflow: "hidden", background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)" }}>
                <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--docs-border)", fontSize: 13, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: "var(--docs-text-faint)" }}>
                  Response Examples
                </div>
                <div style={{ padding: 16, display: "grid", gap: 16 }}>
                  <ResponseBlock title="200" code={RESPONSE_200} />
                  <ResponseBlock title="401" code={RESPONSE_401} />
                </div>
              </div>

              <div style={{ border: "1px solid var(--docs-border)", borderRadius: 20, overflow: "hidden", background: "var(--docs-bg-elevated)", boxShadow: "var(--docs-card-shadow)" }}>
                <div style={{ padding: "14px 18px", borderBottom: "1px solid var(--docs-border)", fontSize: 13, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: "var(--docs-text-faint)" }}>
                  Error Codes
                </div>
                <div style={{ padding: 16 }}>
                  <ErrorTable errors={ERRORS} />
                </div>
              </div>
            </div>
          }
        />
      </ApiReferencePage>
    </>
  );
}
