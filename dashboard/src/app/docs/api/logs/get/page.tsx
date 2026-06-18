"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a dashboard Clerk session." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "integer", description: "Numeric log id, for example 110966." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data.id", type: "integer", description: "Numeric log id." },
  { name: "data.workspace_id", type: "string", description: "Owning workspace. Always your authenticated workspace." },
  { name: "data.ts", type: "string", description: "RFC3339 event timestamp." },
  { name: "data.level", type: "string", description: "Severity level." },
  { name: "data.status", type: "string", description: "Outcome status." },
  { name: "data.category", type: "string", description: "Log category." },
  { name: "data.action", type: "string", description: "Specific log action." },
  { name: "data.source", type: "string", description: "Origin of the log." },
  { name: "data.message", type: "string", description: "Human-readable summary." },
  { name: "data.metadata", type: "object", description: "Structured, log-specific context." },
  { name: "data.request_payload", type: "object | null", description: "Redacted captured request. Sensitive keys are replaced with [REDACTED]." },
  { name: "data.response_payload", type: "object | null", description: "Redacted captured response." },
  { name: "request_id", type: "string", description: "Request identifier for this API call." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/logs/110966" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const log = await client.logs.get(110966);
console.log(log.data.action, log.data.request_payload);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

log = client.logs.get(110966)
print(log["data"]["action"], log["data"]["request_payload"])`,
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

  entry, err := client.Logs.Get(context.Background(), 110966)
  if err != nil {
    log.Fatal(err)
  }
  fmt.Println(entry.Action, entry.RequestPayload)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var log = client.logs().get(110966);
System.out.println(log.get("action").asText());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": 110966,
    "workspace_id": "ae267ee2-298d-4fa8-b6a0-c386000b17af",
    "ts": "2026-06-17T20:16:34.476752Z",
    "level": "error",
    "status": "error",
    "category": "oauth",
    "action": "account.connect.callback_failed",
    "source": "oauth",
    "message": "Failed to persist connected account.",
    "platform": "instagram",
    "error_code": "account_save_failed",
    "metadata": { "external_user_id": "0669764b-8862-4094-be5f-db7bb70361ad" },
    "request_payload": { "headers": { "Authorization": "[REDACTED]" } },
    "response_payload": { "error": "Provider returned validation_error" }
  },
  "request_id": "req_response_123"
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Log not found"
  },
  "request_id": "req_response_123"
}`,
  },
];

export default function GetLogPage() {
  return (
    <SingleEndpointReferencePage
      section="logs"
      title="Get log"
      description="Returns one log for your workspace, including redacted request and response payloads. A log id that belongs to another workspace returns 404 NOT_FOUND."
      method="GET"
      path="/v1/logs/:id"
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
