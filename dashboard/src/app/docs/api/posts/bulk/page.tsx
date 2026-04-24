"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Content-Type", type: "application/json", meta: "In header", description: "Request body format." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "posts", type: "array", description: "Required batch of publish requests. Minimum 1, maximum 50." },
  { name: "posts[]", type: "object", description: "Each entry uses the same request shape as POST /v1/posts." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data", type: "array", description: "Per-post outcomes in input order." },
  { name: "data[].status", type: "integer", description: "Equivalent single-post HTTP status for that slot." },
  { name: "data[].data", type: "object", description: "Accepted post payload for that slot. Immediate bulk publish returns async post objects, not final platform results." },
  { name: "data[].data.id", type: "string", description: "UniPost post ID for that slot." },
  { name: "data[].data.execution_mode", type: "string", description: 'Successful immediate slots return "async".' },
  { name: "data[].data.status", type: "string", description: 'Initial post state, typically "queued" or "publishing".' },
  { name: "data[].error", type: "object", description: "Per-slot error when that post failed validation before enqueue or the server failed before acceptance." },
  { name: "data[].error.code", type: "string", description: "Machine-readable error code for the slot." },
  { name: "data[].error.normalized_code", type: "string", description: "Lowercase alias for the slot error code." },
  { name: "data[].error.message", type: "string", description: "Human-readable error message for the slot." },
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
    code: `curl -X POST "https://api.unipost.dev/v1/posts/bulk" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "posts": [
      {
        "platform_posts": [
          {
            "account_id": "sa_twitter_789",
            "caption": "Post one"
          }
        ]
      },
      {
        "platform_posts": [
          {
            "account_id": "sa_linkedin_456",
            "caption": "Post two"
          }
        ]
      }
    ]
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const batch = await client.posts.bulk({
  posts: [
    {
      platformPosts: [
        {
          accountId: "sa_twitter_789",
          caption: "Post one",
        },
      ],
    },
    {
      platformPosts: [
        {
          accountId: "sa_linkedin_456",
          caption: "Post two",
        },
      ],
    },
  ],
});`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": [
    {
      "status": 200,
      "data": {
        "id": "post_abc123",
        "execution_mode": "async",
        "status": "queued"
      }
    },
    {
      "status": 422,
      "error": {
        "code": "VALIDATION_ERROR",
        "normalized_code": "validation_error",
        "message": "drafts are not supported in bulk publish — use POST /v1/posts"
      }
    }
  ]
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "posts must contain at least one entry"
  },
  "request_id": "req_123"
}`,
  },
];

export default function BulkPostsPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Bulk publish"
      description="Accepts up to 50 immediate publish requests in one call. Each slot is validated and enqueued independently, so successful slots return async post resources while invalid slots return per-slot errors."
      method="POST"
      path="/v1/posts/bulk"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <div style={{ borderTop: "1px solid var(--docs-border)", paddingTop: 20 }}>
        <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", margin: 0 }}>
          Bulk publish is also asynchronous at the post level. A `200` slot means UniPost accepted that item and queued background delivery. Final publish outcome should be read from the returned post IDs or received via webhooks.
        </p>
      </div>
    </SingleEndpointReferencePage>
  );
}
