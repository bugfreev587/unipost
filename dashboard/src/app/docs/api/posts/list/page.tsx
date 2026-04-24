"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "status?", type: "string", description: 'Comma-separated status filter such as "draft,published".' },
  { name: "from?", type: "string", description: "Inclusive RFC-3339 lower bound on created_at." },
  { name: "to?", type: "string", description: "Exclusive RFC-3339 upper bound on created_at." },
  { name: "limit?", type: "integer", description: "Page size. Default 25, max 100." },
  { name: "cursor?", type: "string", description: "Opaque cursor returned as meta.next_cursor from the previous page." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data", type: "array", description: "List of posts in reverse chronological order." },
  { name: "data[].id", type: "string", description: "UniPost post ID." },
  { name: "data[].status", type: "string", description: "Derived lifecycle status for the post." },
  { name: "data[].created_at", type: "string", description: "Creation timestamp." },
  { name: "data[].scheduled_at", type: "string | null", description: "Scheduled publish time when present." },
  { name: "data[].published_at", type: "string | null", description: "Publish time when available." },
  { name: "data[].target_platforms", type: "string[]", description: "Platforms inferred from stored metadata." },
  { name: "meta.limit", type: "integer", description: "Applied page size for this cursor page." },
  { name: "meta.has_more", type: "boolean", description: "Whether another page is available." },
  { name: "meta.next_cursor", type: "string", description: "Cursor to fetch the next page. Empty string means no more results." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
  { name: "next_cursor", type: "string", description: "Legacy top-level alias for meta.next_cursor during the migration window." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "VALIDATION_ERROR".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "validation_error".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/posts?status=published,partial&limit=25" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const page = await client.posts.list({
  status: "published,partial",
  limit: 25,
});

console.log(page.data.length);
console.log(page.nextCursor);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "id": "post_abc123",
      "status": "published",
      "created_at": "2026-04-22T10:00:00Z",
      "published_at": "2026-04-22T10:00:02Z",
      "target_platforms": ["twitter", "linkedin"]
    }
  ],
  "meta": {
    "limit": 25,
    "has_more": false,
    "next_cursor": ""
  },
  "request_id": "req_123",
  "next_cursor": ""
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "Invalid cursor."
  },
  "request_id": "req_123"
}`,
  },
];

export default function ListPostsPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="List posts"
      description="Returns social posts in reverse chronological order with cursor pagination. Use it for feeds, history views, and sync jobs."
      method="GET"
      path="/v1/posts"
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
