"use client";

import { EnumValues, type ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];
const PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "Draft post ID to publish." },
];
const RESPONSE_202_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "Draft post ID." },
  { name: "execution_mode", type: "string", description: <>Draft publish enqueues delivery jobs and returns before workers finish dispatch.<EnumValues values={["async"]} /></> },
  { name: "status", type: "string", description: <>Initial post state after draft publish begins, then later converges to a final aggregate state.<EnumValues values={["queued", "publishing", "published", "partial", "failed"]} /></> },
  { name: "queued_results_count", type: "integer", description: "How many per-account delivery results were queued." },
  { name: "active_job_count", type: "integer", description: "How many queue jobs are currently active." },
  { name: "results", type: "array", description: "Initial per-platform result rows created at enqueue time." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
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
    code: `curl -X POST "https://api.unipost.dev/v1/posts/post_abc123/publish" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

await client.posts.publish("post_abc123");`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

result = client.posts.publish("post_abc123")
print(result["data"]["status"])`,
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

  post, err := client.Posts.Publish(context.Background(), "post_abc123")
  if err != nil {
    log.Fatal(err)
  }

  fmt.Println(post.Status)
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var result = client.posts().publish("post_abc123");
System.out.println(result.get("status").asText());`,
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
    "status": "queued",
    "queued_results_count": 1,
    "active_job_count": 1,
    "results": [
      {
        "platform": "twitter",
        "status": "queued"
      }
    ]
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
    "message": "Post is not a draft (already publishing, published, or not found in this workspace)"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "402",
    code: `{
  "error": {
    "code": "PLAN_POST_QUOTA_EXCEEDED",
    "normalized_code": "plan_post_quota_exceeded",
    "message": "Free plan monthly post quota exceeded. You have used 100 of 100 posts this month, and this request needs 1 more. Upgrade to continue posting."
  },
  "request_id": "req_123"
}`,
  },
];

export default function PublishDraftPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Publish draft"
      description="Accepts an existing draft for publication. UniPost flips the draft into a publishable state, creates result rows, and enqueues background delivery jobs."
      method="POST"
      path="/v1/posts/:post_id/publish"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "202", fields: RESPONSE_202_FIELDS },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "500", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    >
      <div style={{ borderTop: "1px solid var(--docs-border)", paddingTop: 20 }}>
        <p style={{ fontSize: 14.5, lineHeight: 1.7, color: "var(--docs-text-soft)", margin: 0 }}>
          Draft publish is asynchronous. UniPost now returns <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>202 Accepted</code> once the draft is claimed, result rows are created, and delivery jobs are queued. Read final status from the post resource or via publish webhooks.
        </p>
      </div>
    </SingleEndpointReferencePage>
  );
}
