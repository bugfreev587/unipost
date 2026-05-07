"use client";

import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "Social post ID such as post_abc123." },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  { name: "post_id", type: "string", description: "Requested social post ID." },
  { name: "metrics", type: "object", description: "Normalized engagement and reach metrics." },
  { name: "metrics.likes", type: "number", description: "Like count for the post." },
  { name: "metrics.comments", type: "number", description: "Comment count (replies on X / Threads)." },
  { name: "metrics.shares", type: "number", description: "Share count. On X this is retweets + quote tweets combined; the breakdown is in platform_specific." },
  { name: "metrics.saves", type: "number", description: "Save / bookmark count when the platform exposes it." },
  { name: "metrics.impressions", type: "number", description: "Impression count when the platform exposes it." },
  { name: "metrics.reach", type: "number", description: "Reach value when the platform exposes it." },
  { name: "metrics.video_views", type: "number", description: "Video view count for video posts." },
  { name: "metrics.engagement_rate", type: "number", description: "Computed engagement rate (0-1) when impressions are available." },
  {
    name: "metrics.platform_specific",
    type: "object",
    description: "Untransformed platform-native fields the normalized shape doesn't capture. On X: retweet_count, quote_count, bookmark_count.",
  },
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
    code: `curl "https://api.unipost.dev/v1/posts/post_abc123/analytics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const postAnalytics = await client.posts.analytics("post_abc123");`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

analytics = client.posts.analytics("post_abc123")
print(analytics["data"])`,
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

  metrics, err := client.Posts.Analytics(context.Background(), "post_abc123", false)
  if err != nil {
    log.Fatal(err)
  }

  _ = metrics
}`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var analytics = client.posts().analytics("post_abc123");
System.out.println(analytics.get("metrics").get("likes").asInt());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "post_id": "post_abc123",
    "metrics": {
      "likes": 214,
      "comments": 19,
      "shares": 47,
      "saves": 12,
      "impressions": 18420,
      "reach": 4210,
      "video_views": 0,
      "engagement_rate": 0.0152,
      "platform_specific": {
        "retweet_count": 31,
        "quote_count": 16,
        "bookmark_count": 12
      }
    }
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
    "message": "Post not found."
  },
  "request_id": "req_123"
}`,
  },
];

export default function PostAnalyticsPage() {
  return (
    <SingleEndpointReferencePage
      section="analytics"
      title="Post analytics"
      description="Returns normalized analytics metrics for one social post. Use it when your UI needs a detailed post-level performance view."
      method="GET"
      path="/v1/posts/:post_id/analytics"
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
    />
  );
}
