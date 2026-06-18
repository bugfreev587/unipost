"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { EnumValues } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key. Stream auth uses the header, never a query-string secret." },
  { name: "Accept", type: "text/event-stream", meta: "In header", description: "Required to open the SSE stream." },
  { name: "Last-Event-ID", type: "integer", meta: "In header", description: "Optional. On reconnect, replays retained rows with a greater id. Ignored when after_id is present." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "category?", type: "string", description: <>Exact category filter.<EnumValues values={["publishing", "api_request", "oauth", "webhook", "system"]} /></> },
  { name: "status?", type: "string", description: <>Exact status filter.<EnumValues values={["success", "warning", "error"]} /></> },
  { name: "level?", type: "string", description: <>Exact level filter.<EnumValues values={["debug", "info", "warn", "error"]} /></> },
  { name: "platform?", type: "string", description: "Exact platform filter." },
  { name: "profile_id?", type: "string", description: "Exact profile filter." },
  { name: "social_account_id?", type: "string", description: "Exact account filter." },
  { name: "post_id?", type: "string", description: "Exact post filter." },
  { name: "request_id?", type: "string", description: "Exact request filter." },
  { name: "error_code?", type: "string", description: "Exact error-code filter." },
  { name: "after_id?", type: "integer", description: "Replays retained rows with id greater than this value before entering live mode. Wins over Last-Event-ID." },
];

const EVENT_FIELDS: ApiFieldItem[] = [
  { name: "event", type: "string", description: "Always log.created." },
  { name: "id", type: "integer", description: "The log id. SSE clients echo it as Last-Event-ID on reconnect." },
  { name: "data", type: "object", description: "Compact JSON log object, the same shape as a list row. Never includes raw payloads." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Such as "UNAUTHORIZED", "VALIDATION_ERROR", or "RATE_LIMITED".' },
  { name: "error.normalized_code", type: "string", description: "Lowercase alias." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -N "https://api.unipost.dev/v1/logs/stream?status=error" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Accept: text/event-stream"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

// Async iterator over live log events, with optional replay.
for await (const log of client.logs.stream({ status: "error", after_id: 110000 })) {
  console.log(log.id, log.action);
}`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

for log in client.logs.stream(status="error", after_id=110000):
    print(log["id"], log["action"])`,
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

  stream, err := client.Logs.Stream(context.Background(), unipost.LogStreamParams{Status: "error"})
  if err != nil {
    log.Fatal(err)
  }
  defer stream.Close()

  for stream.Next() {
    l := stream.Event()
    fmt.Println(l.ID, l.Action)
  }
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "text",
    label: "stream",
    code: `event: log.created
id: 110966
data: {"id":110966,"workspace_id":"ae267ee2-298d-4fa8-b6a0-c386000b17af","ts":"2026-06-17T20:16:34.476752Z","level":"error","status":"error","category":"oauth","action":"account.connect.callback_failed","source":"oauth","message":"Failed to persist connected account.","platform":"instagram","error_code":"account_save_failed"}

: keepalive`,
  },
  {
    lang: "json",
    label: "429",
    code: `{
  "error": {
    "code": "RATE_LIMITED",
    "normalized_code": "rate_limited",
    "message": "Too many concurrent log streams"
  },
  "request_id": "req_response_123"
}`,
  },
];

export default function StreamLogsPage() {
  return (
    <SingleEndpointReferencePage
      section="logs"
      title="Stream logs"
      description="Opens a Server-Sent Events stream of new workspace logs for near real-time ingestion. Filter at connection time and optionally replay retained rows before entering live mode."
      method="GET"
      path="/v1/logs/stream"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
        { title: "Event", items: EVENT_FIELDS },
      ]}
      responses={[
        { code: "401", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "429", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <div style={{ borderTop: "1px solid var(--docs-border)", paddingTop: 20 }}>
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 12, background: "var(--docs-bg-elevated)", padding: "18px 20px", display: "grid", gap: 12 }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)" }}>Replay and reconnect</div>
          <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            Each event carries its log <code>id</code>. To resume after a disconnect, either reconnect with <code>after_id=&#123;last_id&#125;</code> or let your SSE client send the standard <code>Last-Event-ID</code> header automatically. When both are present, <code>after_id</code> wins. The server replays retained rows with a greater id in ascending order, then switches to live delivery without dropping or duplicating events.
          </div>
          <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            Replay only reaches rows still within your plan&apos;s retention window. For deep historical backfill, use <code>GET /v1/logs</code> with cursor pagination instead.
          </div>
          <div style={{ fontSize: 14, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            The server sends <code>: keepalive</code> comments every 25 seconds. Events and replay rows are always filtered to your authenticated workspace before they are sent.
          </div>
        </div>
      </div>
    </SingleEndpointReferencePage>
  );
}
