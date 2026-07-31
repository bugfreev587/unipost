"use client";

import { X_CREDITS_CATALOG_VERSION } from "@/data/x-credits-catalog.generated";
import { usePublicDocsFeatureFlags } from "@/lib/use-public-docs-feature-flags";
import type { ApiFieldItem } from "../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
  { name: "Idempotency-Key", type: "string", meta: "Required header", description: "Caller-owned key for this logical read. Reuse it only for an exact retry." },
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
  { name: "meta.credits", type: "object", description: "Accounting mode and, when enabled, the estimate, reservation, charge, and release for this read." },
  { name: "meta.credits.accounting_enabled", type: "boolean", description: "True only for UniPost-managed X accounts while x_credits_billing_v1 is enabled." },
  { name: "meta.credits.bypass_reason", type: "string", description: "feature_disabled while billing is off, or customer_x_app for workspace-owned X credentials." },
  { name: "meta.replayed", type: "boolean", description: "Present and true when this is the stored result of an exact idempotent retry. Absence means false." },
  { name: "request_id", type: "string", description: "Request identifier for logs and support." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: "Stable uppercase machine-readable error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "error.details", type: "object", description: "Retry or Credits details when available." },
  { name: "error.is_retriable", type: "boolean", description: "Whether the same logical read may be retried." },
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

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_x_123/profile?external_user_id=user_42" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Idempotency-Key: profile-user-42-v1"`,
  },
  {
    lang: "js",
    label: "JavaScript",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({ apiKey: process.env.UNIPOST_API_KEY });
const result = await client.accounts.getProfile("sa_x_123", {
  externalUserId: "user_42",
  idempotencyKey: "profile-user-42-v1",
});

console.log(result.request_id, result.meta.credits, result.data.username);`,
  },
  {
    lang: "python",
    label: "Python",
    code: `from unipost import UniPost

client = UniPost()
result = client.accounts.get_profile(
    "sa_x_123",
    external_user_id="user_42",
    idempotency_key="profile-user-42-v1",
)

print(result.request_id, result.meta.credits, result.data.username)`,
  },
  {
    lang: "go",
    label: "Go",
    code: `result, err := client.Accounts.Profile(ctx, "sa_x_123", &unipost.XAccountProfileParams{
  ExternalUserID: "user_42",
  IdempotencyKey: "profile-user-42-v1",
})
if err != nil {
  log.Fatal(err)
}

fmt.Println(result.RequestID, result.Meta.Credits, result.Data.Username)`,
  },
  {
    lang: "java",
    label: "Java",
    code: `var result = client.accounts().profile("sa_x_123", Map.of(
    "external_user_id", "user_42",
    "idempotency_key", "profile-user-42-v1"
));

System.out.println(result.path("request_id").asText());
System.out.println(result.path("meta").path("credits"));
System.out.println(result.path("data").path("username").asText());`,
  },
];

const INSUFFICIENT_CREDITS = `{
  "error": {
    "code": "INSUFFICIENT_X_CREDITS",
    "message": "The Workspace does not have enough X Credits for this request",
    "details": {
      "estimated_credits": 10,
      "available_credits": 5,
      "max_affordable_limit": 0
    },
    "is_retriable": false
  },
  "request_id": "req_123"
}`;

export default function XAccountProfilePage() {
  const publicFeatureFlags = usePublicDocsFeatureFlags();
  const xCreditsEnabled = publicFeatureFlags.x_credits_billing_v1;

  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[{ label: "API Reference", href: "/docs/api" }, { label: "Accounts", href: "/docs/api/accounts/list" }, { label: "X profile" }]}
      section="accounts"
      title="Get X account profile"
      description="Read the live profile for the connected X account owned by one Managed User. The connection must preserve its X app identity and include users.read plus offline.access. The x_credits_billing_v1 flag controls customer accounting, not endpoint availability."
      method="GET"
      path="/v1/accounts/:account_id/profile"
      guideLinks={[{ label: "Read X profiles and post history", href: "/docs/guides/x/profile-and-post-history" }]}
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
        { title: "Query Params", items: QUERY_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "400", fields: ERROR_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        ...(xCreditsEnabled ? [{ code: "402", fields: ERROR_FIELDS }] : []),
        { code: "403", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "422", fields: ERROR_FIELDS },
        { code: "429", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={[
        { lang: "json", label: "200", code: SUCCESS },
        ...(xCreditsEnabled ? [{ lang: "json", label: "402", code: INSUFFICIENT_CREDITS }] : []),
      ]}
    >
      <section>
        <h2>Stable errors and exact retries</h2>
        <p>
          Handle <code>VALIDATION_ERROR</code>, <code>IDEMPOTENCY_KEY_REQUIRED</code>,{" "}
          <code>ACCOUNT_NOT_FOUND</code>, <code>WRONG_PLATFORM</code>, <code>ACCOUNT_ACCESS_DENIED</code>,{" "}
          <code>ACCOUNT_REAUTHORIZATION_REQUIRED</code>, <code>IDEMPOTENCY_CONFLICT</code>,{" "}
          <code>READ_IN_PROGRESS</code>, <code>READ_SETTLEMENT_PENDING</code>, <code>RATE_LIMITED</code>, and{" "}
          <code>X_UPSTREAM_ERROR</code> by their stable <code>error.code</code>. For a retriable response, inspect{" "}
          <code> is_retriable</code> and the HTTP <code>Retry-After</code> header, then repeat the exact request with
          the same Idempotency-Key. Never generate a replacement key for an exact retry.
        </p>
        {xCreditsEnabled ? (
          <p>
            <code>INSUFFICIENT_X_CREDITS</code> is rejected before provider delivery. Inspect the estimate and available
            allowance in <code>error.details</code>; do not retry unchanged.
          </p>
        ) : (
          <p>
            Customer Credits accounting is currently disabled. The response still reports{" "}
            <code>accounting_enabled=false</code> and <code>bypass_reason=feature_disabled</code>. A workspace-owned X
            app instead reports <code>bypass_reason=customer_x_app</code>.
          </p>
        )}
      </section>
    </SingleEndpointReferencePage>
  );
}
