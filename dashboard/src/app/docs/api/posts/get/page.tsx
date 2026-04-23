"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "UniPost post ID such as post_abc123." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "UniPost post ID." },
  { name: "caption", type: "string | null", description: "Top-level caption when one exists." },
  { name: "status", type: "string", description: "Derived lifecycle status." },
  { name: "source", type: "string", description: 'Origin such as "api" or "ui".' },
  { name: "profile_ids", type: "string[]", description: "Distinct profile IDs targeted by the post." },
  { name: "results", type: "array", description: "Per-account publish results." },
  { name: "results[].id", type: "string", description: "Result row ID used for retries and diagnostics." },
  { name: "results[].social_account_id", type: "string", description: "Destination account ID." },
  { name: "results[].platform", type: "string", description: "Destination platform." },
  { name: "results[].account_name", type: "string", description: "Resolved handle or account display name when available." },
  { name: "results[].status", type: "string", description: "Per-account publish status." },
  { name: "results[].url", type: "string | null", description: "Canonical public post URL when available." },
  { name: "results[].debug_curl", type: "string | null", description: "Redacted failing curl trace for debugging, only on failed results." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "unauthorized" or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/social-posts/post_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const post = await client.posts.get("post_abc123");
console.log(post.data.status);
console.log(post.data.results);`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "post_abc123",
    "caption": "Launch update",
    "status": "published",
    "source": "api",
    "profile_ids": ["profile_1"],
    "results": [
      {
        "id": "spr_1",
        "social_account_id": "sa_twitter_789",
        "platform": "twitter",
        "account_name": "@unipost",
        "status": "published",
        "url": "https://x.com/unipost/status/191234567890"
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "404",
    code: `{
  "error": {
    "code": "NOT_FOUND",
    "normalized_code": "not_found",
    "message": "Post not found"
  },
  "request_id": "req_123"
}`,
  },
];

export default function GetPostPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Get post"
      description="Returns one social post with per-account results, diagnostics, and resolved destination metadata."
      method="GET"
      path="/v1/social-posts/:post_id"
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
