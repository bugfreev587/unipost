"use client";

import {
  ApiInlineLink,
  type ApiFieldItem,
  RelatedEndpoints,
} from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
  {
    name: "Content-Type",
    type: "application/json",
    meta: "In header",
    description: "Request body format.",
  },
  {
    name: "Idempotency-Key?",
    type: "string",
    meta: "In header",
    description: "Optional alternative to the body field. Max 64 chars, 24 hour replay window.",
  },
];

const BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "caption?",
    type: "string",
    description: "Shared caption sent to every target account. Required unless platform_posts is set.",
  },
  {
    name: "account_ids?",
    type: "string[]",
    description: "Target social account IDs when the same caption/media should fan out everywhere.",
  },
  {
    name: "platform_posts?",
    type: "array",
    description: "Recommended per-account request shape for multi-platform publishing.",
  },
  {
    name: "media_urls?",
    type: "string[]",
    description: "Public URLs for hosted media. Ignored when platform_posts overrides are used.",
  },
  {
    name: "media_ids?",
    type: "string[]",
    description: <>Media library IDs returned by <ApiInlineLink endpoint="POST /v1/media" />. Use these for local files and larger videos.</>,
  },
  {
    name: "scheduled_at?",
    type: "string",
    description: "ISO-8601 timestamp. If present, UniPost stores the post in scheduled state and the scheduler enqueues delivery later.",
  },
  {
    name: "idempotency_key?",
    type: "string",
    description: "Optional body-level idempotency key. Replays the original response for the same workspace and payload.",
  },
  {
    name: "status?",
    type: '"draft"',
    description: <>Set to <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>&quot;draft&quot;</code> to save without dispatching any platform jobs. Drafts are published later via <ApiInlineLink endpoint="POST /v1/social-posts/:post_id/publish" />.</>,
  },
];

const PLATFORM_POST_FIELDS: ApiFieldItem[] = [
  {
    name: "platform_posts[].account_id",
    type: "string",
    description: "Connected social account to publish to.",
  },
  {
    name: "platform_posts[].caption?",
    type: "string",
    description: "Account-specific caption override.",
  },
  {
    name: "platform_posts[].media_urls?",
    type: "string[]",
    description: "Account-specific hosted asset URLs.",
  },
  {
    name: "platform_posts[].media_ids?",
    type: "string[]",
    description: "Account-specific media library asset IDs.",
  },
  {
    name: "platform_posts[].thread_position?",
    type: "integer",
    description: "1-indexed thread slot. Supported for X and Bluesky thread publishing.",
  },
  {
    name: "platform_posts[].first_comment?",
    type: "string",
    description: "Optional first reply/comment after publish. Supported on X, LinkedIn, and Instagram.",
  },
  {
    name: "platform_posts[].platform_options?",
    type: "object",
    description: "Platform-specific options such as Instagram media type or YouTube metadata.",
  },
];

const RESPONSE_202_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "UniPost post ID.",
  },
  {
    name: "execution_mode",
    type: "string",
    description: 'Immediate publish returns "async" because the request enqueues delivery jobs and returns before platform dispatch finishes.',
  },
  {
    name: "caption",
    type: "string | null",
    description: "Top-level shared caption when one exists.",
  },
  {
    name: "status",
    type: "string",
    description: 'Initial lifecycle state for the accepted post. Immediate creates usually return "queued" or "publishing", not final success/failure.',
  },
  {
    name: "queued_results_count",
    type: "integer",
    description: "How many per-account delivery results were queued for background processing.",
  },
  {
    name: "active_job_count",
    type: "integer",
    description: "How many queue jobs are currently active for this post.",
  },
  {
    name: "created_at",
    type: "string",
    description: "Creation timestamp.",
  },
  {
    name: "scheduled_at",
    type: "string | null",
    description: "Scheduled publish timestamp when queued.",
  },
  {
    name: "published_at",
    type: "string | null",
    description: "Final publish timestamp when at least one result was published.",
  },
  {
    name: "results",
    type: "array",
    description: "Initial per-account result rows created at enqueue time.",
  },
  {
    name: "results[].social_account_id",
    type: "string",
    description: "Destination account ID.",
  },
  {
    name: "results[].platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "results[].status",
    type: "string",
    description: 'Initial per-account state, usually "queued" or another pre-final status until workers finish dispatch.',
  },
  {
    name: "results[].external_id",
    type: "string | null",
    description: "Platform-native post identifier when available after delivery completes.",
  },
  {
    name: "results[].error_message",
    type: "string | null",
    description: "Platform-specific failure reason when delivery eventually fails.",
  },
  {
    name: "request_id",
    type: "string",
    description: "Request identifier for debugging and support.",
  },
];

