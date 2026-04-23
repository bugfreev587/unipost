"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "url", type: "string", description: "HTTPS endpoint that should receive UniPost events." },
  { name: "events", type: "string[]", description: "Event names to subscribe to, such as post.published or post.failed." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Webhook subscription ID." },
  { name: "url", type: "string", description: "Stored destination URL." },
  { name: "events", type: "string[]", description: "Subscribed event names." },
  { name: "active", type: "boolean", description: "Whether this subscription is currently enabled." },
  { name: "secret", type: "string", description: "Plaintext signing secret. Returned only once at creation time." },
  { name: "secret_preview", type: "string", description: "Short prefix shown on later reads so humans can identify the secret." },
  { name: "created_at", type: "string", description: "Creation timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Validation errors use "VALIDATION_ERROR"; auth errors use "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error" or "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/webhooks" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "url": "https://api.example.com/unipost/webhooks",
    "events": ["post.published", "post.partial", "post.failed"]
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const webhook = await client.webhooks.create({
  url: "https://api.example.com/unipost/webhooks",
  events: ["post.published", "post.partial", "post.failed"],
});

console.log(webhook.secret); // store this now`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "wh_abc123",
    "url": "https://api.example.com/unipost/webhooks",
    "events": ["post.published", "post.partial", "post.failed"],
    "active": true,
    "secret": "whsec_4b08d9f3ab2a0d11d5728d0389cdb7e5",
    "secret_preview": "whsec_4b…",
    "created_at": "2026-04-23T10:00:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "at least one event is required"
  },
  "request_id": "req_123"
}`,
  },
];

export default function CreateWebhookPage() {
  return (
    <SingleEndpointReferencePage
      section="developer-webhooks"
      title="Create webhook"
      description="Creates a developer webhook subscription for your workspace. UniPost generates the signing secret server-side and returns it exactly once in this response."
      method="POST"
      path="/v1/webhooks"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
