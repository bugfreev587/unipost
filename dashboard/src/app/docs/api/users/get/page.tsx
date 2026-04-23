"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const PATH_FIELDS: ApiFieldItem[] = [
  { name: "external_user_id", type: "string", description: "Managed user identifier in your own product." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "external_user_id", type: "string", description: "Your stable user identifier." },
  { name: "accounts", type: "array", description: "Connected social accounts for that user." },
  { name: "accounts[].id", type: "string", description: "Publishable account ID." },
  { name: "accounts[].platform", type: "string", description: "Normalized platform name." },
  { name: "accounts[].status", type: "string", description: "Connection health for that account." },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];
const SNIPPETS = [
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
const RESPONSE_SNIPPETS = [
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

export default function GetUserPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get user"
      description="Returns one managed user and the connected accounts that belong to that user. Use it to render a connected-accounts view inside your product."
      method="GET"
      path="/v1/users/:external_user_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
