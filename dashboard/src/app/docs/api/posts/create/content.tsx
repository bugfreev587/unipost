"use client";

import {
  ApiInlineLink,
  Breadcrumbs, EndpointHeader, DocSection, ParamTable, CodeTabs, ResponseBlock,
  ErrorTable, RelatedEndpoints, InfoBox, ChangelogEntry,
  type ParamRow, type ErrorCodeRow,
} from "../../_components/doc-components";

const BODY_PARAMS: ParamRow[] = [
  { name: "caption", type: "string", required: false, description: "Text content. Max length varies by platform. Required unless platform_posts is set." },
  { name: "account_ids", type: "string[]", required: false, description: "Social account IDs to fan out to. Use this OR platform_posts, not both." },
  { name: "platform_posts", type: "object[]", required: false, description: "Per-platform posts with individual captions, media, and options. Preferred for multi-platform fan-out." },
  { name: "media_urls", type: "string[]", required: false, description: "Public URLs of images/videos to attach. Ignored when platform_posts is set." },
  { name: "media_ids", type: "string[]", required: false, description: <>IDs returned by <ApiInlineLink endpoint="POST /v1/media" /> after you upload a local file to UniPost storage. Preferred over <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>media_urls</code> for local files and large videos. Resolved server-side to presigned download URLs.</> },
  { name: "scheduled_at", type: "string", required: false, description: "ISO 8601 timestamp. If set, post is queued and published by the scheduler at that time. Must be at least 60 seconds in the future." },
  { name: "idempotency_key", type: "string", required: false, description: "Unique string (max 64 chars). Same key + same workspace within 24h returns the original response unchanged." },
  { name: "status", type: "string", required: false, description: <>Set to <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>&quot;draft&quot;</code> to persist without publishing. Use <ApiInlineLink endpoint="POST /v1/social-posts/:id/publish" /> to ship later.</> },
];

const PLATFORM_POST_PARAMS: ParamRow[] = [
  { name: "account_id", type: "string", required: true, description: "Target social account ID." },
  { name: "caption", type: "string", required: false, description: "Platform-specific caption. Overrides the top-level caption." },
  { name: "media_urls", type: "string[]", required: false, description: "Platform-specific media URLs." },
  { name: "media_ids", type: "string[]", required: false, description: "Platform-specific media IDs from the media library. Create these with POST /v1/media, then upload the bytes to the returned upload_url before publish." },
  { name: "thread_position", type: "integer", required: false, description: "1-indexed position in a multi-post thread. All entries with the same account_id and non-zero thread_position form one thread. Twitter + Bluesky supported." },
  { name: "first_comment", type: "string", required: false, description: "Text posted as the first reply/comment after the main post lands. Supported on Twitter, LinkedIn, Instagram. Bluesky/Threads reject this — use thread_position instead." },
  { name: "platform_options", type: "object", required: false, description: "Platform-specific key-value options. Example: `platform_options.youtube` supports required `title` and `made_for_kids`, while `platform_options.instagram.mediaType` supports `feed`, `reels`, or `story` and participates in preflight validation." },
];

const HEADER_PARAMS: ParamRow[] = [
  { name: "Authorization", type: "string", required: true, description: "Bearer {api_key}" },
  { name: "Content-Type", type: "string", required: true, description: "application/json" },
  { name: "Idempotency-Key", type: "string", required: false, description: "Alternative to the body field. Max 64 characters, 24h TTL." },
];

const ERRORS: ErrorCodeRow[] = [
  { code: "VALIDATION_ERROR", http: 422, description: "Request body failed validation (caption too long, missing required fields, invalid media, etc.). Check error.issues[] for per-field details." },
  { code: "VALIDATION_ERROR", http: 400, description: "Structural validation error (malformed JSON, invalid field types)." },
  { code: "UNAUTHORIZED", http: 401, description: "Missing or invalid API key." },
  { code: "NOT_FOUND", http: 404, description: "Account ID not found in this workspace." },
  { code: "CONFLICT", http: 409, description: "Idempotency key already used with different request body." },
  { code: "INTERNAL_ERROR", http: 500, description: "Server error. Retry with the same idempotency key." },
];

