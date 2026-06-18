"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { EnumValues } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key, or a dashboard Clerk session." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "category?", type: "string", description: <>Filter by log category.<EnumValues values={["publishing", "api_request", "oauth", "webhook", "system"]} /></> },
  { name: "action?", type: "string", description: "Exact log action, for example post.publish.platform_failed." },
  { name: "source?", type: "string", description: <>Origin of the log.<EnumValues values={["api", "dashboard", "worker", "webhook", "oauth"]} /></> },
  { name: "level?", type: "string", description: <>Severity level.<EnumValues values={["debug", "info", "warn", "error"]} /></> },
  { name: "status?", type: "string", description: <>Outcome status.<EnumValues values={["success", "warning", "error"]} /></> },
  { name: "platform?", type: "string", description: "Platform token such as instagram, tiktok, youtube, or linkedin." },
  { name: "profile_id?", type: "string", description: "Filter logs associated with one profile." },
  { name: "social_account_id?", type: "string", description: "Filter logs associated with one connected account." },
  { name: "post_id?", type: "string", description: "Filter logs associated with one post." },
  { name: "request_id?", type: "string", description: "Filter logs associated with one API request." },
  { name: "error_code?", type: "string", description: "Filter logs by normalized error code." },
  { name: "q?", type: "string", description: "Search message, action, request id, post id, and error code." },
  { name: "from?", type: "string", description: "RFC3339 timestamp. Inclusive lower bound. Defaults to 7 days ago — set it explicitly to backfill further." },
  { name: "to?", type: "string", description: "RFC3339 timestamp. Inclusive upper bound." },
  { name: "limit?", type: "integer", description: "Page size. Default 100, maximum 500." },
  { name: "cursor?", type: "string", description: "Opaque cursor from the previous response's meta.next_cursor." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data", type: "array", description: "Log rows, newest first. Never includes request_payload or response_payload." },
  { name: "data[].id", type: "integer", description: "Numeric log id. Use it as after_id when switching to the stream." },
  { name: "data[].workspace_id", type: "string", description: "Owning workspace. Always your authenticated workspace." },
  { name: "data[].ts", type: "string", description: "RFC3339 event timestamp." },
  { name: "data[].level", type: "string", description: "Severity level." },
  { name: "data[].status", type: "string", description: "Outcome status." },
  { name: "data[].category", type: "string", description: "Log category." },
  { name: "data[].action", type: "string", description: "Specific log action." },
  { name: "data[].source", type: "string", description: "Origin of the log." },
  { name: "data[].message", type: "string", description: "Human-readable summary." },
  { name: "data[].request_id", type: "string", description: "Correlates with the X-Request-Id of the originating API call." },
  { name: "data[].error_code", type: "string", description: "Normalized error code when the log represents a failure." },
  { name: "meta.limit", type: "integer", description: "Echoed page size." },
  { name: "meta.has_more", type: "boolean", description: "True when another page exists." },
  { name: "meta.next_cursor", type: "string | null", description: "Cursor for the next page, or null on the last page. meta.total is intentionally omitted." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Such as "UNAUTHORIZED" or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/logs?status=error&limit=50" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

// Backfill every retained log, following cursors.
let cursor;
do {
  const page = await client.logs.list({ status: "error", limit: 100, cursor });
  for (const log of page.data) console.log(log.id, log.action);
  cursor = page.meta.next_cursor;
} while (cursor);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

cursor = None
while True:
    page = client.logs.list(status="error", limit=100, cursor=cursor)
    for log in page["data"]:
        print(log["id"], log["action"])
    cursor = page["meta"]["next_cursor"]
    if not cursor:
        break`,
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

  page, err := client.Logs.List(context.Background(), unipost.LogListParams{Status: "error", Limit: 100})
  if err != nil {
    log.Fatal(err)
  }
  for _, l := range page.Data {
    fmt.Println(l.ID, l.Action)
  }
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var page = client.logs().list(Map.of("status", "error", "limit", "100"));
for (var log : page.get("data")) {
  System.out.println(log.get("id") + " " + log.get("action"));
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": 110966,
      "workspace_id": "ae267ee2-298d-4fa8-b6a0-c386000b17af",
      "ts": "2026-06-17T20:16:34.476752Z",
      "level": "error",
      "status": "error",
      "category": "oauth",
      "action": "account.connect.callback_failed",
      "source": "oauth",
      "message": "Failed to persist connected account.",
      "request_id": "req_abc123",
      "platform": "instagram",
      "error_code": "account_save_failed"
    }
  ],
  "meta": {
    "limit": 100,
    "has_more": false,
    "next_cursor": null
  },
  "request_id": "req_response_123"
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "Invalid cursor"
  },
  "request_id": "req_response_123"
}`,
  },
];

export default function ListLogsPage() {
  return (
    <SingleEndpointReferencePage
      section="logs"
      title="List logs"
      description="Returns workspace logs newest-first with cursor pagination. List rows never include raw request or response payloads. Follow meta.next_cursor to backfill every retained log."
      method="GET"
      path="/v1/logs"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
