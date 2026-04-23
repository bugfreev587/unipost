"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Webhook subscription ID." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "url?", type: "string", description: "Replace the destination URL." },
  { name: "events?", type: "string[]", description: "Replace the subscribed event list." },
  { name: "active?", type: "boolean", description: "Enable or disable delivery without deleting the webhook." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Webhook subscription ID." },
  { name: "url", type: "string", description: "Updated destination URL." },
  { name: "events", type: "string[]", description: "Updated event list." },
  { name: "active", type: "boolean", description: "Updated active state." },
  { name: "secret_preview", type: "string", description: "Preview of the existing secret." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "NOT_FOUND", "UNAUTHORIZED", or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "not_found", "unauthorized", or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X PATCH "https://api.unipost.dev/v1/webhooks/wh_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "active": false,
    "events": ["post.failed"]
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const webhook = await client.webhooks.update("wh_abc123", {
  active: false,
  events: ["post.failed"],
});

console.log(webhook.active);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "wh_abc123",
    "url": "https://api.example.com/unipost/webhooks",
    "events": ["post.failed"],
    "active": false,
    "secret_preview": "whsec_4b…",
    "created_at": "2026-04-23T10:00:00Z"
  }
}`,
  },
];

export default function UpdateWebhookPage() {
  return (
    <SingleEndpointReferencePage
      section="developer-webhooks"
      title="Update webhook"
      description="Updates the destination URL, event list, or active state for an existing webhook subscription. This endpoint cannot change the signing secret."
      method="PATCH"
      path="/v1/webhooks/:id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
