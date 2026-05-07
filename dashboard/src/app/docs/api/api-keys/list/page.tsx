"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "API keys for the authenticated workspace." },
  { name: "data[].id", type: "string", description: "API key record ID." },
  { name: "data[].name", type: "string", description: "Human-readable label." },
  { name: "data[].prefix", type: "string", description: "Safe key prefix such as up_live_abcd." },
  { name: "data[].environment", type: "string", description: '"production" or "test".' },
  { name: "data[].created_at", type: "string", description: "Creation timestamp." },
  { name: "data[].last_used_at", type: "string | null", description: "Last request timestamp, if used." },
  { name: "data[].expires_at", type: "string | null", description: "Optional expiration timestamp." },
  { name: "meta.total", type: "number", description: "Total API keys returned." },
  { name: "meta.limit", type: "number", description: "Applied list limit." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/api-keys" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const { data: keys } = await client.apiKeys.list();
console.log(keys.map((key) => key.prefix));`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

keys = client.api_keys.list()
print([k["prefix"] for k in keys["data"]])`,
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

  page, err := client.APIKeys.List(context.Background())
  if err != nil {
    log.Fatal(err)
  }

  for _, key := range page.Data {
    fmt.Println(key.Prefix)
  }
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var keys = client.apiKeys().list().getData();
keys.forEach(key -> System.out.println(key.get("prefix").asText()));`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "key_123",
      "name": "Production backend",
      "prefix": "up_live_abcd",
      "environment": "production",
      "created_at": "2026-04-23T18:00:00Z",
      "last_used_at": "2026-04-23T18:15:00Z",
      "expires_at": null
    }
  ],
  "meta": { "total": 1, "limit": 1 },
  "request_id": "req_123"
}`,
  },
];

export default function ListApiKeysPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="List API keys"
      description="Lists API keys for the authenticated workspace. After creating a key, set UNIPOST_API_KEY in your environment — the UniPost SDK clients read it automatically by default."
      method="GET"
      path="/v1/api-keys"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
