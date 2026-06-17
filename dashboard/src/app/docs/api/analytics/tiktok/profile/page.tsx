"use client";

import type { ApiFieldItem } from "../../../_components/doc-components";
import { SingleEndpointReferencePage } from "../../../_components/single-endpoint-page";

const AUTH_FIELDS: ApiFieldItem[] = [
  { name: "Authorization", type: "Bearer <token>", meta: "In header", description: "Workspace API key." },
];

const PATH_FIELDS: ApiFieldItem[] = [
  { name: "account_id", type: "string", description: "Connected TikTok account ID such as sa_tiktok_123." },
];

const RESPONSE_FIELDS: ApiFieldItem[] = [
  { name: "social_account_id", type: "string", description: "UniPost account ID." },
  { name: "platform", type: "string", description: 'Always "tiktok".' },
  { name: "open_id", type: "string", description: "TikTok OpenID for the connected user." },
  { name: "display_name", type: "string", description: "TikTok display name from user.info.profile." },
  { name: "avatar_url", type: "string", description: "Avatar image URL from TikTok." },
  { name: "username", type: "string", description: "TikTok username without the @ prefix." },
  { name: "profile_web_link", type: "string", description: "Canonical web profile URL when TikTok returns it." },
  { name: "profile_deep_link", type: "string", description: "TikTok app deep link when TikTok returns it." },
  { name: "bio_description", type: "string", description: "Profile bio text." },
  { name: "is_verified", type: "boolean", description: "Whether TikTok reports the account as verified." },
  { name: "fetched_at", type: "string", description: "UTC timestamp when UniPost fetched the profile." },
];

const ERROR_FIELDS: ApiFieldItem[] = [
  { name: "error.code", type: "string", description: 'UNAUTHORIZED, FEATURE_DISABLED, NOT_FOUND, WRONG_PLATFORM, NEEDS_RECONNECT, or TIKTOK_ERROR.' },
  { name: "error.normalized_code", type: "string", description: "Lowercase error code." },
  { name: "error.message", type: "string", description: "Human-readable error message." },
  { name: "request_id", type: "string", description: "Request identifier for debugging and support." },
];

const SNIPPETS = [
  {
    lang: "curl",
    label: "cURL",
    code: `curl "https://api.unipost.dev/v1/accounts/sa_tiktok_123/tiktok/profile" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY"`,
  },
  {
    lang: "js",
    label: "Node.js",
    code: `const res = await fetch("https://api.unipost.dev/v1/accounts/sa_tiktok_123/tiktok/profile", {
  headers: { Authorization: \`Bearer \${process.env.UNIPOST_API_KEY}\` },
});

const profile = await res.json();`,
  },
];

const RESPONSE_SNIPPETS = [
  {
    lang: "json",
    label: "200",
    code: `{
  "data": {
    "social_account_id": "sa_tiktok_123",
    "platform": "tiktok",
    "open_id": "open_abc",
    "display_name": "Studio Alex",
    "avatar_url": "https://p16-sign-va.tiktokcdn.com/avatar.jpg",
    "username": "studioalex",
    "profile_web_link": "https://www.tiktok.com/@studioalex",
    "profile_deep_link": "snssdk1233://user/profile/studioalex",
    "bio_description": "Launch updates and creator workflows.",
    "is_verified": false,
    "fetched_at": "2026-06-17T18:30:00Z"
  }
}`,
  },
  {
    lang: "json",
    label: "403",
    code: `{
  "error": {
    "code": "FEATURE_DISABLED",
    "normalized_code": "feature_disabled",
    "message": "TikTok analytics is not enabled in this environment."
  },
  "request_id": "req_123"
}`,
  },
];

export default function TikTokProfileAnalyticsPage() {
  return (
    <SingleEndpointReferencePage
      breadcrumbItems={[
        { label: "API Reference", href: "/docs/api" },
        { label: "Analytics", href: "/docs/api/analytics/summary" },
        { label: "TikTok Analytics", href: "/docs/api/analytics/tiktok" },
        { label: "Profile" },
      ]}
      section="analytics"
      title="Get TikTok profile"
      description="Returns TikTok profile fields for one connected account. TikTok has approved user.info.profile for production use; enable tiktok.analytics_scopes in the target environment to serve this public-ready endpoint."
      method="GET"
      path="/v1/accounts/:account_id/tiktok/profile"
      requestSections={[
        { title: "Authorization", items: AUTH_FIELDS },
        { title: "Path Params", items: PATH_FIELDS },
      ]}
      responses={[
        { code: "200", fields: RESPONSE_FIELDS },
        { code: "401", fields: ERROR_FIELDS },
        { code: "403", fields: ERROR_FIELDS },
        { code: "404", fields: ERROR_FIELDS },
        { code: "409", fields: ERROR_FIELDS },
        { code: "502", fields: ERROR_FIELDS },
      ]}
      snippets={SNIPPETS}
      responseSnippets={RESPONSE_SNIPPETS}
    />
  );
}
