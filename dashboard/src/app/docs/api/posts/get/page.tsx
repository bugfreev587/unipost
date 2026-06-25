"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { CodeTabs, EnumValues } from "../../_components/doc-components";
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
  { name: "status", type: "string", description: <>Derived lifecycle status. For multi-platform posts, this is the aggregate parent status.<EnumValues values={["draft", "scheduled", "publishing", "published", "partial", "failed", "cancelled"]} /></> },
  { name: "source", type: "string", description: <>Origin of the post.<EnumValues values={["api", "ui"]} /></> },
  { name: "profile_ids", type: "string[]", description: "Distinct profile IDs targeted by the post." },
  { name: "results", type: "array", description: "Per-account publish results." },
  { name: "results[].id", type: "string", description: "Result row ID used for retries and diagnostics." },
  { name: "results[].social_account_id", type: "string", description: "Destination account ID." },
  { name: "results[].platform", type: "string", description: <>Destination platform.<EnumValues values={["twitter", "linkedin", "instagram", "facebook", "threads", "youtube", "tiktok", "bluesky", "pinterest"]} /></> },
  { name: "results[].account_name", type: "string", description: "Resolved handle or account display name when available." },
  { name: "results[].status", type: "string", description: <>Per-account publish status.<EnumValues values={["queued", "publishing", "processing", "published", "failed"]} /></> },
  { name: "results[].url", type: "string | null", description: "Canonical public post URL when available." },
  { name: "results[].error_code", type: "string | null", description: "Normalized publish failure code for failed rows, such as platform_request_invalid, media_error, or account_reconnect_required." },
  { name: "results[].failure_stage", type: "string | null", description: "Stage where the failure occurred, such as validation, publish, status_check, or worker_status_check." },
  { name: "results[].platform_error_code", type: "string | null", description: "Provider-specific error code when UniPost can safely extract one, such as TikTok invalid_params." },
  { name: "results[].is_retriable", type: "boolean | null", description: "Whether retrying this result is expected to help." },
  { name: "results[].next_action", type: "string | null", description: "Stable action enum for UI and automation, such as review_platform_options, reconnect_account, or wait_and_retry." },
  { name: "results[].error_source", type: "string | null", description: <>Where the failure originated.<EnumValues values={["unipost", "platform", "worker", "unknown"]} /></> },
  { name: "results[].error_temporality", type: "string | null", description: <>Whether the condition is temporary, permanent, or unknown.<EnumValues values={["temporary", "permanent", "unknown"]} /></> },
  { name: "results[].provider_error", type: "object | null", description: "Sanitized provider detail object. May include provider, http_status, code, subcode, type, reason, domain, quota_limit, quota_location, and is_transient." },
  { name: "results[].retry_policy", type: "object | null", description: "Best-effort queue snapshot with is_retriable, will_retry, retry_state, next_run_at, attempts_made, max_attempts, attempts_remaining, manual_retry_allowed, and reason." },
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
    code: `curl "https://api.unipost.dev/v1/posts/post_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const post = await client.posts.get("post_abc123");
console.log(post.data.status);
console.log(post.data.results);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

post = client.posts.get("post_abc123")
print(post["data"]["status"])
print(post["data"]["results"])`,
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

  post, err := client.Posts.Get(context.Background(), "post_abc123")
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(post.Status)
  fmt.Println(post.Results)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var post = client.posts().get("post_abc123");
