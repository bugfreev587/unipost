"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  {
    name: "Authorization",
    type: "Bearer <token>",
    meta: "In header",
    description: "Workspace API key.",
  },
];

const BODY_FIELDS: ApiFieldItem[] = [
  {
    name: "platform_posts",
    type: "array",
    description: "Recommended unified request shape for per-account content.",
  },
  {
    name: "scheduled_at?",
    type: "string",
    description: "Optional scheduled publish time in ISO-8601 format.",
  },
];

const PLATFORM_POST_FIELDS: ApiFieldItem[] = [
  {
    name: "platform_posts[].account_id",
    type: "string",
    description: "Connected social account to validate against.",
  },
  {
    name: "platform_posts[].caption?",
    type: "string",
    description: "Caption or post text to validate.",
  },
  {
    name: "platform_posts[].media_urls?",
    type: "string[]",
    description: "Public asset URLs to validate for that destination.",
  },
  {
    name: "platform_posts[].media_ids?",
    type: "string[]",
    description: "Media library IDs to validate for that destination.",
  },
  {
    name: "platform_posts[].thread_position?",
    type: "integer",
    description: "1-indexed thread slot for X and Bluesky thread validation.",
  },
  {
    name: "platform_posts[].first_comment?",
    type: "string",
    description: "Optional first reply/comment validation for supported platforms.",
  },
  {
    name: "platform_posts[].platform_options?",
    type: "object",
    description: "Platform-specific options such as Instagram media type or Pinterest board selection.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "valid",
    type: "boolean",
    description: "Whether the payload can be published without fatal validation errors.",
  },
  {
    name: "errors",
    type: "array",
    description: "Blocking validation issues.",
  },
  {
    name: "errors[].platform_post_index",
    type: "integer",
    description: "0-based input index that produced the issue.",
  },
  {
    name: "errors[].account_id",
    type: "string",
    description: "Account ID associated with the issue when known.",
  },
  {
    name: "errors[].platform",
    type: "string",
    description: "Resolved platform associated with the issue when known.",
  },
  {
    name: "errors[].field",
    type: "string",
    description: "Request field that failed validation.",
  },
  {
    name: "errors[].code",
    type: "string",
    description: "Machine-readable validation code.",
  },
  {
    name: "errors[].message",
    type: "string",
    description: "Human-readable validation message.",
  },
  {
    name: "errors[].severity",
    type: "string",
    description: 'Issue severity. Blocking items are returned as "error".',
  },
  {
    name: "warnings",
    type: "array",
    description: "Non-blocking validation issues.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description: 'Usually "UNAUTHORIZED", "VALIDATION_ERROR", or "INTERNAL_ERROR".',
  },
  {
    name: "error.normalized_code",
    type: "string",
    description: 'Lowercase alias such as "unauthorized", "validation_error", or "internal_error".',
  },
  {
    name: "error.message",
    type: "string",
    description: "Human-readable error message.",
  },
  {
    name: "request_id",
    type: "string",
    description: "Request identifier for debugging and support.",
  },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/posts/validate" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "platform_posts": [
      {
        "account_id": "sa_twitter_1",
        "caption": "Launch update for X"
      }
    ]
  }'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const result = await client.posts.validate({
  platformPosts: [
    {
      accountId: "sa_twitter_1",
      caption: "Launch update for X",
    },
  ],
});`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

result = client.posts.validate(
  platform_posts=[
    {
      "account_id": "sa_twitter_1",
      "caption": "Launch update for X",
    }
  ]
)`,
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

  validation, err := client.Posts.Validate(context.Background(), &unipost.ValidatePostParams{
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_1",
        Caption:   "Launch update for X",
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = validation
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import java.util.List;
import java.util.Map;

UniPost client = new UniPost();

var result = client.posts().validate(Map.of(
    "platform_posts", List.of(
        Map.of(
            "account_id", "sa_twitter_1",
            "caption", "Launch update for X"
        )
    )
));

System.out.println(result.get("valid").asBoolean());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "valid": false,
    "errors": [
      {
        "platform_post_index": 0,
        "account_id": "sa_twitter_1",
        "platform": "twitter",
        "field": "caption",
        "code": "exceeds_max_length",
        "message": "Caption exceeds maximum length for twitter (280 characters)",
        "severity": "error"
      }
    ],
    "warnings": []
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "200 (plan-gated)",
    code: `{
  "data": {
    "valid": false,
    "errors": [
      {
        "platform_post_index": 0,
        "account_id": "sa_twitter_1",
        "platform": "twitter",
        "field": "account_id",
        "code": "plan_platform_not_allowed",
        "message": "publishing to twitter is not available on your current plan — upgrade at unipost.dev/pricing",
        "severity": "error"
      }
    ],
    "warnings": []
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
    "message": "either platform_posts or account_ids is required"
  },
  "request_id": "req_123"
}`,
  },
];

export default function ValidatePage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Validate post"
      description="Runs preflight checks against the same payload shape as publish. Use it before automation or AI-driven posting so content problems are caught before quota is consumed."
      method="POST"
      path="/v1/posts/validate"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
        { title: "platform_posts[]", items: PLATFORM_POST_FIELDS },
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
