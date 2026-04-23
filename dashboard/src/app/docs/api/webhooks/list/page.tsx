"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Webhook subscriptions for the current workspace." },
  { name: "data[].id", type: "string", description: "Webhook subscription ID." },
  { name: "data[].url", type: "string", description: "Destination URL." },
  { name: "data[].events", type: "string[]", description: "Subscribed event names." },
  { name: "data[].active", type: "boolean", description: "Whether delivery is enabled." },
  { name: "data[].secret_preview", type: "string", description: "Short preview of the active signing secret." },
  { name: "meta.total", type: "number", description: "Total webhook count in the response." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/webhooks" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const { data: webhooks } = await client.webhooks.list();
console.log(webhooks.length);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "wh_abc123",
      "url": "https://api.example.com/unipost/webhooks",
      "events": ["post.published", "post.failed"],
      "active": true,
      "secret_preview": "whsec_4b…",
      "created_at": "2026-04-23T10:00:00Z"
    }
  ],
  "meta": {
    "total": 1
  }
}`,
  },
];

export default function ListWebhooksPage() {
  return (
    <SingleEndpointReferencePage
      section="developer-webhooks"
      title="List webhooks"
      description="Returns every developer webhook subscription configured for the current workspace. Plaintext secrets are never returned from list reads."
      method="GET"
      path="/v1/webhooks"
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