const SNIPPETS_BASIC = [
  { lang: "js", label: "Node.js", code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const post = await client.posts.create({
  caption: "Hello from UniPost! 🚀",
  accountIds: ["sa_instagram_123", "sa_linkedin_456"],
});

console.log(post.id);
console.log(post.status);` },
  { lang: "python", label: "Python", code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

post = client.posts.create(
  caption="Hello from UniPost! 🚀",
  account_ids=["sa_instagram_123", "sa_linkedin_456"],
)

print(post.id)
print(post.status)` },
  { lang: "go", label: "Go", code: `package main

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
    Caption:    "Hello from UniPost! 🚀",
    AccountIDs: []string{"sa_instagram_123", "sa_linkedin_456"},
  })
  if err != nil {
    log.Fatal(err)
  }

  _, _ = post.ID, post.Status
}` },
];

const SNIPPETS_ADVANCED = [
  { lang: "js", label: "Node.js", code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const post = await client.posts.create({
  scheduledAt: "2026-05-01T09:00:00Z",
  idempotencyKey: "release-v1.4.0-2026-04-08",
  platformPosts: [
    {
      accountId: "sa_twitter_789",
      caption: "v1.4 is live 🎉 webhooks + bulk publish",
    },
    {
      accountId: "sa_linkedin_456",
      caption: "We just shipped v1.4 with two features our customers have been asking for.",
    },
    {
      accountId: "sa_bluesky_012",
      caption: "v1.4 shipped — webhooks and bulk publish are live",
    },
  ],
});` },
  { lang: "python", label: "Python", code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

post = client.posts.create(
  scheduled_at="2026-05-01T09:00:00Z",
  idempotency_key="release-v1.4.0-2026-04-08",
  platform_posts=[
    {
      "account_id": "sa_twitter_789",
      "caption": "v1.4 is live 🎉 webhooks + bulk publish",
    },
    {
      "account_id": "sa_linkedin_456",
      "caption": "We just shipped v1.4 with two features our customers have been asking for.",
    },
  ],
)` },
  { lang: "go", label: "Go", code: `package main

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

  _, err := client.Posts.Create(context.Background(), &unipost.CreatePostParams{
    ScheduledAt:    "2026-05-01T09:00:00Z",
    IdempotencyKey: "release-v1.4.0-2026-04-08",
    PlatformPosts: []unipost.PlatformPost{
      {
        AccountID: "sa_twitter_789",
        Caption:   "v1.4 is live 🎉 webhooks + bulk publish",
      },
      {
        AccountID: "sa_linkedin_456",
        Caption:   "We just shipped v1.4 with two features our customers have been asking for.",
      },
    },
  })
  if err != nil {
    log.Fatal(err)
  }
}` },
];

