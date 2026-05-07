"use client";
import Link from "next/link";
import {
  type ApiFieldItem,
} from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

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
    description: <>Only return accounts for one platform. Current values include <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>twitter</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>linkedin</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>instagram</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>facebook</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>threads</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>youtube</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>tiktok</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>bluesky</code>, and <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>pinterest</code>. <Link href="/docs/platforms#platform-names">[available platforms]</Link></>,
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
    description: 'Normalized platform name. Current values include "twitter", "linkedin", "instagram", "facebook", "threads", "youtube", "tiktok", "bluesky", and "pinterest".',
  },
  {
    name: "data[].account_name",
    type: "string | null",
    description: "Handle or display name.",
  },
  {
    name: "data[].status",
    type: "string",
    description: 'Connection state. Current values are "active", "reconnect_required", and "disconnected" (the list endpoint normally returns non-deleted rows, so most callers see "active" or "reconnect_required").',
  },
  {
    name: "data[].connection_type",
    type: "string",
    description: 'Connection origin. Current values are "byo" and "managed".',
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

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED" or "INTERNAL_ERROR".',
  },
  {
    name: "error.normalized_code",
    type: "string",
    description: 'Lowercase compatibility alias such as "unauthorized" or "internal_error".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable auth error.",
  },
  {
    name: "request_id",
    type: "string",
    description: "Request identifier for debugging and support.",
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

const client = new UniPost();

const { data: accounts } = await client.accounts.list({
  platform: "instagram",
});

console.log(accounts);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

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

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

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

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
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
}`,
  },
];

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

      <SingleEndpointReferencePage
        section="accounts"
        title="List accounts"
        description={<>Returns connected social accounts in the current workspace. Use it to discover publishable <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>account_id</code> values.</>}
      method="GET"
      path="/v1/accounts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
    </>
  );
}
