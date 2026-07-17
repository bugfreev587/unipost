"use client";

import Link from "next/link";
import {
  ApiInlineLink,
  EnumValues,
  type ApiFieldItem,
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
    description: <>Public URLs for hosted media. Do not reserve these with <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />; that endpoint is only for raw file uploads that have a known byte size. Ignored when platform_posts overrides are used.</>,
  },
  {
    name: "media_ids?",
    type: "string[]",
    description: <>Media library IDs returned by <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />. Use these for local files and larger videos after <ApiInlineLink endpoint="GET /v1/media/:media_id" href="/docs/api/media/get" /> reports uploaded.</>,
  },
  {
    name: "scheduled_at?",
    type: "string",
    description: "ISO-8601 timestamp. If present, UniPost stores the post in scheduled state and the scheduler enqueues delivery later. Free workspaces can hold up to 50 undeleted parent posts in scheduled status at once; paid plans do not cap active scheduled backlog.",
  },
  {
    name: "idempotency_key?",
    type: "string",
    description: "Optional body-level key for scheduled posts. While a matching post is still scheduled, UniPost returns the existing scheduled post for the same payload and returns 409 for a different payload.",
  },
  {
    name: "status?",
    type: '"draft"',
    description: <>
      Save without dispatching any platform jobs. Omit the field entirely for immediate publish. Drafts are published later via <ApiInlineLink endpoint="POST /v1/posts/:post_id/publish" />.
      <EnumValues values={["draft"]} />
    </>,
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
    description: <>Account-specific hosted asset URLs. Use this field directly when the asset already has a public URL; <code>size_bytes</code> is only needed for <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" /> local-file uploads.</>,
  },
  {
    name: "platform_posts[].media_ids?",
    type: "string[]",
    description: <>Account-specific media library asset IDs. Pending uploads fail pre-publish validation with media_not_uploaded.</>,
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
    description: <>
      Flat destination options for this platform post. Do not nest by platform name inside <code>platform_posts</code>;
      platform-scoped nesting belongs only to the legacy <code>account_ids</code> shape. See{" "}
      <Link href="/docs/guides/platform-options">common platform options examples</Link> for YouTube, Instagram, TikTok, Facebook, and Pinterest.
    </>,
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
    description: <>
      Immediate publish enqueues delivery jobs and returns before platform dispatch finishes.
      <EnumValues values={["async"]} />
    </>,
  },
  {
    name: "caption",
    type: "string | null",
    description: "Top-level shared caption when one exists.",
  },
  {
    name: "status",
    type: "string",
    description: <>
      Initial lifecycle state for the accepted post. Immediate creates usually start as queued or publishing, then converge to a final aggregate state.
      <EnumValues values={["queued", "publishing", "published", "partial", "failed", "draft", "scheduled", "cancelled"]} />
    </>,
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
    description: <>
      Normalized platform name.
      <EnumValues values={["twitter", "linkedin", "instagram", "facebook", "threads", "youtube", "tiktok", "bluesky", "pinterest"]} />
    </>,
  },
  {
    name: "results[].status",
    type: "string",
    description: <>
      Initial per-account state. Rows move through in-flight values first, then settle on final result values.
      <EnumValues values={["queued", "publishing", "processing", "published", "failed"]} />
    </>,
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
    description: <>
      Created resource state for non-immediate creates.
      <EnumValues values={["scheduled", "draft"]} />
    </>,
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
  { name: "error.code", type: "string", description: 'Machine-readable error code. Paid scheduling capacity uses "PLAN_MONTHLY_SCHEDULING_CAPACITY_EXCEEDED" with HTTP 402.' },
  { name: "error.normalized_code", type: "string", description: "Lowercase compatibility alias for the error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "error.issues?", type: "array", description: "Structured pre-publish validation issues such as media_not_uploaded." },
  { name: "error.details?", type: "object", description: "For paid scheduling capacity errors: plan, period, completed/scheduled/held/effective usage, limit, projected usage, requested units, reset time, and scheduling_allowed=false." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "scheduled_at": "2026-04-22T10:00:00Z",
    "idempotency_key": "launch-day-2026-04-22",
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

const client = new UniPost();

const post = await client.posts.create({
  scheduledAt: "2026-04-22T10:00:00Z",
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
console.log(post.status); // scheduled`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

post = client.posts.create(
  scheduled_at="2026-04-22T10:00:00Z",
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
print(post["data"]["status"])  # scheduled`,
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

  post, err := client.Posts.Create(context.Background(), &unipost.CreatePostParams{
    ScheduledAt:    "2026-04-22T10:00:00Z",
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

  _, _ = post.ID, post.Status
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import java.util.List;
import java.util.Map;

UniPost client = new UniPost();

var post = client.posts().create(Map.of(
    "scheduled_at", "2026-04-22T10:00:00Z",
    "idempotency_key", "launch-day-2026-04-22",
    "platform_posts", List.of(
        Map.of(
            "account_id", "sa_twitter_789",
            "caption", "v1.4 is live"
        ),
        Map.of(
            "account_id", "sa_linkedin_456",
            "caption", "We shipped v1.4 with webhooks and bulk publishing."
        )
    )
));

System.out.println(post.get("id").asText());
System.out.println(post.get("status").asText()); // scheduled`,
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
    label: "400",
    code: `{
  "error": {
    "code": "VALIDATION_ERROR",
    "normalized_code": "validation_error",
    "message": "request failed pre-publish validation",
    "issues": [
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
    ]
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
    "code": "IDEMPOTENCY_KEY_CONFLICT",
    "normalized_code": "idempotency_key_conflict",
    "message": "A scheduled post with the same idempotency_key already exists for this workspace."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "402 quota",
    code: `{
  "error": {
    "code": "PLAN_POST_QUOTA_EXCEEDED",
    "normalized_code": "plan_post_quota_exceeded",
    "message": "Free plan monthly post quota exceeded. You have used 100 of 100 posts this month, and this request needs 1 more. Upgrade to continue posting."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "402 scheduled cap",
    code: `{
  "error": {
    "code": "PLAN_SCHEDULED_POST_LIMIT_EXCEEDED",
    "normalized_code": "plan_scheduled_post_limit_exceeded",
    "message": "Free plan active scheduled post limit exceeded. You already have 50 active scheduled posts; Free allows up to 50. Upgrade to schedule more posts."
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
      description={<>Creates a UniPost post resource using the same payload shape as <ApiInlineLink endpoint="POST /v1/posts/validate" />. Immediate publish requests are accepted and queued asynchronously; background workers perform the actual platform delivery. Use scheduling or draft mode when you want creation without immediate dispatch.</>}
      method="POST"
      path="/v1/posts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
        { title: "platform_posts[]", items: PLATFORM_POST_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_202_FIELDS },
        { code: "201", fields: RESPONSE_201_FIELDS },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <div style={{ paddingTop: 8 }}>
        <section style={{ display: "grid", gap: 14, marginBottom: 24 }}>
          <h2 style={{ color: "var(--docs-text)", fontSize: 21, lineHeight: 1.25, letterSpacing: "-.02em", margin: 0 }}>
            Media Inputs
          </h2>
          <div style={{ display: "grid", gap: 12, maxWidth: 880 }}>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Hosted URL:</strong> if your image or video is already publicly reachable, send it in <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>platform_posts[].media_urls</code>. Do not call <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" /> for this path.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Local file bytes:</strong> reserve an upload with <ApiInlineLink endpoint="POST /v1/media" href="/docs/api/media/reserve" />, PUT the bytes to the returned upload URL, then publish with <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>platform_posts[].media_ids</code>. <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>size_bytes</code> is optional; UniPost hydrates the actual byte length after upload.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>GIF conversion:</strong> X and Facebook can publish the original GIF. For a video-only destination, call <ApiInlineLink endpoint="POST /v1/media/gif-conversions" href="/docs/api/media/gif-conversions" />, wait for success, and send the returned <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>output_media_id</code> here as normal video media. Conversion does not create the post automatically.
            </p>
          </div>
        </section>

        <section style={{ display: "grid", gap: 14, marginBottom: 24 }}>
          <h2 style={{ color: "var(--docs-text)", fontSize: 21, lineHeight: 1.25, letterSpacing: "-.02em", margin: 0 }}>
            Scheduling Limits
          </h2>
          <div style={{ display: "grid", gap: 12, maxWidth: 880 }}>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Free active backlog:</strong> Free workspaces can keep up to 50 undeleted parent posts in scheduled status. Exceeding that cap returns <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>PLAN_SCHEDULED_POST_LIMIT_EXCEEDED</code>. The existing 100 posts/month quota still applies when posts are created and delivered. Paid plans do not cap active scheduled backlog.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Media retention:</strong> scheduled posts keep uploaded media until they reach a final status. After success, failed, partial, or cancellation, UniPost retains media according to the workspace plan before cleanup.
            </p>
          </div>
        </section>

        <section style={{ display: "grid", gap: 14, marginBottom: 24 }}>
          <h2 style={{ color: "var(--docs-text)", fontSize: 21, lineHeight: 1.25, letterSpacing: "-.02em", margin: 0 }}>
            X Credits
          </h2>
          <div style={{ display: "grid", gap: 12, maxWidth: 880 }}>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              Managed X publishes consume the workspace&apos;s included X Credits allowance. A conclusively URL-free
              X post uses 15 Credits; a URL or domain-like candidate is conservatively counted at 200 Credits.
              Bring-your-own X API connections do not consume UniPost X Credits. Inspect remaining capacity with{" "}
              <ApiInlineLink endpoint="GET /v1/billing/x-credits" href="/docs/api/x-credits" />.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              Final platform results include <code>x_credits_counted</code>, <code>x_credit_operation</code>,{" "}
              <code>x_credit_catalog_version</code>, and <code>x_credit_billing_mode</code>. BYO X results return zero
              counted Credits with billing mode <code>customer_x_app</code>.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              At the hard limit, managed-X delivery fails with <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>x_monthly_usage_limit_exceeded</code>.
              The independent safety cap of 20 X posts per connected account per UTC day still applies.
            </p>
          </div>
        </section>

        <section style={{ display: "grid", gap: 14, marginBottom: 24 }}>
          <h2 style={{ color: "var(--docs-text)", fontSize: 21, lineHeight: 1.25, letterSpacing: "-.02em", margin: 0 }}>
            Publishing Result
          </h2>
          <div style={{ display: "grid", gap: 12, maxWidth: 880 }}>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Poll Model:</strong> immediate publish returns <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>202</code> after UniPost accepts and queues delivery. Poll <ApiInlineLink endpoint="GET /v1/posts/:post_id" /> to read the final publishing result.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Failure Model:</strong> failed result rows include <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>error_source</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>error_temporality</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>provider_error</code>, and <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>retry_policy</code>. Use <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>retry_policy.will_retry</code> for automatic retries and avoid parsing <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>error_message</code>.
            </p>
            <p style={{ color: "var(--docs-text-soft)", fontSize: 14.5, lineHeight: 1.68, margin: 0 }}>
              <strong style={{ color: "var(--docs-text)" }}>Push Model:</strong> create a webhook subscription with <ApiInlineLink endpoint="POST /v1/webhooks" /> to receive final publishing events from UniPost.
            </p>
          </div>
        </section>

      </div>
    </SingleEndpointReferencePage>
  );
}