System.out.println(post.get("status").asText());
System.out.println(post.get("results"));`,
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

const POLLING_EXAMPLE_SNIPPETS = [
  {
    lang: "json",
    label: "Single destination success",
    code: `{
  "data": {
    "id": "post_single_123",
    "status": "published",
    "results": [
      {
        "id": "spr_1",
        "social_account_id": "sa_twitter_789",
        "platform": "twitter",
        "account_name": "@unipost",
        "status": "published",
        "external_id": "191234567890",
        "url": "https://x.com/unipost/status/191234567890",
        "published_at": "2026-04-22T10:00:02Z"
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "Multi-destination partial",
    code: `{
  "data": {
    "id": "post_multi_123",
    "status": "partial",
    "results": [
      {
        "id": "spr_ok",
        "social_account_id": "sa_twitter_789",
        "platform": "twitter",
        "account_name": "@unipost",
        "status": "published",
        "external_id": "191234567890",
        "url": "https://x.com/unipost/status/191234567890",
        "published_at": "2026-04-22T10:00:02Z"
      },
      {
        "id": "spr_fail",
        "social_account_id": "sa_linkedin_456",
        "platform": "linkedin",
        "account_name": "UniPost",
        "status": "failed",
        "error_message": "LinkedIn rejected the caption because it exceeded the platform limit.",
        "error_code": "validation_error",
        "failure_stage": "publish",
        "platform_error_code": null,
        "is_retriable": false,
        "next_action": "fix_request",
        "error_source": "unipost",
        "error_temporality": "permanent",
        "provider_error": null,
        "retry_policy": {
          "is_retriable": false,
          "will_retry": false,
          "retry_state": "not_retriable",
          "manual_retry_allowed": true,
          "reason": "classification_not_retriable"
        }
      }
    ]
  }
}`,
  },
  {
    lang: "json",
    label: "All destinations failed",
    code: `{
  "data": {
    "id": "post_failed_123",
    "status": "failed",
    "results": [
      {
        "id": "spr_a",
        "social_account_id": "sa_instagram_123",
        "platform": "instagram",
        "status": "failed",
        "error_message": "Instagram rejected the media because the aspect ratio was unsupported.",
        "error_code": "media_error",
        "failure_stage": "publish",
        "platform_error_code": null,
        "is_retriable": false,
        "next_action": "fix_media",
        "error_source": "platform",
        "error_temporality": "permanent",
        "retry_policy": {
          "is_retriable": false,
          "will_retry": false,
          "retry_state": "not_retriable",
          "manual_retry_allowed": true,
          "reason": "classification_not_retriable"
        }
      },
      {
        "id": "spr_b",
        "social_account_id": "sa_threads_456",
        "platform": "threads",
        "status": "failed",
        "error_message": "Threads rejected the request because the token was expired.",
        "error_code": "account_reconnect_required",
        "failure_stage": "publish",
        "platform_error_code": "190",
        "is_retriable": false,
        "next_action": "reconnect_account",
        "error_source": "platform",
        "error_temporality": "permanent",
        "provider_error": {
          "provider": "meta",
          "code": "190"
        },
        "retry_policy": {
          "is_retriable": false,
          "will_retry": false,
          "retry_state": "not_retriable",
          "manual_retry_allowed": true,
          "reason": "classification_not_retriable"
        }
      }
    ]
  }
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
      path="/v1/posts/:post_id"
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
    >
      <div style={{ borderTop: "1px solid var(--docs-border)", paddingTop: 20 }}>
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 12, background: "var(--docs-bg-elevated)", overflow: "hidden", marginBottom: 18 }}>
          <div style={{ padding: "16px 18px", borderBottom: "1px solid var(--docs-border)", fontSize: 15, fontWeight: 700, color: "var(--docs-text)" }}>
            How to use this endpoint from a client
          </div>
          <div style={{ padding: "18px", display: "grid", gap: 12 }}>
            <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              <strong style={{ color: "var(--docs-text)" }}>Use the parent status for the big picture.</strong> This is the fastest way to know whether the post is done, partially successful, or fully failed.
            </div>
            <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              <strong style={{ color: "var(--docs-text)" }}>Use <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>results[]</code> for the truth per destination.</strong> Each result row is independent and tells you which account published, which one failed, and which public URL or error belongs to that account.
            </div>
            <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              <strong style={{ color: "var(--docs-text)" }}>Read retry state from <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>retry_policy</code>.</strong> <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>is_retriable</code> means a retry may help; <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>will_retry</code> means UniPost has an automatic attempt scheduled or running. Treat this object as a best-effort queue snapshot and poll again before showing destructive actions. If a pending job has a past <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>next_run_at</code>, it is due now rather than invalid.
            </div>
            <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
              <strong style={{ color: "var(--docs-text)" }}>Stop polling when the parent status is final.</strong> In most clients that means polling until the post becomes <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>published</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>partial</code>, or <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>failed</code>.
            </div>
          </div>
        </div>
        <div style={{ marginBottom: 18 }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 12 }}>Polling examples users can compare against</div>
          <CodeTabs snippets={POLLING_EXAMPLE_SNIPPETS} />
        </div>
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 12, background: "var(--docs-bg-elevated)", padding: "16px 18px" }}>
          <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 10 }}>Rule of thumb</div>
          <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)" }}>
            <strong style={{ color: "var(--docs-text)" }}>Single-platform posts:</strong> the parent post status usually matches the only result row.
          </div>
          <div style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", marginTop: 8 }}>
            <strong style={{ color: "var(--docs-text)" }}>Multi-platform posts:</strong> read the parent post status as the aggregate outcome, then read <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>results[]</code> to know what happened on each destination account.
          </div>
        </div>
      </div>
    </SingleEndpointReferencePage>
  );
}
