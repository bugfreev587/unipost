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

const PATH_FIELDS: ApiFieldItem[] = [
  {
    name: "account_id",
    type: "string",
    description: "Connected social account ID such as sa_twitter_1.",
  },
];

const RESPONSE_200_FIELDS: ApiFieldItem[] = [
  {
    name: "social_account_id",
    type: "string",
    description: "UniPost account ID the metrics snapshot belongs to.",
  },
  {
    name: "platform",
    type: "string",
    description: "Normalized platform name.",
  },
  {
    name: "follower_count",
    type: "number",
    description: "Followers reported by the platform at fetch time.",
  },
  {
    name: "following_count",
    type: "number",
    description: "Number of accounts this account is following.",
  },
  {
    name: "post_count",
    type: "number",
    description: "Lifetime post / tweet count exposed by the platform.",
  },
  {
    name: "platform_specific",
    type: "object",
    description:
      "Untransformed platform-native fields. On X: tweet_count, listed_count. On TikTok, likes_count is included when analytics scopes are granted. On YouTube, view_count, hidden_subscriber_count, following_count_supported, subscriber_count_rounded, and post_count_public_only describe channel-statistics semantics. Some platforms can return upstream_status and upstream_error for recoverable upstream failures.",
  },
  {
    name: "fetched_at",
    type: "string",
    description: "Timestamp at which the response was fetched. Not cached — every call hits the upstream platform API.",
  },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  {
    name: "error.code",
    type: "string",
    description:
      'Possible values: "UNAUTHORIZED", "NOT_FOUND", "ACCOUNT_DISCONNECTED" (409), "NEEDS_RECONNECT" (409 — provider credentials, scopes, or channel identity must be refreshed), "NOT_SUPPORTED" (501 — platform has no metrics endpoint), "UPSTREAM_ERROR" (502 — platform fetch failed), "INTERNAL_ERROR".',
  },
  {
    name: "error.normalized_code",
    type: "string",
    description: 'Lowercase alias such as "unauthorized", "not_found", "account_disconnected", "not_supported", "upstream_error", or "internal_error".',
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
    code: `curl "https://api.unipost.dev/v1/accounts/sa_twitter_1/metrics" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost();

const metrics = await client.accounts.metrics("sa_twitter_1");`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()

metrics = client.accounts.metrics("sa_twitter_1")
print(metrics["data"])`,
  },
  {
    lang: "java",
    label: "Java",
    code: `import dev.unipost.UniPost;

UniPost client = new UniPost();

var metrics = client.accounts().metrics("sa_twitter_1");
System.out.println(metrics.get("follower_count").asInt());`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "social_account_id": "sa_twitter_1",
    "platform": "twitter",
    "follower_count": 12480,
    "following_count": 327,
    "post_count": 2841,
    "platform_specific": {
      "tweet_count": 2841,
      "listed_count": 41
    },
    "fetched_at": "2026-04-28T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "200 (YouTube)",
    code: `{
  "data": {
    "social_account_id": "sa_youtube_1",
    "platform": "youtube",
    "follower_count": 123000,
    "following_count": 0,
    "post_count": 42,
    "platform_specific": {
      "view_count": 9876543,
      "hidden_subscriber_count": false,
      "following_count_supported": false,
      "subscriber_count_rounded": true,
      "post_count_public_only": true
    },
    "fetched_at": "2026-07-03T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "200 (upstream rate-limited)",
    code: `{
  "data": {
    "social_account_id": "sa_twitter_1",
    "platform": "twitter",
    "follower_count": 0,
    "following_count": 0,
    "post_count": 0,
    "platform_specific": {
      "upstream_status": 429,
      "upstream_error": "{\\"title\\":\\"Too Many Requests\\"}"
    },
    "fetched_at": "2026-04-28T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "409",
    code: `{
  "error": {
    "code": "NEEDS_RECONNECT",
    "normalized_code": "needs_reconnect",
    "message": "Reconnect YouTube to enable analytics."
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "501",
    code: `{
  "error": {
    "code": "NOT_SUPPORTED",
    "normalized_code": "not_supported",
    "message": "Account metrics are not available for linkedin yet"
  },
  "request_id": "req_123"
}`,
  },
  {
    lang: "json",
    label: "502",
    code: `{
  "error": {
    "code": "UPSTREAM_ERROR",
    "normalized_code": "upstream_error",
    "message": "Failed to fetch account metrics from twitter"
  },
  "request_id": "req_123"
}`,
  },
];

export default function AccountMetricsPage() {
  return (
    <SingleEndpointReferencePage
      section="accounts"
      title="Get account metrics"
      description="Returns follower / following / post counts for one connected social account, fetched live from the platform's API. Currently supported on X, Instagram, Threads, TikTok, and YouTube when the relevant platform scopes are granted. Other platforms return 501 NOT_SUPPORTED. Not cached — every call hits the upstream API, so prefer caching client-side if your dashboard polls frequently."
      method="GET"
      path="/v1/accounts/:account_id/metrics"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_200_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "501", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
