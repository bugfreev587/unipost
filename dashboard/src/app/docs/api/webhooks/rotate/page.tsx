"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Webhook subscription ID." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Webhook subscription ID." },
  { name: "name", type: "string", description: "Human-readable webhook label." },
  { name: "secret", type: "string", description: "New plaintext signing secret. Returned only in this rotate response." },
  { name: "secret_preview", type: "string", description: "Short preview of the new secret." },
  { name: "events", type: "string[]", description: "Current subscribed events." },
  { name: "active", type: "boolean", description: "Current active state." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "NOT_FOUND" or "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "not_found" or "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/webhooks/wh_abc123/rotate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const rotated = await client.webhooks.rotate("wh_abc123");
console.log(rotated.secret); // store immediately`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "wh_abc123",
    "name": "Publishing status webhook",
    "url": "https://api.example.com/unipost/webhooks",
    "events": ["post.published", "post.failed"],
    "active": true,
    "secret": "whsec_9f1d67a0f3d9b7c8f4c1f2219bb61a13",
    "secret_preview": "whsec_9f…",
    "created_at": "2026-04-23T10:00:00Z"
  }
}`,
  },
];

export default function RotateWebhookPage() {
  return (
    <SingleEndpointReferencePage
      section="developer-webhooks"
      title="Rotate webhook secret"
      description="Generates a new signing secret for one webhook subscription. Store the returned plaintext secret immediately because later reads show only secret_preview."
      method="POST"
      path="/v1/webhooks/:id/rotate"
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
