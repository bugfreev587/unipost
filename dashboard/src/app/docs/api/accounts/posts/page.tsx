import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import { X_CREDITS_CATALOG_VERSION } from "@/data/x-credits-catalog.generated";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Required header", description: "Unique key for this exact page request. Reuse it only when retrying identical parameters." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected X account ID." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "external_user_id", type: "string", meta: "Required", description: "Managed User that owns the connected account." },
  { name: "limit", type: "integer", meta: "Required", description: "Upstream resources to scan. Must be between 5 and 100; UniPost never silently reduces it." },
  { name: "cursor", type: "string", description: "Opaque cursor returned by the preceding request. Cursors are scoped to the account, Managed User, time range, and filters." },
  { name: "start_time", type: "string", description: "Optional inclusive RFC 3339 lower bound." },
  { name: "end_time", type: "string", description: "Optional exclusive RFC 3339 upper bound. Must be after start_time." },
  { name: "exclude_reposts", type: "boolean", description: "Exclude reposts after UniPost scans the upstream page." },
  { name: "exclude_replies_to_others", type: "boolean", description: "Exclude replies to other users while retaining self-replies." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data[]", type: "array", description: "Normalized posts authored by the connected X account, deduplicated by external_post_id." },
  { name: "data[].account_id", type: "string", description: "UniPost connected-account ID." },
  { name: "data[].external_post_id", type: "string", description: "Stable X post ID and the deduplication key." },
  { name: "data[].text", type: "string", description: "Post text returned by X." },
  { name: "data[].created_at", type: "string", description: "RFC 3339 publication time." },
  { name: "data[].content_type", type: "string", description: "original_post, reply, quote_post, or repost." },
  { name: "data[].is_reply", type: "boolean", description: "Whether the post is a reply." },
  { name: "data[].is_self_reply", type: "boolean", description: "Whether the reply targets the same connected author." },
  { name: "data[].is_repost", type: "boolean", description: "Whether the post is a repost." },
  { name: "data[].is_quote", type: "boolean", description: "Whether the post quotes another post." },
  { name: "data[].thread.thread_id", type: "string", description: "X conversation ID used as the thread_id. Thread position is not exposed because X does not return a reliable authored-timeline position." },
  { name: "data[].media", type: "array", description: "Normalized media types: image, video, or gif." },
  { name: "data[].public_metrics", type: "object", description: "Likes, replies, reposts, quotes, and impressions." },
  { name: "meta.limit", type: "integer", description: "Requested upstream scan limit." },
  { name: "meta.scanned_count", type: "integer", description: "Resources inspected upstream and used for final Credits charging." },
  { name: "meta.returned_count", type: "integer", description: "Posts remaining after validation, deduplication, and filters." },
  { name: "meta.has_more", type: "boolean", description: "Whether another upstream page is available." },
  { name: "meta.next_cursor", type: "string", description: "Opaque next-page cursor when has_more is true." },
  { name: "meta.cursor_expires_at", type: "string", description: "RFC 3339 cursor expiry, normally seven days after issuance." },
  { name: "meta.credits", type: "object", description: "Estimate, reservation, charge, release, operation, catalog version, and accounting mode." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Stable machine-readable error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "error.details", type: "object", description: "May include estimated_credits, available_credits, and max_affordable_limit." },
  { name: "error.details.retry_cursor", type: "string", description: "Refreshed opaque cursor when a retriable delay would outlive the submitted cursor. Retry with a new Idempotency-Key." },
  { name: "error.details.retry_cursor_expires_at", type: "string", description: "RFC 3339 expiry for retry_cursor." },
  { name: "request_id", type: "string", description: "Request identifier for support." },
];

const SUCCESS = `{
  "data": [{
    "account_id": "sa_x_123",
    "external_post_id": "1819012345678901234",
    "text": "Shipping a simpler publishing workflow today.",
    "created_at": "2026-07-29T17:20:00Z",
    "language": "en",
    "conversation_id": "1819012345678901234",
    "content_type": "original_post",
    "is_reply": false,
    "is_self_reply": false,
    "is_repost": false,
    "is_quote": false,
    "public_metrics": { "likes": 18, "replies": 2, "reposts": 3, "quotes": 1, "impressions": 940 },
    "thread": { "thread_id": "1819012345678901234" }
  }],
  "meta": {
    "limit": 20,
    "scanned_count": 20,
    "returned_count": 1,
    "has_more": true,
    "next_cursor": "xc_opaque...",
    "cursor_expires_at": "2026-08-05T18:42:10Z",
    "credits": {
      "operation_id": "xread_01J...",
      "status": "succeeded",
      "accounting_enabled": true,
      "billing_mode": "unipost_managed_app",
      "operation": "post.read",
      "estimated": 100,
      "reserved": 100,
      "charged": 100,
      "released": 0,
      "catalog_version": "${X_CREDITS_CATALOG_VERSION}"
    }
  },
  "request_id": "req_123"
}`;

export default function XAccountPostsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "Accounts", href: "/docs/api/accounts/list" }, { label: "X authored posts" }]}
      section="accounts"
      title="List X authored posts"
      description="Reads a synchronous, cursor-paginated page of posts authored by one connected X account. Filtering happens after the upstream scan, so returned_count can be lower than scanned_count. The existing x_credits_billing_v1 flag controls customer accounting only."
      method="GET"
      path="/v1/accounts/:account_id/posts"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "402", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "429", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={[{
        lang: "curl",
        label: "cURL",
        code: `curl "https://api.unipost.dev/v1/accounts/sa_x_123/posts?external_user_id=user_42&limit=20&exclude_reposts=true" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: posts-user-42-page-1"`,
      }]}
      responseSnippets={[
        { lang: "json", label: "200", code: SUCCESS },
        {
          lang: "json",
          label: "402",
          code: `{
  "error": {
    "code": "INSUFFICIENT_X_CREDITS",
    "message": "The Workspace does not have enough X Credits for this request",
    "details": { "estimated_credits": 100, "available_credits": 65, "max_affordable_limit": 13 }
  },
  "request_id": "req_123"
}`,
        },
      ]}
    />
  );
}
