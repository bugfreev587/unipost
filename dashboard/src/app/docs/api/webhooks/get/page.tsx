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
  { name: "url", type: "string", description: "Destination URL." },
  { name: "events", type: "string[]", description: "Subscribed event names." },
  { name: "active", type: "boolean", description: "Whether delivery is enabled." },
  { name: "secret_preview", type: "string", description: "Short preview of the current secret." },
  { name: "created_at", type: "string", description: "Creation timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "NOT_FOUND" or "UNAUTHORIZED".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/webhooks/wh_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
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
    "events": ["post.published", "post.failed"],
    "active": true,
    "secret_preview": "whsec_4b…",
    "created_at": "2026-04-23T10:00:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "message": "Webhook not found"
  }
}`,
  },
];

export default function GetWebhookPage() {
  return (
    <SingleEndpointReferencePage
      section="developer-webhooks"
      title="Get webhook"
      description="Returns one webhook subscription. Read calls expose secret_preview only; use rotate when you need a new plaintext signing secret."
      method="GET"
      path="/v1/webhooks/:id"
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
