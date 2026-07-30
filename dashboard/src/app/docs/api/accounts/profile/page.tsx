import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";
import { X_CREDITS_CATALOG_VERSION } from "@/data/x-credits-catalog.generated";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Required header", description: "Unique key for this logical read. Reuse it only when retrying the exact same request." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected X account ID." },
];

const QUERY_FIELDS: ApiFieldItem[] = [
  { name: "external_user_id", type: "string", meta: "Required", description: "Managed User that owns the connected account. It must exactly match the account record." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "data.account_id", type: "string", description: "UniPost connected-account ID." },
  { name: "data.platform", type: "string", description: 'Always "twitter".' },
  { name: "data.external_account_id", type: "string", description: "X user ID read from the authorized account record." },
  { name: "data.username", type: "string", description: "Current X username." },
  { name: "data.display_name", type: "string", description: "Current X display name." },
  { name: "data.description", type: "string", description: "Current profile biography." },
  { name: "data.profile_image_url", type: "string", description: "Current profile image URL." },
  { name: "data.location", type: "string", description: "Optional public location." },
  { name: "data.website_url", type: "string", description: "Optional public website URL." },
  { name: "data.account_created_at", type: "string", description: "RFC 3339 account creation time." },
  { name: "data.verified", type: "boolean", description: "Whether X currently reports the account as verified." },
  { name: "data.public_metrics", type: "object", description: "Followers, following, posts, and listed counts." },
  { name: "data.retrieved_at", type: "string", description: "RFC 3339 time when UniPost completed the upstream read." },
  { name: "meta.credits", type: "object", description: "Estimate, reservation, charge, release, operation, catalog version, and accounting mode for this read." },
  { name: "meta.credits.accounting_enabled", type: "boolean", description: "True only for UniPost-managed X accounts while x_credits_billing_v1 is enabled." },
  { name: "meta.replayed", type: "boolean", description: "True when this is the stored result of an idempotent retry." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Stable machine-readable error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "error.details", type: "object", description: "Retry or Credits details when available." },
  { name: "request_id", type: "string", description: "Request identifier for support." },
];

const SUCCESS = `{
  "data": {
    "account_id": "sa_x_123",
    "platform": "twitter",
    "external_account_id": "2244994945",
    "username": "unipost",
    "display_name": "UniPost",
    "description": "Social publishing infrastructure",
    "profile_image_url": "https://pbs.twimg.com/profile_images/example_normal.jpg",
    "account_created_at": "2022-01-12T09:30:00Z",
    "verified": false,
    "public_metrics": { "followers": 1200, "following": 180, "posts": 640, "listed": 12 },
    "retrieved_at": "2026-07-29T18:42:10Z"
  },
  "meta": {
    "credits": {
      "operation_id": "xread_01J...",
      "status": "succeeded",
      "accounting_enabled": true,
      "billing_mode": "unipost_managed_app",
      "operation": "user.read",
      "estimated": 10,
      "reserved": 10,
      "charged": 10,
      "released": 0,
      "catalog_version": "${X_CREDITS_CATALOG_VERSION}"
    }
  },
  "request_id": "req_123"
}`;

export default function XAccountProfilePage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "Accounts", href: "/docs/api/accounts/list" }, { label: "X profile" }]}
      section="accounts"
      title="Get X account profile"
      description="Reads the live profile for one connected X account using users.read and offline.access. The account is confined to the supplied Managed User; arbitrary X user lookup is not supported. The x_credits_billing_v1 flag controls customer accounting, not endpoint availability."
      method="GET"
      path="/v1/accounts/:account_id/profile"
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
        code: `curl "https://api.unipost.dev/v1/accounts/sa_x_123/profile?external_user_id=user_42" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: profile-user-42-20260729"`,
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
    "details": { "estimated_credits": 10, "available_credits": 5, "max_affordable_limit": 0 }
  },
  "request_id": "req_123"
}`,
        },
      ]}
    />
  );
}