const SNIPPETS_MEDIA_WORKFLOW = [
  { lang: "js", label: "Node.js", code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const mediaId = await client.media.uploadFile("./quarterly-update.mp4");

const post = await client.posts.create({
  caption: "Quarterly product update",
  accountIds: ["sa_youtube_1"],
  mediaIds: [mediaId],
  platformOptions: {
    youtube: {
      title: "Quarterly product update",
      made_for_kids: false,
      privacy_status: "public",
      category_id: "22",
    },
  },
});

console.log(post.id);` },
];

const RESPONSE_SUCCESS = `{
  "data": {
    "id": "post_abc123",
    "caption": "Hello from UniPost!",
    "status": "published",
    "scheduled_at": null,
    "created_at": "2026-04-08T10:00:00Z",
    "results": [
      {
        "social_account_id": "sa_instagram_123",
        "platform": "instagram",
        "status": "published",
        "external_id": "17841234567890",
        "published_at": "2026-04-08T10:00:01Z",
        "error_message": null
      },
      {
        "social_account_id": "sa_linkedin_456",
        "platform": "linkedin",
        "status": "published",
        "external_id": "urn:li:share:7049876543210",
        "published_at": "2026-04-08T10:00:02Z",
        "error_message": null
      }
    ]
  }
}`;

const RESPONSE_ERROR = `{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Caption exceeds maximum length for twitter (280 characters)",
    "issues": [
      {
        "field": "platform_posts[0].caption",
        "code": "caption_too_long",
        "message": "Caption is 312 characters, max is 280 for twitter"
      }
    ]
  }
}`;

const LIMITS: ParamRow[] = [
  { name: "Max account_ids per request", type: "", required: false, description: "20" },
  { name: "Max platform_posts per request", type: "", required: false, description: "20" },
  { name: "Rate limit", type: "", required: false, description: "100 requests/min per API key" },
  { name: "Idempotency key TTL", type: "", required: false, description: "24 hours" },
  { name: "Max media per post", type: "", required: false, description: "Varies by platform (1-10)" },
];

const PLATFORM_LIMITS = [
  { platform: "X / Twitter", max: "280" },
  { platform: "LinkedIn", max: "3,000" },
  { platform: "Instagram", max: "2,200" },
  { platform: "Threads", max: "500" },
  { platform: "TikTok", max: "2,200" },
  { platform: "YouTube", max: "5,000" },
  { platform: "Bluesky", max: "300 (graphemes)" },
];

const SCHEMA_ORG = {
  "@context": "https://schema.org",
  "@type": "TechArticle",
  name: "UniPost API — POST /v1/social-posts",
  description: "Publish content to multiple social platforms with one API call",
  url: "https://unipost.dev/docs/api/posts/create",
  author: { "@type": "Organization", name: "UniPost" },
  dateModified: "2026-04-09",
};

export function CreatePostContent() {
  return (
    <>
      <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify(SCHEMA_ORG) }} />

      <Breadcrumbs items={[
        { label: "Docs", href: "/docs" },
        { label: "API Reference", href: "/docs/api/posts/create" },
        { label: "Posts" },
        { label: "Create Post" },
      ]} />

      <EndpointHeader
        method="POST"
        path="/v1/social-posts"
        description="Publish content to connected social platforms. Supports immediate publish, scheduled posts, drafts, per-platform captions, media attachments, threads, and first comments."
        badges={["Requires Auth", "Idempotent", "Rate Limited"]}
      />

      {/* Overview */}
      <DocSection id="overview" title="Overview">
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          The social posts endpoint is the core of UniPost. It accepts a caption and a list of account IDs, and publishes the content to every connected platform in a single API call.
        </p>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          For multi-platform fan-out where each platform should get different copy, use <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>platform_posts[]</code> instead of <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>caption + account_ids</code>. Each entry becomes one platform post with its own caption, media, and options.
        </p>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7 }}>
          Posts that fail on some platforms but succeed on others return <code style={{ color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>status: &quot;partial&quot;</code> with per-platform results — the successful publishes are never rolled back.
        </p>
      </DocSection>

      {/* Authentication */}
      <DocSection id="authentication" title="Authentication">
        <div style={{ background: "var(--docs-tech-bg)", border: "1px solid var(--docs-tech-border)", borderRadius: 10, padding: "18px 22px" }}>
          <p style={{ fontSize: 13.5, color: "var(--docs-text-soft)", marginBottom: 10 }}>All requests require a Bearer token in the Authorization header.</p>
          <code style={{ fontSize: 14, fontFamily: "var(--docs-mono)", color: "var(--docs-tech-text)" }}>Authorization: Bearer up_live_xxxx</code>
          <p style={{ fontSize: 12.5, color: "var(--docs-text-faint)", marginTop: 10 }}>
            Get your API key at <a href="https://app.unipost.dev" style={{ color: "var(--docs-link)", textDecoration: "none" }}>app.unipost.dev</a>
          </p>
        </div>
      </DocSection>

      {/* Request */}
      <DocSection id="request" title="Request">
        <div style={{ fontSize: 13, fontFamily: "var(--docs-mono)", color: "var(--docs-text-muted)", marginBottom: 16 }}>
          Base URL: <span style={{ color: "var(--docs-text)" }}>https://api.unipost.dev</span>
        </div>
        <ParamTable params={HEADER_PARAMS} title="Headers" />
        <ParamTable params={BODY_PARAMS} title="Body Parameters" />
        <ParamTable params={PLATFORM_POST_PARAMS} title="platform_posts[] object" />
        <InfoBox>
          <strong style={{ color: "var(--docs-link)" }}>Two request shapes</strong> — pass exactly one: <code>caption + account_ids</code> (same caption everywhere) or <code>platform_posts[]</code> (different caption per platform). Mixing both is rejected with VALIDATION_ERROR.
        </InfoBox>
        <InfoBox>
          <strong style={{ color: "var(--docs-link)" }}>Local files vs hosted URLs</strong> — if your image or video already lives at a public URL, send <code>media_urls</code>. If you are starting from a local file on disk, first call <ApiInlineLink endpoint="POST /v1/media" />, upload the bytes to the returned <code>upload_url</code>, then publish with <code>media_ids</code>.
        </InfoBox>
        <InfoBox>
          <strong style={{ color: "var(--docs-link)" }}>Instagram publish surface</strong> — use <code>platform_options.instagram.mediaType</code> to choose <code>feed</code>, <code>reels</code>, or <code>story</code>. UniPost preflight now rejects invalid combinations such as a Reel with an image, or a Story with more than one media item.
        </InfoBox>
      </DocSection>

      <DocSection id="media-workflow" title="Media Upload Workflow">
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          The create-post endpoint does not accept raw multipart file bodies. When you want to publish a local image or video, use the media library first and then reference the returned media ID during publish.
        </p>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 12 }}>
          The end-to-end sequence is: reserve an upload with <ApiInlineLink endpoint="POST /v1/media" />, upload the file bytes directly to the returned <code>upload_url</code>, optionally confirm the media row with <ApiInlineLink endpoint="GET /v1/media/{id}" href="/docs/api/media" />, and finally call <ApiInlineLink endpoint="POST /v1/social-posts" /> with <code>media_ids</code>.
        </p>
        <p style={{ fontSize: 14.5, color: "var(--docs-text-soft)", lineHeight: 1.7, marginBottom: 16 }}>
          This is the recommended path for large videos, especially YouTube uploads. A placeholder like <code>med_uploaded_video_1</code> in the examples below means “the media ID returned by the media API after your upload was reserved.”
        </p>
        <CodeTabs snippets={SNIPPETS_MEDIA_WORKFLOW} />
      </DocSection>

      {/* Examples */}
      <DocSection id="examples" title="Examples">
        <p style={{ fontSize: 13, fontWeight: 600, color: "var(--docs-text-muted)", marginBottom: 10, fontFamily: "var(--docs-mono)" }}>Basic — same caption to multiple platforms</p>
        <CodeTabs snippets={SNIPPETS_BASIC} />
        <p style={{ fontSize: 13, fontWeight: 600, color: "var(--docs-text-muted)", marginBottom: 10, marginTop: 24, fontFamily: "var(--docs-mono)" }}>Advanced — per-platform captions + scheduling + idempotency</p>
        <CodeTabs snippets={SNIPPETS_ADVANCED} />
      </DocSection>

      {/* Response */}
      <DocSection id="response" title="Response">
        <ResponseBlock title="200 — Success" code={RESPONSE_SUCCESS} />
        <div style={{ marginBottom: 16 }}>
          <p style={{ fontSize: 13.5, color: "var(--docs-text-soft)", lineHeight: 1.6 }}>
            <strong style={{ color: "var(--docs-text)" }}>Status values:</strong>{" "}
            <code>published</code> (all platforms succeeded),{" "}
            <code>scheduled</code> (queued for future),{" "}
            <code>partial</code> (some succeeded, some failed),{" "}
            <code>failed</code> (all platforms failed),{" "}
            <code>draft</code> (saved, not published).
          </p>
        </div>
        <ResponseBlock title="422 — Validation Error" code={RESPONSE_ERROR} />
      </DocSection>

      {/* Errors */}
      <DocSection id="errors" title="Error Codes">
        <ErrorTable errors={ERRORS} />
        <p style={{ fontSize: 13, color: "var(--docs-text-faint)", marginTop: 12, lineHeight: 1.6 }}>
          Platform-specific errors (e.g. Twitter rate limit, Instagram media rejection) are returned in <code>results[].error_message</code> per-platform, not as top-level errors.
        </p>
      </DocSection>

      {/* Limits */}
      <DocSection id="limits" title="Limits & Constraints">
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", marginBottom: 20, background: "var(--docs-bg-elevated)" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
            <tbody>
              {LIMITS.map((l, i) => (
                <tr key={l.name} style={{ borderBottom: i < LIMITS.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
                  <td style={{ padding: "10px 14px", color: "var(--docs-text)", fontWeight: 500 }}>{l.name}</td>
                  <td style={{ padding: "10px 14px", color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>{l.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p style={{ fontSize: 13, fontWeight: 600, color: "var(--docs-text-muted)", marginBottom: 10, fontFamily: "var(--docs-mono)" }}>Platform caption limits</p>
        <div style={{ border: "1px solid var(--docs-border)", borderRadius: 10, overflow: "hidden", background: "var(--docs-bg-elevated)" }}>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 13.5 }}>
            <thead>
              <tr style={{ background: "var(--docs-bg-muted)" }}>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Platform</th>
                <th style={{ textAlign: "left", padding: "10px 14px", color: "var(--docs-text-faint)", fontWeight: 600, fontSize: 11, textTransform: "uppercase", letterSpacing: ".06em", borderBottom: "1px solid var(--docs-border)" }}>Max Characters</th>
              </tr>
            </thead>
            <tbody>
              {PLATFORM_LIMITS.map((p, i) => (
                <tr key={p.platform} style={{ borderBottom: i < PLATFORM_LIMITS.length - 1 ? "1px solid var(--docs-border)" : undefined }}>
                  <td style={{ padding: "10px 14px", color: "var(--docs-text)" }}>{p.platform}</td>
                  <td style={{ padding: "10px 14px", color: "var(--docs-accent)", fontFamily: "var(--docs-mono)", fontSize: 13 }}>{p.max}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </DocSection>

      {/* Related */}
      <DocSection id="related" title="Related Endpoints">
        <RelatedEndpoints items={[
          { method: "POST", path: "/v1/media", label: "Reserve media upload", href: "/docs/api/media" },
          { method: "GET", path: "/v1/media/:id", label: "Get media status", href: "/docs/api/media" },
          { method: "GET", path: "/v1/social-posts", label: "List posts", href: "/docs/api" },
          { method: "GET", path: "/v1/social-posts/:id", label: "Get post details", href: "/docs/api" },
          { method: "GET", path: "/v1/social-posts/:id/analytics", label: "Post analytics", href: "/docs/api/analytics" },
          { method: "POST", path: "/v1/social-posts/bulk", label: "Bulk publish (up to 50)", href: "/docs/api/posts/create" },
          { method: "GET", path: "/v1/social-accounts", label: "List connected accounts", href: "/docs/api/accounts/list" },
          { method: "GET", path: "/v1/platforms/capabilities", label: "Platform capabilities", href: "/docs/platforms" },
        ]} />
      </DocSection>

      {/* Changelog */}
      <DocSection id="changelog" title="Changelog">
        <ChangelogEntry version="v1.4" date="April 2026" items={[
          "Added first_comment field on platform_posts[]",
          "Added media_ids (replaces media_urls for managed uploads)",
          "Added per-account monthly quota enforcement",
        ]} />
        <ChangelogEntry version="v1.3" date="March 2026" items={[
          "Added thread_position for Twitter + Bluesky threads",
          "Added bulk endpoint (POST /v1/social-posts/bulk)",
        ]} />
        <ChangelogEntry version="v1.2" date="February 2026" items={[
          "Added platform_posts[] for per-platform overrides",
          "Added scheduled_at for deferred publishing",
          "Added idempotency_key support",
        ]} />
        <ChangelogEntry version="v1.0" date="January 2026" items={[
          "Initial release — caption + account_ids shape",
        ]} />
      </DocSection>

      {/* Back to full docs */}
      <div style={{ marginTop: 48, paddingTop: 24, borderTop: "1px solid var(--docs-border)", fontSize: 13, color: "var(--docs-text-faint)" }}>
        <a href="/docs" style={{ color: "var(--docs-link)", textDecoration: "none" }}>&larr; View full docs</a>
        <span style={{ margin: "0 12px" }}>|</span>
        <span>Last updated: April 2026 &middot; API v1</span>
      </div>
    </>
  );
}
