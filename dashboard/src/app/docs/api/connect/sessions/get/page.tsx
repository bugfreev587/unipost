"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const PATH_FIELDS: ApiFieldItem[] = [
  { name: "session_id", type: "string", description: "Connect session ID such as cs_abc123." },
];
const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Connect session ID." },
  { name: "status", type: "string", description: 'Session state such as "pending", "completed", or "expired".' },
  { name: "managed_account_id", type: "string | null", description: "Resulting UniPost account when the flow completes." },
  { name: "external_user_id", type: "string", description: "Your user identifier associated with the flow." },
];
const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];
const SNIPPETS = [
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
const RESPONSE_SNIPPETS = [
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
    "normalized_code": "not_found",
    "message": "Connect session not found."
  },
  "request_id": "req_123"
}`,
  },
];

export default function GetConnectSessionPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get connect session"
      description="Returns the current status of a hosted Connect session. Use it when polling for completion during onboarding."
      method="GET"
      path="/v1/connect/sessions/:session_id"
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
