"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "profile_id?", type: "string", description: "Profile that should own the connected account. Required when the workspace has multiple profiles." },
  { name: "platform", type: "string", description: "Platform key such as twitter, linkedin, instagram, threads, tiktok, youtube, or bluesky." },
  { name: "credentials", type: "object", description: "Adapter-specific credentials payload used for direct account connection." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Connected UniPost account ID." },
  { name: "profile_id", type: "string", description: "Profile the account was attached to." },
  { name: "platform", type: "string", description: "Normalized platform name." },
  { name: "account_name", type: "string | null", description: "Resolved username or display name." },
  { name: "status", type: "string", description: 'Connection state such as "active".' },
  { name: "connection_type", type: "string", description: '"byo" or "managed".' },
  { name: "connected_at", type: "string", description: "Connection timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Common values include "UNAUTHORIZED", "VALIDATION_ERROR", and "ACCOUNT_ALREADY_CONNECTED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized", "validation_error", or "account_already_connected".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/accounts/connect" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "profile_id": "pr_brand_us",
    "platform": "bluesky",
    "credentials": {
      "identifier": "alex.bsky.social",
      "password": "app-password"
    }
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const account = await client.accounts.connect({
  profileId: "pr_brand_us",
  platform: "bluesky",
  credentials: {
    identifier: "alex.bsky.social",
    password: process.env.BLUESKY_APP_PASSWORD,
  },
});

console.log(account.id);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "sa_bluesky_123",
    "profile_id": "pr_brand_us",
    "platform": "bluesky",
    "account_name": "alex.bsky.social",
    "status": "active",
    "connection_type": "byo",
    "connected_at": "2026-04-23T09:18:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "ACCOUNT_ALREADY_CONNECTED",
    "normalized_code": "account_already_connected",
    "message": "This bluesky account (alex.bsky.social) is already connected in your workspace. Disconnect the existing one first if you want to reconnect."
  },
  "request_id": "req_123"
}`,
  },
];

export default function ConnectAccountPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Connect account"
      description="Directly connects one social account to a profile in your workspace. When the workspace has multiple profiles, pass profile_id explicitly instead of relying on an implicit default."
      method="POST"
      path="/v1/accounts/connect"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
