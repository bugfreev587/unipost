"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "name", type: "string", description: "Human-readable label for the key." },
  { name: "environment?", type: '"production" | "test"', description: 'Defaults to "production" when omitted.' },
  { name: "expires_at?", type: "string", description: "Optional RFC3339 expiration timestamp." },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "API key record ID." },
  { name: "name", type: "string", description: "Human-readable key label." },
  { name: "key", type: "string", description: "Full plaintext API key. Returned only once at creation time." },
  { name: "prefix", type: "string", description: "Safe prefix for future display." },
  { name: "environment", type: "string", description: '"production" or "test".' },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "VALIDATION_ERROR" or "UNAUTHORIZED".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/api-keys" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "Production backend",
    "environment": "production"
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const created = await client.apiKeys.create({
  name: "Production backend",
  environment: "production",
});

console.log(created.key); // store this now`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

created = client.api_keys.create(
  name="Production backend",
  environment="production",
)

print(created["data"]["key"])  # store this now`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "fmt"
  "log"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

  created, err := client.APIKeys.Create(context.Background(), &unipost.CreateAPIKeyParams{
    Name:        "Production backend",
    Environment: "production",
  })
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(created.Key) // store this now
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "key_123",
    "name": "Production backend",
    "key": "up_live_abc123secret",
    "prefix": "up_live_abc1",
    "environment": "production",
    "created_at": "2026-04-23T18:00:00Z"
  },
  "request_id": "req_123"
}`,
  },
];

export default function CreateApiKeyPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Create API key"
      description="Creates a new API key for the authenticated workspace. The plaintext key is only returned in this creation response — store it before navigating away. An existing API key can mint additional keys, matching common SaaS patterns; the very first key must be created via the dashboard since no API key exists yet."
      method="POST"
      path="/v1/api-keys"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
