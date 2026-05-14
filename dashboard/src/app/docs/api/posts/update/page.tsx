"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "UniPost post ID." },
];

const BODY_FIELDS: ApiFieldItem[] = [
  { name: "platform_posts?", type: "array", description: "Draft-only content update. Replaces the stored per-account payload." },
  { name: "scheduled_at?", type: "string", description: "Scheduled-only update. Reschedules a scheduled post when set to a future RFC3339 timestamp." },
  { name: "archived?", type: "boolean", description: "Canonical lifecycle patch. Set `true` to archive or `false` to restore." },
  { name: "status?", type: '"canceled"', description: "Canonical lifecycle patch for draft or scheduled posts. Cancels the post without dispatching more work." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "id", type: "string", description: "UniPost post ID." },
  { name: "status", type: "string", description: "Updated lifecycle state." },
  { name: "archived_at", type: "string | null", description: "Archive timestamp when the post is archived." },
  { name: "scheduled_at", type: "string | null", description: "Updated scheduled publish time for scheduled posts." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'Usually "VALIDATION_ERROR", "CONFLICT", or "NOT_FOUND".' },
  { name: "error.normalized_code", type: "string", description: 'Lowercase alias such as "validation_error", "conflict", or "not_found".' },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `# Archive a post
curl -X PATCH "https://api.unipost.dev/v1/posts/post_abc123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"archived": true}'

# Cancel a scheduled post
curl -X PATCH "https://api.unipost.dev/v1/posts/post_sched_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"status": "canceled"}'

# Reschedule
curl -X PATCH "https://api.unipost.dev/v1/posts/post_sched_123" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"scheduled_at": "2026-04-24T18:00:00Z"}'`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

// Archive
await client.posts.update("post_abc123", { archived: true });

// Cancel a scheduled post
await client.posts.update("post_sched_123", { status: "canceled" });

// Reschedule
await client.posts.update("post_sched_123", {
  scheduledAt: "2026-04-24T18:00:00Z",
});`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

# Archive
client.posts.update("post_abc123", archived=True)

# Cancel a scheduled post
client.posts.update("post_sched_123", status="canceled")

# Reschedule
client.posts.update(
  "post_sched_123",
  scheduled_at="2026-04-24T18:00:00Z",
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

  ctx := context.Background()

  // Archive
  archived := true
  if _, err := client.Posts.Update(ctx, "post_abc123", &unipost.UpdatePostParams{
    Archived: &archived,
  }); err != nil {
    log.Fatal(err)
  }

  // Cancel a scheduled post
  if _, err := client.Posts.Cancel(ctx, "post_sched_123"); err != nil {
    log.Fatal(err)
  }

  // Reschedule
  if _, err := client.Posts.Update(ctx, "post_sched_123", &unipost.UpdatePostParams{
    ScheduledAt: unipost.String("2026-04-24T18:00:00Z"),
  }); err != nil {
    log.Fatal(err)
  }
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

import java.util.Map;

UniPost client = new UniPost();

// Archive
client.posts().update("post_abc123", Map.of("archived", true));

// Cancel a scheduled post
client.posts().update("post_sched_123", Map.of("status", "canceled"));

// Reschedule
client.posts().update("post_sched_123", Map.of(
    "scheduled_at", "2026-04-24T18:00:00Z"
));`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "id": "post_abc123",
    "status": "draft",
    "archived_at": "2026-04-23T18:00:00Z"
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
    "message": "Post cannot be cancelled (not a draft or scheduled post in this workspace)"
  },
  "request_id": "req_123"
}`,
  },
];

export default function UpdatePostPage() {
  return (
    <SingleEndpointReferencePage
      section="publishing"
      title="Update post"
      description="Canonical update endpoint for draft edits, scheduled-post rescheduling, and lifecycle transitions such as archive, restore, and cancel."
      method="PATCH"
      path="/v1/posts/:post_id"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Request Body", items: BODY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
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
          Use PATCH for direct resource state changes. Legacy command routes such as <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>POST /v1/posts/:post_id/archive</code>, <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>/restore</code>, and <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>/cancel</code> still work during the migration window, but PATCH is now the canonical route.
        </p>
      </div>
    </SingleEndpointReferencePage>
  );
}
