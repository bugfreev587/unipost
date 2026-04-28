"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers. A key can revoke itself; the next request authenticated with that key will fail." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "key_id", type: "string", description: "API key record ID to revoke." },
];

const RESPONSE_204_FIELDS: ApiFieldItem[] = [
  { name: "status", type: "204 No Content", description: "Revoked successfully; no response body." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED", "NOT_FOUND", or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X DELETE "https://api.unipost.dev/v1/api-keys/key_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

await client.apiKeys.revoke("key_123");`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

client.api_keys.revoke("key_123")`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "log"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient()

  if err := client.APIKeys.Revoke(context.Background(), "key_123"); err != nil {
    log.Fatal(err)
  }
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "API key not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function DeleteApiKeyPage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Revoke API key"
      description="Revokes one API key from the authenticated workspace. Subsequent requests using the revoked key fail with 401. Revoking a key cannot be undone — generate a new one if you need to restore access."
      method="DELETE"
      path="/v1/api-keys/:key_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "204", fields: RESPONSE_204_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
