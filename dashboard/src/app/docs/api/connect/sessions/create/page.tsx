"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "platform", type: "string", description: "Destination platform for the hosted onboarding flow." },
  { name: "external_user_id", type: "string", description: "Your stable end-user identifier." },
  { name: "external_user_email?", type: "string", description: "Optional email for reconciliation and support." },
  { name: "return_url?", type: "string", description: "Where UniPost redirects the user after completion." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Connect session ID." },
  { name: "url", type: "string", description: "Hosted onboarding URL to redirect the user to." },
  { name: "status", type: "string", description: 'Initial status, usually "pending".' },
  { name: "expires_at", type: "string | null", description: "Expiration timestamp for the hosted session." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
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

const RESPONSE_SNIPPETS = [
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

export default function CreateConnectSessionPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Create connect session"
      description="Creates a hosted onboarding session for a customer-owned social account. Use the returned URL to send the end user into UniPost's managed Connect flow."
      method="POST"
      path="/v1/connect/sessions"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