const RESPONSE_201_FIELDS: ApiFieldItem[] = [
  {
    name: "id",
    type: "string",
    description: "UniPost post ID.",
  },
  {
    name: "status",
    type: "string",
    description: 'Created resource state such as "scheduled" or "draft".',
  },
  {
    name: "created_at",
    type: "string",
    description: "Creation timestamp.",
  },
  {
    name: "scheduled_at",
    type: "string | null",
    description: "Scheduled publish time when the post was created as scheduled content.",
  },
  {
    name: "request_id",
    type: "string",
    description: "Request identifier for debugging and support.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Machine-readable error code." },
  { name: "error.normalized_code", type: "string", description: "Lowercase compatibility alias for the error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/social-posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: launch-day-2026-04-22" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_789",
        "caption": "v1.4 is live"
      },
      {
        "account_id": "sa_linkedin_456",
        "caption": "We shipped v1.4 with webhooks and bulk publishing."
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

const post = await client.posts.create({
  idempotencyKey: "launch-day-2026-04-22",
  platformPosts: [
    {
      accountId: "sa_twitter_789",
      caption: "v1.4 is live",
    },
    {
      accountId: "sa_linkedin_456",
      caption: "We shipped v1.4 with webhooks and bulk publishing.",
    },
  ],
});

console.log(post.id);
console.log(post.status); // queued/publishing
console.log(post.executionMode); // async`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

post = client.posts.create(
  idempotency_key="launch-day-2026-04-22",
  platform_posts=[
    {
      "account_id": "sa_twitter_789",
      "caption": "v1.4 is live",
    },
    {
      "account_id": "sa_linkedin_456",
      "caption": "We shipped v1.4 with webhooks and bulk publishing.",
    },
  ],
)

print(post["data"]["id"])
print(post["data"]["status"])  # queued/publishing
print(post["data"]["execution_mode"])`,
  },
  {
    lang: "go",
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  post, err := client.Posts.Create(context.Background(), &unipost.CreatePostParams{
    IdempotencyKey: "launch-day-2026-04-22",
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_789",
        Caption:   "v1.4 is live",
      },
      {
        AccountID: "sa_linkedin_456",
        Caption:   "We shipped v1.4 with webhooks and bulk publishing.",
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }

  _, _, _ = post.ID, post.Status, post.ExecutionMode
}`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "202",
    code: `{
  "data": {
    "id": "post_abc123",
    "execution_mode": "async",
    "caption": null,
    "status": "queued",
    "queued_results_count": 2,
    "active_job_count": 2,
    "created_at": "2026-04-22T10:00:00Z",
    "published_at": null,
    "results": [
      {
        "social_account_id": "sa_twitter_789",
        "platform": "twitter",
        "status": "queued",
        "external_id": null,
        "error_message": null
      },
      {
        "social_account_id": "sa_linkedin_456",
        "platform": "linkedin",
        "status": "queued",
        "external_id": null,
        "error_message": null
      }
    ]
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "201",
    code: `{
  "data": {
    "id": "post_sched_123",
    "caption": "Launch update",
    "status": "scheduled",
    "created_at": "2026-04-22T10:00:00Z",
    "scheduled_at": "2026-04-23T16:00:00Z"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "422",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "Caption exceeds maximum length for twitter (280 characters)"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "CONFLICT",
    "normalized_code": "conflict",
    "message": "Idempotency key already used with different request body."
  },
  "request_id": "req_123"
}`,
  },
];

export function CreatePostContent() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Create post"
      description={<>Creates a UniPost post resource using the same payload shape as <ApiInlineLink endpoint="POST /v1/social-posts/validate" />. Immediate publish requests are accepted and queued asynchronously; background workers perform the actual platform delivery. Use scheduling or draft mode when you want creation without immediate dispatch.</>}
      method="POST"
      path="/v1/social-posts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
        { title: "platform_posts[]", items: PLATFORM_POST_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_202_FIELDS },
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <div style={{ borderTop: "1px solid var(--docs-border)", paddingTop: 20 }}>
        <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", margin: "0 0 14px" }}>
          Immediate publish is asynchronous. A successful create response means UniPost accepted the request, created per-account result rows, and enqueued delivery jobs. It does not guarantee every destination already published.
        </p>
        <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", margin: "0 0 18px" }}>
          To observe final outcome, poll <ApiInlineLink endpoint="GET /v1/social-posts/:post_id" />. If you need queue-level progress, use <ApiInlineLink endpoint="GET /v1/social-posts/:post_id/queue" />. For push delivery, subscribe to webhook events such as <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>post.published</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>post.partial</code>, and <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>post.failed</code>.
        </p>
        <RelatedEndpoints
          items={[
            { method: "POST", path: "/v1/social-posts/validate", label: "Validate post", href: "/docs/api/posts/validate" },
            { method: "GET", path: "/v1/social-posts", label: "List posts", href: "/docs/api/posts/list" },
            { method: "GET", path: "/v1/social-posts/:post_id", label: "Get post", href: "/docs/api/posts/get" },
            { method: "POST", path: "/v1/social-posts/bulk", label: "Bulk publish", href: "/docs/api/posts/bulk" },
            { method: "GET", path: "/v1/social-posts/:post_id/analytics", label: "Post analytics", href: "/docs/api/analytics/posts" },
            { method: "GET", path: "/v1/webhooks", label: "List webhooks", href: "/docs/api/webhooks" },
            { method: "POST", path: "/v1/media", label: "Reserve media upload", href: "/docs/api/media/reserve" },
            { method: "GET", path: "/v1/media/:media_id", label: "Get media", href: "/docs/api/media/get" },
            { method: "GET", path: "/v1/social-accounts", label: "List accounts", href: "/docs/api/accounts/list" },
          ]}
        />
      </div>
    </SingleEndpointReferencePage>
  );
}
