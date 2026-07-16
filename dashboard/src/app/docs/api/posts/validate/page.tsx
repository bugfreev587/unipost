"use client";

import Link from "next/link";
import { ApiInlineLink, type ApiFieldItem } from "../../_components/doc-components";
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
    description: <>Media library IDs to validate for that destination. Poll <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> until uploaded before publishing.</>,
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
    description: "Flat destination options for this platform post, such as { \"mediaType\": \"story\" } for Instagram. Do not nest these by platform name inside platform_posts; { \"instagram\": { \"mediaType\": \"story\" } } is legacy-only syntax.",
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
    name: "errors[].hint?",
    type: "string",
    description: "Specific remediation guidance when UniPost can safely suggest a fix.",
  },
  {
    name: "errors[].next_action?",
    type: "string",
    description: "Stable action enum for blocking validation errors, such as shorten_caption or fix_request.",
  },
  {
    name: "errors[].actual?",
    type: "any",
    description: "Actual submitted value or count, such as a caption length of 91.",
  },
  {
    name: "errors[].limit?",
    type: "any",
    description: "Platform limit that was exceeded or missed, such as 90 for TikTok photo titles.",
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
    name: "error.hint?",
    type: "string",
    description: "Remediation guidance for request-shape errors.",
  },
  {
    name: "error.next_action?",
    type: "string",
    description: 'Stable action enum. Request-shape validation errors use "fix_request".',
  },
  {
    name: "error.is_retriable?",
    type: "boolean",
    description: "Whether the same request should be retried without changing the payload. Request-shape errors are false.",
  },
  {
    name: "error.docs_url?",
    type: "string",
    description: "API reference URL for correcting the request.",
  },
  {
    name: "error.issues?",
    type: "array",
    description: "Structured validation issues when create/publish preflight rejects the payload.",
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
    label: "200 (media pending)",
    code: `{
  "data": {
    "valid": false,
    "errors": [
      {
        "platform_post_index": 0,
        "account_id": "sa_instagram_123",
        "platform": "instagram",
        "field": "media_ids",
        "code": "media_not_uploaded",
        "message": "media_id media_123 is in status pending; PUT bytes to the upload_url returned by POST /v1/media, then poll GET /v1/media/media_123 until status is uploaded before publishing",
        "actual": {
          "media_id": "media_123",
          "media_status": "pending",
          "next_step": "PUT bytes to upload_url, then poll GET /v1/media/{media_id} until status=uploaded",
          "docs_url": "https://unipost.dev/docs/api/media/reserve"
        },
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
    "message": "platform_posts[0].platform_options.instagram uses legacy platform-scoped options inside the new platform_posts shape. In platform_posts, platform_options must be flat, for example {\"mediaType\":\"story\"}; use top-level account_ids with platform_options.instagram only for the legacy shape.",
    "hint": "Use either the legacy account_ids shape or the new platform_posts shape exactly as documented, then retry.",
    "next_action": "fix_request",
    "is_retriable": false,
    "docs_url": "https://unipost.dev/docs/api/posts/validate"
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
    >
      <section className="api-field-section">
        <h2 className="api-field-section-title">Status Code Boundary</h2>
        <p>
          <code>POST /v1/posts/validate</code> separates request-shape errors from publishability checks. A request that
          follows the API contract returns <code>200</code>; read <code>data.valid</code>, <code>data.errors</code>, and{" "}
          <code>data.warnings</code> to decide whether the content can be published.
        </p>
        <p>
          UniPost returns <code>422</code> when the JSON request is outside the API contract, including mixed legacy and
          new publishing shapes. For example, <code>platform_posts[].platform_options.instagram</code> is invalid because
          the recommended <code>platform_posts[]</code> shape expects flat destination options.
        </p>
      </section>
      <section className="api-field-section">
        <h2 className="api-field-section-title">X Credits Preflight</h2>
        <p>
          Validation does not consume X Credits. Use this endpoint to catch request and platform constraints before
          calling <Link href="/docs/api/posts/create">POST /v1/posts</Link>, then inspect the live allowance with{" "}
          <Link href="/docs/api/x-credits">GET /v1/billing/x-credits</Link>. Managed X publishes consume the allowance;
          bring-your-own X API connections do not.
        </p>
      </section>
      <section className="api-field-section">
        <h2 className="api-field-section-title">Platform Options Shape</h2>
        <p>
          Use one request shape at a time. The legacy <code>account_ids</code> shape accepts top-level platform-scoped options such as <code>platform_options.instagram.mediaType</code>. The recommended <code>platform_posts[]</code> shape is already scoped to one destination, so each <code>platform_posts[].platform_options</code> object must be flat, such as <code>{'{"mediaType":"story"}'}</code>.
        </p>
        <p>
          For Instagram Stories in the recommended shape, send <code>{'{"mediaType":"story"}'}</code> directly under{" "}
          <code>platform_posts[].platform_options</code>. Do not send <code>{'{"instagram":{"mediaType":"story"}}'}</code>{" "}
          there; that nested object is only valid in the legacy top-level <code>platform_options</code> shape. See{" "}
          <Link href="/docs/guides/instagram-stories">Publish Instagram Stories</Link> for examples.
        </p>
      </section>
    </SingleEndpointReferencePage>
  );
}
