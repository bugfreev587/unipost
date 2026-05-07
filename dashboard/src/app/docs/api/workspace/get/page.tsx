"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a Clerk session JWT for dashboard callers. Both auth modes resolve to the same workspace." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Workspace ID." },
  { name: "name", type: "string", description: "Human-readable workspace name." },
  { name: "per_account_monthly_limit", type: "number | null", description: "Optional per-account monthly publish quota." },
  { name: "usage_modes", type: "string[]", description: 'Active usage modes selected during onboarding. Current values are "publishing" and "agentic".', },
  { name: "created_at", type: "string", description: "Creation timestamp." },
  { name: "updated_at", type: "string", description: "Last update timestamp." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "INTERNAL_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "internal_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/workspace" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const workspace = await client.workspace.get();
console.log(workspace.name);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

workspace = client.workspace.get()
print(workspace["data"]["name"])`,
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

  workspace, err := client.Workspace.Get(context.Background())
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(workspace.Name)
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "ws_123",
    "name": "Acme",
    "per_account_monthly_limit": null,
    "usage_modes": ["publishing"],
    "created_at": "2026-01-04T10:00:00Z",
    "updated_at": "2026-04-23T18:00:00Z"
  }
}`,
  },
];

export default function GetWorkspacePage() {
  return (
    <SingleEndpointReferencePage
      section="core"
      title="Get workspace"
      description="Returns the workspace bound to the authenticated caller. API-key callers get the workspace the key belongs to; dashboard (Clerk) callers get the user's workspace. There is one workspace per account."
      method="GET"
      path="/v1/workspace"
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
