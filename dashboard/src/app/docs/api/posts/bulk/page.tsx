"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Content-Type", type: "application/json", meta: "In header", description: "Request body format." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "posts", type: "array", description: "Required batch of publish requests. Minimum 1, maximum 50." },
  { name: "posts[]", type: "object", description: "Each entry uses the same request shape as POST /v1/social-posts." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "data", type: "array", description: "Per-post outcomes in input order." },
  { name: "data[].status", type: "integer", description: "Equivalent single-post HTTP status for that slot." },
  { name: "data[].data", type: "object", description: "Successful post payload when the slot succeeded." },
  { name: "data[].error", type: "object", description: "Per-slot error when that post failed validation or publish." },
  { name: "data[].error.code", type: "string", description: "Machine-readable error code for the slot." },
  { name: "data[].error.message", type: "string", description: "Human-readable error message for the slot." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "UNAUTHORIZED" or "VALIDATION_ERROR".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/social-posts/bulk" \\
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
        "status": "published"
      }
    },
    {
      "status": 422,
      "error": {
        "code": "VALIDATION_ERROR",
        "message": "drafts are not supported in bulk publish — use POST /v1/social-posts"
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
    "message": "posts must contain at least one entry"
  }
}`,
  },
];

export default function BulkPostsPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Bulk publish"
      description="Publishes up to 50 immediate posts in one request. Each slot is processed independently, so partial success is returned in the response body instead of failing the entire batch."
      method="POST"
      path="/v1/social-posts/bulk"
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
    />
  );
}
