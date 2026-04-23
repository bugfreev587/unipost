"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "external_user_id", type: "string", description: "Your own stable end-user identifier." },
  { name: "external_user_email", type: "string | null", description: "Optional email captured during onboarding." },
  { name: "account_count", type: "number", description: "Total connected accounts for that managed user." },
  { name: "platform_counts", type: "object", description: "Breakdown of connected accounts by platform." },
  { name: "reconnect_count", type: "number", description: "How many accounts need reconnect attention." },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];
const SNIPPETS = [
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
const RESPONSE_SNIPPETS = [
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

export default function ListUsersPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="List users"
      description="Returns managed users grouped by your external_user_id values. Use it to see which end users have connected accounts in UniPost."
      method="GET"
      path="/v1/users"
      requestSections={[{ title: "Authorization", items: AUTH_FIELDS }]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
