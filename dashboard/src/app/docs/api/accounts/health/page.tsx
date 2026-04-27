"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "account_id",
    type: "string",
    description: "Connected social account ID such as sa_twitter_1.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "social_account_id",
    type: "string",
    description: "UniPost account ID the health snapshot belongs to.",
  },
  {
    name: "platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "status",
    type: "string",
    description: 'Derived health state such as "ok", "degraded", or "disconnected".',
  },
  {
    name: "last_successful_post_at",
    type: "string | null",
    description: "Most recent successfully published post timestamp, if one exists.",
  },
  {
    name: "last_error",
    type: "object | null",
    description: "Most recent publish failure, when health is degraded.",
  },
  {
    name: "last_error.code",
    type: "string",
    description: "Coarse failure category such as token expiry or rate limiting.",
  },
  {
    name: "last_error.message",
    type: "string",
    description: "Raw downstream error message when available.",
  },
  {
    name: "last_error.occurred_at",
    type: "string",
    description: "When the most recent failure occurred.",
  },
  {
    name: "token_expires_at",
    type: "string | null",
    description: "Stored token expiration time when the platform provides one.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED", "NOT_FOUND", or "INTERNAL_ERROR".',
  },
  {
    name: "error.normalized_code",
    type: "string",
    description: 'Lowercase alias such as "unauthorized", "not_found", or "internal_error".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable error message.",
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
    code: `curl "https://api.unipost.dev/v1/accounts/sa_twitter_1/health" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const health = await client.accounts.health("sa_twitter_1");
console.log(health.status);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "social_account_id": "sa_twitter_1",
    "platform": "twitter",
    "status": "ok",
    "last_successful_post_at": "2026-04-22T08:30:00Z",
    "last_error": null,
    "token_expires_at": "2026-05-22T10:12:00Z"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Account not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function AccountHealthPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Account health"
      description="Returns the current operational health for one connected account. Use it to decide whether reconnect attention is needed before your app tries to publish."
      method="GET"
      path="/v1/accounts/:account_id/health"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
